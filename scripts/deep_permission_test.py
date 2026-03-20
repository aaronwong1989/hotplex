#!/usr/bin/env python3
"""深度测试 Claude Code control_request 协议"""

import subprocess
import json
import sys
import os
import select
import time
import signal

def test_control_request_protocol():
    """
    核心测试：验证 --permission-prompt-tool stdio 是否真正启用
    control_request/control_response 双向协议
    """
    print("🔬 控制请求协议深度测试")
    print("=" * 60)

    # 检查 Claude 版本
    result = subprocess.run(["claude", "--version"], capture_output=True, text=True, timeout=5)
    version = result.stdout.strip()
    print(f"Claude 版本: {version}")

    # 检查 --permission-prompt-tool 参数
    print("\n测试 1: 检查 --permission-prompt-tool 参数")
    print("-" * 40)
    result = subprocess.run(
        ["claude", "-p",
         "--permission-prompt-tool", "stdio",
         "--help"],
        capture_output=True,
        text=True,
        timeout=10
    )
    if "--permission-prompt-tool" in result.stdout:
        print("✅ --permission-prompt-tool 在帮助文档中")
    else:
        print("❌ --permission-prompt-tool 不在帮助文档中")

    # 检查 stderr 中是否有警告
    if "permission-prompt-tool" in result.stderr.lower():
        print(f"stderr 提到此参数: {result.stderr[:200]}")

    # 测试 2: 双向协议测试
    print("\n\n测试 2: 双向协议 (stdin/stdout)")
    print("-" * 40)

    # 启动进程
    proc = subprocess.Popen(
        ["claude", "-p",
         "--permission-prompt-tool", "stdio",
         "--output-format", "stream-json",
         "--input-format", "stream-json",
         "--verbose",
         "执行: echo hello"],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        cwd="/tmp",
        text=True,
        bufsize=1
    )

    stdout_lines = []
    stderr_lines = []
    events = []
    control_requests = []
    start_time = time.time()
    timeout = 20

    print("等待事件...")

    try:
        while time.time() - start_time < timeout:
            # 检查进程是否退出
            ret = proc.poll()
            if ret is not None:
                print(f"进程已退出，returncode={ret}")
                break

            # 读取 stdout
            readable, _, _ = select.select([proc.stdout.fileno()], [], [], 0.5)

            if proc.stdout.fileno() in readable:
                line = proc.stdout.readline()
                if line:
                    stdout_lines.append(line.strip())
                    try:
                        evt = json.loads(line)
                        events.append(evt)

                        if evt.get("type") == "control_request":
                            print(f"\n✅ 收到 control_request!")
                            print(f"  request_id: {evt.get('request_id', '')}")
                            print(f"  keys: {list(evt.keys())}")

                            # 检查请求内容
                            request = evt.get("request", {})
                            print(f"  request.subtype: {request.get('subtype', '')}")
                            print(f"  request.tool_name: {request.get('tool_name', '')}")

                            control_requests.append(evt)

                            # 发送响应
                            response = {
                                "type": "control_response",
                                "response": {
                                    "subtype": "success",
                                    "request_id": evt.get("request_id", ""),
                                    "response": {
                                        "behavior": "allow"
                                    }
                                }
                            }
                            print(f"\n发送响应: behavior=allow")
                            proc.stdin.write(json.dumps(response) + "\n")
                            proc.stdin.flush()

                    except json.JSONDecodeError:
                        pass

            # 读取 stderr
            readable, _, _ = select.select([proc.stderr.fileno()], [], [], 0.1)
            if proc.stderr.fileno() in readable:
                line = proc.stderr.readline()
                if line:
                    stderr_lines.append(line.strip())

    except Exception as e:
        print(f"异常: {e}")
    finally:
        if proc.poll() is None:
            proc.kill()
            proc.wait()

    # 分析结果
    print(f"\n\n结果分析:")
    print(f"  stdout 行数: {len(stdout_lines)}")
    print(f"  事件数: {len(events)}")
    print(f"  control_request: {len(control_requests)}")

    event_types = {}
    for evt in events:
        t = evt.get("type", "unknown")
        event_types[t] = event_types.get(t, 0) + 1

    print(f"\n事件类型统计:")
    for t, count in sorted(event_types.items()):
        print(f"  {t}: {count}")

    # 检查 stderr
    if stderr_lines:
        print(f"\nstderr 输出 (前5行):")
        for line in stderr_lines[:5]:
            print(f"  {line}")

    return {
        "events": events,
        "control_requests": control_requests,
        "event_types": event_types,
    }

def test_permission_denials():
    """测试 permission_denials 结构"""
    print("\n\n测试 3: permission_denials 结构")
    print("-" * 40)

    result = subprocess.run(
        ["claude", "-p",
         "--output-format", "stream-json",
         "--verbose",
         "执行命令: rm -rf /important/system/file",
         ],
        capture_output=True,
        text=True,
        timeout=30,
        cwd="/tmp"
    )

    for line in result.stdout.strip().split("\n"):
        try:
            evt = json.loads(line)
            if evt.get("type") == "result":
                print(f"result 事件 keys: {list(evt.keys())}")
                if "permission_denials" in evt:
                    print(f"permission_denials: {json.dumps(evt['permission_denials'], indent=2)}")
                if "stop_reason" in evt:
                    print(f"stop_reason: {evt['stop_reason']}")
        except:
            pass

def test_claude_json_options():
    """检查所有 JSON 相关选项"""
    print("\n\n测试 4: JSON 选项探索")
    print("-" * 40)

    # 测试不同的 input/output format
    formats = [
        ("stream-json", "stream-json"),
        ("stream-json", "text"),
    ]

    for output_fmt, input_fmt in formats:
        print(f"\n测试 {output_fmt}/{input_fmt}:")

        proc = subprocess.Popen(
            ["claude", "-p",
             f"--output-format={output_fmt}",
             f"--input-format={input_fmt}",
             "说 hello"],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            cwd="/tmp",
            text=True
        )

        try:
            stdout, stderr = proc.communicate(timeout=15)
            lines = stdout.strip().split("\n") if stdout.strip() else []
            print(f"  stdout 行数: {len(lines)}")

            # 检查是否是 JSON
            if lines:
                try:
                    json.loads(lines[0])
                    print(f"  ✅ 第一行是有效 JSON")
                except:
                    print(f"  ❌ 第一行不是 JSON")
                    print(f"  内容: {lines[0][:100]}...")

        except subprocess.TimeoutExpired:
            proc.kill()
            print(f"  ❌ 超时")
        except Exception as e:
            print(f"  ❌ 错误: {e}")

def main():
    print("=" * 60)
    print("Claude Code 控制请求协议调研")
    print("=" * 60)

    try:
        result = test_control_request_protocol()
    except Exception as e:
        print(f"❌ 测试1 失败: {e}")

    try:
        test_permission_denials()
    except Exception as e:
        print(f"❌ 测试3 失败: {e}")

    try:
        test_claude_json_options()
    except Exception as e:
        print(f"❌ 测试4 失败: {e}")

    print("\n" + "=" * 60)
    print("📊 最终结论")
    print("=" * 60)
    print("""
基于测试结果:

1. --permission-prompt-tool stdio:
   - 不在官方帮助文档中
   - 但不报错，可能是隐藏/未公开的参数

2. control_request 事件:
   - 需要进一步验证是否真的存在

3. 建议:
   - 查看 cc-connect 源码确认具体实现
   - 考虑使用其他方式实现权限询问
""")

if __name__ == "__main__":
    main()
