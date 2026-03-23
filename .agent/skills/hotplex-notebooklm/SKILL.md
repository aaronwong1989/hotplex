---
name: hotplex-notebooklm
description: HotPlex 项目 NotebookLM 智能助手。统一 HotPlex 品牌风格调用 NotebookLM，支持文档同步、知识查询、信息图生成、PPT 生成四大功能。**仅在同时提到 HotPlex 和 NotebookLM 时触发**（例如 "用 NotebookLM 查 HotPlex 文档"、"从 NotebookLM 生成 HotPlex 信息图"、"基于 HotPlex 知识库创建 PPT"）。否则使用用户级的 notebooklm skill。所有 NotebookLM 操作委托给 notebooklm skill 执行。
---

# HotPlex NotebookLM 智能助手

HotPlex 项目的 NotebookLM 集成工具，提供统一的品牌风格和四大核心功能：

1. **文档同步** - 扫描源码文档并上传到 NotebookLM（自己实现）
2. **知识查询** - 查询 HotPlex 技术知识库（委托 notebooklm skill）
3. **信息图生成** - 生成符合 HotPlex 品牌风格的技术信息图（委托 notebooklm skill）
4. **PPT 生成** - 生成符合 HotPlex 品牌风格的技术演示 PPT（委托 notebooklm skill）

---

## 核心架构

```
hotplex-notebooklm (HotPlex 专用)
├── 📄 文档同步 (自己实现)
├── 🎨 HotPlex 品牌规范 (定义标准)
└── 🔗 NotebookLM 操作
    └── 委托 notebooklm skill (传递品牌规范)
```

**职责分工**：
- **hotplex-notebooklm**: HotPlex 品牌规范 + 文档同步 + 协调调用
- **notebooklm skill**: 实际操作 NotebookLM CLI（信息图、PPT、查询）

---

## HotPlex 品牌规范（传递给 notebooklm skill）

所有生成的内容必须遵循以下品牌规范。当调用 notebooklm skill 时，**必须传递这些品牌参数**：

### 品牌元素

| 元素 | 值 | 用途 |
|------|-----|------|
| **品牌色** | `#00ADD8` | 标题、重点、链接、主色调 |
| **辅助色** | `#333333` | 深灰文字 |
| **背景色** | `#F5F5F5` | 浅灰背景 |
| **强调色** | `#00ADD8` | HotPlex 青色 |
| **字体** | `Inter, system-ui` | 标题和正文 |
| **Logo** | HotPlex Logo | 青色 lightning bolt |
| **标语** | "Strategic Bridge for AI Agent Engineering" | 用于封面或结尾 |

### 内容风格

- **技术术语**：保留英文原名（Engine, SessionPool, Provider 等）
- **代码示例**：使用 Go 语法高亮，符合 Uber Go Style Guide
- **架构图**：组件化表示，标注数据流
- **示例场景**：优先使用 Slack、Feishu 等实际集成场景

### 品牌配置 JSON

```json
{
  "hotplex_brand": {
    "colors": {
      "primary": "#00ADD8",
      "text": "#333333",
      "background": "#F5F5F5",
      "accent": "#00ADD8"
    },
    "fonts": {
      "heading": "Inter",
      "body": "system-ui"
    },
    "logo": "HotPlex Logo (lightning bolt)",
    "tagline": "Strategic Bridge for AI Agent Engineering",
    "style": {
      "terminology": "English technical terms",
      "code_highlight": "Go syntax",
      "examples": ["Slack", "Feishu", "DingTalk"]
    }
  }
}
```

---

## 功能模块

### 模块 1：文档同步（自己实现）

扫描 HotPlex 源码目录的高价值文档并上传到 NotebookLM。

#### 状态管理

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

#### 高价值文档清单

| 文件 | 描述性名称 |
|------|-----------|
| `README.md` | HotPlex 项目介绍与快速开始 |
| `CLAUDE.md` | HotPlex AI Agent 协作规范 |
| `.agent/rules/uber-go-style-guide.md` | HotPlex Uber Go 编码规范 |
| `.agent/rules/git-workflow.md` | HotPlex Git 工作流规范 |
| `.agent/rules/chatapps-sdk-first.md` | HotPlex ChatApps SDK 开发规范 |
| `brain/README.md` | HotPlex Brain 模块 - Native Brain 编排 |
| `engine/README.md` | HotPlex Engine 模块 - 执行引擎 |
| `chatapps/README.md` | HotPlex ChatApps 核心 |
| `chatapps/slack/README.md` | HotPlex Slack 集成 |
| `provider/README.md` | HotPlex Provider 模块 - AI Provider 集成 |

完整清单见附录 A。

#### 增量同步工作流

```bash
# 1. 检查当前状态
cat ~/.hotplex-notebooklm/state.json | jq '{notebook_title, last_sync_commit, source_count: (.sources | length)}'

# 2. 检测 Git 变更
LAST_COMMIT=$(jq -r '.last_sync_commit // empty' ~/.hotplex-notebooklm/state.json)
if [ "$LAST_COMMIT" = "null" ] || [ -z "$LAST_COMMIT" ]; then
  echo "首次同步"
else
  git diff --name-only $LAST_COMMIT HEAD -- "*.md" | grep -v "_zh.md" | grep -v "docs-site" | grep -v "CHANGELOG"
fi

# 3. 上传文档（使用 notebooklm skill）
# 调用 notebooklm skill 的 add source 功能
# 传递参数：
#   - notebook_id: HotPlex notebook ID
#   - file_path: 临时文件路径
#   - display_name: 描述性名称

# 4. 更新状态
new_commit=$(git rev-parse --short HEAD)
jq ".last_sync_commit = \"$new_commit\"" ~/.hotplex-notebooklm/state.json > tmp.json && mv tmp.json ~/.hotplex-notebooklm/state.json
jq ".last_sync_time = \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"" ~/.hotplex-notebooklm/state.json > tmp.json && mv tmp.json ~/.hotplex-notebooklm/state.json
```

---

### 模块 2：知识查询（委托 notebooklm skill）

使用 notebooklm skill 查询 HotPlex 技术知识库。

#### 工作流

1. **检查 notebooklm skill 认证状态**
2. **调用 notebooklm skill 查询**，传递参数：
   - `notebook_id`: HotPlex notebook ID
   - `question`: 用户问题
   - `brand_context`: HotPlex 品牌规范（可选，用于格式化答案）

#### 调用示例

```bash
# 使用 Skill tool 调用 notebooklm skill
# 在 Claude Code 中：

# 1. 查询架构
"Use notebooklm skill to ask: Explain HotPlex SessionPool state management and PGID isolation mechanism. Use HotPlex notebook ID: <notebook-id>"

# 2. 查询集成
"Use notebooklm skill to ask: How does HotPlex Slack adapter handle message deduplication? Use HotPlex notebook ID: <notebook-id>"

# 3. 查询安全机制
"Use notebooklm skill to ask: Explain HotPlex WAF regex detector and capability governance. Use HotPlex notebook ID: <notebook-id>"
```

**重要**：每个问题都是独立的会话，需要在问题中包含足够的上下文。

---

### 模块 3：信息图生成（委托 notebooklm skill）

基于 NotebookLM 查询结果，使用 notebooklm skill 生成符合 HotPlex 品牌风格的技术信息图。

#### 工作流

```
用户请求 → 调用 notebooklm skill 生成信息图 → 传递品牌规范 → 返回结果
```

#### 调用步骤

1. **准备查询内容**：明确信息图主题（如 "HotPlex SessionPool 架构"）

2. **调用 notebooklm skill**，传递参数：
   - `notebook_id`: HotPlex notebook ID
   - `action`: "create_infographic"（或 notebooklm skill 定义的信息图生成命令）
   - `topic`: 信息图主题
   - `brand_config`: HotPlex 品牌配置 JSON（见上文）

3. **等待 notebooklm skill 生成结果**

4. **验证品牌一致性**：
   - ✅ 主色调使用 `#00ADD8`
   - ✅ 文字使用 `#333333` 或 `#fff`
   - ✅ 包含 HotPlex 标语或 Logo（如适用）
   - ✅ 技术术语保留英文原名

#### 调用示例

```bash
# 使用 Skill tool 调用 notebooklm skill

# 示例 1：生成 SessionPool 架构信息图
"Use notebooklm skill to create an infographic about HotPlex SessionPool architecture.
- Notebook ID: <hotplex-notebook-id>
- Topic: SessionPool state management, PGID isolation, and GC mechanism
- Brand config: {colors: {primary: '#00ADD8', text: '#333333', background: '#F5F5F5'}, tagline: 'Strategic Bridge for AI Agent Engineering'}"

# 示例 2：生成 ChatApps 集成信息图
"Use notebooklm skill to create an infographic about HotPlex ChatApps integration.
- Notebook ID: <hotplex-notebook-id>
- Topic: Slack and Feishu adapter architecture, message handling flow
- Brand config: {colors: {primary: '#00ADD8'}, style: {examples: ['Slack', 'Feishu']}}"
```

#### 常见信息图类型

| 类型 | 适用场景 | 示例主题 |
|------|----------|---------|
| **架构图** | 系统组件关系 | "HotPlex Engine Pool Architecture" |
| **流程图** | 请求处理流程 | "HotPlex Message Routing Flow" |
| **时序图** | 组件交互 | "HotPlex WebSocket Session Lifecycle" |
| **组件图** | 模块依赖 | "HotPlex ChatApps Adapter Dependencies" |

---

### 模块 4：PPT 生成（委托 notebooklm skill）

基于 NotebookLM 查询结果，使用 notebooklm skill 生成符合 HotPlex 品牌风格的技术演示 PPT。

#### 工作流

```
用户请求 → 调用 notebooklm skill 生成 PPT → 传递品牌规范 → 返回结果
```

#### 调用步骤

1. **准备 PPT 主题**：明确演示内容（如 "HotPlex 架构概览"）

2. **调用 notebooklm skill**，传递参数：
   - `notebook_id`: HotPlex notebook ID
   - `action`: "create_presentation"（或 notebooklm skill 定义的 PPT 生成命令）
   - `topic`: PPT 主题
   - `outline`: 可选，PPT 大纲（如需要）
   - `brand_config`: HotPlex 品牌配置 JSON（见上文）

3. **等待 notebooklm skill 生成结果**

4. **验证品牌一致性**：
   - ✅ 标题使用 `#00ADD8` 颜色
   - ✅ 代码使用 Go 语法高亮
   - ✅ 包含 HotPlex Logo 和标语
   - ✅ 表格和图表使用品牌样式
   - ✅ 最后一页包含标语 "Strategic Bridge for AI Agent Engineering"

#### 调用示例

```bash
# 使用 Skill tool 调用 notebooklm skill

# 示例 1：生成架构概览 PPT
"Use notebooklm skill to create a presentation about HotPlex architecture overview.
- Notebook ID: <hotplex-notebook-id>
- Topic: HotPlex system architecture, including Gateway Layer, Engine Layer, and Adapter Layer
- Brand config: {
    colors: {primary: '#00ADD8', text: '#333333'},
    fonts: {heading: 'Inter', body: 'system-ui'},
    tagline: 'Strategic Bridge for AI Agent Engineering',
    style: {code_highlight: 'Go syntax'}
  }"

# 示例 2：生成 ChatApps 集成 PPT
"Use notebooklm skill to create a presentation about HotPlex ChatApps integration.
- Notebook ID: <hotplex-notebook-id>
- Topic: Slack and Feishu adapter implementation, SDK usage patterns, and message handling
- Brand config: {
    colors: {primary: '#00ADD8'},
    style: {examples: ['Slack', 'Feishu']}
  }"
```

#### 常见 PPT 主题

| 主题 | 内容建议 | 目标受众 |
|------|----------|---------|
| **架构概览** | 系统组件、数据流、安全机制 | 技术团队、架构师 |
| **快速开始** | 安装、配置、Hello World 示例 | 新用户、开发者 |
| **ChatApps 集成** | Slack/Feishu 适配器、SDK 使用 | 集成开发者 |
| **安全机制** | WAF、PGID、Capability Governance | 安全团队、运维 |
| **性能优化** | I/O 复用、零拷贝、会话 GC | 性能工程师 |

---

## 完整工作流示例

### 示例 1：生成 HotPlex SessionPool 信息图

**用户请求**："用 NotebookLM 生成 HotPlex SessionPool 架构信息图"

**Skill 执行流程**：

1. **触发 hotplex-notebooklm skill**（同时提到 HotPlex 和 NotebookLM）

2. **调用 notebooklm skill**：
   ```
   Use notebooklm skill to create an infographic about HotPlex SessionPool.
   - Notebook ID: <hotplex-notebook-id>
   - Topic: SessionPool state management, PGID isolation, and GC mechanism
   - Brand config: {
       colors: {primary: '#00ADD8', text: '#333333', background: '#F5F5F5'},
       tagline: 'Strategic Bridge for AI Agent Engineering'
     }
   ```

3. **notebooklm skill 执行**：
   - 查询 NotebookLM 获取 SessionPool 相关信息
   - 生成信息图
   - 应用品牌样式

4. **返回结果**给用户

### 示例 2：生成 HotPlex ChatApps 集成 PPT

**用户请求**："从 NotebookLM 创建 HotPlex ChatApps 集成演示 PPT"

**Skill 执行流程**：

1. **触发 hotplex-notebooklm skill**

2. **调用 notebooklm skill**：
   ```
   Use notebooklm skill to create a presentation about HotPlex ChatApps integration.
   - Notebook ID: <hotplex-notebook-id>
   - Topic: Slack and Feishu adapter architecture, SDK patterns, and message handling
   - Brand config: {
       colors: {primary: '#00ADD8', text: '#333333'},
       fonts: {heading: 'Inter', body: 'system-ui'},
       tagline: 'Strategic Bridge for AI Agent Engineering',
       style: {code_highlight: 'Go syntax', examples: ['Slack', 'Feishu']}
     }
   ```

3. **notebooklm skill 执行**：
   - 查询 NotebookLM 获取 ChatApps 相关信息
   - 生成 PPT
   - 应用品牌样式

4. **返回结果**给用户

---

## 品牌检查清单

生成内容前，确保符合以下规范：

### 信息图

- [ ] 主色调使用 `#00ADD8`
- [ ] 文字使用 `#333333` 或 `#fff`（深色背景）
- [ ] 标题使用粗体
- [ ] 图表清晰标注组件名称
- [ ] 数据流方向明确（从上到下或从左到右）
- [ ] 包含 HotPlex Logo 或标语（可选）

### PPT

- [ ] 标题使用 `#00ADD8` 颜色
- [ ] 代码使用 Go 语法高亮
- [ ] 图表使用品牌配色
- [ ] 表格使用品牌样式
- [ ] 最后一页包含 "Strategic Bridge for AI Agent Engineering" 标语

---

## 故障排除

| 问题 | 解决方案 |
|------|----------|
| notebooklm skill 未认证 | 先调用 notebooklm skill 进行认证 |
| NotebookLM 找不到 HotPlex notebook | 运行文档同步（模块 1） |
| 品牌样式未应用 | 检查 brand_config 参数是否正确传递 |
| notebooklm skill 生成失败 | 检查 notebooklm skill 错误信息，可能需要重新认证 |

---

## 与 notebooklm skill 的协作

### notebooklm skill 的职责

notebooklm skill 是用户级的 NotebookLM CLI 操作手册，负责：

1. **认证管理**：Google 账号登录、状态检查
2. **Notebook 管理**：创建、查询、激活 notebook
3. **知识查询**：向 NotebookLM 提问
4. **信息图生成**：通过 NotebookLM 创建信息图
5. **PPT 生成**：通过 NotebookLM 创建演示文稿

### hotplex-notebooklm skill 的职责

1. **文档同步**：扫描并上传 HotPlex 文档
2. **品牌规范**：定义和管理 HotPlex 品牌标准
3. **协调调用**：委托 notebooklm skill 执行 NotebookLM 操作
4. **结果验证**：确保生成内容符合品牌规范

### 调用模式

```
用户 → hotplex-notebooklm skill
         ↓ (传递品牌规范)
         → notebooklm skill
              ↓ (操作 NotebookLM CLI)
              → NotebookLM 服务
```

---

## 附录 A：完整文档清单

### 1. 项目根目录

| 文件 | 描述性名称 |
|------|-----------|
| `README.md` | HotPlex 项目介绍与快速开始 |
| `CLAUDE.md` | HotPlex AI Agent 协作规范 |
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

---

## 附录 B：notebooklm skill 调用参考

**详细 API 参考**：查看 notebooklm skill 的完整文档。

**常用命令**（通过 Skill tool 调用）：

```bash
# 检查认证状态
"Use notebooklm skill to check authentication status"

# 认证（浏览器可见）
"Use notebooklm skill to setup authentication (show browser)"

# 列出笔记本
"Use notebooklm skill to list all notebooks"

# 查询问题
"Use notebooklm skill to ask a question:
- Notebook ID: <id>
- Question: <your question>"

# 创建信息图
"Use notebooklm skill to create an infographic:
- Notebook ID: <id>
- Topic: <topic>
- Brand config: <hotplex-brand-config>"

# 创建 PPT
"Use notebooklm skill to create a presentation:
- Notebook ID: <id>
- Topic: <topic>
- Brand config: <hotplex-brand-config>"
```

**重要**：始终通过 Skill tool 调用 notebooklm skill，不要直接执行 notebooklm CLI 命令。
