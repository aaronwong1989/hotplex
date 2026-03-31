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

所有生成的内容必须遵循以下品牌规范。

### 品牌元素

| 元素 | 值 | 用途 |
|------|-----|------|
| **品牌色** | `#00ADD8` | 标题、重点、链接、主色调 |
| **辅助色** | `#333333` | 深灰文字 |
| **背景色** | `#F5F5F5` | 浅灰背景 |
| **字体** | `Inter, system-ui` | 标题和正文 |
| **Logo** | HotPlex Logo | 青色 lightning bolt |
| **标语** | "Strategic Bridge for AI Agent Engineering" | 用于封面或结尾 |

### 内容风格

- **技术术语**：保留英文原名（Engine, SessionPool, Provider 等）
- **代码示例**：使用 Go 语法高亮，符合 Uber Go Style Guide
- **架构图**：组件化表示，标注数据流
- **示例场景**：优先使用 Slack、Feishu 等实际集成场景

---

## 模块 1：文档同步（委托 notebooklm skill）

扫描 HotPlex 源码目录的高价值文档并上传到 **"HotPlex Docs Hub"** 专用笔记本。

### 专用笔记本

**名称**：`HotPlex Docs Hub`（固定）

### 状态管理

状态文件: `~/.hotplex-notebooklm/state.json`

```json
{
  "notebook_id": "c326b88e-bcca-4e80-bc61-477d65995833",
  "notebook_title": "HotPlex Docs Hub",
  "last_sync_commit": "",
  "last_sync_time": "",
  "sources": {}
}
```

**初始化逻辑**（首次使用）：
1. 调用 notebooklm skill 列出所有笔记本
2. 查找名为 "HotPlex Docs Hub" 的笔记本
3. 如果不存在，创建新笔记本
4. 保存 notebook_id 到状态文件

```bash
mkdir -p ~/.hotplex-notebooklm
echo '{"notebook_title": "HotPlex Docs Hub", "notebook_id": "", "last_sync_commit": "", "sources": {}}' > ~/.hotplex-notebooklm/state.json
```

### 高价值文档清单

详见 [`references/document-index.md`](references/document-index.md)（包含完整的 11 类文档清单）。

**注意：中英文双语文档只上传英文版本。排除 CHANGELOG。**

### 增量同步

```bash
# 1. 检测 Git 变更
LAST_COMMIT=$(jq -r '.last_sync_commit // empty' ~/.hotplex-notebooklm/state.json)
git diff --name-only $LAST_COMMIT HEAD -- "*.md" \
  | grep -v "_zh.md" | grep -v "docs-site" | grep -v "CHANGELOG"

# 2. 调用 notebooklm skill 上传文档（传递 notebook_id + file_path + display_name）

# 3. 更新状态
new_commit=$(git rev-parse --short HEAD)
jq ".last_sync_commit = \"$new_commit\"" ~/.hotplex-notebooklm/state.json > tmp.json && mv tmp.json ~/.hotplex-notebooklm/state.json
```

---

## 模块 2：知识查询（委托 notebooklm skill）

使用 notebooklm skill 查询 **"HotPlex Docs Hub"** 笔记本。

### 调用方式

```bash
# 使用 Skill tool 调用 notebooklm skill
"Use notebooklm skill to ask: Explain HotPlex SessionPool state management.
Use notebook: HotPlex Docs Hub"
```

**重要**：
- 每个问题都是独立会话，需要包含足够的上下文
- **永远使用 "HotPlex Docs Hub" 笔记本**，不需要指定 notebook_id

---

## 模块 3：信息图 Prompt 生成

生成用于 NotebookLM Web 界面的信息图 Prompt 指令，包含 HotPlex 品牌风格和内容要求。

### 输出结构

生成的 Prompt 包含：主题描述、品牌风格要求、内容结构、视觉建议。

### 品牌检查清单

生成 Prompt 前，确保包含以下品牌要求：
- [ ] 包含品牌色 `#00ADD8`
- [ ] 包含文字颜色 `#333333` 和 `#fff`
- [ ] 包含背景色 `#F5F5F5`
- [ ] 包含字体要求 `Inter, system-ui`
- [ ] 包含技术术语保留英文原名的说明
- [ ] 包含数据流方向建议

### 完整模板与示例

**完整 Prompt 模板和工作流示例**：参考 [`references/examples.md`](references/examples.md)。

---

## 模块 4：PPT Prompt 生成

生成用于 NotebookLM Web 界面的 PPT Prompt 指令，包含 HotPlex 品牌风格和内容要求。

### 输出结构

生成的 Prompt 包含：主题描述、品牌风格要求、内容大纲（章节结构 + 每页要点）、视觉建议。

### 品牌检查清单

生成 Prompt 前，确保包含以下品牌要求：
- [ ] 包含品牌色 `#00ADD8`
- [ ] 包含文字颜色 `#333333`
- [ ] 包含字体要求 `Inter, system-ui`
- [ ] 包含 Logo 要求（首页和页眉）
- [ ] 包含标语（最后一页）
- [ ] 包含代码语法高亮要求（Go syntax）
- [ ] 包含内容大纲（每页标题和要点）

### 完整模板与示例

**完整 Prompt 模板和工作流示例**：参考 [`references/examples.md`](references/examples.md)。

---

## notebooklm skill 协作

### hotplex-notebooklm 职责

1. **文档同步协调**：委托 notebooklm skill 上传文档
2. **知识查询协调**：委托 notebooklm skill 查询知识库
3. **品牌规范**：定义和管理 HotPlex 品牌标准
4. **Prompt 生成**：生成信息图和 PPT 的 Prompt 指令

### 常用 notebooklm skill 调用

```bash
# 检查认证状态
"Use notebooklm skill to check authentication status"

# 认证（浏览器可见）
"Use notebooklm skill to setup authentication (show browser)"

# 列出笔记本
"Use notebooklm skill to list all notebooks"

# 查询问题
"Use notebooklm skill to ask a question:
- Notebook: HotPlex Docs Hub
- Question: <your question>"

# 上传文档
"Use notebooklm skill to add a source:
- Notebook: HotPlex Docs Hub
- File: <file_path>
- Display name: <descriptive_name>"
```

---

## 故障排除

| 问题 | 解决方案 |
|------|----------|
| notebooklm skill 未认证 | 先调用 notebooklm skill 进行认证 |
| NotebookLM 找不到 HotPlex notebook | 运行文档同步（模块 1） |
| Prompt 不完整 | 检查是否包含品牌风格 + 内容要求 |
| 生成结果不符合品牌风格 | 检查 Prompt 中的品牌要求是否完整 |

---

## 总结

**hotplex-notebooklm skill 的核心价值**：

1. **品牌一致性**：所有生成的 Prompt 包含完整的 HotPlex 品牌规范
2. **职责清晰**：文档同步和知识查询委托给 notebooklm skill，Prompt 生成由自己完成
3. **用户体验**：生成的 Prompt 指令清晰完整，用户只需复制粘贴到 NotebookLM Web 界面

**使用流程**：
```
用户 → hotplex-notebooklm skill → 生成 Prompt → 用户复制 → NotebookLM Web → 生成内容
```
