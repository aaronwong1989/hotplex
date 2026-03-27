# Bot-03 Obsidian 大师 - 快速开始指南

> **版本**: v1.0
> **配置日期**: 2026-03-27
> **Vault**: /Users/huangzhonghui/Documents/second-brain

---

## 🎯 Bot-03 定位

Bot-03 是你的 **AI 知识管家**,深度融合 Claude Code 与 Obsidian CLI,主动管理你的第二大脑。

### 核心能力

| 能力 | 描述 | 交互模式 |
|------|------|---------|
| 🎯 **智能捕获** | 快速记录、自动标签、智能分类 | 混合模式 |
| 🔗 **知识关联** | 图谱分析、相似度计算、孤岛检测 | 建议确认 |
| 📊 **项目管理** | 任务追踪、进度报告、资源关联 | 混合模式 |
| 💻 **技术学习** | TIL 管理、代码片段、技术图谱 | 混合模式 |
| 💡 **创意激发** | 灵感聚合、头脑风暴、概念可视化 | 建议确认 |
| 🔧 **定期维护** | Inbox 清理、归档整理、断裂检测 | 自动执行 |

---

## 🚀 快速开始

### 1. 启动 Bot-03

```bash
# 在 HotPlex 项目目录
cd /Users/huangzhonghui/hotplex

# 重启 HotPlex (加载新配置)
make restart

# 验证 Bot-03 已启动
docker logs hotplex-matrix-standalone | grep "bot-03"
```

### 2. 确保 Obsidian 运行

```bash
# 打开 Obsidian
open -a Obsidian

# 验证 vault 路径
ls ~/Documents/second-brain/.obsidian
```

### 3. 测试基础功能

在 Slack 中向 Bot-03 发送消息:

```
# 测试 1: 快速捕获
@bot-03 记录一个想法: 用 AI 自动分析 Obsidian 图谱

# 测试 2: 知识关联
@bot-03 分析笔记 "React Hooks" 的关联

# 测试 3: 创建 TIL
@bot-03 TIL: Go 1.26 新增了 WaitTimeout 方法
```

---

## 📋 核心工作流

### 工作流 1: 灵感捕获

```
用户: 记录一个灵感: [想法内容]

Bot-03 执行:
1. ✅ 创建笔记 → 0-Inbox/灵感-YYYYMMDD-HHMM
2. 📊 分析关键词 → 提取主题
3. 🏷️ 推荐标签 → #灵感/[主题] #待处理
4. 🔗 推荐关联 → 相关笔记 Top 5
5. 📂 建议位置 → PARA 分类建议
```

**示例**:
```
用户: @bot-03 记录一个想法: 可以用 Obsidian CLI 自动化知识管理

Bot-03:
✅ 已创建笔记: 灵感-20260327-2145
📍 位置: 0-Inbox/

📊 分析结果:
- 关键词: Obsidian CLI, 自动化, 知识管理
- 建议标签: #灵感/AI #技术/Obsidian #待处理
- 关联笔记:
  • [[知识图谱分析方法]]
  • [[Obsidian 插件开发]]
  • [[自动化工作流]]

是否应用建议? (是/部分/跳过)
```

---

### 工作流 2: 知识关联

```
用户: 分析 [[笔记名]] 的关联

Bot-03 执行:
1. 🔍 搜索相关笔记 (标签 + 内容)
2. 📊 计算相似度 (标签匹配 + 语义相似)
3. 🔗 分析当前连接 (inlinks + outlinks)
4. 💡 推荐新连接 (Top 5, 带理由)
5. 🏝️ 检测知识孤岛
```

**示例**:
```
用户: @bot-03 分析 [[React Hooks]] 的关联

Bot-03:
📊 关联分析: [[React Hooks]]

🔗 当前连接:
- 入链 (5): [[React 项目]], [[前端技术栈]], [[状态管理]], ...
- 出链 (3): [[useState]], [[useEffect]], [[自定义 Hook]]

💡 推荐新连接:
1. [[Vue Composition API]] (相似度 78%)
   理由: 同为响应式状态管理方案
2. [[状态管理对比]] (相似度 65%)
   理由: 共享标签 #状态管理 #响应式
3. [[React 性能优化]] (相似度 60%)
   理由: 相关项目引用

🏝️ 发现孤岛笔记:
- [[useReducer 详解]] - 无任何连接
  建议: 添加到 "状态管理" 系列笔记

是否添加推荐的连接? (全选/部分/跳过)
```

---

### 工作流 3: TIL (Today I Learned)

```
用户: TIL: [技术知识点]

Bot-03 执行:
1. ✅ 创建 TIL 笔记 → 3-Resources/TIL/YYYY-MM-DD-主题
2. 🏷️ 自动标签 → #til #技术栈 #主题
3. 💻 代码高亮 → 自动检测语言
4. 🔗 推荐关联 → 相关技术笔记
```

**示例**:
```
用户: @bot-03 TIL: Go 1.26 新增了 sync.WaitGroup.WaitTimeout 方法

Bot-03:
✅ 已创建 TIL 笔记: TIL-2026-03-27-Go-WaitTimeout

📝 笔记内容:
---
type: til
language: go
topic: concurrency
tags:
  - til
  - go
  - concurrency
---

# Today I Learned: Go 1.26 WaitTimeout

```go
var wg sync.WaitGroup
wg.Add(1)

if wg.WaitTimeout(5 * time.Second) {
    // 超时处理
}
```

## 关键点
- 新方法: WaitTimeout(duration) bool
- 替代方案: 之前需要用 context + select 实现
- 适用场景: 需要超时控制的并发等待

🔗 推荐关联:
- [[Go 并发模式]]
- [[sync.WaitGroup 源码分析]]
- [[Go 1.26 新特性汇总]]

是否添加关联?
```

---

### 工作流 4: 定期维护

**自动触发时间**:
- 每天 9:00: 生成每日任务看板
- 每周日 20:00: 清理 Inbox、检测断裂链接
- 每月 1 号: 归档已完成项目、生成月报

**手动触发**:
```
用户: @bot-03 执行每周维护

Bot-03:
🔧 每周维护开始

📦 Inbox 清理:
- 7 个笔记超过 7 天
- 建议移动:
  • 灵感-AI-图谱 → Zettelkasten/Fleeting-Notes
  • 项目-重构计划 → 1-Projects/HotPlex/
  • 学习笔记-React → 3-Resources/Frontend/

🔗 断裂链接检测:
- ⚠️ [[已删除的笔记]] - 在 [[项目总结]] 中引用
- ⚠️ [[旧文档]] - 在 [[技术栈]] 中引用

📊 本周统计:
- 新建笔记: 15 篇
- 更新笔记: 23 篇
- 完成任务: 8 个
- 新增连接: 12 条

是否执行建议的整理? (是/查看详情/跳过)
```

---

## 🎨 高级用法

### 1. 头脑风暴

```
用户: @bot-03 头脑风暴: 如何提升知识管理效率

Bot-03:
🎯 头脑风暴会话已开启

📊 灵感聚合 (来自 5 个笔记):
- 自动分析知识图谱
- 智能推荐相关笔记
- 语音快速捕获
- AI 辅助写作
- 自动生成摘要

💡 概念关联图:
[Mermaid 图: 知识管理 → AI 增强 → 效率提升 → ...]

📚 相关素材:
- [[知识图谱算法]]
- [[AI 辅助写作工具]]
- [[Obsidian 插件开发]]

继续深入哪个方向?
```

### 2. 项目管理

```
用户: @bot-03 创建项目: HotPlex Provider 重构

Bot-03:
✅ 已创建项目笔记: HotPlex Provider 重构
📍 位置: 1-Projects/HotPlex/

📋 项目结构:
---
type: project
status: active
priority: high
start_date: 2026-03-27
tags:
  - project
  - hotplex
  - refactoring
---

# HotPlex Provider 重构

## 目标
[待填写]

## 任务
- [ ] 设计新架构
- [ ] 实现 Provider 接口
- [ ] 迁移现有代码

## 资源
- [[相关技术文档]]
- [[参考实现]]

## 进度追踪
\`\`\`dataview
TASK FROM "1-Projects/HotPlex Provider 重构"
WHERE !completed
\`\`\`

是否添加初始任务?
```

### 3. 知识图谱报告

```
用户: @bot-03 生成知识图谱报告

Bot-03:
📊 知识图谱报告 (2026-03-27)

## 统计数据
- 总笔记数: 234
- 总连接数: 567
- 平均连接度: 2.4 条/笔记
- 孤岛笔记: 12 个

## 热门标签
1. #技术/React (45 笔记)
2. #项目/HotPlex (38 笔记)
3. #学习/Go (32 笔记)

## 知识集群
- 前端技术栈 (28 笔记, 45 连接)
- HotPlex 架构 (35 笔记, 67 连接)
- 知识管理 (22 笔记, 38 连接)

## 推荐
- 🔗 建议连接: [[React Hooks]] ↔ [[Vue Composition API]]
- 🏝️ 孤岛修复: [[useReducer 详解]] 需要添加连接
- 📂 归档建议: 3 个项目已完成, 可归档

是否执行推荐操作?
```

---

## ⚙️ 配置说明

### Vault 路径

当前配置: `/Users/huangzhonghui/Documents/second-brain`

如需修改,编辑配置文件:
```yaml
# docker/matrix/configs/bot-03/slack.yaml
ai:
  system_prompt: |
    ...
    ## Vault 结构
    用户 vault: /Users/huangzhonghui/Documents/second-brain
    ...
```

### PARA 文件夹

确保你的 vault 有以下结构:
```
second-brain/
├── 0-Inbox/
├── 1-Projects/
├── 2-Areas/
├── 3-Resources/
│   └── TIL/
├── 4-Archives/
├── Zettelkasten/
│   ├── Fleeting-Notes/
│   ├── Literature-Notes/
│   ├── Permanent-Notes/
│   └── Concept-Maps/
└── Templates/
```

### 模板文件

Bot-03 会使用以下模板:
- Template-每日笔记.md
- Template-技术开发.md
- Template-学习研究.md
- Template-项目管理.md
- Template-创意写作.md
- Template-快速捕获.md
- Template-Zettelkasten永久笔记.md

---

## 🔧 故障排查

### 问题 1: Obsidian 未运行

```
❌ Obsidian is not running. Please open Obsidian first.
```

**解决**:
```bash
open -a Obsidian
```

### 问题 2: 笔记创建失败

```
❌ Failed to create note. Check vault path.
```

**检查**:
```bash
# 1. 验证 vault 路径
ls ~/Documents/second-brain/.obsidian

# 2. 检查 Obsidian 是否打开了正确的 vault
# 在 Obsidian 中: 设置 → 关于 → 打开 vault 路径
```

### 问题 3: 标签格式错误

**症状**: Dataview 查询无法识别标签

**原因**: 使用了字符串格式而不是 YAML list

**修复**:
Bot-03 已使用 `type=list` 参数,自动生成正确格式:
```yaml
---
tags:
  - tag1
  - tag2
  - tag3
---
```

### 问题 4: 权限不足

```
❌ Permission denied
```

**解决**:
```bash
# 检查文件权限
ls -la ~/Documents/second-brain/

# 修复权限
chmod -R 755 ~/Documents/second-brain/
```

---

## 📚 进阶配置

### 自定义标签推荐

在配置中添加你的标签体系:
```yaml
ai:
  system_prompt: |
    ...
    ## 标签体系
    技术栈: #tech/{language}/{framework}
    项目: #project/{name}
    学习: #learning/{topic}
    灵感: #idea/{category}
    ...
```

### 自定义工作流

添加你的专属工作流:
```yaml
ai:
  system_prompt: |
    ...
    ## 自定义工作流
    每日回顾:
      1. 查询今天创建的笔记
      2. 提取关键收获
      3. 更新相关项目
      4. 规划明天任务
    ...
```

---

## 🎯 最佳实践

### 1. 渐进式使用

**第 1 周**: 基础功能
- 每天用 Bot-03 捕获灵感
- 尝试知识关联分析
- 熟悉混合交互模式

**第 2 周**: 深度应用
- 开始用 TIL 记录技术学习
- 使用项目管理功能
- 建立知识图谱

**第 3 周**: 形成系统
- 定期执行维护任务
- 优化标签体系
- 建立个人工作流

### 2. 标签命名规范

```
#层级1/层级2/层级3

示例:
#tech/frontend/react
#project/hotplex/provider
#learning/go/concurrency
#idea/automation/workflow
```

### 3. 定期维护习惯

- **每天**: 查看任务看板
- **每周**: 清理 Inbox
- **每月**: 回顾知识图谱、归档项目

---

## 📞 获取帮助

### 查看设计文档
```bash
cat docs/superpowers/specs/2026-03-27-obsidian-master-bot03-design.md
```

### 查看配置文件
```bash
cat docker/matrix/configs/bot-03/slack.yaml
```

### 日志调试
```bash
# 查看 Bot-03 日志
docker logs hotplex-matrix-standalone | grep "bot-03"

# 实时监控
docker logs -f hotplex-matrix-standalone | grep "bot-03"
```

---

## 🎉 开始使用

现在就在 Slack 中试试吧:

```
@bot-03 记录一个想法: [你的第一个灵感]
```

祝你知识管理愉快! 🚀

---

**文档版本**: v1.0
**最后更新**: 2026-03-27
**维护者**: Claude Code + Bot-03
