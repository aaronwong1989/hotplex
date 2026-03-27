# Bot-03 Obsidian Skills 套件

> **版本**: v1.0
> **创建日期**: 2026-03-27
> **Skills 数量**: 7 个核心 skills

---

## ✅ 已创建 Skills

### 1. obsidian-capture (快速捕获)

**路径**: `~/.claude/skills/obsidian-capture/SKILL.md`

**核心能力**:
- ✅ 快速创建笔记到 0-Inbox/
- ✅ 自动提取关键词
- ✅ 智能推荐标签 (YAML list 格式)
- ✅ 推荐关联笔记
- ✅ 建议存放位置 (PARA 方法)

**使用示例**:
```
用户: 记录一个想法: 用 AI 自动分析 Obsidian 图谱

Bot (调用 skill):
✅ 已创建笔记: 灵感-20260327-2145
📊 分析结果:
- 关键词: AI, Obsidian, 知识图谱
- 建议标签: #灵感/AI #技术/Obsidian #待处理
- 关联笔记: [[知识图谱分析方法]] [[Obsidian 插件开发]]
```

---

### 2. obsidian-connect (知识关联)

**路径**: `~/.claude/skills/obsidian-connect/SKILL.md`

**核心能力**:
- ✅ 分析笔记语义相似度
- ✅ 发现隐含概念连接
- ✅ 识别知识孤岛
- ✅ 生成知识图谱报告
- ✅ 推荐新连接

**使用示例**:
```
用户: 分析 [[React Hooks]] 的关联

Bot:
📊 关联分析: [[React Hooks]]

💡 推荐新连接:
1. [[Vue Composition API]] (87%) - 共享标签: #状态管理
2. [[状态管理对比]] (75%) - 主题相关

🏝️ 发现孤岛:
- [[useReducer 详解]] - 无连接，建议整合
```

---

### 3. obsidian-til (TIL 管理)

**路径**: `~/.claude/skills/obsidian-til/SKILL.md`

**核心能力**:
- ✅ 自动格式化 TIL 笔记
- ✅ 代码语法高亮
- ✅ 技术栈分类
- ✅ 关联技术笔记
- ✅ 生成 TIL 索引

**使用示例**:
```
用户: TIL: Go 1.26 新增了 WaitTimeout 方法

Bot:
✅ 已创建 TIL: TIL-2026-03-27-go-waittimeout

📝 笔记内容:
---
type: til
language: go
tags:
  - til
  - go
  - concurrency
---

# Today I Learned: Go 1.26 WaitTimeout
[自动生成的完整笔记]
```

---

### 4. obsidian-maintain (定期维护)

**路径**: `~/.claude/skills/obsidian-maintain/SKILL.md`

**核心能力**:
- ✅ 每周清理 0-Inbox
- ✅ 自动归档完成项目
- ✅ 检测断裂的 wikilinks
- ✅ 生成周报/月报
- ✅ 更新知识图谱统计

**使用示例**:
```
用户: 执行每周维护

Bot:
🔧 开始每周维护...

📦 任务 1/4: Inbox 清理
扫描 0-Inbox: 发现 7 个超过 7 天的笔记
✅ 已清理 7 个笔记

🔗 任务 2/4: 断裂链接检测
✅ 无断裂链接

📊 任务 3/4: 生成健康报告
✅ 健康评分: 87/100

✅ 每周维护完成！
```

---

### 5. obsidian-brainstorm (头脑风暴)

**路径**: `~/.claude/skills/obsidian-brainstorm/SKILL.md`

**核心能力**:
- ✅ 灵感聚合 (从多个笔记提取主题)
- ✅ 头脑风暴辅助 (Mermaid 图)
- ✅ 素材关联推荐
- ✅ 创意可视化
- ✅ SCAMPER 创新法

**使用示例**:
```
用户: 头脑风暴: 如何提升 Obsidian 效率?

Bot:
🎯 头脑风暴会话已开启

## Step 1: 问题分析
- 关键词: Obsidian, 效率, 提升

## Step 2: 灵感聚合
✅ 找到 15 个相关笔记

## Step 3: 概念连接
[Mermaid 概念图]

## Step 4: 创意生成
| 创意 | 可行性 | 影响力 | 评分 |
|------|--------|--------|------|
| AI 驱动的图谱分析 | 80 | 90 | **170** |

✅ 完整报告已生成: 头脑风暴-20260327-2145
```

---

### 6. obsidian-project (项目管理)

**路径**: `~/.claude/skills/obsidian-project/SKILL.md`

**核心能力**:
- ✅ 创建项目笔记
- ✅ 任务追踪 (Tasks 插件语法)
- ✅ 进度报告 (Dataview 查询)
- ✅ 资源关联
- ✅ 里程碑管理

**使用示例**:
```
用户: 创建项目: HotPlex Provider 重构

Bot:
✅ 项目笔记已创建: 1-Projects/hotplex-provider-refactoring.md

📋 初始任务清单:
- [ ] 明确项目目标 #task 📅 2026-03-30 ⏫
- [ ] 制定详细计划 #task 📅 2026-04-01
- [ ] 分配资源 #task 📅 2026-04-03

🎯 里程碑:
- M1: 项目启动 (2026-04-03)
- M2: 核心功能完成 (2026-04-10)
- M3: 项目上线 (2026-04-26)
```

---

### 7. obsidian-graph (知识图谱)

**路径**: `~/.claude/skills/obsidian-graph/SKILL.md`

**核心能力**:
- ✅ 生成知识图谱报告
- ✅ 知识集群分析
- ✅ 连接强度监控
- ✅ 图谱可视化 (Mermaid)
- ✅ 演进追踪

**使用示例**:
```
用户: 分析知识图谱

Bot:
📊 知识图谱分析中...

## 总览
- 总笔记: 234
- 总连接: 567
- 平均连接度: 2.4 🟢
- 知识孤岛: 12 🟡
- 知识集群: 8

## 🕸️ 知识集群 (Top 5)
1. #技术/前端 (45 笔记)
2. #技术/Go (38 笔记)
3. #方法论 (25 笔记)

## 🔗 强连接 (Top 10)
| 笔记对 | 强度 |
|--------|------|
| [[React Hooks]] ↔ [[状态管理]] | 95 |

✅ 完整报告已生成
```

---

## 📋 Bot-03 配置更新

Bot-03 的 `system_prompt` 需要更新为使用这些 skills:

```yaml
ai:
  system_prompt: |
    你是 Bot-03,一位 **AI 知识管家**,通过调用专用的 Obsidian skills
    主动管理用户的第二大脑。

    ## 核心技能 (Skills)

    必须使用以下 skills 完成任务:

    ### 1. 快速捕获
    - **触发**: 用户记录灵感、想法、快速笔记
    - **Skill**: `obsidian-capture`
    - **用法**: `Skill(obsidian-capture)`

    ### 2. 知识关联分析
    - **触发**: 分析笔记关联、找相关笔记、检测孤岛
    - **Skill**: `obsidian-connect`
    - **用法**: `Skill(obsidian-connect)`

    ### 3. TIL 管理
    - **触发**: 记录技术学习、代码片段、Today I Learned
    - **Skill**: `obsidian-til`
    - **用法**: `Skill(obsidian-til)`

    ### 4. 定期维护
    - **触发**: 清理 Inbox、归档项目、检测断裂链接
    - **Skill**: `obsidian-maintain`
    - **用法**: `Skill(obsidian-maintain)`

    ### 5. 头脑风暴
    - **触发**: 创意问题、灵感聚合、概念可视化
    - **Skill**: `obsidian-brainstorm`
    - **用法**: `Skill(obsidian-brainstorm)`

    ### 6. 项目管理
    - **触发**: 创建项目、任务追踪、进度报告
    - **Skill**: `obsidian-project`
    - **用法**: `Skill(obsidian-project)`

    ### 7. 知识图谱
    - **触发**: 图谱分析、集群检测、连接强度
    - **Skill**: `obsidian-graph`
    - **用法**: `Skill(obsidian-graph)`

    ## 交互模式

    ### 直接调用 Skills
    当用户请求明确时，直接调用对应的 skill:

    ```
    用户: 记录一个想法: ...

    Bot: [调用 Skill(obsidian-capture)]
    [执行 skill 的工作流]
    [返回结果给用户]
    ```

    ### 主动建议
    当检测到用户需求匹配某个 skill 时，主动建议:

    ```
    用户: 我在研究 React Hooks...

    Bot: 检测到你在研究技术主题。
    是否需要:
    1. 创建 TIL 笔记? (使用 obsidian-til)
    2. 分析知识关联? (使用 obsidian-connect)
    ```

    ## Skill 调用最佳实践

    1. **单一职责**: 每次只调用一个 skill
    2. **明确参数**: 传递完整的上下文给 skill
    3. **结果呈现**: 以友好方式展示 skill 的输出
    4. **错误处理**: skill 失败时提供清晰的错误信息

    ## 示例对话

    ### 示例 1: 快速捕获

    ```
    用户: @bot-03 记录一个想法: 用 AI 分析 Obsidian 图谱

    Bot-03: [调用 Skill(obsidian-capture)]

    ✅ 已创建笔记: 灵感-20260327-2145

    📊 分析结果:
    - 关键词: AI, Obsidian, 知识图谱
    - 建议标签: #灵感/AI #技术/Obsidian #待处理
    - 关联笔记: [[知识图谱分析方法]] [[Obsidian 插件开发]]

    是否应用建议?
    ```

    ### 示例 2: 知识关联

    ```
    用户: @bot-03 分析 [[React Hooks]] 的关联

    Bot-03: [调用 Skill(obsidian-connect)]

    📊 关联分析: [[React Hooks]]

    💡 推荐新连接:
    1. [[Vue Composition API]] (87%)
    2. [[状态管理对比]] (75%)

    是否添加推荐的连接?
    ```

    ### 示例 3: TIL

    ```
    用户: @bot-03 TIL: Go 1.26 新增了 WaitTimeout 方法

    Bot-03: [调用 Skill(obsidian-til)]

    ✅ 已创建 TIL: TIL-2026-03-27-go-waittimeout

    📝 笔记已格式化:
    - 代码语法高亮 (go)
    - 标签: #til #go #concurrency
    - 关联推荐: [[Go 并发模式]]

    查看完整笔记?
    ```
```

---

## 🎯 下一步行动

### 1. ✅ 创建 7 个 Skills (已完成)

所有 7 个核心 skills 已创建完成:
- ✅ obsidian-capture (快速捕获)
- ✅ obsidian-connect (知识关联)
- ✅ obsidian-til (TIL 管理)
- ✅ obsidian-maintain (定期维护)
- ✅ obsidian-brainstorm (头脑风暴)
- ✅ obsidian-project (项目管理)
- ✅ obsidian-graph (知识图谱)

### 2. 更新 Bot-03 配置

```bash
# 编辑配置文件
vim docker/matrix/configs/bot-03/slack.yaml

# 更新 system_prompt 为使用 skills 的版本
```

### 3. 测试 Skills

```bash
# 重启 HotPlex
make restart

# 在 Slack 中测试
@bot-03 记录一个想法: 测试 skill 调用
@bot-03 分析 [[某笔记]] 的关联
@bot-03 TIL: 测试 TIL skill
@bot-03 执行每周维护
@bot-03 头脑风暴: 测试头脑风暴
@bot-03 创建项目: 测试项目
@bot-03 分析知识图谱
```

### 4. 文档完善

```bash
# 创建 skills 使用手册
vim docs/admin/obsidian-skills-manual.md
```

---

## 📊 Skills 功能矩阵

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

## 🌟 Skills 设计原则

### 1. 单一职责

每个 skill 只做一件事，做好一件事:
- ✅ obsidian-capture: 只负责快速捕获
- ✅ obsidian-connect: 只负责关联分析
- ✅ obsidian-til: 只负责 TIL 管理

### 2. 可组合性

Skills 可以组合使用:
```
用户: 研究 React Hooks 并创建学习笔记

Bot:
1. [调用 obsidian-capture] 捕获研究笔记
2. [调用 obsidian-connect] 分析知识关联
3. [调用 obsidian-til] 创建 TIL (如果是技术发现)
```

### 3. 用户友好

所有 skills 都提供清晰的反馈:
- ✅ 操作成功/失败提示
- ✅ 推荐建议 (需要确认)
- ✅ 错误处理和恢复方案

### 4. 2026 最佳实践

所有 skills 遵循 Obsidian 2026 规范:
- ✅ YAML list tags 格式
- ✅ Properties 管理
- ✅ Obsidian CLI 原生命令
- ✅ Dataview 查询集成

---

## 📚 参考资源

### Skills 文档
- [obsidian-capture](~/.claude/skills/obsidian-capture/SKILL.md) - 快速捕获
- [obsidian-connect](~/.claude/skills/obsidian-connect/SKILL.md) - 知识关联
- [obsidian-til](~/.claude/skills/obsidian-til/SKILL.md) - TIL 管理
- [obsidian-maintain](~/.claude/skills/obsidian-maintain/SKILL.md) - 定期维护
- [obsidian-brainstorm](~/.claude/skills/obsidian-brainstorm/SKILL.md) - 头脑风暴
- [obsidian-project](~/.claude/skills/obsidian-project/SKILL.md) - 项目管理
- [obsidian-graph](~/.claude/skills/obsidian-graph/SKILL.md) - 知识图谱

### Bot-03 配置
- [配置文件](../../docker/matrix/configs/bot-03/base/slack.yaml)
- [设计文档](2026-03-27-obsidian-master-bot03-design.md)
- [快速开始](obsidian-master-bot03-quickstart.md)

### Obsidian 资源
- [Obsidian CLI](https://help.obsidian.md/cli)
- [Obsidian Flavored Markdown](https://help.obsidian.md/obsidian-flavored-markdown)
- [Dataview 文档](https://blacksmithgu.github.io/obsidian-dataview/)

---

**创建日期**: 2026-03-27
**维护者**: Claude Code
**状态**: ✅ **7/7 Skills 已完成**
