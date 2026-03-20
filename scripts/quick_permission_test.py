#!/usr/bin/env python3
"""简化的权限协议测试 - 直接用 subprocess"""

import subprocess
import json
import sys
import os
import signal
import time

def timeout_handler(signum, frame):
    raise TimeoutError("Command timeout")

def run_simple_test():
    """运行简单测试"""
    print("测试 1: 检查 --permission-prompt-tool 是否是有效参数")
    print("=" * 60)

    # 尝试使用 --permission-prompt-tool stdio
    # 如果参数无效，stderr 会显示错误
    result = subprocess.run(
        ["claude", "-p",
         "--permission-prompt-tool", "stdio",
         "--output-format", "stream-json",
         "--input-format", "stream-json",
         "--verbose",
         "说hello"],
        capture_output=True,
        text=True,
        timeout=10,
        cwd="/tmp"
    )

    print(f"returncode: {result.returncode}")
    print(f"\nstderr ({len(result.stderr)} bytes):")
    stderr_lines = result.stderr.strip().split("\n")
    for line in stderr_lines[:10]:
        if line.strip():
            print(f"  {line}")

    # 检查是否有 unknown option 错误
    if "unknown option" in result.stderr.lower() or "not a valid" in result.stderr.lower():
        print("\n❌ --permission-prompt-tool stdio 不是有效参数")
    else:
        print("\n✅ 参数似乎有效")

    print(f"\nstdout 前3行:")
    stdout_lines = result.stdout.strip().split("\n")
    for line in stdout_lines[:3]:
        print(f"  {line[:100]}...")

    # 分析 stdout 中的事件类型
    event_types = set()
    control_requests = []
    permission_requests = []

    for line in stdout_lines:
        try:
            evt = json.loads(line)
            t = evt.get("type", "")
            event_types.add(t)
            if t == "control_request":
                control_requests.append(evt)
            elif t == "permission_request":
                permission_requests.append(evt)
        except:
            pass

    print(f"\nevent_types 找到: {event_types}")
    print(f"control_request 数量: {len(control_requests)}")
    print(f"permission_request 数量: {len(permission_requests)}")

    if control_requests:
        print("\n✅ 找到 control_request 事件!")
        print(json.dumps(control_requests[0], indent=2, ensure_ascii=False)[:500])

    if permission_requests:
        print("\n找到 permission_request 事件:")
        print(json.dumps(permission_requests[0], indent=2, ensure_ascii=False)[:500])

def test_claude_events():
    """测试 Claude Code 输出的所有事件类型"""
    print("\n\n测试 2: 分析 Claude Code 所有事件类型")
    print("=" * 60)

    result = subprocess.run(
        ["claude", "-p",
         "--output-format", "stream-json",
         "--verbose",
         "你好，简单打个招呼"],
        capture_output=True,
        text=True,
        timeout=30,
        cwd="/tmp"
    )

    print(f"returncode: {result.returncode}")

    stdout_lines = result.stdout.strip().split("\n")
    print(f"stdout 行数: {len(stdout_lines)}")

    # 收集所有事件类型和结构
    all_types = {}
    sample_events = {}

    for line in stdout_lines:
        try:
            evt = json.loads(line)
            t = evt.get("type", "unknown")
            all_types[t] = all_types.get(t, 0) + 1

            # 保存每种类型的示例
            if t not in sample_events:
                sample_events[t] = evt
        except:
            pass

    print(f"\n事件类型统计:")
    for t, count in sorted(all_types.items(), key=lambda x: -x[1]):
        print(f"  {t}: {count} 次")

    # 显示 permission_request 示例
    if "permission_request" in sample_events:
        print(f"\npermission_request 示例 (keys: {list(sample_events['permission_request'].keys())}):")
        print(json.dumps(sample_events["permission_request"], indent=2, ensure_ascii=False)[:800])

def main():
    print("🔬 Claude Code 权限协议调研")
    print("=" * 60)

    # 检查版本
    result = subprocess.run(["claude", "--version"], capture_output=True, text=True, timeout=5)
    print(f"Claude 版本: {result.stdout.strip()}")

    try:
        run_simple_test()
    except subprocess.TimeoutExpired:
        print("❌ 命令超时 (10秒)")
    except Exception as e:
        print(f"❌ 错误: {e}")

    try:
        test_claude_events()
    except subprocess.TimeoutExpired:
        print("❌ 命令超时 (30秒)")
    except Exception as e:
        print(f"❌ 错误: {e}")

if __name__ == "__main__":
    main()
