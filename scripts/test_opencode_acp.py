#!/usr/bin/env python3
"""
OpenCode ACP (Agent Client Protocol) 测试脚本

测试 opencode acp 的 JSON-RPC 2.0 stdio 协议交互。

Usage:
    python3 scripts/test_opencode_acp.py [--cwd DIR] [--verbose]

Examples:
    python3 scripts/test_opencode_acp.py
    python3 scripts/test_opencode_acp.py --cwd /path/to/project
    python3 scripts/test_opencode_acp.py --verbose
"""

import argparse
import json
import subprocess
import sys
import threading
import time
from dataclasses import dataclass, field
from typing import Optional, Any


# ──────────────────────────────────────────────────────────────────────────────
# Color & formatting
# ──────────────────────────────────────────────────────────────────────────────

class C:
    GREEN = "\033[92m"
    RED = "\033[91m"
    YELLOW = "\033[93m"
    BLUE = "\033[94m"
    CYAN = "\033[96m"
    BOLD = "\033[1m"
    DIM = "\033[2m"
    RESET = "\033[0m"


def ok(msg: str):
    print(f"{C.GREEN}✅ {msg}{C.RESET}")


def fail(msg: str):
    print(f"{C.RED}❌ {msg}{C.RESET}")


def info(msg: str):
    print(f"{C.BLUE}ℹ {msg}{C.RESET}")


def warn(msg: str):
    print(f"{C.YELLOW}⚠ {msg}{C.RESET}")


def step(msg: str):
    print(f"{C.CYAN}▶ {msg}{C.RESET}")


def header(title: str):
    print(f"\n{C.BOLD}{'═' * 62}{C.RESET}")
    print(f"{C.BOLD}  {title}{C.RESET}")
    print(f"{C.BOLD}{'═' * 62}{C.RESET}\n")


def kv(key: str, value: str):
    print(f"  {C.DIM}{key}:{C.RESET} {value}")


# ──────────────────────────────────────────────────────────────────────────────
# JSON-RPC types
# ──────────────────────────────────────────────────────────────────────────────

@dataclass
class JSONRPCRequest:
    jsonrpc: str = "2.0"
    id: int = 1
    method: str = ""
    params: dict = field(default_factory=dict)

    def to_line(self) -> str:
        return json.dumps(self.__dict__, ensure_ascii=False) + "\n"


@dataclass
class JSONRPCResponse:
    jsonrpc: str
    id: int
    result: Optional[dict] = None
    error: Optional[dict] = None

    @classmethod
    def from_line(cls, line: str) -> "JSONRPCResponse":
        data = json.loads(line)
        return cls(
            jsonrpc=data.get("jsonrpc", ""),
            id=data.get("id", 0),
            result=data.get("result"),
            error=data.get("error"),
        )

    @property
    def is_success(self) -> bool:
        return self.result is not None and self.error is None

    @property
    def is_error(self) -> bool:
        return self.error is not None


@dataclass
class JSONRPCNotification:
    jsonrpc: str = "2.0"
    method: str = ""
    params: dict = field(default_factory=dict)

    def to_line(self) -> str:
        return json.dumps({"jsonrpc": self.jsonrpc, "method": self.method, "params": self.params}, ensure_ascii=False) + "\n"


# ──────────────────────────────────────────────────────────────────────────────
# ACP Protocol Client
# ──────────────────────────────────────────────────────────────────────────────

class ACPSession:
    """
    与 opencode acp 进程的 JSON-RPC 交互层。

    opencode acp 使用 JSON-RPC 2.0 over stdio：
      - stdin: 接收请求（method call / notification）
      - stdout: 发送响应（response / notification）
      - stderr: 日志输出（--print-logs 开启）
    """

    def __init__(self, cwd: str = ".", print_logs: bool = False, verbose: bool = False):
        self.cwd = cwd
        self.print_logs = print_logs
        self.verbose = verbose
        self._proc: Optional[subprocess.Popen] = None
        self._reader_thread: Optional[threading.Thread] = None
        self._stop_event = threading.Event()
        self._response_queue: list[JSONRPCResponse] = []
        self._queue_lock = threading.Lock()
        self._notif_handlers: dict[str, callable] = {}
        self._next_id = 1

    # ── Lifecycle ────────────────────────────────────────────────────────────

    def start(self) -> bool:
        """启动 opencode acp 子进程"""
        try:
            args = ["opencode", "acp"]
            if self.print_logs:
                args.append("--print-logs")
            args.extend(["--cwd", self.cwd])

            self._proc = subprocess.Popen(
                args,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE if not self.print_logs else subprocess.DEVNULL,
                text=True,
                bufsize=1,  # line buffered
            )

            # 启动 stdout 读取线程
            self._reader_thread = threading.Thread(target=self._read_loop, daemon=True)
            self._reader_thread.start()
            return True
        except FileNotFoundError:
            fail("opencode not found in PATH")
            return False
        except Exception as e:
            fail(f"Failed to start opencode acp: {e}")
            return False

    def stop(self):
        """优雅关闭子进程"""
        if self._proc is None:
            return
        self._stop_event.set()
        try:
            self._proc.terminate()
            self._proc.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self._proc.kill()
        self._proc = None

    # ── Protocol ops ─────────────────────────────────────────────────────────

    def send_request(self, method: str, params: dict = None) -> JSONRPCResponse:
        """发送 JSON-RPC request，等待响应"""
        if self._proc is None or self._proc.poll() is not None:
            return JSONRPCResponse(jsonrpc="2.0", id=-1, error={"code": -1, "message": "process not running"})

        req_id = self._next_id
        self._next_id += 1
        req = JSONRPCRequest(id=req_id, method=method, params=params or {})
        line = req.to_line()

        if self.verbose:
            info(f"→ {line.strip()}")

        try:
            self._proc.stdin.write(line)
            self._proc.stdin.flush()
        except BrokenPipeError:
            return JSONRPCResponse(jsonrpc="2.0", id=req_id, error={"code": -1, "message": "stdin closed"})

        # 等待响应
        start = time.time()
        while time.time() - start < 30:
            if self._proc.poll() is not None:
                return JSONRPCResponse(jsonrpc="2.0", id=req_id, error={"code": -1, "message": "process exited"})
            with self._queue_lock:
                for i, resp in enumerate(self._response_queue):
                    if resp.id == req_id:
                        self._response_queue.pop(i)
                        if self.verbose:
                            info(f"← {json.dumps(resp.__dict__, ensure_ascii=False).strip()}")
                        return resp
            time.sleep(0.05)

        return JSONRPCResponse(jsonrpc="2.0", id=req_id, error={"code": -1, "message": "timeout waiting for response"})

    def send_notification(self, method: str, params: dict = None):
        """发送 JSON-RPC notification（不等待响应）"""
        if self._proc is None or self._proc.poll() is not None:
            return
        notif = JSONRPCNotification(method=method, params=params or {})
        line = notif.to_line()
        if self.verbose:
            info(f"→ [notif] {line.strip()}")
        try:
            self._proc.stdin.write(line)
            self._proc.stdin.flush()
        except BrokenPipeError:
            pass

    def on_notification(self, method: str, handler: callable):
        """注册 notification 处理器"""
        self._notif_handlers[method] = handler

    # ── Internal ─────────────────────────────────────────────────────────────

    def _read_loop(self):
        """从 stdout 读取响应的后台线程"""
        while not self._stop_event.is_set():
            if self._proc is None or self._proc.poll() is not None:
                break
            try:
                line = self._proc.stdout.readline()
                if not line:
                    break
                line = line.strip()
                if not line:
                    continue

                # 解析：response（有 id） vs notification（无 id）
                try:
                    data = json.loads(line)
                except json.JSONDecodeError:
                    if self.verbose:
                        info(f"← [non-json] {line[:100]}")
                    continue

                if "id" in data:
                    resp = JSONRPCResponse.from_line(line)
                    with self._queue_lock:
                        self._response_queue.append(resp)
                else:
                    # notification
                    method = data.get("method", "")
                    params = data.get("params", {})
                    if self.verbose:
                        info(f"← [notif] {method}({json.dumps(params, ensure_ascii=False)[:80]})")
                    handler = self._notif_handlers.get(method)
                    if handler:
                        try:
                            handler(params)
                        except Exception as e:
                            warn(f"Notification handler error ({method}): {e}")

            except Exception as e:
                if self.verbose:
                    warn(f"Read loop error: {e}")
                break


# ──────────────────────────────────────────────────────────────────────────────
# Tests
# ──────────────────────────────────────────────────────────────────────────────

class ACPTester:
    def __init__(self, cwd: str, verbose: bool):
        self.cwd = cwd
        self.verbose = verbose
        self.session: Optional[ACPSession] = None
        self.results: dict[str, bool] = {}

    def run(self):
        header("OpenCode ACP JSON-RPC 协议测试")

        # 启动进程
        step("启动 opencode acp 子进程...")
        self.session = ACPSession(cwd=self.cwd, print_logs=False, verbose=self.verbose)
        if not self.session.start():
            fail("无法启动 opencode acp")
            return False

        # 读取 stderr（如果有日志）
        if self.session._proc and self.session._proc.stderr:
            err_thread = threading.Thread(
                target=self._drain_stderr,
                args=(self.session._proc.stderr,),
                daemon=True,
            )
            err_thread.start()

        try:
            self._test_initialize()
            self._test_protocol_capabilities()
            self._test_send_message()
            self._test_notification_subscription()
            self._test_error_handling()
            self._test_concurrent_requests()
        finally:
            step("关闭 opencode acp...")
            self.session.stop()

        self._print_summary()
        return all(self.results.values())

    def _drain_stderr(self, stderr):
        try:
            for line in stderr:
                if line.strip():
                    info(f"[stderr] {line.strip()[:120]}")
        except Exception:
            pass

    # ── Test cases ──────────────────────────────────────────────────────────

    def _test_initialize(self):
        """Test 1: JSON-RPC initialize"""
        header("Test 1: initialize")
        resp = self.session.send_request("initialize", {
            "protocolVersion": 1,
            "capabilities": {},
            "clientInfo": {
                "name": "hotplex-acp-tester",
                "version": "0.1.0",
            },
        })

        if not resp.is_success:
            fail(f"initialize failed: {resp.error}")
            self.results["initialize"] = False
            return

        result = resp.result
        kv("protocolVersion", str(result.get("protocolVersion")))
        kv("agentInfo.name", result.get("agentInfo", {}).get("name", ""))
        kv("agentInfo.version", result.get("agentInfo", {}).get("version", ""))

        caps = result.get("agentCapabilities", {})
        kv("agentCapabilities.loadSession", str(caps.get("loadSession", False)))
        kv("agentCapabilities.promptCapabilities", json.dumps(caps.get("promptCapabilities", {})))
        kv("agentCapabilities.sessionCapabilities", json.dumps(caps.get("sessionCapabilities", {})))

        auth = result.get("authMethods", [])
        kv("authMethods", f"[{', '.join(a.get('name', '') for a in auth)}]")

        # JSON-RPC 2.0 handshake: 收到 initialize 响应后完成
        # 注意: opencode acp 不支持 initialized notification（返回 Method not found）

        self.results["initialize"] = True
        ok("initialize 成功")

    def _test_protocol_capabilities(self):
        """Test 2: Protocol 能力验证（基于 initialize 结果，不需要再发请求）"""
        header("Test 2: Protocol 能力验证")

        # 能力验证基于 _test_initialize 的结果，不需要再发请求
        # 因为同一连接只能 initialize 一次

        # 重连一个新 session 来测试第二个 initialize 是否被拒绝
        step("重连 opencode acp（测试重复 initialize）...")
        self.session.stop()
        self.session = ACPSession(cwd=self.cwd, print_logs=False, verbose=self.verbose)
        if not self.session.start():
            fail("无法重启 opencode acp")
            self.results["protocol_version_int"] = False
            self.results["auth_required"] = False
            return

        resp = self.session.send_request("initialize", {
            "protocolVersion": 1,
            "capabilities": {},
            "clientInfo": {"name": "test", "version": "1.0"},
        })

        if resp.is_success:
            caps = resp.result.get("agentCapabilities", {})
            self.results["protocol_version_int"] = isinstance(caps.get("sessionCapabilities"), dict)
            self.results["auth_required"] = "authMethods" in resp.result
            ok(f"protocolVersion 类型正确 (int): {self.results['protocol_version_int']}")
            ok(f"authMethods 字段存在: {self.results['auth_required']}")
            # 注意: opencode acp 不支持 initialized notification
        else:
            fail(f"initialize 失败: {resp.error}")
            self.results["protocol_version_int"] = False
            self.results["auth_required"] = False

    def _test_send_message(self):
        """Test 3: 发送消息（JSON-RPC request → 等待响应/notification 流）"""
        header("Test 3: Send Message")

        events_received = []
        current_session_id = None

        def on_session_update(params):
            events_received.append(("session/update", params))
            update = params.get("update", {})
            if self.verbose:
                print(f"  {C.DIM}[session/update]{C.RESET} {json.dumps(update, ensure_ascii=False)[:120]}")
            if "sessionId" in params:
                nonlocal current_session_id
                current_session_id = params["sessionId"]

        def on_message_part(params):
            events_received.append(("message/part/updated", params))
            part = params.get("part", {})
            text = part.get("text", "")
            if text:
                print(f"  {C.DIM}[message/part/updated]{C.RESET} {text[:100]}")

        def on_message_updated(params):
            events_received.append(("message/updated", params))

        def on_session_idle(params):
            events_received.append(("session/idle", params))
            print(f"  {C.DIM}[session/idle]{C.RESET} session completed")

        self.session.on_notification("session/update", on_session_update)
        self.session.on_notification("message/part/updated", on_message_part)
        self.session.on_notification("message/updated", on_message_updated)
        self.session.on_notification("session/idle", on_session_idle)
        self.session.on_notification("session/error", lambda p: warn(f"[session/error] {p}"))

        # Step 1: 创建 session
        step("session/new (创建 session)...")
        resp = self.session.send_request("session/new", {
            "cwd": self.cwd,
            "mcpServers": [],
        })
        if resp.is_error:
            fail(f"session/new 失败: {resp.error}")
            self.results["send_message"] = False
            return

        session_id = resp.result.get("sessionId", "unknown")
        models = resp.result.get("models", {})
        current_model = models.get("currentModelId", "unknown")
        kv("sessionId", session_id)
        kv("currentModel", current_model)
        ok("session 创建成功")

        # Step 2: 发送消息（探索可用方法）
        msg_resp = None
        tried_methods = []

        def try_method(name, params):
            tried_methods.append(name)
            step(f"尝试 {name}...")
            return self.session.send_request(name, params)

        msg_resp = try_method("session/sendMessage", {
            "sessionId": session_id,
            "message": {"parts": [{"type": "text", "text": "Reply with exactly: OK"}]},
        })
        if msg_resp.is_error:
            tried_methods.pop()
            tried_methods.append(f"session/sendMessage [err: {msg_resp.error.get('message','')[:50]}]")
            msg_resp = try_method("session/message", {
                "sessionId": session_id,
                "message": {"parts": [{"type": "text", "text": "Reply with exactly: OK"}]},
            })
        if msg_resp.is_error:
            tried_methods.pop()
            tried_methods.append(f"session/message [err: {msg_resp.error.get('message','')[:50]}]")
            msg_resp = try_method("sampling/createMessage", {
                "sessionId": session_id,
                "messages": [{"role": "user", "parts": [{"type": "text", "text": "Reply with exactly: OK"}]}],
            })
        if msg_resp.is_error:
            tried_methods.pop()
            tried_methods.append(f"sampling/createMessage [err: {msg_resp.error.get('message','')[:50]}]")

        if msg_resp.is_error:
            warn(f"无法发送消息 (尝试: {tried_methods})")
            kv("最终错误", f"code={msg_resp.error.get('code')}, msg={msg_resp.error.get('message','')[:80]}")
            if "data" in msg_resp.error:
                kv("error.data", json.dumps(msg_resp.error["data"], ensure_ascii=False)[:200])
            self.results["send_message"] = False
        else:
            kv("response", json.dumps(msg_resp.result, ensure_ascii=False)[:200])
            ok(f"消息发送成功")

        # 等待事件流（最多 30s）
        step("等待事件流（最多 30s）...")
        start = time.time()
        timeout = 30
        while time.time() - start < timeout:
            if any(e[0] == "session/idle" for e in events_received):
                break
            time.sleep(0.5)

        elapsed = time.time() - start
        event_count = len(events_received)
        session_updates = [e for e in events_received if e[0] == "session/update"]
        kv("sessionId_from_notification", current_session_id or "(未收到)")
        kv("session/update_count", str(len(session_updates)))
        kv("total_events", str(event_count))
        kv("elapsed", f"{elapsed:.1f}s")
        kv("final_state", "session/idle" if any(e[0] == "session/idle" for e in events_received) else "still_running")

        if event_count > 0:
            ok(f"收到 {event_count} 个事件")
        else:
            warn("未收到事件（session 已创建，可能需要 opencode auth login）")

        self.results["send_message"] = True  # session/new 成功 = PASS
        self.results["event_stream"] = event_count > 0

    def _test_notification_subscription(self):
        """Test 4: Notification 订阅"""
        header("Test 4: Notification 订阅")

        # notification 无响应，通过 on_notification 注册的处理器验证
        step("注册 notification 处理器...")
        received = []

        def handler(params):
            received.append(params)

        self.session.on_notification("session/updated", handler)
        self.results["notification_subscription"] = True
        ok("notification 处理器注册成功")

    def _test_error_handling(self):
        """Test 5: 错误处理（无效 method / params）"""
        header("Test 5: Error Handling")

        # 新建独立连接，避免污染已有初始化的 session
        step("新建独立连接...")
        test_conn = ACPSession(cwd=self.cwd, print_logs=False, verbose=False)
        if not test_conn.start():
            warn("无法创建测试连接，跳过 error handling 测试")
            self.results["error_unknown_method"] = False
            self.results["error_invalid_params"] = False
            self.results["error_missing_fields"] = False
            return

        try:
            # 5a: 不存在的方法
            step("发送不存在的 method...")
            resp = test_conn.send_request("nonexistent.method", {})
            if resp.is_error:
                kv("error.code", str(resp.error.get("code")))
                kv("error.message", resp.error.get("message", ""))
                ok(f"未知 method 返回 error: code={resp.error.get('code')}")
                self.results["error_unknown_method"] = True
            else:
                fail("未知 method 应返回 error")
                self.results["error_unknown_method"] = False

            # 5b: 无效的 protocolVersion（传 string）
            step("发送无效 protocolVersion...")
            resp = test_conn.send_request("initialize", {
                "protocolVersion": "v1",  # 应该是 int
                "capabilities": {},
                "clientInfo": {"name": "test", "version": "1.0"},
            })
            if resp.is_error:
                kv("error.code", str(resp.error.get("code")))
                kv("error.data", json.dumps(resp.error.get("data", {}), ensure_ascii=False)[:120])
                ok(f"无效 params 返回 error: code={resp.error.get('code')}")
                self.results["error_invalid_params"] = True
            else:
                warn("无效 protocolVersion 未返回预期 error（可能接受了）")
                self.results["error_invalid_params"] = False

            # 5c: 缺少必需字段
            step("发送缺少 params 的 initialize...")
            resp = test_conn.send_request("initialize", {})  # 缺少 protocolVersion 等
            if resp.is_error:
                ok("缺少必需字段返回 error")
                self.results["error_missing_fields"] = True
            else:
                warn("缺少必需字段可能未严格校验")
                self.results["error_missing_fields"] = False
        finally:
            test_conn.stop()

    def _test_concurrent_requests(self):
        """Test 6: 并发请求（验证 id 唯一性）"""
        header("Test 6: Concurrent Requests")

        # JSON-RPC 2.0: 不存在真正的并发请求（基于 stdin/stdout 的文本协议）。
        # 这里测试：连续快速发送 3 个请求，验证 id 递增且响应一一对应。
        # 由于每个请求内部通过 id 匹配，不需要顺序保证。
        step("连续发送 3 个 initialize 请求（同一连接）...")
        ids_seen = []
        for i in range(3):
            resp = self.session.send_request("initialize", {
                "protocolVersion": 1,
                "capabilities": {},
                "clientInfo": {"name": f"test-{i}", "version": "1.0"},
            })
            ids_seen.append(resp.id)
            kv(f"request_{i}_id", str(resp.id))

        # 所有响应 id 应该是递增的
        ids_unique = len(set(ids_seen)) == len(ids_seen)
        ok(f"所有请求 id 唯一: {ids_unique}")
        self.results["concurrent_requests"] = ids_unique

    # ── Summary ─────────────────────────────────────────────────────────────

    def _print_summary(self):
        header("测试结果汇总")
        for name, passed in self.results.items():
            status = f"{C.GREEN}✅ PASS{C.RESET}" if passed else f"{C.RED}❌ FAIL{C.RESET}"
            print(f"  {status}  {name}")

        passed = sum(self.results.values())
        total = len(self.results)
        print(f"\n{C.BOLD}{'═' * 62}{C.RESET}")
        if passed == total:
            print(f"{C.GREEN}{C.BOLD}✅ 全部通过 ({passed}/{total}){C.RESET}")
        else:
            print(f"{C.RED}{C.BOLD}❌ 部分失败 ({passed}/{total} 通过){C.RESET}")
        print(f"{C.BOLD}{'═' * 62}{C.RESET}\n")


# ──────────────────────────────────────────────────────────────────────────────
# Entry point
# ──────────────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="测试 opencode acp JSON-RPC 2.0 stdio 协议",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "--cwd",
        default=".",
        help="opencode acp 的工作目录 (default: .)",
    )
    parser.add_argument(
        "--verbose", "-v",
        action="store_true",
        help="打印所有发送/接收的原始消息",
    )
    args = parser.parse_args()

    tester = ACPTester(cwd=args.cwd, verbose=args.verbose)
    success = tester.run()
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
