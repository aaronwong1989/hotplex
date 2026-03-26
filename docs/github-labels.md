# HotPlex GitHub Label System

**版本**: v2.0
**最后更新**: 2026-03-22

本文档定义 HotPlex 项目的 GitHub 标签体系,基于 Kubernetes, React, VS Code 等大型开源项目的最佳实践。

---

## 标签体系概览

```
📊 标签分类结构
├── 🎯 Priority (优先级) - 4 个
│   ├── priority/critical    # 🔴 P0 - 阻塞核心功能
│   ├── priority/high        # 🟠 P1 - 严重影响用户体验
│   ├── priority/medium      # 🟡 P2 - 中等影响
│   └── priority/low         # 🔵 P3 - 小问题
│
├── 🏷️ Type (类型) - 7 个
│   ├── type/bug             # 🐛 Bug
│   ├── type/feature         # ✨ Feature
│   ├── type/enhancement     # 💪 Enhancement
│   ├── type/docs            # 📚 Documentation
│   ├── type/test            # 🧪 Testing
│   ├── type/refactor        # ♻️ Refactor
│   └── type/security        # 🔒 Security
│
├── 📏 Size (规模) - 3 个
│   ├── size/small           # < 1 天
│   ├── size/medium          # 1-3 天
│   └── size/large           # > 3 天
│
├── 🔄 Status (状态) - 5 个
│   ├── status/needs-triage      # 🔍 需要评估
│   ├── status/ready-for-work    # ✅ 可以开始
│   ├── status/blocked           # 🚫 被阻塞
│   ├── status/in-progress       # 🚧 进行中
│   └── status/stale             # 💤 过期
│
├── 💬 Platform (平台) - 4 个
│   ├── platform/slack       # 💬 Slack
│   ├── platform/telegram    # ✈️ Telegram
│   ├── platform/feishu      # 🪶 飞书
│   └── platform/discord     # 🎮 Discord
│
├── 🏗️ Area (模块) - 6 个
│   ├── area/engine          # ⚙️ Engine
│   ├── area/adapter         # 🔌 Adapter
│   ├── area/provider        # 🤖 Provider
│   ├── area/security        # 🛡️ Security
│   ├── area/admin           # 📊 Admin
│   └── area/brain           # 🧠 Brain
│
└── ⭐ Special (特殊) - 4 个
    ├── good first issue     # 👋 Good First Issue
    ├── help wanted          # 🆘 Help Wanted
    ├── epic                 # 📦 Epic
    ├── wontfix              # ❌ Wontfix
    └── duplicate            # 📎 Duplicate
```

---

## 详细说明

### 🎯 Priority (优先级)

| 标签 | 颜色 | Emoji | 判断标准 | 示例信号 |
|------|------|-------|---------|---------|
| `priority/critical` | 🔴 `#d73a4a` | 🔴 | 阻塞核心功能、安全漏洞、数据丢失 | P0, 阻塞, 生产故障, 安全漏洞 |
| `priority/high` | 🟠 `#ff6b35` | 🟠 | 严重影响用户体验、频繁出现 | P1, 用户体验严重受影响 |
| `priority/medium` | 🟡 `#fbca04` | 🟡 | 中等影响、有 workaround | P2, 需要修复但不紧急 |
| `priority/low` | 🔵 `#c5def5` | 🔵 | 小问题、nice-to-have | P3, 改进建议 |

**判断流程**:
1. 检查 body 中的 P0/P1/P2/P3 标记
2. 分析关键词：严重程度、影响范围、阻塞
3. 安全相关 → `priority/critical`
4. 用户报告的生产问题 → `priority/high` 或 `priority/medium`

---

### 🏷️ Type (类型)

| 标签 | 颜色 | Emoji | 说明 | 识别规则 |
|------|------|-------|------|---------|
| `type/bug` | 🔴 `#d73a4a` | 🐛 | 功能异常、错误行为 | 标题包含 bug/error/fail/错误/问题 |
| `type/feature` | 🟢 `#0e8a16` | ✨ | 新功能请求 | 标题包含 [feat]/feature/新功能 |
| `type/enhancement` | 🔵 `#a2eeef` | 💪 | 改进现有功能 | 标题包含 enhancement/优化/改进 |
| `type/docs` | 📘 `#0075ca` | 📚 | 文档改进 | 文档、README、API 文档相关 |
| `type/test` | 🧪 `#bfd4f2` | 🧪 | 测试相关 | 测试覆盖、测试用例 |
| `type/refactor` | 🟣 `#7057ff` | ♻️ | 代码重构 | 标题包含 refactor/重构 |
| `type/security` | 🔒 `#d93f0b` | 🔒 | 安全相关 | CVE, 安全漏洞, 权限问题 |

---

### 📏 Size (规模)

| 标签 | 颜色 | Emoji | 工作量 | 判断标准 |
|------|------|-------|--------|---------|
| `size/small` | 🟢 `#c2e0c6` | 📏 | < 1 天 | 单文件修改、配置更新、文档 |
| `size/medium` | 🟡 `#fbca04` | 📏 | 1-3 天 | 多文件修改、新功能、重构 |
| `size/large` | 🔴 `#e99695` | 📏 | > 3 天 | 架构变更、多模块影响 |

**估算维度**:
- 涉及模块数（单模块 vs 多模块）
- 是否需要架构变更
- 是否涉及多个子系统（engine, adapter, security 等）

---

### 🔄 Status (状态)

| 标签 | 颜色 | Emoji | 说明 |
|------|------|-------|------|
| `status/needs-triage` | 🔍 `#bfdadc` | 🔍 | 新 issue，需要进一步评估 |
| `status/ready-for-work` | ✅ `#0e8a16` | ✅ | 信息完整，可以开始工作 |
| `status/blocked` | 🚫 `#d93f0b` | 🚫 | 依赖其他 issues/外部因素 |
| `status/in-progress` | 🚧 `#fbca04` | 🚧 | 正在处理中 |
| `status/stale` | 💤 `#ececec` | 💤 | 60+ 天无更新 |

**状态流转**:
```
[新建] → status/needs-triage
         ↓ (信息完整)
   status/ready-for-work ←→ status/blocked
         ↓ (开始工作)
   status/in-progress
         ↓ (完成)
   closed
```

---

### 💬 Platform (平台)

| 标签 | 颜色 | Emoji | 说明 |
|------|------|-------|------|
| `platform/slack` | 💜 `#4a154b` | 💬 | Slack 平台相关 |
| `platform/telegram` | 🔵 `#0088cc` | ✈️ | Telegram 平台相关 |
| `platform/feishu` | 🔵 `#3370ff` | 🪶 | 飞书平台相关 |
| `platform/discord` | 🟣 `#5865f2` | 🎮 | Discord 平台相关 |

---

### 🏗️ Area (模块)

| 标签 | 颜色 | Emoji | 说明 |
|------|------|-------|------|
| `area/engine` | 🔵 `#1d76db` | ⚙️ | 核心引擎 (internal/engine) |
| `area/adapter` | 🟣 `#5319e7` | 🔌 | 平台适配器 (chatapps/) |
| `area/provider` | 🟢 `#0e8a16` | 🤖 | AI Provider 集成 (provider/) |
| `area/security` | 🔒 `#d93f0b` | 🛡️ | 安全模块 (internal/security) |
| `area/admin` | 🟡 `#fbca04` | 📊 | Admin API (internal/admin) |
| `area/brain` | 🟣 `#7057ff` | 🧠 | Native Brain 路由 (brain/) |

---

### ⭐ Special (特殊标签)

| 标签 | 颜色 | Emoji | 说明 |
|------|------|-------|------|
| `good first issue` | 🟣 `#7057ff` | 👋 | 适合新手，工作量小且独立 |
| `help wanted` | 🟢 `#008672` | 🆘 | 需要社区帮助 |
| `epic` | 🔵 `#3e4b9e` | 📦 | 高层目标，包含多个子任务 |
| `wontfix` | ⚪ `#ffffff` | ❌ | 不会修复 |
| `duplicate` | ⚪ `#cfd3d7` | 📎 | 重复的 issue |

---

## 使用指南

### 1. 创建 Issue 时

**必须标注**:
- ✅ 1 个 **Type** 标签（bug/feature/enhancement 等）
- ✅ 1 个 **Priority** 标签（根据严重程度）
- ✅ 1 个 **Size** 标签（预估工作量）
- ✅ 1 个 **Status** 标签（默认 `status/needs-triage`）

**可选标注**:
- 1 个或多个 **Area** 标签
- 1 个或多个 **Platform** 标签
- `good first issue` 或 `help wanted`（如适用）

**示例**:
```
Issue: [feat] 支持 Telegram 平台

标签:
- type/feature
- priority/high
- size/large
- status/needs-triage
- platform/telegram
- area/adapter
```

### 2. 创建 PR 时

**必须标注**:
- ✅ 与 Issue 相同的 **Type** 和 **Area** 标签
- ✅ `status/in-progress`（未合并时）

**PR 合并后**:
- 自动关闭关联的 Issue（通过 `Resolves #XXX`）
- 移除所有标签（GitHub 自动处理）

### 3. 自动化规则

**自动标注**:
- 新创建的 issue → `status/needs-triage`
- 标题包含 `[feat]` → `type/feature`
- 标题包含 `bug` → `type/bug`
- 涉及 `chatapps/slack` → `platform/slack` + `area/adapter`

**Stale Issue 清理**:
- 60 天无更新 → `status/stale`
- 14 天后仍无响应 → 自动关闭
- **豁免标签**: `priority/critical`, `priority/high`, `status/in-progress`, `type/security`

---

## 迁移指南

### 从旧标签迁移

| 旧标签 | 新标签 | 说明 |
|--------|--------|------|
| `bug` | `type/bug` | 标准化为 type 命名空间 |
| `enhancement` | `type/enhancement` | 标准化为 type 命名空间 |
| `documentation` | `type/docs` | 简化命名 |
| `critical` | `priority/critical` | 标准化为 priority 命名空间 |
| `testing` | `type/test` | 标准化为 type 命名空间 |
| `chatapps` | `area/adapter` | 更明确的模块名称 |
| `slack` | `platform/slack` | 标准化为 platform 命名空间 |
| `chatapps/feishu` | `platform/feishu` | 统一为 platform 命名空间 |
| `core` | `area/engine` | 更明确的模块名称 |
| `admin` | `area/admin` | 标准化为 area 命名空间 |
| `admin-api` | `area/admin` | 合并重复标签 |
| `provider` | `area/provider` | 标准化为 area 命名空间 |
| `security` | `area/security` 或 `type/security` | 根据上下文区分 |

---

## 标签颜色设计原则

### 1. 语义化颜色

| 颜色 | 含义 | 使用场景 |
|------|------|---------|
| 🔴 红色系 | 高优先级、严重问题 | critical, bug, blocked |
| 🟠 橙色系 | 高优先级、警告 | high priority, security |
| 🟡 黄色系 | 中等优先级、进行中 | medium, in-progress |
| 🟢 绿色系 | 低优先级、完成、新增 | low, ready-for-work, feature |
| 🔵 蓝色系 | 信息、文档 | docs, platform |
| 🟣 紫色系 | 重构、特殊任务 | refactor, brain, good first issue |
| ⚪ 灰色系 | 无效、过期 | stale, wontfix, duplicate |

### 2. 对比度与可读性

- **高对比度**: critical, high priority（深色背景 + 白色文字）
- **中等对比度**: medium, low priority（浅色背景 + 深色文字）
- **GitHub 默认**: 文字颜色自动根据背景色调整（黑/白）

### 3. 避免颜色冲突

- 同一分类的标签使用相近颜色
- 不同分类的标签使用明显不同的颜色
- 避免使用过多灰色（`#ededed`），在 GitHub UI 中几乎不可见

---

## 工具与脚本

### 1. 标签管理脚本

```bash
# 预览变更
python scripts/tools/manage-labels.py --dry-run

# 应用变更
python scripts/tools/manage-labels.py --apply

# 跳过删除旧标签
python scripts/tools/manage-labels.py --apply --skip-delete
```

### 2. GitHub Actions 自动化

创建 `.github/workflows/issue-triage.yml`:

```yaml
name: Issue Triage
on:
  issues:
    types: [opened, edited]

jobs:
  auto-label:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/labeler@v5
        with:
          repo-token: ${{ secrets.GITHUB_TOKEN }}
```

---

## 最佳实践

### 1. 标签组合示例

**高优先级 Bug**:
```
type/bug + priority/critical + size/medium + status/ready-for-work + area/engine
```

**新功能请求**:
```
type/feature + priority/high + size/large + status/needs-triage + platform/slack + area/adapter
```

**文档改进**:
```
type/docs + priority/low + size/small + status/ready-for-work
```

### 2. 避免过度标注

- ❌ 不要同时使用多个 **Priority** 标签
- ❌ 不要同时使用多个 **Type** 标签
- ❌ 不要同时使用多个 **Status** 标签
- ✅ 可以使用多个 **Area** 或 **Platform** 标签

### 3. 标签维护

- 定期清理 `status/stale` issues
- 关闭时移除标签（GitHub 自动处理）
- 定期审核标签体系（每季度）

---

## 参考资料

- **Kubernetes Labels**: https://github.com/kubernetes/kubernetes/labels
- **React Labels**: https://github.com/facebook/react/labels
- **VS Code Issue Triage**: https://github.com/microsoft/vscode/wiki/Automated-Issue-Triaging
- **GitHub Actions Labeler**: https://github.com/actions/labeler
- **GitHub Actions Stale**: https://github.com/actions/stale

---

**维护者**: HotPlex Team
**反馈**: 如有建议或问题,请创建 issue 并标注 `type/docs`
