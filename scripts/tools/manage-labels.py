#!/usr/bin/env python3
"""
HotPlex GitHub Label Manager
优化 GitHub issue/PR 标签体系并增强 UI 效果

Usage:
    python scripts/manage-labels.py --dry-run  # 预览变更
    python scripts/manage-labels.py --apply    # 应用变更
"""

import argparse
import subprocess
import json
from typing import Dict, List

REPO = "hrygo/hotplex"

# 标签体系定义（基于 Kubernetes, React, VS Code 最佳实践）
LABELS = {
    # ===== 优先级 (Priority) =====
    "priority/critical": {
        "color": "d73a4a",  # 红色 - 最高优先级
        "description": "🔴 P0 - 阻塞核心功能、安全漏洞、数据丢失"
    },
    "priority/high": {
        "color": "ff6b35",  # 橙色 - 高优先级
        "description": "🟠 P1 - 严重影响用户体验、频繁出现的问题"
    },
    "priority/medium": {
        "color": "fbca04",  # 黄色 - 中等优先级
        "description": "🟡 P2 - 中等影响、有临时解决方案"
    },
    "priority/low": {
        "color": "c5def5",  # 浅蓝 - 低优先级
        "description": "🔵 P3 - 小问题、nice-to-have 改进"
    },

    # ===== 类型 (Type) =====
    "type/bug": {
        "color": "d73a4a",  # 红色
        "description": "🐛 Bug - 功能异常、错误行为"
    },
    "type/feature": {
        "color": "0e8a16",  # 绿色
        "description": "✨ Feature - 新功能请求"
    },
    "type/enhancement": {
        "color": "a2eeef",  # 青色
        "description": "💪 Enhancement - 改进现有功能"
    },
    "type/docs": {
        "color": "0075ca",  # 蓝色
        "description": "📚 Documentation - 文档改进"
    },
    "type/test": {
        "color": "bfd4f2",  # 浅蓝
        "description": "🧪 Testing - 测试相关"
    },
    "type/refactor": {
        "color": "7057ff",  # 紫色
        "description": "♻️ Refactor - 代码重构"
    },
    "type/security": {
        "color": "d93f0b",  # 深橙
        "description": "🔒 Security - 安全相关"
    },

    # ===== 规模 (Size) =====
    "size/small": {
        "color": "c2e0c6",  # 浅绿
        "description": "📏 Small - < 1 天工作量"
    },
    "size/medium": {
        "color": "fbca04",  # 黄色
        "description": "📏 Medium - 1-3 天工作量"
    },
    "size/large": {
        "color": "e99695",  # 浅红
        "description": "📏 Large - > 3 天工作量"
    },

    # ===== 状态 (Status) =====
    "status/needs-triage": {
        "color": "bfdadc",  # 浅灰蓝
        "description": "🔍 Needs Triage - 需要进一步评估"
    },
    "status/ready-for-work": {
        "color": "0e8a16",  # 绿色
        "description": "✅ Ready for Work - 信息完整，可以开始"
    },
    "status/blocked": {
        "color": "d93f0b",  # 深橙
        "description": "🚫 Blocked - 依赖其他 issues/外部因素"
    },
    "status/in-progress": {
        "color": "fbca04",  # 黄色
        "description": "🚧 In Progress - 正在处理中"
    },
    "status/stale": {
        "color": "ececec",  # 浅灰
        "description": "💤 Stale - 60+ 天无更新"
    },

    # ===== 平台 (Platform) =====
    "platform/slack": {
        "color": "4a154b",  # Slack 紫色
        "description": "💬 Slack - Slack 平台相关"
    },
    "platform/telegram": {
        "color": "0088cc",  # Telegram 蓝
        "description": "✈️ Telegram - Telegram 平台相关"
    },
    "platform/feishu": {
        "color": "3370ff",  # 飞书蓝
        "description": "🪶 Feishu - 飞书平台相关"
    },
    "platform/discord": {
        "color": "5865f2",  # Discord 紫
        "description": "🎮 Discord - Discord 平台相关"
    },

    # ===== 模块 (Area) =====
    "area/engine": {
        "color": "1d76db",  # 蓝色
        "description": "⚙️ Engine - 核心引擎"
    },
    "area/adapter": {
        "color": "5319e7",  # 紫色
        "description": "🔌 Adapter - 平台适配器"
    },
    "area/provider": {
        "color": "0e8a16",  # 绿色
        "description": "🤖 Provider - AI Provider 集成"
    },
    "area/security": {
        "color": "d93f0b",  # 深橙
        "description": "🛡️ Security - 安全模块 (WAF, 权限)"
    },
    "area/admin": {
        "color": "fbca04",  # 黄色
        "description": "📊 Admin - Admin API 和管理功能"
    },
    "area/brain": {
        "color": "7057ff",  # 紫色
        "description": "🧠 Brain - Native Brain 路由"
    },

    # ===== 特殊标签 =====
    "good first issue": {
        "color": "7057ff",  # 紫色
        "description": "👋 Good First Issue - 适合新手"
    },
    "help wanted": {
        "color": "008672",  # 绿色
        "description": "🆘 Help Wanted - 需要社区帮助"
    },
    "epic": {
        "color": "3e4b9e",  # 深蓝
        "description": "📦 Epic - 高层目标，包含多个子任务"
    },
    "wontfix": {
        "color": "ffffff",  # 白色
        "description": "❌ Wontfix - 不会修复"
    },
    "duplicate": {
        "color": "cfd3d7",  # 灰色
        "description": "📎 Duplicate - 重复的 issue"
    },
}

# 需要删除的旧标签
OLD_LABELS = [
    "bug",  # 替换为 type/bug
    "enhancement",  # 替换为 type/enhancement
    "documentation",  # 替换为 type/docs
    "critical",  # 替换为 priority/critical
    "testing",  # 替换为 type/test
    "performance",  # 替换为 type/enhancement + area/performance
    "windows",  # 替换为 platform/windows
    "chatapps",  # 替换为 area/adapter
    "slack",  # 替换为 platform/slack
    "ui/ux",  # 替换为 type/enhancement
    "permission",  # 替换为 area/security
    "research",  # 保留但需更新颜色
    "telemetry",  # 替换为 area/telemetry
    "claude",
    "integration",
    "pi",
    "provider",  # 替换为 area/provider
    "architecture",  # 替换为 type/refactor
    "config",
    "admin-api",  # 替换为 area/admin
    "cli",
    "strategic",
    "chatapps/feishu",  # 替换为 platform/feishu
    "core",  # 替换为 area/engine
    "admin",  # 替换为 area/admin
    "api",
    "concurrency",
    "security",  # 替换为 type/security 或 area/security
]


def run_gh_command(cmd: List[str], dry_run: bool = True) -> bool:
    """运行 gh 命令"""
    if dry_run:
        print(f"  [DRY RUN] {' '.join(cmd)}")
        return True

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            check=True
        )
        return True
    except subprocess.CalledProcessError as e:
        print(f"  ❌ Error: {e.stderr}")
        return False


def create_label(name: str, color: str, description: str, dry_run: bool = True):
    """创建或更新标签"""
    cmd = [
        "gh", "label", "create", name,
        "--repo", REPO,
        "--color", color,
        "--description", description,
        "--force"  # 如果已存在则更新
    ]
    return run_gh_command(cmd, dry_run)


def delete_label(name: str, dry_run: bool = True):
    """删除标签"""
    cmd = ["gh", "label", "delete", name, "--repo", REPO, "--yes"]
    return run_gh_command(cmd, dry_run)


def main():
    parser = argparse.ArgumentParser(description="HotPlex GitHub Label Manager")
    parser.add_argument("--dry-run", action="store_true", help="预览变更而不执行")
    parser.add_argument("--apply", action="store_true", help="应用变更")
    parser.add_argument("--skip-delete", action="store_true", help="跳过删除旧标签")

    args = parser.parse_args()

    if not args.dry_run and not args.apply:
        print("❌ 必须指定 --dry-run 或 --apply")
        return

    dry_run = args.dry_run

    print("🏷️  HotPlex GitHub Label Manager")
    print("=" * 60)
    print(f"模式: {'DRY RUN (预览)' if dry_run else 'APPLY (执行)'}")
    print(f"仓库: {REPO}")
    print()

    # Step 1: 创建/更新标签
    print(f"📝 创建/更新标签 ({len(LABELS)} 个)...")
    created = 0
    for name, config in LABELS.items():
        print(f"  {'[DRY RUN] ' if dry_run else ''}✨ {name}: {config['description']}")
        if create_label(name, config['color'], config['description'], dry_run):
            created += 1

    print(f"\n✅ 成功处理 {created}/{len(LABELS)} 个标签")

    # Step 2: 删除旧标签
    if not args.skip_delete:
        print(f"\n🗑️  删除旧标签 ({len(OLD_LABELS)} 个)...")
        deleted = 0
        for name in OLD_LABELS:
            print(f"  {'[DRY RUN] ' if dry_run else ''}🗑️  {name}")
            if delete_label(name, dry_run):
                deleted += 1

        print(f"\n✅ 成功删除 {deleted}/{len(OLD_LABELS)} 个标签")

    print("\n" + "=" * 60)
    if dry_run:
        print("🎉 预览完成！运行 --apply 应用变更")
    else:
        print("🎉 标签优化完成！")
        print("\n📊 标签体系统计:")
        print(f"  • 优先级: 4 个 (critical/high/medium/low)")
        print(f"  • 类型: 7 个 (bug/feature/enhancement/docs/test/refactor/security)")
        print(f"  • 规模: 3 个 (small/medium/large)")
        print(f"  • 状态: 5 个 (needs-triage/ready-for-work/blocked/in-progress/stale)")
        print(f"  • 平台: 4 个 (slack/telegram/feishu/discord)")
        print(f"  • 模块: 6 个 (engine/adapter/provider/security/admin/brain)")
        print(f"  • 特殊: 4 个 (good first issue/help wanted/epic/wontfix/duplicate)")
        print("\n🔗 查看标签: https://github.com/hrygo/hotplex/labels")


if __name__ == "__main__":
    main()
