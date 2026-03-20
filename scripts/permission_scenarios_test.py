#!/usr/bin/env python3
"""测试触发权限请求的场景"""

import subprocess
import json
import sys
import os

def test_permission_mode_default():
    """测试 --permission-mode=default 下的权限请求"""
    print("测试 1: --permission-mode=default 触发权限请求")
    print("=" * 60)

    # 触发需要权限的操作
    result = subprocess.run(
        ["claude", "-p",
         "--output-format", "stream-json",
         "--verbose",
         "--permission-mode", "default",
         "执行命令: rm -f /tmp/test_file_that_does_not_exist_xyz"],
        capture_output=True,
        text=True,
        timeout=30,
        cwd="/tmp"
    )

    print(f"returncode: {result.returncode}")

    stdout_lines = result.stdout.strip().split("\n")
    print(f"stdout 行数: {len(stdout_lines)}")

    all_types = {}
    sample_events = {}

    for line in stdout_lines:
        try:
            evt = json.loads(line)
            t = evt.get("type", "unknown")
            all_types[t] = all_types.get(t, 0) + 1
            if t not in sample_events:
                sample_events[t] = evt
        except:
            pass

    print(f"\n事件类型:")
    for t, count in sorted(all_types.items(), key=lambda x: -x[1]):
        print(f"  {t}: {count} 次")

    # 检查 permission_request
    if "permission_request" in sample_events:
        print(f"\n✅ 找到 permission_request!")
        print(f"keys: {list(sample_events['permission_request'].keys())}")
        print(json.dumps(sample_events["permission_request"], indent=2, ensure_ascii=False)[:1000])

    # 检查是否有 can_use_tool 或其他权限相关事件
    for t in all_types:
        if "permission" in t or "control" in t or "tool" in t:
            print(f"\n{t} 事件:")
            print(json.dumps(sample_events[t], indent=2, ensure_ascii=False)[:500])

def test_dangerous_operation():
    """测试危险操作触发权限"""
    print("\n\n测试 2: 危险操作触发权限")
    print("=" * 60)

    result = subprocess.run(
        ["claude", "-p",
         "--output-format", "stream-json",
         "--verbose",
         "删除文件 /tmp/test_hotplex_xyz，使用 rm 命令"],
        capture_output=True,
        text=True,
        timeout=30,
        cwd="/tmp"
    )

    print(f"returncode: {result.returncode}")

    stdout_lines = result.stdout.strip().split("\n")
    all_types = {}

    for line in stdout_lines:
        try:
            evt = json.loads(line)
            t = evt.get("type", "unknown")
            all_types[t] = all_types.get(t, 0) + 1
        except:
            pass

    print(f"事件类型: {all_types}")

    # 检查 result 事件中的 permission_denials
    for line in stdout_lines:
        try:
            evt = json.loads(line)
            if evt.get("type") == "result":
                if "permission_denials" in evt:
                    print(f"\n✅ 找到 permission_denials!")
                    print(json.dumps(evt["permission_denials"], indent=2, ensure_ascii=False)[:500])
                if "stop_reason" in evt:
                    print(f"\nstop_reason: {evt['stop_reason']}")
        except:
            pass

def test_stdio_mode():
    """测试 stdio 模式下的事件"""
    print("\n\n测试 3: stdio 模式测试")
    print("=" * 60)

    result = subprocess.run(
        ["claude", "-p",
         "--output-format", "stream-json",
         "--input-format", "stream-json",
         "--permission-prompt-tool", "stdio",
         "--verbose",
         "执行命令: echo hello"],
        capture_output=True,
        text=True,
        timeout=30,
        cwd="/tmp"
    )

    print(f"returncode: {result.returncode}")
    print(f"stderr: {result.stderr[:200]}")

    stdout_lines = result.stdout.strip().split("\n")
    all_types = {}

    for line in stdout_lines:
        try:
            evt = json.loads(line)
            t = evt.get("type", "unknown")
            all_types[t] = all_types.get(t, 0) + 1
        except:
            pass

    print(f"事件类型: {all_types}")

    # 检查是否有需要 stdin 响应的提示
    if "prompt" in result.stderr.lower():
        print(f"\nstderr 中提到 prompt")

def test_stream_json_structure():
    """详细分析 stream-json 输出结构"""
    print("\n\n测试 4: 详细分析 stream-json 结构")
    print("=" * 60)

    result = subprocess.run(
        ["claude", "-p",
         "--output-format", "stream-json",
         "--verbose",
         "分析 /tmp 目录"],
        capture_output=True,
        text=True,
        timeout=30,
        cwd="/tmp"
    )

    print(f"returncode: {result.returncode}")

    stdout_lines = result.stdout.strip().split("\n")
    all_keys = set()

    for line in stdout_lines:
        try:
            evt = json.loads(line)
            all_keys.update(evt.keys())
        except:
            pass

    print(f"所有事件中的 keys: {sorted(all_keys)}")

    # 检查所有事件类型的完整结构
    for line in stdout_lines[:10]:
        try:
            evt = json.loads(line)
            t = evt.get("type", "unknown")
            keys = list(evt.keys())
            print(f"\n{t}: {keys}")
        except:
            pass

def main():
    print("🔬 Claude Code 权限场景测试")
    print("=" * 60)

    # 检查版本
    result = subprocess.run(["claude", "--version"], capture_output=True, text=True, timeout=5)
    print(f"Claude 版本: {result.stdout.strip()}")

    try:
        test_permission_mode_default()
    except subprocess.TimeoutExpired:
        print("❌ 命令超时")
    except Exception as e:
        print(f"❌ 错误: {e}")

    try:
        test_dangerous_operation()
    except subprocess.TimeoutExpired:
        print("❌ 命令超时")
    except Exception as e:
        print(f"❌ 错误: {e}")

    try:
        test_stdio_mode()
    except subprocess.TimeoutExpired:
        print("❌ 命令超时")
    except Exception as e:
        print(f"❌ 错误: {e}")

    try:
        test_stream_json_structure()
    except subprocess.TimeoutExpired:
        print("❌ 命令超时")
    except Exception as e:
        print(f"❌ 错误: {e}")

    print("\n" + "=" * 60)
    print("📊 总结")
    print("=" * 60)
    print("""
关键发现:
1. --permission-mode 选项: acceptEdits, bypassPermissions, default, dontAsk, plan, auto
2. --permission-prompt-tool stdio 不在官方帮助文档中
3. 需要测试控制请求 (control_request) 是否存在
""")

if __name__ == "__main__":
    main()
