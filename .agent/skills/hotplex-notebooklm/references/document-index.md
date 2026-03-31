# HotPlex 文档索引

本文件是 `hotplex-notebooklm` skill 的完整文档清单，用于文档同步时的源文件列表。

> **注意**：只上传英文版本，排除 CHANGELOG 和 docs-site。

---

## 优先级分类

### P1 - 项目核心文档

| 文件 | 描述性名称 |
|------|-----------|
| `README.md` | HotPlex 项目介绍与快速开始 |
| `CLAUDE.md` | HotPlex AI Agent 协作规范 |
| `INSTALL.md` | HotPlex 安装指南 |
| `CONTRIBUTING.md` | HotPlex 贡献指南 |

### P2 - 开发规范

| 文件 | 描述性名称 |
|------|-----------|
| `.agent/rules/uber-go-style-guide.md` | HotPlex Uber Go 编码规范 |
| `.agent/rules/git-workflow.md` | HotPlex Git 工作流规范 |
| `.agent/rules/chatapps-sdk-first.md` | HotPlex ChatApps SDK 开发规范 |

### P3 - 核心模块

| 文件 | 描述性名称 |
|------|-----------|
| `brain/README.md` | Native Brain 编排 |
| `cache/README.md` | 缓存层 |
| `engine/README.md` | 执行引擎 |
| `event/README.md` | 事件系统 |
| `types/README.md` | 类型定义 |
| `provider/README.md` | AI Provider 集成 |
| `internal/README.md` | 内部组件 |

### P4 - ChatApps

| 文件 | 描述性名称 |
|------|-----------|
| `chatapps/README.md` | ChatApps 核心 |
| `chatapps/slack/README.md` | Slack 集成 |
| `chatapps/feishu/README.md` | 飞书集成 |
| `chatapps/dedup/README.md` | 去重模块 |

### P5 - Storage & SDKs

| 文件 | 描述性名称 |
|------|-----------|
| `plugins/storage/README.md` | Storage 插件 |
| `sdks/README.md` | SDK 概览 |
| `sdks/python/README.md` | Python SDK |
| `sdks/typescript/README.md` | TypeScript SDK |

### P6 - Scripts & Docker

| 文件 | 描述性名称 |
|------|-----------|
| `scripts/README.md` | Scripts 脚本 |
| `docker/README.md` | Docker 部署 |
| `docker/matrix/README.md` | Matrix 多容器部署 |

### P7 - 设计文档

| 文件 | 描述性名称 |
|------|-----------|
| `docs/design/logging-package-design.md` | 日志包设计方案 |
| `docs/design/deterministic-session-id.md` | 确定性会话 ID 设计 |
| `docs/design/bot-behavior-spec.md` | Bot 行为规范 |
| `docs/architecture.md` | 架构概述 |
| `docs/development.md` | 开发指南 |
| `docs/production-guide.md` | 生产部署指南 |
| `docs/configuration.md` | 配置指南 |
| `docs/hooks-architecture.md` | Hooks 架构 |
| `docs/sdk-guide.md` | SDK 指南 |
| `docs/provider-extension-guide.md` | Provider 扩展指南 |

### P8 - Provider 文档

| 文件 | 描述性名称 |
|------|-----------|
| `docs/providers/claudecode.md` | Claude Code Provider 集成 |
| `docs/providers/opencode.md` | OpenCode Provider 集成 |
| `docs/providers/pi.md` | PI Provider 集成 |

### P9 - ChatApps 文档

| 文件 | 描述性名称 |
|------|-----------|
| `docs/chatapps/chatapps-architecture.md` | ChatApps 架构 |
| `docs/chatapps/chatapps-slack-architecture.md` | Slack 架构 |

### P10 - 验证报告

| 文件 | 描述性名称 |
|------|-----------|
| `docs/verification/engine-event-verification-report.md` | Engine 事件验证报告 |
| `docs/verification/slack-ux-verification-report.md` | Slack UX 验证报告 |

### P11 - Server API

| 文件 | 描述性名称 |
|------|-----------|
| `docs/server/api.md` | API 参考 |

---

## 增量同步规则

使用 `git diff` 检测变更：

```bash
LAST_COMMIT="<上次同步的 commit SHA>"
git diff --name-only $LAST_COMMIT HEAD \
  -- "*.md" \
  | grep -v "_zh.md" \
  | grep -v "docs-site" \
  | grep -v "CHANGELOG"
```

**上传要求**：
- 只上传英文版本（排除 `*_zh.md`）
- 排除 `docs-site/`（NotebookLM 不适合生成文档站点内容）
- 排除 `CHANGELOG`（变更日志不适合作为知识源）
