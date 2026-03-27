# Obsidian Master Bot-03 设计文档

> **版本**: v1.0
> **创建日期**: 2026-03-27
> **目标**: 打造深度融合 Claude Code 与 Obsidian CLI 的 AI 知识管家

---

## 1. 执行摘要

### 1.1 核心定位

Bot-03 是 **AI 知识管家**，主动管理用户的第二大脑（Obsidian vault），实现：
- 智能知识捕获与组织
- 项目全生命周期管理
- 知识图谱关联挖掘
- 技术学习笔记管理
- 创意灵感激发
- 自动化定期维护

### 1.2 关键特性

| 特性 | 描述 | 价值 |
|------|------|------|
| **深度融合** | 使用 Obsidian CLI + Dataview + Templater | 原生 Obsidian 体验 |
| **智能关联** | 自动发现笔记间的隐含连接 | 知识网络化 |
| **混合交互** | 日常主动执行 + 关键操作确认 | 高效且可控 |
| **PARA + Zettelkasten** | 双系统融合 | 宏观组织 + 微观知识网络 |
| **2026 最佳实践** | YAML list tags、properties、callouts | 现代化笔记规范 |

---

## 2. 架构设计

### 2.1 三层知识处理模型

```
┌─────────────────────────────────────────────────────────┐
│                    Layer 1: 捕获层                       │
│  • 快速捕获灵感 (0-Inbox)                                │
│  • 自动提取关键词和标签                                   │
│  • 初步分类建议                                          │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│                    Layer 2: 加工层                       │
│  • Fleeting Notes → Literature Notes → Permanent Notes  │
│  • 建立 wikilinks 关联                                   │
│  • 提取原子化概念                                         │
└─────────────────────────────────────────────────────────┘
                         ↓
┌─────────────────────────────────────────────────────────┐
│                    Layer 3: 应用层                       │
│  • 集成到 Projects/Areas/Resources                      │
│  • Dataview 动态查询                                     │
│  • 知识图谱可视化                                        │
└─────────────────────────────────────────────────────────┘
```

### 2.2 核心工作流引擎

#### 工作流 1: 灵感捕获与加工

```
用户输入 → 提取关键词 → 建议标签 → 创建笔记
         → 推荐关联笔记 → 建议存放位置 → 更新图谱
```

**自动化规则**:
- ✅ **直接执行**: 创建笔记、设置基础属性
- ⚠️ **建议确认**: 推荐标签、关联笔记、存放位置

#### 工作流 2: 项目管理

```
每日检查 → 更新任务状态 → 识别阻塞项 → 生成进度报告
         → 推荐相关资源 → 关联技术笔记
```

**触发条件**:
- 用户询问项目状态
- 定时触发（每天 9:00）
- 项目里程碑节点

#### 工作流 3: 知识关联挖掘

```
分析当前笔记 → 查找相似概念 → 推荐相关笔记 → 建议新连接
             → 发现知识孤岛 → 提示补充内容
```

**关联分析策略**:
1. **标签相似度**: 共享相同标签的笔记
2. **链接密度**: 互相引用的笔记群
3. **时间聚类**: 同一时期创建的相关笔记
4. **内容相似**: 使用全文搜索找到相似内容

---

## 3. 核心组件详细设计

### 3.1 组件 1: 智能捕获助手 (Smart Capture Assistant)

#### 功能清单

- [x] 快速创建笔记（使用 Obsidian CLI）
- [x] 自动生成 YAML frontmatter（2026 最佳实践）
- [x] 智能推荐 tags（基于历史 + 内容分析）
- [x] 建议存放位置（PARA 分类规则）
- [x] 推荐关联笔记（图谱分析）

#### 实现细节

**命令**: `捕获灵感`, `记录想法`, `快速记录`

**工作流**:
```bash
# Step 1: 创建笔记
obsidian create \
  name="灵感-$(date +%Y%m%d-%H%M)" \
  content="$USER_INPUT" \
  path="0-Inbox/" \
  silent

# Step 2: 分析内容并推荐标签
RECOMMENDED_TAGS=$(analyze_content_and_suggest_tags "$USER_INPUT")
# 示例输出: "灵感/产品,技术/AI,待处理"

# Step 3: 推荐关联笔记
RELATED_NOTES=$(obsidian search query="$KEYWORDS" limit=5)

# Step 4: 建议存放位置
SUGGESTED_LOCATION=$(suggest_para_location "$CONTENT_TYPE")

# Step 5: 用户确认后执行
if user_confirms; then
  # 设置标签（YAML list 格式）
  obsidian property:set name="tags" value="$RECOMMENDED_TAGS" type=list file="..."

  # 添加关联链接
  for note in $RELATED_NOTES; do
    obsidian append file="..." content="\n- 相关: [[$note]]"
  done
fi
```

**标签推荐算法**:
1. 提取关键词（TF-IDF）
2. 匹配历史高频标签
3. 识别内容类型（代码/创意/任务）
4. 返回 Top 5 标签建议

#### 示例交互

```
用户: 记录一个想法：可以用 AI 自动分析 Obsidian 图谱，发现知识孤岛

Bot-03:
✅ 已创建笔记: 灵感-20260327-2145

📊 分析结果:
- 关键词: AI, Obsidian, 知识图谱, 自动分析
- 建议标签: #灵感/AI #技术/Obsidian #待处理
- 建议位置: 0-Inbox → Zettelkasten/Fleeting-Notes
- 关联笔记:
  • [[知识图谱分析方法]]
  • [[Obsidian 插件开发]]

是否应用建议？(是/否/自定义)
```

---

### 3.2 组件 2: 知识关联引擎 (Knowledge Connection Engine)

#### 功能清单

- [x] 分析笔记语义相似度
- [x] 发现隐含概念连接
- [x] 识别知识孤岛
- [x] 生成知识图谱报告
- [x] 推荐补充内容

#### 实现细节

**命令**: `分析关联`, `找相关笔记`, `发现连接`, `检查孤岛`

**关联分析 API**:

```bash
# 1. 标签相似度分析
obsidian search query="tag:#相关标签" --verbose

# 2. Backlinks 分析
obsidian backlinks file="目标笔记"

# 3. 全文搜索
obsidian search query="关键词1 OR 关键词2" limit=10

# 4. 生成 Dataview 查询（嵌入笔记）
QUERY="
\`\`\`dataview
TABLE
  file.tags as \"标签\",
  file.ctime as \"创建时间\",
  length(file.inlinks) as \"入链数\"
FROM \"\"
WHERE contains(file.tags, \"相关标签\")
  AND file.name != this.file.name
SORT length(file.inlinks) DESC
LIMIT 10
\`\`\`
"
obsidian append file="..." content="$QUERY"
```

**知识孤岛检测**:

```bash
# 检测没有 inlinks 和 outlinks 的笔记
for note in $(obsidian list); do
  inlinks=$(obsidian backlinks file="$note" | wc -l)
  outlinks=$(count_wikilinks_in_note "$note")

  if [ $inlinks -eq 0 ] && [ $outlinks -eq 0 ]; then
    echo "🏝️ 孤岛笔记: $note"
    # 建议关联笔记
    suggest_connections "$note"
  fi
done
```

#### 示例交互

```
用户: 分析笔记 "React Hooks 最佳实践" 的关联

Bot-03:
📊 关联分析报告: [[React Hooks 最佳实践]]

🔗 当前连接:
- 入链 (5): [[React 项目]], [[前端技术栈]], ...
- 出链 (3): [[useState]], [[useEffect]], [[自定义 Hook]]

💡 推荐新连接:
- [[Vue Composition API]] (相似度 78%)
  理由: 同为响应式状态管理方案
- [[状态管理对比]] (相似度 65%)
  理由: 共享标签 #状态管理

🏝️ 发现孤岛笔记:
- [[useReducer 详解]] - 无任何连接
  建议: 添加到 "状态管理" 系列笔记

是否添加推荐的连接？
```

---

### 3.3 组件 3: 定期维护机器人 (Maintenance Bot)

#### 功能清单

- [x] 每周清理 0-Inbox
- [x] 自动归档完成项目
- [x] 生成周报/月报
- [x] 检测断裂 wikilinks
- [x] 更新知识图谱统计

#### 实现细节

**定时任务**:
- 每周日 20:00: 清理 Inbox
- 每月 1 号: 归档已完成项目
- 每天早上: 生成每日任务看板

**清理 Inbox 工作流**:

```bash
# 1. 列出 Inbox 中超过 7 天的笔记
old_notes=$(obsidian list path="0-Inbox" --older-than=7d)

for note in $old_notes; do
  # 2. 分析笔记类型
  note_type=$(classify_note "$note")

  # 3. 建议目标位置
  case $note_type in
    "project")
      target="1-Projects/"
      ;;
    "knowledge")
      target="Zettelkasten/Literature-Notes/"
      ;;
    "resource")
      target="3-Resources/"
      ;;
    *)
      target="4-Archives/"
      ;;
  esac

  echo "📦 $note → $target"
done

# 4. 用户确认后执行
if user_confirms; then
  for note in $old_notes; do
    obsidian move file="$note" path="$target"
  done
fi
```

**断裂链接检测**:

```bash
# 扫描所有 wikilinks
broken_links=""

for note in $(obsidian list); do
  links=$(extract_wikilinks "$note")

  for link in $links; do
    # 检查目标笔记是否存在
    if ! obsidian search query="$link" limit=1 | grep -q "$link"; then
      broken_links+="⚠️ $note → [[$link]] (不存在)\n"
    fi
  done
done

if [ -n "$broken_links" ]; then
  echo "🚨 发现断裂链接:\n$broken_links"
  # 建议修复方案
fi
```

**周报生成**:

```markdown
# 本周回顾 (2026-W13)

## 📊 统计数据
- 新建笔记: 15 篇
- 更新笔记: 23 篇
- 完成任务: 8 个
- 新增连接: 12 条

## 🎯 活跃项目
\`\`\`dataview
TABLE status, priority, updated
FROM "1-Projects"
WHERE updated >= date(today) - dur(7 days)
SORT updated DESC
\`\`\`

## 💡 本周亮点
- 完成 [[HotPlex Provider 重构]]
- 新增技术笔记 5 篇（React, Go, Obsidian）
- 发现知识孤岛 3 个，已修复 2 个

## 📋 下周计划
- [ ] 整理 Inbox (7 个待处理)
- [ ] 归档已完成项目
- [ ] 补充 [[知识图谱]] 相关笔记
```

---

### 3.4 组件 4: 技术学习助手 (Tech Learning Assistant)

#### 功能清单

- [x] 代码片段管理（语法高亮）
- [x] TIL (Today I Learned) 自动归档
- [x] 技术标签体系管理
- [x] 生成技术知识图谱
- [x] 代码审查笔记

#### 实现细节

**命令**: `记录代码`, `TIL`, `技术笔记`, `代码片段`

**TIL 笔记模板**:

```bash
create_til_note() {
  local topic=$1
  local language=$2
  local code=$3
  local key_points=$4

  local note_name="TIL-$(date +%Y-%m-%d)-${topic}"

  # 生成内容
  local content="# Today I Learned: ${topic}

\`\`\`${language}
${code}
\`\`\`

## 关键点
${key_points}

## 相关资源
- [[相关笔记]]

---
#til #${language} #${topic}
"

  # 创建笔记
  obsidian create \
    name="$note_name" \
    content="$content" \
    path="3-Resources/TIL/" \
    silent

  # 设置属性
  obsidian property:set name="type" value="til" file="$note_name"
  obsidian property:set name="language" value="$language" file="$note_name"
  obsidian property:set name="topic" value="$topic" file="$note_name"
  obsidian property:set name="created" value="$(date -I)" file="$note_name"
  obsidian property:set name="tags" value="til,${language},${topic}" type=list file="$note_name"
}
```

**代码片段管理**:

```bash
# 搜索代码片段
search_code_snippets() {
  local language=$1
  local keyword=$2

  obsidian search query="tag:#代码片段 AND ${keyword}" path="3-Resources/代码库/"

  # 或者使用 Dataview
  cat <<EOF
\`\`\`dataview
TABLE
  language as "语言",
  topic as "主题",
  file.ctime as "创建时间"
FROM "3-Resources/代码库"
WHERE contains(file.tags, "代码片段")
  AND language = "${language}"
  AND contains(file.content, "${keyword}")
SORT file.ctime DESC
\`\`\`
EOF
}
```

#### 示例交互

```
用户: TIL: Go 1.26 新增了 sync.WaitGroup.WaitTimeout 方法

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

\`\`\`go
var wg sync.WaitGroup
wg.Add(1)

if wg.WaitTimeout(5 * time.Second) {
    // 超时处理
}
\`\`\`

## 关键点
- 新方法: WaitTimeout(duration) bool
- 替代方案: 之前需要用 context + select 实现
- 适用场景: 需要超时控制的并发等待

🔗 推荐关联:
- [[Go 并发模式]]
- [[sync.WaitGroup 源码分析]]
- [[Go 1.26 新特性汇总]]

是否添加关联？
```

---

### 3.5 组件 5: 创意激发器 (Creativity Booster)

#### 功能清单

- [x] 灵感聚合（从多个笔记提取主题）
- [x] 头脑风暴辅助
- [x] 素材关联推荐
- [x] 创意可视化（Mermaid 图）

#### 实现细节

**命令**: `头脑风暴`, `激发创意`, `聚合灵感`, `创意图谱`

**灵感聚合算法**:

```bash
aggregate_inspirations() {
  local topic=$1

  # 1. 搜索相关灵感笔记
  inspirations=$(obsidian search query="tag:#灵感 AND ${topic}" path="0-Inbox,Zettelkasten/Fleeting-Notes")

  # 2. 提取关键概念
  concepts=$(extract_key_concepts "$inspirations")

  # 3. 生成概念图（Mermaid）
  cat <<EOF
\`\`\`mermaid
graph TD
    A[${topic}] --> B[概念1]
    A --> C[概念2]
    B --> D[具体想法1]
    B --> E[具体想法2]
    C --> F[具体想法3]

    classDef idea fill:#f9f,stroke:#333
    class A,B,C idea
\`\`\`
EOF

  # 4. 推荐素材
  materials=$(find_related_materials "$concepts")
  echo "📚 相关素材:\n$materials"
}
```

**头脑风暴工作流**:

```bash
brainstorm_session() {
  local topic=$1

  # 创建头脑风暴笔记
  obsidian create \
    name="头脑风暴-${topic}-$(date +%Y%m%d)" \
    content="# 头脑风暴: ${topic}

## 核心问题
${QUESTION}

## 想法列表
- 💡 想法1
- 💡 想法2
- 💡 想法3

## 概念图
\`\`\`mermaid
graph TD
    A[${topic}] --> B[...]
\`\`\`

## 相关素材
- [[素材1]]
- [[素材2]]

---
#头脑风暴 #${topic}
" \
    path="0-Inbox/" \
    silent
}
```

#### 示例交互

```
用户: 头脑风暴：如何用 AI 提升 Obsidian 使用体验

Bot-03:
🎯 头脑风暴会话已开启

📊 灵感聚合（来自 5 个笔记）:
- 自动分析知识图谱
- 智能推荐相关笔记
- 语音快速捕获
- AI 辅助写作
- 自动生成摘要

💡 概念关联图:
[Mermaid 图: AI → 图谱分析 → 推荐系统 → 知识网络]

📚 相关素材:
- [[Obsidian 插件开发]]
- [[知识图谱算法]]
- [[AI 辅助写作工具]]

🔧 工具建议:
- 使用 Templater 实现自动化
- 集成 Claude API 做内容分析
- 用 Dataview 构建动态看板

继续深入哪个方向？
```

---

## 4. 交互模式规范

### 4.1 混合交互模式 (Hybrid Mode)

**直接执行** (无需确认):
- ✅ 创建笔记
- ✅ 设置基础属性（created, type）
- ✅ 添加标签（基于用户明确指示）
- ✅ 更新任务状态
- ✅ 定期维护任务

**建议确认** (需要用户确认):
- ⚠️ 推荐标签（用户未明确指定时）
- ⚠️ 推荐关联笔记
- ⚠️ 建议移动笔记位置
- ⚠️ 批量操作（>5 个笔记）
- ⚠️ 删除或归档操作

**禁止操作** (必须用户手动):
- ❌ 删除笔记
- ❌ 修改关键属性（title, aliases）
- ❌ 合并笔记

### 4.2 交互示例

#### 场景 1: 快速捕获（直接执行）

```
用户: 记录：Go 1.26 的 slices 包很实用

Bot-03:
✅ 已创建笔记: 灵感-20260327-2145
📍 位置: 0-Inbox/
🏷️ 标签: #Go #技术/标准库
🔗 关联: [[Go 1.26 新特性]]

笔记已准备就绪，随时可以扩展。
```

#### 场景 2: 知识关联（建议确认）

```
用户: 分析 [[React Hooks]] 的关联

Bot-03:
📊 关联分析完成

💡 推荐新连接:
1. [[Vue Composition API]] (相似度 78%)
2. [[状态管理模式对比]] (相似度 65%)

是否添加这些连接？(全选/部分/跳过)
```

#### 场景 3: 定期维护（主动执行）

```
Bot-03 (周日 20:00):
🔧 每周维护开始

📦 Inbox 清理:
- 7 个笔记超过 7 天
- 建议移动:
  • 灵感-AI-图谱 → Zettelkasten/Fleeting-Notes
  • 项目-重构计划 → 1-Projects/

是否执行建议的整理？(是/否/查看详情)
```

---

## 5. 数据流设计

### 5.1 知识流向

```
[用户输入]
    ↓
[智能捕获] → 0-Inbox/ (快速捕获)
    ↓
[内容分析] → 提取关键词、推荐标签
    ↓
[关联挖掘] → 推荐相关笔记、建议连接
    ↓
[用户确认] → 应用标签、添加连接
    ↓
[知识沉淀]
    ├→ Zettelkasten/ (知识网络)
    ├→ 1-Projects/ (项目管理)
    ├→ 2-Areas/ (责任领域)
    └→ 3-Resources/ (参考资料)
    ↓
[定期维护] → 归档、清理、报告
    ↓
[4-Archives/] (历史归档)
```

### 5.2 Obsidian CLI 使用规范

#### 5.2.1 创建笔记

```bash
# ✅ 正确：使用 Obsidian CLI
obsidian create \
  name="笔记名称" \
  content="内容" \
  path="目标路径/" \
  silent  # 不自动打开

# ❌ 错误：直接写文件
echo "content" > /path/to/vault/note.md
```

#### 5.2.2 设置属性（2026 最佳实践）

```bash
# ✅ 正确：使用 type=list 创建 YAML list
tags="tag1,tag2,tag3"
obsidian property:set name="tags" value="$tags" type=list file="笔记"

# ❌ 错误：字符串格式
obsidian property:set name="tags" value="tag1,tag2,tag3" file="笔记"
# 结果: tags: "tag1,tag2,tag3" (错误格式)
```

#### 5.2.3 搜索与查询

```bash
# 搜索笔记
obsidian search query="关键词" limit=10

# 精确匹配
obsidian search query="精确名称" --exact

# 按路径搜索
obsidian search query="关键词" path="1-Projects/"
```

#### 5.2.4 关联管理

```bash
# 查看反向链接
obsidian backlinks file="目标笔记"

# 添加关联（追加内容）
obsidian append file="笔记A" content="\n- 相关: [[笔记B]]"
```

---

## 6. 错误处理与容错

### 6.1 常见错误场景

| 错误 | 检测方法 | 处理策略 |
|------|---------|---------|
| **Obsidian 未运行** | `obsidian info` 失败 | 提示用户打开 Obsidian |
| **笔记已存在** | `obsidian search` 返回结果 | 使用 `overwrite` 或建议重命名 |
| **无效标签格式** | 标签包含特殊字符 | 自动清理或提示修正 |
| **断裂的 wikilink** | 目标笔记不存在 | 建议创建或移除链接 |
| **权限不足** | 文件操作失败 | 检查文件权限，提示用户 |

### 6.2 错误恢复机制

```bash
# 示例：创建笔记时的错误处理
create_note_with_retry() {
  local note_name=$1
  local content=$2
  local max_retries=3
  local retry=0

  while [ $retry -lt $max_retries ]; do
    if obsidian create name="$note_name" content="$content" silent; then
      echo "✅ 笔记创建成功"
      return 0
    else
      retry=$((retry + 1))
      echo "⚠️ 创建失败，重试 $retry/$max_retries"

      # 检查 Obsidian 是否运行
      if ! obsidian info &>/dev/null; then
        echo "❌ Obsidian 未运行，请先打开 Obsidian"
        return 1
      fi

      sleep 2
    fi
  done

  echo "❌ 创建笔记失败，请手动检查"
  return 1
}
```

---

## 7. 性能优化

### 7.1 批量操作优化

**问题**: 逐个设置属性会产生大量 CLI 调用

**优化方案**:

```bash
# ❌ 低效：10 次 CLI 调用
obsidian property:set name="title" value="..." file="$note"
obsidian property:set name="type" value="..." file="$note"
# ... 8 more calls

# ✅ 高效：创建时设置属性（如果 CLI 支持）
obsidian create \
  name="$note_name" \
  content="$content" \
  --property title="..." \
  --property type="..." \
  silent

# ✅ 或者：使用 JSON 批量设置（如果 CLI 支持）
obsidian property:set-batch \
  file="$note" \
  properties='{"title": "...", "type": "..."}'
```

### 7.2 缓存策略

- 缓存常用标签列表（1 小时刷新）
- 缓存笔记索引（5 分钟刷新）
- 缓存知识图谱结构（仅在变更时更新）

---

## 8. 测试策略

### 8.1 单元测试

```bash
# 测试标签推荐算法
test_tag_recommendation() {
  content="React Hooks 使用心得"
  tags=$(recommend_tags "$content")

  # 验证标签格式
  assert_contains "$tags" "React"
  assert_contains "$tags" "技术"
}

# 测试关联分析
test_connection_analysis() {
  note="React Hooks 最佳实践"
  related=$(find_related_notes "$note")

  # 验证返回的笔记确实相关
  for n in $related; do
    assert_similarity "$note" "$n" > 0.5
  done
}
```

### 8.2 集成测试

```bash
# 测试完整工作流：捕获 → 分析 → 关联
test_capture_workflow() {
  # 1. 捕获
  note=$(capture_idea "AI 知识图谱分析")

  # 2. 验证笔记创建
  assert_note_exists "$note"

  # 3. 验证标签设置
  tags=$(obsidian property:get name="tags" file="$note")
  assert_not_empty "$tags"

  # 4. 验证关联添加
  related=$(obsidian property:get name="related" file="$note")
  assert_not_empty "$related"
}
```

---

## 9. 部署与配置

### 9.1 Bot-03 配置文件

**文件**: `docker/matrix/configs/bot-03/slack.yaml`

```yaml
inherits: ./base/slack.yaml

ai:
  system_prompt: |
    你是 Bot-03，一位 **AI 知识管家**，深度融合 Claude Code 与 Obsidian CLI，
    主动管理用户的第二大脑，挖掘知识关联，提供智能建议。

    ## 核心职责

    ### 1. 智能知识捕获
    - 使用 `obsidian create` 快速创建笔记
    - 自动生成符合 YAML 2026 最佳实践的 frontmatter
    - 智能推荐 tags（YAML list 格式）
    - 建议存放位置（基于 PARA 方法）
    - 推荐关联笔记（图谱分析）

    ### 2. 项目管理
    - 追踪任务状态（使用 Tasks 插件语法）
    - 生成进度报告（Dataview 查询）
    - 识别阻塞项和风险
    - 关联技术笔记和资源

    ### 3. 知识关联挖掘
    - 分析笔记语义相似度
    - 发现隐含概念连接
    - 识别知识孤岛（无连接笔记）
    - 生成知识图谱报告

    ### 4. 技术学习助手
    - 管理 TIL (Today I Learned) 笔记
    - 代码片段管理（带语法高亮）
    - 技术标签体系维护
    - 生成技术知识图谱

    ### 5. 创意激发器
    - 灵感聚合（从多个笔记提取主题）
    - 头脑风暴辅助（Mermaid 图）
    - 素材关联推荐
    - 创意可视化

    ### 6. 定期维护
    - 每周清理 0-Inbox（超过 7 天的笔记）
    - 自动归档完成项目
    - 检测断裂的 wikilinks
    - 生成周报/月报

    ## 交互模式：混合模式

    ### 直接执行（无需确认）
    - ✅ 创建笔记
    - ✅ 设置基础属性（created, type）
    - ✅ 添加标签（基于明确指示）
    - ✅ 更新任务状态
    - ✅ 定期维护任务

    ### 建议确认（需要用户确认）
    - ⚠️ 推荐标签（用户未明确指定）
    - ⚠️ 推荐关联笔记
    - ⚠️ 建议移动笔记位置
    - ⚠️ 批量操作（>5 个笔记）
    - ⚠️ 删除或归档操作

    ## Obsidian CLI 使用规范（2026 最佳实践）

    ### 创建笔记
    ```bash
    obsidian create name="笔记名" content="内容" path="路径/" silent
    ```

    ### 设置属性（关键：YAML list 格式）
    ```bash
    # ✅ 正确：使用 type=list
    tags="tag1,tag2,tag3"
    obsidian property:set name="tags" value="$tags" type=list file="笔记"

    # ❌ 错误：字符串格式
    # 会导致 tags: "tag1,tag2,tag3" (不是 list)
    ```

    ### 搜索与查询
    ```bash
    obsidian search query="关键词" limit=10
    obsidian backlinks file="笔记"
    ```

    ## Vault 结构

    用户 vault: /Users/huangzhonghui/Documents/second-brain

    ### PARA 方法
    - 0-Inbox/ (快速捕获)
    - 1-Projects/ (活跃项目)
    - 2-Areas/ (责任领域)
    - 3-Resources/ (参考资料)
    - 4-Archives/ (已完成)

    ### Zettelkasten 系统
    - Zettelkasten/Fleeting-Notes/ (瞬时笔记)
    - Zettelkasten/Literature-Notes/ (文献笔记)
    - Zettelkasten/Permanent-Notes/ (永久笔记)
    - Zettelkasten/Concept-Maps/ (概念图谱)

    ### 模板库
    - Templates/Template-每日笔记.md
    - Templates/Template-技术开发.md
    - Templates/Template-学习研究.md
    - Templates/Template-项目管理.md
    - Templates/Template-创意写作.md
    - Templates/Template-快速捕获.md
    - Templates/Template-Zettelkasten永久笔记.md

    ## 核心工作流

    ### 灵感捕获
    1. 用户输入 → 提取关键词
    2. 创建笔记到 0-Inbox/
    3. 推荐标签 → 用户确认 → 设置 tags (type=list)
    4. 推荐关联笔记 → 用户确认 → 添加 wikilinks
    5. 建议存放位置 → 用户确认 → 移动笔记

    ### 知识关联
    1. 分析当前笔记（关键词、标签、内容）
    2. 搜索相关笔记（obsidian search）
    3. 计算相似度（标签匹配 + 内容相似）
    4. 推荐新连接（Top 5）
    5. 用户确认 → 添加 wikilinks

    ### 定期维护
    - 每天 9:00: 生成每日任务看板
    - 每周日 20:00: 清理 Inbox、检测断裂链接
    - 每月 1 号: 归档已完成项目、生成月报

    ## 错误处理

    - Obsidian 未运行: 提示用户打开 Obsidian
    - 笔记已存在: 使用 overwrite 或建议重命名
    - 无效标签格式: 自动清理特殊字符
    - 断裂 wikilink: 建议创建或移除链接
    - 权限不足: 检查文件权限

    ## 技能调用

    必须使用以下 skills:
    - obsidian-cli: 所有 Obsidian 操作
    - obsidian-markdown: 生成符合 Obsidian 规范的内容
    - smart-filer: PARA 文件组织（如果需要）

    可选 skills:
    - brainstorming: 复杂创意问题
    - mermaid-diagrams: 生成概念图

    ## 示例对话

    用户: 记录一个想法：用 AI 分析 Obsidian 图谱发现知识孤岛

    Bot-03:
    ✅ 已创建笔记: 灵感-20260327-2145
    📍 位置: 0-Inbox/

    📊 分析结果:
    - 关键词: AI, Obsidian, 知识图谱, 知识孤岛
    - 建议标签: #灵感/AI #技术/Obsidian #待处理
    - 关联笔记:
      • [[知识图谱分析方法]]
      • [[Obsidian 插件开发]]

    是否应用建议？(是/部分/跳过)

    ---

    用户: 分析 [[React Hooks]] 的关联

    Bot-03:
    📊 关联分析: [[React Hooks]]

    🔗 当前连接:
    - 入链 (5): [[React 项目]], [[前端技术栈]], ...
    - 出链 (3): [[useState]], [[useEffect]], ...

    💡 推荐新连接:
    1. [[Vue Composition API]] (相似度 78%)
       理由: 同为响应式状态管理方案
    2. [[状态管理对比]] (相似度 65%)
       理由: 共享标签 #状态管理

    是否添加推荐的连接？

  provider: opencode-server
  engine:
    type: native
    config:
      model: claude-sonnet-4-6
      temperature: 0.7
      max_tokens: 4096

security:
  permission:
    bot_user_id: U0AL7H8UU75
    broadcast_response: ""
```

---

## 10. 后续优化方向

### 10.1 短期优化（1-2 周）

- [ ] 实现标签推荐算法（基于历史 + TF-IDF）
- [ ] 实现知识孤岛检测
- [ ] 完善错误处理和重试机制
- [ ] 添加性能监控（CLI 调用次数）

### 10.2 中期优化（1-2 月）

- [ ] 集成 Graph Analysis 插件 API
- [ ] 实现语义相似度计算（基于 embeddings）
- [ ] 添加自然语言查询（"找最近学习的 React 知识"）
- [ ] 生成知识演进报告（时间维度）

### 10.3 长期愿景（3-6 月）

- [ ] 多模态知识管理（图片、PDF、视频）
- [ ] 跨 vault 知识关联
- [ ] AI 辅助写作（基于笔记库）
- [ ] 知识图谱可视化（D3.js）

---

## 11. 参考资源

### 官方文档
- [Obsidian CLI](https://help.obsidian.md/cli)
- [Obsidian Flavored Markdown](https://help.obsidian.md/obsidian-flavored-markdown)
- [Dataview 文档](https://blacksmithgu.github.io/obsidian-dataview/)

### 方法论
- [PARA Method](https://fortelabs.com/blog/para-method/)
- [Zettelkasten Introduction](https://zettelkasten.de/introduction/)
- [Building a Second Brain](https://www.buildingasecondbrain.com/)

### 社区资源
- [Obsidian Community Forum](https://forum.obsidian.md)
- [Obsidian Discord](https://discord.gg/obsidianmd)

---

**设计完成日期**: 2026-03-27
**下一步**: 实现配置文件并测试核心工作流
