# Claude Code 必备插件安装指南 (2026 官方核实版)

本指南内容已于 2026 年 3 月 23 日针对 Claude Code v2.1.81 进行核实，确保安装指令完全准确。

## 1. 基础安装指令
Claude Code 支持两类扩展方式：**插件 (Plugins)** 和 **MCP 服务器 (MCP Servers)**。

### 1. 插件市场说明
您的系统中目前已配置以下三个核心市场，用途各异：

1. **`claude-plugins-official` (Anthropic 官方)**
   - **用途**：提供由官方维护、经过安全审计的核心插件（如 `feature-dev`）。建议作为首选。
2. **`superpowers-marketplace` (社区精选)**
   - **用途**：包含大量由社区贡献的高级技能和增强工具，是扩展 AI 编码能力的宝库。
3. **`voltagent-subagents` (子代理专场)**
   - **用途**：专注于 **Subagents (子代理)** 的分发，适合需要多代理协作完成复杂任务的场景。

您可以运行以下命令实时查看：
`claude plugin marketplace list`


### MCP 服务器添加
语法：`claude mcp add [服务名] --command "[执行命令]"`

---

## 2. 核心插件与服务推荐

### 2.1 Context7 (实时文档注入)
- **推荐理由**：解决 AI 训练数据过时的问题，实时注入最新 API 文档。
- **安装方法**：
  在终端运行：`npx ctx7 setup`
  根据提示完成 OAuth 认证并选择安装到 Claude。

### 2.2 GitHub MCP (深度代码库协作)
- **推荐理由**：实现 Issue 搜索、PR 创建、代码仓库深度读取。
- **安装方法**：
  `claude mcp add github --command "npx -y @modelcontextprotocol/server-github"`
  *注：首次运行需按提示配置 GitHub Personal Access Token。*

### 2.3 Feature-Dev (标准开发流)
- **推荐理由**：官方出品，将复杂需求自动化拆解为 7 个标准开发阶段。
- **安装方法**：
  `claude plugin install feature-dev@claude-plugins-official`

### 2.4 Playwright (UI 自动化与测试)
- **推荐方案**：使用 **Playwright CLI** (`@playwright/cli`)。
- **推荐理由**：相比 MCP Server，CLI 方案在 2026 年被证实能节省约 4 倍的 Token 消耗。它通过将浏览器状态保存到本地磁盘（YAML/PNG）供 Claude 显式读取，避免了冗余的上下文堆积。
- **安装方法**：
  1. 全局安装：`npm install -g @playwright/cli@latest`
  2. 安装 AI 技能：`playwright-cli install --skills`
  3. 安装浏览器内核：`npx playwright install`
- **MCP 备选方案**：若环境受限无法运行全局命令，可使用：
  `claude mcp add playwright --command "npx -y @modelcontextprotocol/server-playwright"`


### 2.5 Claude-Mem (持久化记忆与上下文优化)
- **核心作用**：解决 AI 助手“由于会话重启而丢失项目背景”的痛点。它不只是简单的日志记录，而是通过 AI 进行**语义压缩**，建立长效记忆。
- **2026 关键特性**：
  - **极致节省 Token**：通过智能过滤和摘要技术，在处理长任务时可减少约 **90% - 95%** 的 Token 消耗。
  - **智能上下文注入**：自动识别当前任务所需的历史背景（如同行评审意见、旧 Bug 修复逻辑），并进行精准注入，而非全量加载。
  - **可视化管理**：提供本地 Web UI (`localhost:37777`)，让开发者能直观审阅、编辑或删除 AI 的记忆片段。
  - **隐私可见性**：支持 `<private>` 标签，确保敏感代码或私密指令不进入持久化数据库。
- **安装方法**：
  1. 添加市场：`claude plugin marketplace add thedotmack/claude-mem`
  2. 执行安装：`claude plugin install claude-mem`

---
*注：部分插件可能需要环境变量支持，建议在 `~/.claude/settings.json` 中统一管理。*
