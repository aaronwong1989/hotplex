# Obsidian AI 知识管理系统

> **Bot-03** - AI 驱动的 Obsidian 知识管家
> 基于 2026 年最佳实践，深度融合 Claude Code + Obsidian CLI

---

## 📚 文档导航

### 🎯 快速开始

- **[Bot-03 快速开始](obsidian-master-bot03-quickstart.md)** - 3 步启动 AI 知识管家
- **[Skills 套件](obsidian-skills-suite.md)** - 7 个专业 Obsidian Skills 完整文档

### 📐 设计文档

- **[系统设计](2026-03-27-obsidian-master-bot03-design.md)** - 完整的架构设计和最佳实践

---

## 🚀 核心功能

### 7 个专业 Skills

| Skill | 功能 | 触发场景 |
|-------|------|---------|
| 📝 **obsidian-capture** | 快速捕获灵感 | 记录想法、灵感捕获 |
| 🔗 **obsidian-connect** | 知识关联分析 | 分析关联、发现孤岛 |
| 💡 **obsidian-til** | TIL 管理 | 技术学习、代码片段 |
| 🧹 **obsidian-maintain** | 定期维护 | Inbox 清理、归档整理 |
| 🧠 **obsidian-brainstorm** | 头脑风暴 | 创意激发、概念图生成 |
| 📋 **obsidian-project** | 项目管理 | 任务追踪、进度报告 |
| 🕸️ **obsidian-graph** | 知识图谱 | 集群检测、演进追踪 |

---

## 🎯 使用场景

### 场景 1: 快速捕获灵感

```
用户: 记录一个想法: 用 AI 分析 Obsidian 知识图谱

Bot-03: ✅ 已创建笔记: 灵感-20260327-2145
📊 分析结果:
- 关键词: AI, Obsidian, 知识图谱
- 建议标签: #灵感/AI #技术/Obsidian #待处理
- 关联笔记: [[知识图谱分析方法]]
```

### 场景 2: TIL 记录

```
用户: TIL: Go 1.26 新增了 WaitTimeout 方法

Bot-03: ✅ 已创建 TIL: TIL-2026-03-27-go-waittimeout
📝 自动格式化:
- 代码语法高亮 (go)
- 标签: #til #go #concurrency
- 关联推荐: [[Go 并发模式]]
```

### 场景 3: 知识关联分析

```
用户: 分析 [[React Hooks]] 的关联

Bot-03: 📊 关联分析: [[React Hooks]]
💡 推荐新连接:
1. [[Vue Composition API]] (87%) - 共享标签: #状态管理
2. [[状态管理对比]] (75%) - 主题相关
```

---

## 🛠️ 技术栈

### 核心技术

- **Obsidian CLI** - 原生命令行集成
- **Claude Code** - AI 智能助手
- **Dataview** - 动态查询和聚合
- **Tasks Plugin** - 任务管理语法
- **Mermaid** - 图表可视化

### 2026 最佳实践

- ✅ YAML list tags 格式 (`type=list`)
- ✅ PARA 方法组织 (Projects/Areas/Resources/Archives)
- ✅ Zettelkasten 方法论 (Fleeting/Literature/Permanent Notes)
- ✅ Obsidian Flavored Markdown
- ✅ Properties 管理

---

## 📂 目录结构

```
~/Documents/second-brain/
├── 0-Inbox/          # 快速捕获 (PARA)
├── 1-Projects/       # 活跃项目
├── 2-Areas/          # 责任领域
├── 3-Resources/      # 资源库
├── 4-Archives/       # 归档内容
├── Zettelkasten/     # 卡片盒笔记
│   ├── Fleeting-Notes/
│   ├── Literature-Notes/
│   └── Permanent-Notes/
├── Templates/        # 模板库
└── Configs/          # 配置文件
```

---

## 🔧 配置文件

### Bot-03 配置

- **Slack 配置**: `docker/matrix/configs/bot-03/base/slack.yaml`
- **Capabilities**: `docker/matrix/configs/bot-03/base/slack_capabilities.yaml`
- **Skills 目录**: `~/.claude/skills/obsidian-*/`

### 环境变量

```bash
# Obsidian Vault 路径
OBSIDIAN_VAULT_PATH=~/Documents/second-brain

# Bot-03 用户 ID
HOTPLEX_SLACK_BOT_USER_ID_03=U08SY7W2XS8
```

---

## 🎓 学习路径

### Week 1: 基础功能

1. **Day 1-2**: 快速捕获 + TIL
   - 练习记录灵感和想法
   - 记录每天学到的技术知识

2. **Day 3-4**: 知识关联
   - 分析笔记间的关联
   - 发现和整合知识孤岛

3. **Day 5-7**: 定期维护
   - 执行每周维护任务
   - 生成健康报告

### Week 2: 高级功能

1. **头脑风暴**: 创意激发和概念图
2. **项目管理**: 任务追踪和进度报告
3. **知识图谱**: 集群分析和演进追踪

### Week 3: 工作流整合

- 建立个人知识管理工作流
- 优化 PARA 目录结构
- 定制 Bot-03 交互模式

---

## 🔗 相关资源

### 官方文档

- [Obsidian CLI](https://help.obsidian.md/cli)
- [Obsidian Flavored Markdown](https://help.obsidian.md/obsidian-flavored-markdown)
- [Dataview 文档](https://blacksmithgu.github.io/obsidian-dataview/)
- [Tasks 插件](https://publish.obsidian.md/tasks/)

### 方法论

- [PARA 方法](https://fortelabs.com/blog/para-method/)
- [Zettelkasten 方法](https://zettelkasten.de/)
- [卡片盒笔记法](https://www.soenkeahrens.de/en/takesmartnotes)

### 社区资源

- [Obsidian Hub](https://publish.obsidian.md/hub/)
- [Obsidian Roundup](https://www.obsidianroundup.org/)
- [r/ObsidianMD](https://www.reddit.com/r/ObsidianMD/)

---

## 📊 功能矩阵

| Skill | 核心功能 | 自动化级别 | 调用频率 |
|-------|---------|-----------|---------|
| **obsidian-capture** | 快速捕获、标签推荐 | 混合模式 | 高 (每天 5-10 次) |
| **obsidian-connect** | 关联分析、孤岛检测 | 建议确认 | 中 (每周 3-5 次) |
| **obsidian-til** | TIL 管理、代码片段 | 混合模式 | 中 (每周 2-3 次) |
| **obsidian-maintain** | 定期清理、归档 | 自动执行 | 低 (每周 1 次) |
| **obsidian-brainstorm** | 头脑风暴、创意激发 | 建议确认 | 低 (每月 1-2 次) |
| **obsidian-project** | 项目管理、任务追踪 | 混合模式 | 中 (每周 3-5 次) |
| **obsidian-graph** | 图谱分析、集群检测 | 建议确认 | 低 (每月 1 次) |

---

## 🚦 状态

- ✅ **7/7 Skills 已完成**
- ✅ **Bot-03 配置已更新**
- ✅ **文档已迁移到 `docs/obsidian/`**

---

**创建日期**: 2026-03-27
**维护者**: Claude Code
**版本**: v1.0
