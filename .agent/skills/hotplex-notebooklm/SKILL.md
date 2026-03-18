---
name: hotplex-notebooklm
description: HotPlex 项目 NotebookLM 同步工具。扫描源码目录高价值文档（各级源码包 README）、基于内容生成描述性名称、创建临时副本上传。排除 docs-site 和 CHANGELOG。支持增量维护，记录同步状态和 git commit。核心操作使用原版 notebooklm skill。
---

# HotPlex NotebookLM 同步

HotPlex 项目文档同步到 NotebookLM。排除 docs-site，只同步源码目录的高价值文档。

## 状态管理

状态文件: `~/.hotplex-notebooklm/state.json`

```json
{
  "notebook_id": "c326b88e-bcca-4e80-bc61-477d65995833",
  "notebook_title": "HotPlex Docs v2",
  "last_sync_commit": "86bf1b5",
  "last_sync_time": "2026-03-14T08:30:00Z",
  "sources": {}
}
```

**注意：中英文双语文档只上传英文版本。排除 CHANGELOG。**

## 高价值文档清单

### 1. 项目根目录

| 文件 | 描述性名称 |
|------|-----------|
| `README.md` | HotPlex 项目介绍与快速开始 |
| `AGENT.md` | HotPlex AI Agent 协作规范 |
| `INSTALL.md` | HotPlex 安装指南 |
| `CONTRIBUTING.md` | HotPlex 贡献指南 |

### 2. 开发规范 (.agent/rules/)

| 文件 | 描述性名称 |
|------|-----------|
| `uber-go-style-guide.md` | HotPlex Uber Go 编码规范 |
| `git-workflow.md` | HotPlex Git 工作流规范 |
| `chatapps-sdk-first.md` | HotPlex ChatApps SDK 开发规范 |

### 3. 核心模块 (源码包 README)

| 文件 | 描述性名称 |
|------|-----------|
| `brain/README.md` | HotPlex Brain 模块 - Native Brain 编排 |
| `cache/README.md` | HotPlex Cache 模块 - 缓存层 |
| `engine/README.md` | HotPlex Engine 模块 - 执行引擎 |
| `event/README.md` | HotPlex Event 模块 - 事件系统 |
| `types/README.md` | HotPlex Types 模块 - 类型定义 |
| `provider/README.md` | HotPlex Provider 模块 - AI Provider 集成 |
| `internal/README.md` | HotPlex Internal 模块 - 内部组件 |

### 4. ChatApps

| 文件 | 描述性名称 |
|------|-----------|
| `chatapps/README.md` | HotPlex ChatApps 核心 |
| `chatapps/slack/README.md` | HotPlex Slack 集成 |
| `chatapps/feishu/README.md` | HotPlex 飞书集成 |
| `chatapps/dedup/README.md` | HotPlex 去重模块 |

### 5. Storage

| 文件 | 描述性名称 |
|------|-----------|
| `plugins/storage/README.md` | HotPlex Storage 插件 |

### 6. SDKs

| 文件 | 描述性名称 |
|------|-----------|
| `sdks/README.md` | HotPlex SDK 概览 |
| `sdks/python/README.md` | HotPlex Python SDK |
| `sdks/typescript/README.md` | HotPlex TypeScript SDK |

### 7. Scripts & Docker

| 文件 | 描述性名称 |
|------|-----------|
| `scripts/README.md` | HotPlex Scripts 脚本 |
| `docker/README.md` | HotPlex Docker 部署 |
| `docker/matrix/README.md` | HotPlex Matrix 多容器部署 |

### 8. 设计文档 (docs/)

| 文件 | 描述性名称 |
|------|-----------|
| `docs/design/logging-package-design.md` | HotPlex 日志包设计方案 |
| `docs/design/deterministic-session-id.md` | HotPlex 确定性会话 ID 设计 |
| `docs/design/bot-behavior-spec.md` | HotPlex Bot 行为规范 |

### 9. Provider 文档 (docs/providers/)

| 文件 | 描述性名称 |
|------|-----------|
| `docs/providers/claudecode.md` | HotPlex Claude Code Provider 集成指南 |
| `docs/providers/opencode.md` | HotPlex OpenCode Provider 集成指南 |
| `docs/providers/pi.md` | HotPlex PI Provider 集成指南 |

### 10. ChatApps 文档 (docs/chatapps/)

| 文件 | 描述性名称 |
|------|-----------|
| `docs/chatapps/chatapps-architecture.md` | HotPlex ChatApps 架构 |
| `docs/chatapps/chatapps-slack-architecture.md` | HotPlex Slack 架构 |

### 11. 验证报告 (docs/verification/)

| 文件 | 描述性名称 |
|------|-----------|
| `docs/verification/engine-event-verification-report.md` | HotPlex Engine 事件验证报告 |
| `docs/verification/slack-ux-verification-report.md` | HotPlex Slack UX 验证报告 |

### 12. 开发指南 (docs/)

| 文件 | 描述性名称 |
|------|-----------|
| `docs/architecture.md` | HotPlex 架构概述 |
| `docs/development.md` | HotPlex 开发指南 |
| `docs/production-guide.md` | HotPlex 生产部署指南 |
| `docs/configuration.md` | HotPlex 配置指南 |
| `docs/hooks-architecture.md` | HotPlex Hooks 架构 |
| `docs/sdk-guide.md` | HotPlex SDK 指南 |
| `docs/provider-extension-guide.md` | HotPlex Provider 扩展指南 |

### 13. Server API (docs/server/)

| 文件 | 描述性名称 |
|------|-----------|
| `docs/server/api.md` | HotPlex API 参考 |

## 增量同步工作流

### 1. 检查当前状态

```bash
cat ~/.hotplex-notebooklm/state.json | jq '{notebook_title, last_sync_commit, source_count: (.sources | length)}'
```

### 2. 检测 Git 变更

```bash
LAST_COMMIT=$(jq -r '.last_sync_commit // empty' ~/.hotplex-notebooklm/state.json)

if [ "$LAST_COMMIT" = "null" ] || [ -z "$LAST_COMMIT" ]; then
  echo "首次同步"
else
  git diff --name-only $LAST_COMMIT HEAD -- "*.md" | grep -v "_zh.md" | grep -v "docs-site" | grep -v "CHANGELOG"
fi
```

### 3. 上传文档

```bash
cp <file> "/tmp/<描述性名称>.md"
notebooklm source add "/tmp/<描述性名称>.md"
rm "/tmp/<描述性名称>.md"
```

### 4. 更新状态

```bash
new_commit=$(git rev-parse --short HEAD)
jq ".last_sync_commit = \"$new_commit\"" ~/.hotplex-notebooklm/state.json > tmp.json && mv tmp.json ~/.hotplex-notebooklm/state.json
jq ".last_sync_time = \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"" ~/.hotplex-notebooklm/state.json > tmp.json && mv tmp.json ~/.hotplex-notebooklm/state.json
```

## 常用命令

```bash
# 状态
cat ~/.hotplex-notebooklm/state.json | jq .

# Git 变更
git diff --name-only <commit> HEAD -- "*.md" | grep -v "_zh.md" | grep -v "docs-site" | grep -v "CHANGELOG"

# HEAD commit
git rev-parse --short HEAD
```
