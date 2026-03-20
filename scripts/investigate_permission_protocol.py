#!/usr/bin/env python3
"""
Claude Code Permission Protocol Investigation Script

调研目标：
1. 验证 --permission-prompt-tool stdio 是否是有效参数
2. 捕获 control_request / control_response 协议事件
3. 分析不同权限模式下的事件差异

参考：cc-connect 的权限协议实现
"""

import subprocess
import json
import sys
import os
import select
import time
from dataclasses import dataclass, field
from typing import Optional


@dataclass
class ControlRequest:
    """control_request 事件结构"""
    request_id: str
    subtype: str
    tool_name: str
    input: dict = field(default_factory=dict)
    raw: dict = field(default_factory=dict)


class PermissionProtocolVerifier:
    """Claude Code 权限协议验证器"""

    def __init__(self, work_dir: str = "/tmp/hotplex_permission_test"):
        self.work_dir = work_dir
        os.makedirs(work_dir, exist_ok=True)
        self.control_requests: list[ControlRequest] = []

    def check_claude_version(self) -> str:
        """检查 Claude Code 版本"""
        try:
            result = subprocess.run(
                ["claude", "--version"], capture_output=True, text=True, timeout=10
            )
            return result.stdout.strip() if result.returncode == 0 else "unknown"
        except Exception:
            return "not_installed"

    def test_permission_prompt_tool_stdio(self) -> dict:
        """
        测试 --permission-prompt-tool stdio 参数是否存在
        """
        print("\n" + "=" * 60)
        print("测试 1: --permission-prompt-tool stdio 参数有效性")
        print("=" * 60)

        # 简单测试命令，触发权限请求
        prompt = '执行: echo "hello world"'

        print(f"\n执行: claude -p --permission-prompt-tool stdio ...")

        try:
            proc = subprocess.Popen(
                [
                    "claude", "-p",
                    "--permission-prompt-tool", "stdio",
                    "--output-format", "stream-json",
                    "--input-format", "stream-json",
                    prompt
                ],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                cwd=self.work_dir,
                text=True,
            )

            stdout_lines = []
            stderr_output = []

            # 使用非阻塞读取
            while True:
                # 检查进程是否退出
                ret = proc.poll()
                reads = [proc.stdout.fileno(), proc.stderr.fileno()]
                readable, _, _ = select.select(reads, [], [], 2.0)

                if not readable and ret is not None:
                    break

                for fd in readable:
                    if fd == proc.stdout.fileno():
                        line = proc.stdout.readline()
                        if line:
                            stdout_lines.append(line.strip())
                    elif fd == proc.stderr.fileno():
                        line = proc.stderr.readline()
                        if line:
                            stderr_output.append(line.strip())

            proc.wait(timeout=5)

            # 分析输出
            json_events = []
            for line in stdout_lines:
                try:
                    data = json.loads(line)
                    json_events.append(data)
                except json.JSONDecodeError:
                    pass

            event_types = set(e.get("type", "") for e in json_events)

            # 检查是否有错误
            has_error = any("not a valid" in e.lower() or "unknown option" in e.lower()
                           for e in stderr_output)

            return {
                "param_valid": not has_error,
                "event_types": list(event_types),
                "control_requests": self._extract_control_requests(json_events),
                "stderr": stderr_output[:10],  # 只保留前10行
                "stdout_count": len(stdout_lines),
                "json_event_count": len(json_events),
            }

        except subprocess.TimeoutExpired:
            proc.kill()
            return {"error": "timeout"}
        except Exception as e:
            return {"error": str(e)}

    def test_control_request_events(self) -> dict:
        """
        测试 control_request 事件类型
        触发一个需要权限的操作，观察事件流
        """
        print("\n" + "=" * 60)
        print("测试 2: control_request 事件类型捕获")
        print("=" * 60)

        # 触发可能需要权限的操作
        prompt = '运行: rm -rf /tmp/test_permission_safety_check'
        self.control_requests = []

        try:
            proc = subprocess.Popen(
                [
                    "claude", "-p",
                    "--output-format", "stream-json",
                    prompt
                ],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                cwd=self.work_dir,
                text=True,
            )

            stdout_lines = []
            all_events = []

            while True:
                ret = proc.poll()
                reads = [proc.stdout.fileno(), proc.stderr.fileno()]
                readable, _, _ = select.select(reads, [], [], 2.0)

                if not readable and ret is not None:
                    break

                for fd in readable:
                    if fd == proc.stdout.fileno():
                        line = proc.stdout.readline()
                        if line:
                            stdout_lines.append(line.strip())
                            try:
                                all_events.append(json.loads(line))
                            except json.JSONDecodeError:
                                pass
                    elif fd == proc.stderr.fileno():
                        proc.stderr.readline()

            proc.wait(timeout=10)

            # 分析事件类型
            event_types = {}
            for evt in all_events:
                t = evt.get("type", "unknown")
                event_types[t] = event_types.get(t, 0) + 1

            # 检查 control_request
            control_reqs = self._extract_control_requests(all_events)
            permission_requests = [e for e in all_events if e.get("type") == "permission_request"]

            return {
                "event_types": event_types,
                "control_requests": control_reqs,
                "permission_requests": [
                    {
                        "type": "permission_request",
                        "has_permission": "permission" in e,
                        "has_decision": "decision" in e,
                        "keys": list(e.keys()),
                    }
                    for e in permission_requests[:3]
                ],
                "all_json_events": len(all_events),
            }

        except Exception as e:
            return {"error": str(e)}

    def test_permission_mode_ask(self) -> dict:
        """
        测试 --permission-mode=ask 模式
        """
        print("\n" + "=" * 60)
        print("测试 3: --permission-mode=ask 模式")
        print("=" * 60)

        prompt = '执行: ls -la'
        events = []

        try:
            proc = subprocess.Popen(
                [
                    "claude", "-p",
                    "--permission-mode=ask",
                    "--output-format", "stream-json",
                    prompt
                ],
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                cwd=self.work_dir,
                text=True,
            )

            while True:
                ret = proc.poll()
                reads = [proc.stdout.fileno(), proc.stderr.fileno()]
                readable, _, _ = select.select(reads, [], [], 2.0)

                if not readable and ret is not None:
                    break

                for fd in readable:
                    if fd == proc.stdout.fileno():
                        line = proc.stdout.readline()
                        if line:
                            try:
                                events.append(json.loads(line))
                            except json.JSONDecodeError:
                                pass

            proc.wait(timeout=10)

            event_types = {}
            for evt in events:
                t = evt.get("type", "unknown")
                event_types[t] = event_types.get(t, 0) + 1

            return {
                "permission_mode": "ask",
                "event_types": event_types,
                "control_requests": self._extract_control_requests(events),
                "permission_requests": len([e for e in events if e.get("type") == "permission_request"]),
            }

        except Exception as e:
            return {"error": str(e)}

    def test_control_response_write(self) -> dict:
        """
        测试是否可以写入 control_response 到 stdin
        这是验证双向协议的关键
        """
        print("\n" + "=" * 60)
        print("测试 4: 双向协议 - control_response 写入")
        print("=" * 60)

        # 创建一个会触发权限请求的命令
        prompt = '运行: chmod 777 /tmp/test_file'

        try:
            proc = subprocess.Popen(
                [
                    "claude", "-p",
                    "--permission-prompt-tool", "stdio",
                    "--output-format", "stream-json",
                    "--input-format", "stream-json",
                    prompt
                ],
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                cwd=self.work_dir,
                text=True,
            )

            stdout_lines = []
            control_req_received = False
            response_written = False

            # 分阶段读取输出
            deadline = time.time() + 30

            while time.time() < deadline:
                ret = proc.poll()
                if ret is not None:
                    break

                # 检查是否有可读数据
                reads = [proc.stdout.fileno()]
                readable, _, _ = select.select(reads, [], [], 0.5)

                if proc.stdout.fileno() in readable:
                    line = proc.stdout.readline()
                    if line:
                        stdout_lines.append(line.strip())
                        try:
                            data = json.loads(line)
                            # 检测 control_request
                            if data.get("type") == "control_request":
                                control_req_received = True
                                request_id = data.get("request_id", "")

                                print(f"\n  收到 control_request:")
                                print(f"    request_id: {request_id}")
                                print(f"    subtype: {data.get('request', {}).get('subtype', '')}")
                                print(f"    tool_name: {data.get('request', {}).get('tool_name', '')}")

                                # 写入 control_response
                                response = {
                                    "type": "control_response",
                                    "response": {
                                        "subtype": "success",
                                        "request_id": request_id,
                                        "response": {
                                            "behavior": "allow"
                                        }
                                    }
                                }
                                print(f"\n  写入 control_response: {json.dumps(response)[:100]}...")
                                proc.stdin.write(json.dumps(response) + "\n")
                                proc.stdin.flush()
                                response_written = True
                                time.sleep(0.5)

                        except json.JSONDecodeError:
                            pass

            # 收集最终输出
            proc.wait(timeout=5)

            return {
                "control_req_received": control_req_received,
                "response_written": response_written,
                "stdout_lines": len(stdout_lines),
                "stdout_preview": stdout_lines[:20],
            }

        except Exception as e:
            return {"error": str(e)}

    def test_claude_help_permissions(self) -> list:
        """
        从 claude --help 获取权限相关参数列表
        """
        print("\n" + "=" * 60)
        print("测试 5: Claude CLI 帮助文档中的权限参数")
        print("=" * 60)

        try:
            result = subprocess.run(
                ["claude", "--help"],
                capture_output=True,
                text=True,
                timeout=10
            )

            lines = result.stdout.split("\n")
            permission_lines = []

            for i, line in enumerate(lines):
                lower = line.lower()
                if "permission" in lower or "prompt" in lower or "tool" in lower:
                    permission_lines.append(line.strip())

            # 也检查是否有 --help 输出
            return permission_lines

        except Exception as e:
            return [f"error: {e}"]

    def test_control_request_subtypes(self) -> dict:
        """
        测试 control_request 的不同 subtype
        """
        print("\n" + "=" * 60)
        print("测试 6: control_request subtypes 探索")
        print("=" * 60)

        # 尝试不同命令触发不同类型的权限请求
        test_cases = [
            ("文件删除", 'rm -f /tmp/test_file_xyz'),
            ("系统修改", 'chmod 777 /tmp/test'),
            ("网络访问", 'curl -s http://httpbin.org/get'),
            ("环境变量", 'export TEST_VAR=test'),
        ]

        results = {}

        for name, cmd in test_cases:
            print(f"\n  测试: {name} - {cmd}")

            try:
                proc = subprocess.Popen(
                    [
                        "claude", "-p",
                        "--permission-prompt-tool", "stdio",
                        "--output-format", "stream-json",
                        "--input-format", "stream-json",
                        f"执行命令: {cmd}"
                    ],
                    stdout=subprocess.PIPE,
                    stderr=subprocess.PIPE,
                    cwd=self.work_dir,
                    text=True,
                )

                events = []
                control_reqs = []

                deadline = time.time() + 15
                while time.time() < deadline:
                    ret = proc.poll()
                    if ret is not None:
                        break

                    readable, _, _ = select.select([proc.stdout.fileno()], [], [], 0.5)
                    if proc.stdout.fileno() in readable:
                        line = proc.stdout.readline()
                        if line:
                            try:
                                evt = json.loads(line)
                                events.append(evt)
                                if evt.get("type") == "control_request":
                                    control_reqs.append(evt)
                            except json.JSONDecodeError:
                                pass

                proc.wait(timeout=2)

                results[name] = {
                    "command": cmd,
                    "total_events": len(events),
                    "control_requests": [
                        {
                            "subtype": e.get("request", {}).get("subtype", ""),
                            "tool_name": e.get("request", {}).get("tool_name", ""),
                        }
                        for e in control_reqs
                    ],
                    "event_types": list(set(e.get("type", "") for e in events)),
                }

            except Exception as e:
                results[name] = {"error": str(e)}

        return results

    def _extract_control_requests(self, events: list) -> list:
        """从事件列表中提取 control_request"""
        return [
            {
                "request_id": e.get("request_id", ""),
                "subtype": e.get("request", {}).get("subtype", ""),
                "tool_name": e.get("request", {}).get("tool_name", ""),
                "raw_keys": list(e.keys()),
            }
            for e in events
            if e.get("type") == "control_request"
        ]

    def run_all_tests(self):
        """运行所有测试"""
        print("🔬 Claude Code 权限协议调研")
        print("=" * 60)

        version = self.check_claude_version()
        print(f"Claude Code 版本: {version}")

        results = {}

        # 测试 1: 参数有效性
        results["param_stdio"] = self.test_permission_prompt_tool_stdio()

        # 测试 2: control_request 事件
        results["control_events"] = self.test_control_request_events()

        # 测试 3: permission-mode=ask
        results["permission_mode_ask"] = self.test_permission_mode_ask()

        # 测试 4: 双向协议
        results["control_response"] = self.test_control_response_write()

        # 测试 5: 帮助文档
        results["help_permissions"] = self.test_claude_help_permissions()

        # 测试 6: subtypes 探索
        results["subtypes"] = self.test_control_request_subtypes()

        # 输出报告
        self.print_report(results)

        return results

    def print_report(self, results: dict):
        """打印调研报告"""
        print("\n" + "=" * 70)
        print("📊 调研报告")
        print("=" * 70)

        print("\n【1. --permission-prompt-tool stdio 参数】")
        if "error" in results["param_stdio"]:
            print(f"  ❌ 测试失败: {results['param_stdio']['error']}")
        else:
            print(f"  参数有效性: {'✅ 有效' if results['param_stdio'].get('param_valid') else '❌ 无效'}")
            print(f"  事件类型: {results['param_stdio'].get('event_types', [])}")
            print(f"  stderr 输出: {results['param_stdio'].get('stderr', [])[:3]}")

        print("\n【2. control_request 事件】")
        ctrl_reqs = results["control_events"].get("control_requests", [])
        print(f"  捕获到 control_request: {len(ctrl_reqs)} 个")
        if ctrl_reqs:
            print(f"  示例: {ctrl_reqs[0]}")
        else:
            print(f"  event_types: {results['control_events'].get('event_types', {})}")

        print("\n【3. --permission-mode=ask】")
        print(f"  permission_request 事件: {results['permission_mode_ask'].get('permission_requests', 'N/A')}")
        print(f"  event_types: {results['permission_mode_ask'].get('event_types', {})}")

        print("\n【4. 双向协议 (control_response)】")
        resp_test = results["control_response"]
        if "error" in resp_test:
            print(f"  ❌ 测试失败: {resp_test['error']}")
        else:
            print(f"  control_request 收到: {resp_test.get('control_req_received')}")
            print(f"  control_response 写入: {resp_test.get('response_written')}")
            print(f"  输出行数: {resp_test.get('stdout_lines')}")

        print("\n【5. CLI 帮助中的权限参数】")
        for line in results["help_permissions"][:10]:
            print(f"  {line}")

        print("\n【6. control_request subtypes 探索】")
        for name, data in results["subtypes"].items():
            if "error" in data:
                print(f"  {name}: ❌ {data['error']}")
            else:
                ctrl = data.get("control_requests", [])
                print(f"  {name}:")
                print(f"    - 事件数: {data.get('total_events', 0)}")
                print(f"    - control_requests: {len(ctrl)} 个")
                for req in ctrl[:2]:
                    print(f"      {req}")

        # 总结
        print("\n" + "=" * 70)
        print("📈 结论")
        print("=" * 70)

        has_control_request = any(
            results["control_events"].get("control_requests", []) or
            results["param_stdio"].get("control_requests", []) or
            any(d.get("control_requests") for d in results["subtypes"].values())
        )

        has_stdio_param = results["param_stdio"].get("param_valid", False)

        if has_stdio_param and has_control_request:
            print("✅ Claude Code 原生支持 --permission-prompt-tool stdio 协议")
            print("   可以实现 control_request/control_response 双向权限协议")
        elif has_stdio_param:
            print("⚠️  --permission-prompt-tool stdio 参数有效，但未触发 control_request")
            print("   可能需要特定条件或命令才能触发")
        else:
            print("❌ Claude Code 不支持 --permission-prompt-tool stdio 参数")
            print("   cc-connect 的实现可能需要特殊处理或使用了不同机制")


def main():
    import argparse

    parser = argparse.ArgumentParser(
        description="Claude Code 权限协议调研"
    )
    parser.add_argument(
        "--work-dir",
        default="/tmp/hotplex_permission_test",
        help="测试工作目录"
    )
    parser.add_argument(
        "--test",
        choices=["all", "stdio", "control", "ask", "response", "help", "subtypes"],
        default="all",
        help="运行的测试"
    )

    args = parser.parse_args()

    verifier = PermissionProtocolVerifier(work_dir=args.work_dir)

    if args.test == "all":
        verifier.run_all_tests()
    else:
        # 单个测试
        test_map = {
            "stdio": verifier.test_permission_prompt_tool_stdio,
            "control": verifier.test_control_request_events,
            "ask": verifier.test_permission_mode_ask,
            "response": verifier.test_control_response_write,
            "help": lambda: {"help": verifier.test_claude_help_permissions()},
            "subtypes": verifier.test_control_request_subtypes,
        }
        result = test_map[args.test]()
        print(json.dumps(result, indent=2, ensure_ascii=False, default=str))


if __name__ == "__main__":
    main()
