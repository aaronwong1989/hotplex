---
name: hotplex-notebooklm
description: HotPlex 项目 NotebookLM 智能助手。**永远使用固定的 "HotPlex Docs Hub" 专用笔记本**，统一 HotPlex 品牌风格，支持文档同步、知识查询、信息图 Prompt 生成、PPT Prompt 生成四大功能。**仅在同时提到 HotPlex 和 NotebookLM 时触发**（例如 "用 NotebookLM 查 HotPlex 文档"、"从 NotebookLM 生成 HotPlex 信息图 Prompt"、"基于 HotPlex 知识库创建 PPT Prompt"）。否则使用用户级的 notebooklm skill。文档同步和知识查询委托给 notebooklm skill 执行，使用固定的 "HotPlex Docs Hub" 笔记本。信息图和 PPT 功能生成用于 NotebookLM Web 界面的 Prompt 指令（包含品牌风格和内容指导）。
---

# HotPlex NotebookLM 智能助手

HotPlex 项目的 NotebookLM 集成工具，提供统一的品牌风格和四大核心功能：

1. **文档同步** - 扫描源码文档并上传到 NotebookLM（委托 notebooklm skill）
2. **知识查询** - 查询 HotPlex 技术知识库（委托 notebooklm skill）
3. **信息图 Prompt 生成** - 生成符合 HotPlex 品牌风格的信息图 Prompt 指令
4. **PPT Prompt 生成** - 生成符合 HotPlex 品牌风格的 PPT Prompt 指令

---

## 核心架构

```
hotplex-notebooklm (HotPlex 专用)
├── 📄 专用笔记本: "HotPlex Docs Hub" (固定)
├── 📄 文档同步 (委托 notebooklm skill)
├── 📄 知识查询 (委托 notebooklm skill)
├── 🎨 HotPlex 品牌规范 (定义标准)
└── ✍️ Prompt 生成 (信息图 + PPT)
    └── 生成文本指令供用户在 NotebookLM Web 界面使用
```

**核心原则**：
- **固定笔记本**：永远使用 **"HotPlex Docs Hub"** 专用笔记本，无需每次指定
- **职责分工**：
  - **hotplex-notebooklm**: HotPlex 品牌规范 + 文档同步协调 + Prompt 指令生成
  - **notebooklm skill**: NotebookLM CLI 操作（文档上传、知识查询）
  - **NotebookLM Web**: 信息图和 PPT 生成（用户手动操作）

---

## HotPlex 品牌规范

所有生成的内容必须遵循以下品牌规范。生成的 Prompt 指令将包含这些品牌要求：

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

### 模块 1：文档同步（委托 notebooklm skill）

扫描 HotPlex 源码目录的高价值文档并上传到 **"HotPlex Docs Hub"** 专用笔记本。

#### 专用笔记本

**名称**：`HotPlex Docs Hub`（固定）

**初始化逻辑**（首次使用时）：
1. 调用 notebooklm skill 列出所有笔记本
2. 查找名为 "HotPlex Docs Hub" 的笔记本
3. 如果存在，获取其 notebook_id
4. 如果不存在，创建新笔记本 "HotPlex Docs Hub"
5. 保存 notebook_id 到状态文件

```bash
# 检查是否已初始化
if [ ! -f ~/.hotplex-notebooklm/state.json ]; then
  # 初始化：查找或创建 "HotPlex Docs Hub" 笔记本
  # 使用 notebooklm skill 的 notebook_manager.py

  # 调用 notebooklm skill 查找笔记本
  # "Use notebooklm skill to list notebooks and find 'HotPlex Docs Hub'"

  # 如果找不到，创建
  # "Use notebooklm skill to create notebook 'HotPlex Docs Hub' with description 'HotPlex Project Documentation Hub'"

  # 保存 notebook_id
  mkdir -p ~/.hotplex-notebooklm
  echo '{"notebook_title": "HotPlex Docs Hub", "notebook_id": "<id>", "last_sync_commit": "", "sources": {}}' > ~/.hotplex-notebooklm/state.json
fi
```

#### 状态管理

状态文件: `~/.hotplex-notebooklm/state.json`

```json
{
  "notebook_id": "c326b88e-bcca-4e80-bc61-477d65995833",
  "notebook_title": "HotPlex Docs Hub",
  "last_sync_commit": "86bf1b5",
  "last_sync_time": "2026-03-14T08:30:00Z",
  "sources": {}
}
```

**重要字段**：
- `notebook_title`: 固定为 `"HotPlex Docs Hub"`
- `notebook_id`: 首次初始化时设置，之后不再改变
- `last_sync_commit`: 上次同步的 Git commit SHA

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

使用 notebooklm skill 查询 **"HotPlex Docs Hub"** 笔记本。

#### 工作流

1. **检查 notebooklm skill 认证状态**
2. **确保 HotPlex Docs Hub 笔记本已初始化**（如未初始化，自动创建）
3. **调用 notebooklm skill 查询**，传递参数：
   - `notebook_title`: `"HotPlex Docs Hub"`（固定）
   - `question`: 用户问题

#### 调用示例

```bash
# 使用 Skill tool 调用 notebooklm skill

# 1. 查询架构
"Use notebooklm skill to ask: Explain HotPlex SessionPool state management and PGID isolation mechanism.
Use notebook: HotPlex Docs Hub"

# 2. 查询集成
"Use notebooklm skill to ask: How does HotPlex Slack adapter handle message deduplication?
Use notebook: HotPlex Docs Hub"

# 3. 查询安全机制
"Use notebooklm skill to ask: Explain HotPlex WAF regex detector and capability governance.
Use notebook: HotPlex Docs Hub"
```

**重要**：
- 每个问题都是独立的会话，需要在问题中包含足够的上下文
- **永远使用 "HotPlex Docs Hub" 笔记本**，不需要指定 notebook_id

---

### 模块 3：信息图 Prompt 生成

生成用于 NotebookLM Web 界面的信息图 Prompt 指令，包含 HotPlex 品牌风格和内容要求。

#### 工作流

```
用户请求 → 分析主题 → 生成 Prompt 指令（品牌风格 + 内容结构）→ 返回文本供用户复制
```

#### 输出格式

生成的 Prompt 包含以下部分：

1. **主题描述**：清晰定义信息图的主题和目标
2. **品牌风格要求**：HotPlex 品牌色、字体、标语等
3. **内容结构**：需要包含的关键信息点
4. **视觉建议**：布局、数据流方向、组件表示方式
5. **技术术语说明**：保留英文原名的要求

#### Prompt 模板

```markdown
# NotebookLM 信息图 Prompt

## 主题
[信息图主题，如 "HotPlex SessionPool Architecture"]

## 品牌风格要求
- **主色调**: #00ADD8（用于标题、重点、主色调）
- **文字颜色**: #333333（正文）或 #fff（深色背景）
- **背景色**: #F5F5F5（浅灰背景）
- **字体**: Inter（标题）, system-ui（正文）
- **标语**: "Strategic Bridge for AI Agent Engineering"（可选，用于页眉/页脚）

## 内容要求
[具体内容要求，如:]
1. 展示 [组件A] → [组件B] → [组件C] 的数据流
2. 标注关键组件：[列出组件名]
3. 突出核心机制：[列出关键特性]
4. 示例场景：[具体集成场景]

## 视觉建议
- 使用**组件化框图**表示架构
- **数据流方向**：从上到下 或 从左到右
- 技术术语保留**英文原名**（Engine, SessionPool, Provider 等）
- 使用**箭头**标注数据流向
- 代码示例使用 **Go 语法高亮**风格

## 示例参考
[提供具体示例，如:]
- 标题使用 #00ADD8 颜色，粗体
- 组件框使用 #F5F5F5 背景，#00ADD8 边框
- 箭头使用 #00ADD8 颜色
```

#### 使用流程

1. **用户请求**：例如 "生成 HotPlex SessionPool 架构信息图 Prompt"
2. **Skill 生成 Prompt**：包含品牌风格 + 内容结构
3. **用户复制 Prompt**
4. **打开 NotebookLM Web 界面**：访问 https://notebooklm.google.com/
5. **选择 "HotPlex Docs Hub" 笔记本**
6. **使用信息图功能**：将 Prompt 粘贴到 NotebookLM 的信息图生成界面
7. **NotebookLM 生成信息图**：基于 Prompt 和笔记本中的文档内容

#### 调用示例

```bash
# 示例 1：生成 SessionPool 架构信息图 Prompt
# 用户请求: "用 NotebookLM 生成 HotPlex SessionPool 架构信息图"

# Skill 响应（生成以下 Prompt）:

# NotebookLM 信息图 Prompt

## 主题
HotPlex SessionPool Architecture - State Management, PGID Isolation, and GC Mechanism

## 品牌风格要求
- **主色调**: #00ADD8（用于标题、重点、主色调）
- **文字颜色**: #333333（正文）
- **背景色**: #F5F5F5（浅灰背景）
- **字体**: Inter（标题）, system-ui（正文）
- **标语**: "Strategic Bridge for AI Agent Engineering"（页脚）

## 内容要求
1. 展示 SessionPool 的三大核心组件：
   - Session Pool（会话池）
   - Session Manager（会话管理器）
   - GC Worker（垃圾回收工作器）
2. 标注关键机制：
   - PGID Isolation（进程组隔离）
   - State Persistence（状态持久化）
   - Automatic Cleanup（自动清理）
3. 数据流方向：Client Request → Session Pool → CLI Process → Response
4. 示例场景：Slack Bot 长连接会话管理

## 视觉建议
- 使用**三层架构图**：Gateway Layer → Engine Layer → CLI Layer
- SessionPool 使用**青色 (#00ADD8) 边框**，其他组件使用**灰色边框**
- **箭头**标注数据流：使用 #00ADD8 颜色
- 技术术语保留**英文原名**：SessionPool, PGID, GC Worker
- 代码示例使用 **Go 语法高亮**风格

# 示例 2：生成 ChatApps 集成信息图 Prompt
# 用户请求: "从 NotebookLM 创建 HotPlex ChatApps 集成信息图"

# Skill 响应（生成以下 Prompt）:

# NotebookLM 信息图 Prompt

## 主题
HotPlex ChatApps Integration - Adapter Architecture and Message Handling Flow

## 品牌风格要求
- **主色调**: #00ADD8（用于标题、重点、链接）
- **文字颜色**: #333333（正文）
- **背景色**: #F5F5F5（浅灰背景）
- **字体**: Inter（标题）, system-ui（正文）
- **标语**: "Strategic Bridge for AI Agent Engineering"

## 内容要求
1. 展示 ChatApps 三大适配器：
   - Slack Adapter（Slack 集成）
   - Feishu Adapter（飞书集成）
   - DingTalk Adapter（钉钉集成）
2. 标注消息处理流程：
   - Webhook 接收 → Event 解析 → Engine 路由 → CLI 执行 → Response 返回
3. 突出核心特性：
   - SDK-First 原则（优先使用官方 SDK）
   - Message Deduplication（消息去重）
   - Thread Support（线程支持）
4. 示例场景：Slack Socket Mode 实时通信

## 视觉建议
- 使用**组件依赖图**：展示 Base Layer → Adapter Layer → Platform SDK
- 适配器使用**青色 (#00ADD8) 高亮**
- **数据流箭头**使用 #00ADD8 颜色，从左到右
- 技术术语保留**英文原名**：Adapter, SDK, Webhook, Socket Mode
```

**重要**：
- 生成的 Prompt 是**纯文本**，供用户复制粘贴
- 用户需要在 **NotebookLM Web 界面**手动操作
- Prompt 包含完整的品牌风格说明

---

### 模块 4：PPT Prompt 生成

生成用于 NotebookLM Web 界面的 PPT Prompt 指令，包含 HotPlex 品牌风格和内容要求。

#### 工作流

```
用户请求 → 分析主题 → 生成 Prompt 指令（品牌风格 + 内容大纲）→ 返回文本供用户复制
```

#### 输出格式

生成的 Prompt 包含以下部分：

1. **主题描述**：PPT 的核心主题和目标
2. **品牌风格要求**：HotPlex 品牌色、字体、Logo、标语等
3. **内容大纲**：PPT 的章节结构和每页要点
4. **视觉建议**：配色方案、图表样式、代码展示方式
5. **技术术语说明**：保留英文原名的要求

#### Prompt 模板

```markdown
# NotebookLM PPT Prompt

## 主题
[PPT 主题，如 "HotPlex Architecture Overview"]

## 品牌风格要求
- **主色调**: #00ADD8（用于标题、重点、主色调）
- **文字颜色**: #333333（正文）
- **背景色**: #F5F5F5（浅灰背景）或 #FFFFFF（白色）
- **字体**: Inter（标题）, system-ui（正文）
- **Logo**: HotPlex Logo（青色 lightning bolt）- 首页或每页页眉
- **标语**: "Strategic Bridge for AI Agent Engineering" - 最后一页

## 内容大纲

### 第 1 页：标题页
- 主标题：[标题]
- 副标题：[副标题]
- Logo：HotPlex Logo（青色 lightning bolt）

### 第 2 页：[章节标题]
- [要点 1]
- [要点 2]
- [要点 3]

### 第 3 页：[章节标题]
- [要点 1]
- [要点 2]
- [要点 3]

...（根据内容调整页数）

### 最后一页：总结
- 核心价值：[核心价值主张]
- 标语：**"Strategic Bridge for AI Agent Engineering"**

## 视觉建议
- 标题使用 **#00ADD8 颜色**，粗体
- 代码示例使用 **Go 语法高亮**（蓝色关键字、绿色字符串）
- 表格使用**品牌配色**：表头 #00ADD8 背景，白色文字
- 图表使用**青色系配色**
- 技术术语保留**英文原名**（Engine, SessionPool, Provider 等）

## 示例场景
[提供具体示例场景，如:]
- Slack Bot 集成示例
- Feishu Webhook 配置
- DingTalk 消息处理
```

#### 使用流程

1. **用户请求**：例如 "生成 HotPlex 架构概览 PPT Prompt"
2. **Skill 生成 Prompt**：包含品牌风格 + 内容大纲
3. **用户复制 Prompt**
4. **打开 NotebookLM Web 界面**：访问 https://notebooklm.google.com/
5. **选择 "HotPlex Docs Hub" 笔记本**
6. **使用 PPT 功能**：将 Prompt 粘贴到 NotebookLM 的 PPT 生成界面
7. **NotebookLM 生成 PPT**：基于 Prompt 和笔记本中的文档内容

#### 调用示例

```bash
# 示例 1：生成架构概览 PPT Prompt
# 用户请求: "用 NotebookLM 创建 HotPlex 架构概览演示 PPT"

# Skill 响应（生成以下 Prompt）:

# NotebookLM PPT Prompt

## 主题
HotPlex Architecture Overview - Strategic Bridge for AI Agent Engineering

## 品牌风格要求
- **主色调**: #00ADD8（用于标题、重点、主色调）
- **文字颜色**: #333333（正文）
- **背景色**: #F5F5F5（浅灰背景）
- **字体**: Inter（标题）, system-ui（正文）
- **Logo**: HotPlex Logo（青色 lightning bolt）- 首页和每页页眉
- **标语**: "Strategic Bridge for AI Agent Engineering" - 最后一页

## 内容大纲

### 第 1 页：标题页
- 主标题：**HotPlex Architecture Overview**
- 副标题：Strategic Bridge for AI Agent Engineering
- Logo：HotPlex Logo（青色 lightning bolt）

### 第 2 页：HotPlex 核心概念
- **Cli-as-a-Service**：高性能 AI Agent 控制平面
- **Persistence**：长生命周期、隔离的进程会话
- **Tech Stack**：Go 1.26 | WebSocket Gateway | Regex WAF | PGID Isolation

### 第 3 页：三层架构
- **Gateway Layer**：WebSocket & HTTP Gateway（入口层）
- **Engine Layer**：Session Pool + I/O Multiplexer（核心层）
- **Adapter Layer**：ChatApps + Provider（适配层）

### 第 4 页：核心特性
- **Stateful Sessions**：确定性 Session ID，状态持久化
- **Security**：Regex WAF，Capability Governance，PGID Isolation
- **Performance**：Zero-copy I/O，I/O Multiplexing，Session GC

### 第 5 页：ChatApps 集成
- **支持平台**：Slack, Feishu, DingTalk
- **SDK-First**：优先使用官方 SDK
- **特性**：Message Deduplication, Thread Support, Chunking

### 第 6 页：快速开始
- **安装**：`make build && make run`
- **配置**：复制 `.env.example` 到 `.env`
- **Docker**：`make docker-build && make docker-up`

### 第 7 页：总结
- **核心价值**：High-performance, Secure, Stateful Agent Infrastructure
- **标语**：**Strategic Bridge for AI Agent Engineering**

## 视觉建议
- 标题使用 **#00ADD8 颜色**，粗体，Inter 字体
- 代码使用 **Go 语法高亮**（蓝色关键字、绿色字符串）
- 表格：表头使用 **#00ADD8 背景，白色文字**
- 架构图：使用**组件框图**，标注数据流
- 技术术语保留**英文原名**：SessionPool, PGID, WAF, GC

# 示例 2：生成 ChatApps 集成 PPT Prompt
# 用户请求: "从 NotebookLM 创建 HotPlex ChatApps 集成演示 PPT"

# Skill 响应（生成以下 Prompt）:

# NotebookLM PPT Prompt

## 主题
HotPlex ChatApps Integration - SDK-First Adapter Architecture

## 品牌风格要求
- **主色调**: #00ADD8（用于标题、重点、链接）
- **文字颜色**: #333333（正文）
- **背景色**: #FFFFFF（白色背景）
- **字体**: Inter（标题）, system-ui（正文）
- **Logo**: HotPlex Logo（首页和页眉）
- **标语**: "Strategic Bridge for AI Agent Engineering"（最后一页）

## 内容大纲

### 第 1 页：标题页
- 主标题：**HotPlex ChatApps Integration**
- 副标题：SDK-First Adapter Architecture
- Logo：HotPlex Logo

### 第 2 页：ChatApps 概览
- **定义**：ChatApps 是 HotPlex 的聊天平台集成层
- **目标**：将 Slack/Feishu/DingTalk 等平台连接到 HotPlex Engine
- **原则**：SDK-First（优先使用官方 SDK）

### 第 3 页：Slack 集成
- **Socket Mode**：实时 WebSocket 通信
- **Block Kit**：使用 slack-go SDK 构建消息
- **特性**：Message Threading, Reaction Support, Ephemeral Messages

### 第 4 页：Feishu 集成
- **Webhook**：事件订阅和消息推送
- **Card Message**：富文本卡片消息
- **特性**：Multi-tenant Support, Event Encryption

### 第 5 页：DingTalk 集成
- **Stream Mode**：流式消息推送
- **Markdown Support**：富文本消息
- **特性**：Department Management, Approval Workflow

### 第 6 页：消息处理流程
```
Webhook 接收 → Event 解析 → Engine 路由 → CLI 执行 → Response 返回
```
- **Message Deduplication**：基于 message_id 去重
- **Chunking**：长消息自动分块（4000 字符限制）
- **Threading**：线程消息支持

### 第 7 页：开发规范
- ✅ **使用官方 SDK**：slack-go, lark-sdk, dingtalk-sdk
- ✅ **避免重复造轮子**：Rate Limiting, Signature Verification 使用 SDK 内置
- ✅ **Adapter 职责清晰**：只做平台特定转换，不实现通用逻辑

### 第 8 页：示例场景
- **Slack Bot**：AI 对话机器人，长连接会话管理
- **Feishu Approval**：审批流程自动化
- **DingTalk Notification**：运维告警通知

### 第 9 页：总结
- **核心价值**：Unified Interface, SDK-First, High Performance
- **标语**：**Strategic Bridge for AI Agent Engineering**

## 视觉建议
- 标题使用 **#00ADD8 颜色**，粗体
- 代码使用 **Go 语法高亮**
- 平台 Logo：Slack（紫色）、Feishu（蓝色）、DingTalk（蓝色）
- 架构图：使用**组件依赖图**，标注 SDK 调用
- 技术术语保留**英文原名**：Socket Mode, Block Kit, Webhook
```

**重要**：
- 生成的 Prompt 是**纯文本**，供用户复制粘贴
- 用户需要在 **NotebookLM Web 界面**手动操作
- Prompt 包含完整的品牌风格说明和内容大纲

---

## 完整工作流示例

### 示例 1：生成 HotPlex SessionPool 信息图

**用户请求**："用 NotebookLM 生成 HotPlex SessionPool 架构信息图"

**Skill 执行流程**：

1. **触发 hotplex-notebooklm skill**（同时提到 HotPlex 和 NotebookLM）
2. **生成信息图 Prompt**：
   ```markdown
   # NotebookLM 信息图 Prompt

   ## 主题
   HotPlex SessionPool Architecture - State Management, PGID Isolation, and GC Mechanism

   ## 品牌风格要求
   - **主色调**: #00ADD8
   - **文字颜色**: #333333
   - **背景色**: #F5F5F5
   - **字体**: Inter, system-ui
   - **标语**: "Strategic Bridge for AI Agent Engineering"

   ## 内容要求
   1. 展示 SessionPool 的三大核心组件
   2. 标注关键机制：PGID Isolation, State Persistence, Automatic Cleanup
   3. 数据流方向：Client Request → Session Pool → CLI Process → Response

   ## 视觉建议
   - 使用三层架构图
   - SessionPool 使用青色边框
   - 技术术语保留英文原名
   ```
3. **返回 Prompt 给用户**：用户复制并粘贴到 NotebookLM Web 界面
4. **用户在 NotebookLM 操作**：打开 "HotPlex Docs Hub" → 使用信息图功能 → 粘贴 Prompt → 生成信息图

### 示例 2：生成 HotPlex ChatApps 集成 PPT

**用户请求**："从 NotebookLM 创建 HotPlex ChatApps 集成演示 PPT"

**Skill 执行流程**：

1. **触发 hotplex-notebooklm skill**
2. **生成 PPT Prompt**：
   ```markdown
   # NotebookLM PPT Prompt

   ## 主题
   HotPlex ChatApps Integration - SDK-First Adapter Architecture

   ## 品牌风格要求
   - **主色调**: #00ADD8
   - **文字颜色**: #333333
   - **背景色**: #FFFFFF
   - **字体**: Inter, system-ui
   - **Logo**: HotPlex Logo
   - **标语**: "Strategic Bridge for AI Agent Engineering"

   ## 内容大纲
   ### 第 1 页：标题页
   ### 第 2 页：ChatApps 概览
   ### 第 3 页：Slack 集成
   ...（共 9 页）

   ## 视觉建议
   - 标题使用 #00ADD8 颜色，粗体
   - 代码使用 Go 语法高亮
   - 技术术语保留英文原名
   ```
3. **返回 Prompt 给用户**：用户复制并粘贴到 NotebookLM Web 界面
4. **用户在 NotebookLM 操作**：打开 "HotPlex Docs Hub" → 使用 PPT 功能 → 粘贴 Prompt → 生成 PPT

---

## 品牌检查清单

生成 Prompt 前，确保包含以下品牌要求：

### 信息图 Prompt

- [ ] 包含品牌色 `#00ADD8`
- [ ] 包含文字颜色 `#333333` 和 `#fff`
- [ ] 包含背景色 `#F5F5F5`
- [ ] 包含字体要求 `Inter, system-ui`
- [ ] 包含标语（可选）
- [ ] 包含技术术语保留英文原名的说明
- [ ] 包含数据流方向建议
- [ ] 包含组件化表示建议

### PPT Prompt

- [ ] 包含品牌色 `#00ADD8`
- [ ] 包含文字颜色 `#333333`
- [ ] 包含背景色 `#F5F5F5` 或 `#FFFFFF`
- [ ] 包含字体要求 `Inter, system-ui`
- [ ] 包含 Logo 要求（首页和页眉）
- [ ] 包含标语（最后一页）
- [ ] 包含代码语法高亮要求（Go syntax）
- [ ] 包含技术术语保留英文原名的说明
- [ ] 包含内容大纲（每页标题和要点）

---

## 故障排除

| 问题 | 解决方案 |
|------|----------|
| notebooklm skill 未认证 | 先调用 notebooklm skill 进行认证 |
| NotebookLM 找不到 HotPlex notebook | 运行文档同步（模块 1） |
| Prompt 不完整 | 检查是否包含品牌风格 + 内容要求 |
| NotebookLM 生成结果不符合品牌风格 | 检查 Prompt 中的品牌要求是否完整 |
| NotebookLM 生成的信息图缺少组件 | 检查 Prompt 的"内容要求"部分是否完整 |

---

## 与 notebooklm skill 的协作

### notebooklm skill 的职责

notebooklm skill 是用户级的 NotebookLM CLI 操作手册，负责：

1. **认证管理**：Google 账号登录、状态检查
2. **Notebook 管理**：创建、查询、激活 notebook
3. **知识查询**：向 NotebookLM 提问
4. **文档上传**：上传源文档到笔记本

**不支持**：
- ❌ 信息图生成（NotebookLM Web 界面功能）
- ❌ PPT 生成（NotebookLM Web 界面功能）

### hotplex-notebooklm skill 的职责

1. **文档同步协调**：委托 notebooklm skill 上传文档
2. **知识查询协调**：委托 notebooklm skill 查询知识库
3. **品牌规范**：定义和管理 HotPlex 品牌标准
4. **Prompt 生成**：生成信息图和 PPT 的 Prompt 指令
5. **结果验证**：确保 Prompt 包含完整的品牌要求

### 协作模式

```
用户 → hotplex-notebooklm skill
         │
         ├─ 文档同步/知识查询 → notebooklm skill → NotebookLM API
         │
         └─ 信息图/PPT → 生成 Prompt → 用户复制 → NotebookLM Web 界面
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

# 查询问题（使用 HotPlex Docs Hub 笔记本）
"Use notebooklm skill to ask a question:
- Notebook: HotPlex Docs Hub
- Question: <your question>"

# 上传文档（使用 HotPlex Docs Hub 笔记本）
"Use notebooklm skill to add a source:
- Notebook: HotPlex Docs Hub
- File: <file_path>
- Display name: <descriptive_name>"
```

**重要**：
- 始终通过 Skill tool 调用 notebooklm skill，不要直接执行 notebooklm CLI 命令
- **永远使用 "HotPlex Docs Hub" 笔记本**，不需要指定 notebook_id
- 信息图和 PPT 生成需要用户在 **NotebookLM Web 界面**手动操作

---

## 总结

**hotplex-notebooklm skill 的核心价值**：

1. **品牌一致性**：所有生成的 Prompt 包含完整的 HotPlex 品牌规范
2. **职责清晰**：文档同步和知识查询委托给 notebooklm skill，Prompt 生成由自己完成
3. **用户体验**：生成的 Prompt 指令清晰完整，用户只需复制粘贴到 NotebookLM Web 界面
4. **灵活性**：Prompt 可以根据具体需求调整内容和风格

**使用流程**：
```
用户 → hotplex-notebooklm skill → 生成 Prompt → 用户复制 → NotebookLM Web → 生成内容
```
