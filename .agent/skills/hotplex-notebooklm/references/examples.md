# NotebookLM Prompt 示例与模板

本文件包含 `hotplex-notebooklm` skill 的完整 Prompt 模板和工作流示例。

---

## 1. 信息图 Prompt 模板

### 模板

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
```

### 完整示例 1：SessionPool 架构信息图

```markdown
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
```

### 完整示例 2：ChatApps 集成信息图

```markdown
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

---

## 2. PPT Prompt 模板

### 模板

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
```

### 完整示例 1：架构概览 PPT

```markdown
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
```

### 完整示例 2：ChatApps 集成 PPT

```markdown
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
- 代码使用 Go 语法高亮
- 平台 Logo：Slack（紫色）、Feishu（蓝色）、DingTalk（蓝色）
- 架构图：使用**组件依赖图**，标注 SDK 调用
- 技术术语保留英文原名：Socket Mode, Block Kit, Webhook
```

---

## 3. 工作流示例

### 示例 1：生成 SessionPool 信息图

**用户请求**：用 NotebookLM 生成 HotPlex SessionPool 架构信息图

**执行流程**：

1. 触发 hotplex-notebooklm skill（同时提到 HotPlex 和 NotebookLM）
2. 生成信息图 Prompt（参考上面"完整示例 1"）
3. 返回 Prompt 给用户复制
4. 用户在 NotebookLM Web 界面操作：打开 "HotPlex Docs Hub" → 使用信息图功能 → 粘贴 Prompt → 生成信息图

### 示例 2：生成 ChatApps 集成 PPT

**用户请求**：从 NotebookLM 创建 HotPlex ChatApps 集成演示 PPT

**执行流程**：

1. 触发 hotplex-notebooklm skill
2. 生成 PPT Prompt（参考上面"完整示例 2"）
3. 返回 Prompt 给用户复制
4. 用户在 NotebookLM Web 界面操作：打开 "HotPlex Docs Hub" → 使用 PPT 功能 → 粘贴 Prompt → 生成 PPT
