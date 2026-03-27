# Obsidian 同步功能设计（Obsidian CLI 版本）

> **版本**: v2.0 (使用 Obsidian CLI)
> **更新日期**: 2026-03-27
> **核心变化**: 从文件系统操作迁移到 Obsidian CLI

---

## 1. 核心设计决策

### 1.1 为什么使用 Obsidian CLI？

| 维度 | 文件系统操作 | Obsidian CLI |
|------|-------------|--------------|
| **Frontmatter 管理** | ⚠️ 手动解析/生成 YAML | ✅ `property:set` 自动处理 |
| **Vault 感知** | ❌ 需要手动指定路径 | ✅ 自动检测活跃 vault |
| **搜索** | ❌ 需要遍历文件 | ✅ `obsidian search` 原生支持 |
| **打开笔记** | ❌ 不支持 | ✅ `obsidian open` |
| **实时更新** | ❌ 需要 reload | ✅ Obsidian 自动刷新 |
| **错误处理** | ⚠️ 手动检查 | ✅ CLI 提供明确错误 |
| **代码简洁性** | ❌ 大量文件 I/O 代码 | ✅ 简洁的 CLI 命令 |

### 1.2 同步策略
- **方向**：单向同步（GitHub → Obsidian）
- **模式**：增量同步（默认）+ 强制全量同步（可选）
- **触发**：
  - 手动触发（用户命令）
  - 定时触发（通过 cron）
  - Webhook 触发（未来扩展）

### 1.3 文件组织（PARA 方法）

**推荐结构**：
```
1-Projects/
└── <project-name>/
    └── Issues/
        ├── Issue-{number}-{title-slug}.md
        └── Obsidian Sync State.md
```

**示例**：
- `1-Projects/HotPlex/Issues/Issue-358-feat-opencode-server-provider-server-mode.md`
- `1-Projects/HotPlex/Issues/Obsidian Sync State.md`

**文件命名**：`Issue-{number}-{title-slug}.md`
- 例如：`Issue-358-feat-opencode-server-provider-server-mode.md`

**优势**：
- ✅ 符合 PARA 方法（Projects > Areas > Resources > Archives）
- ✅ 清晰的项目上下文
- ✅ 支持 Dataview 项目级查询
- ✅ 自动同步到移动端时保持结构

### 1.4 状态持久化

**方案A：使用 Obsidian Properties（强烈推荐）**

直接在 Obsidian 中创建一个 **Sync State** 笔记（与 issues 在同一文件夹）：

```markdown
---
cssclasses:
  - sync-state
last_sync_time: 2026-03-27T12:00:00Z
sync_mode: incremental
total_synced: 3
---

# Obsidian Sync State

## Last Sync
- **Time**: 2026-03-27T12:00:00Z
- **Mode**: incremental
- **Issues Processed**: 3

## Synced Issues

| Issue # | GitHub Updated | Sync Version | Status |
|---------|----------------|--------------|--------|
| 335 | 2026-03-27T11:30:00Z | 1 | open |
| 336 | 2026-03-26T15:00:00Z | 1 | open |
| 337 | 2026-03-25T09:00:00Z | 1 | closed |

## Statistics
- **Total Synced**: 3
- **Open**: 2
- **Closed**: 1
- **Last Full Sync**: 2026-03-22T10:00:00Z

## Folder Structure
```
1-Projects/
└── HotPlex/
    └── Issues/
        ├── Issue-335-...md
        ├── ... (22 issues total)
        └── Obsidian Sync State.md (this file)
```
```

**优势**：
- ✅ 无需额外文件
- ✅ 可在 Obsidian 中直接查看/编辑
- ✅ 支持 Dataview 查询
- ✅ 自动同步到移动端
- ✅ 集中管理（与 issues 在同一文件夹）

**方案B：轻量级 JSON（仅用于外部工具集成）**

如果需要与外部工具集成，保留 `.issue-obsidian-sync.json`，但简化结构：

```json
{
  "last_sync_time": "2026-03-27T12:00:00Z",
  "sync_mode": "incremental",
  "synced_issues": {
    "335": {
      "github_updated_at": "2026-03-27T11:30:00Z",
      "sync_version": 1
    }
  }
}
```

**推荐**：优先使用方案 A（Obsidian 笔记），仅在需要外部工具集成时使用方案 B。

---

## 2. 核心工作流（使用 Obsidian CLI）

### 2.1 增量同步流程

```bash
# 1. 获取上次同步时间
last_sync=$(obsidian property:get name="last_sync_time" file="Obsidian Sync State")

# 2. 获取更新的 issues（使用 GitHub MCP）
issues=$(mcp__github__list_issues updated_since="$last_sync")

# 3. 处理每个 issue
for issue in $issues; do
  note_name="Issue-${issue.number}-${slug(issue.title)}"

  # 检查笔记是否存在
  if obsidian search query="$note_name" --exact; then
    # 更新现有笔记
    obsidian property:set name="github_updated_at" value="${issue.updated_at}" file="$note_name"
    obsidian property:set name="priority" value="$(extract_priority $issue)" file="$note_name"
    # ... 更新其他属性
  else
    # 创建新笔记
    obsidian create \
      name="$note_name" \
      content="$(generate_content $issue)" \
      path="Issues/" \
      silent
  fi
done

# 4. 更新同步状态
obsidian property:set name="last_sync_time" value="$(now)" file="Obsidian Sync State"
```

### 2.2 创建 Issue 笔记

```bash
create_issue_note() {
  local issue=$1
  local note_name="Issue-${issue.number}-${slug(issue.title)}"

  # 生成内容（不含 frontmatter）
  local content=$(generate_markdown_content $issue)

  # 使用 Obsidian CLI 创建笔记
  obsidian create \
    name="$note_name" \
    content="$content" \
    path="Issues/" \
    silent \
    overwrite

  # 设置 properties
  obsidian property:set name="title" value="Issue #${issue.number}: ${issue.title}" file="$note_name"
  obsidian property:set name="issue_number" value="${issue.number}" file="$note_name"
  obsidian property:set name="issue_url" value="${issue.html_url}" file="$note_name"
  obsidian property:set name="github_status" value="${issue.state}" file="$note_name"
  obsidian property:set name="priority" value="$(extract_priority $issue)" file="$note_name"
  obsidian property:set name="type" value="$(extract_type $issue)" file="$note_name"
  # ... 设置其他 properties

  # 添加标签
  for tag in $(extract_tags $issue); do
    obsidian property:append name="tags" value="$tag" file="$note_name"
  done
}
```

### 2.3 更新 Issue 笔记

```bash
update_issue_note() {
  local issue=$1
  local note_name="Issue-${issue.number}-${slug(issue.title)}"

  # 更新 properties
  obsidian property:set name="github_updated_at" value="${issue.updated_at}" file="$note_name"
  obsidian property:set name="github_status" value="${issue.state}" file="$note_name"

  if [ -n "$issue.closed_at" ]; then
    obsidian property:set name="closed_at" value="${issue.closed_at}" file="$note_name"
  fi

  # 更新标签
  obsidian property:set name="tags" value="" file="$note_name"  # 清空
  for tag in $(extract_tags $issue); do
    obsidian property:append name="tags" value="$tag" file="$note_name"
  done

  # 如果内容有重大变化，重写整个笔记
  if content_changed $issue; then
    local content=$(generate_markdown_content $issue)
    obsidian edit file="$note_name" content="$content"
  fi
}
```

### 2.4 Tags 格式（Obsidian 2026 最佳实践）

**关键要求**：Tags 必须使用 YAML list 格式，不是逗号分隔字符串。

**错误示例**：
```bash
# ❌ 错误：创建逗号分隔字符串
obsidian property:set name="tags" value="priority/high,type/bug,status/open" file="$note_name"
```

这会生成错误的 frontmatter：
```yaml
---
tags: "priority/high,type/bug,status/open"  # ❌ 字符串，不是 list
---
```

**正确示例**：
```bash
# ✅ 正确：使用 type=list 参数创建 YAML list
tags_array=("priority/high" "type/bug" "status/open")
tags_str=$(IFS=,; echo "${tags_array[*]}")
obsidian property:set name="tags" value="$tags_str" type=list file="$note_name"
```

这会生成正确的 frontmatter：
```yaml
---
tags:
  - priority/high
  - type/bug
  - status/open
---
```

**为什么重要**：
- Obsidian 2026+ 只将 YAML list 识别为有效 tags
- 逗号分隔字符串会被当作单个字符串，无法用于 Dataview 查询
- Tags 面板、图谱视图、搜索功能都依赖正确的 list 格式
- 错误格式会导致 tags 无法在 Obsidian UI 中正确显示和操作

**批量创建 tags 的最佳实践**：
```bash
# 提取 tags 数组
extract_obsidian_tags() {
  local issue=$1
  local tags=()

  # 状态标签
  if [ "$issue.state" = "OPEN" ]; then
    tags+=("issue/open")
  else
    tags+=("issue/closed")
  fi

  # 从 labels 提取
  for label in ${issue.labels}; do
    case $label in
      priority/*|type/*|status/*|platform/*|area/*)
        tags+=("$label")
        ;;
    esac
  done

  echo "${tags[@]}"
}

# 使用（创建 YAML list）
tags=$(extract_obsidian_tags "$issue")
tags_str=$(IFS=,; echo "$tags")
obsidian property:set name="tags" value="$tags_str" type=list file="$note_name"
```

---

## 3. 笔记模板设计

### 3.1 Properties（自动管理）

使用 Obsidian CLI 后，frontmatter 由 Obsidian 自动管理，无需手动生成 YAML：

```yaml
---
title: "Issue #335: Admin API endpoints unreachable"
issue_number: 335
issue_url: https://github.com/hrygo/hotplex/issues/335
github_status: open
github_updated_at: 2026-03-27T11:30:00Z
priority: critical
type: bug
size: medium
area: admin
platform: []
created_at: 2026-03-22T10:30:00Z
updated_at: 2026-03-27T11:30:00Z
closed_at:
assignees:
  - hrygo
labels:
  - priority/critical
  - type/bug
  - size/medium
  - status/ready-for-work
  - area/admin
tags:
  - issue/active
  - priority/critical
  - type/bug
  - status/ready-for-work
cssclasses:
  - issue-note
---
```

**设置方式**：
```bash
obsidian property:set name="title" value="Issue #335: Admin API endpoints unreachable" file="Issue-335-..."
obsidian property:set name="issue_number" value=335 file="Issue-335-..."
# ... 以此类推
```

### 3.2 Markdown 内容模板

```markdown
# Issue #335: Admin API endpoints unreachable

> [!info] Issue Metadata
> - **Status**: `#issue/open` | **Priority**: `#priority/critical`
> - **Type**: `#type/bug` | **Size**: `#size/medium`
> - **Created**: 2026-03-22 | **Updated**: 2026-03-27
> - **Assignee**: @hrygo
> - **GitHub**: [#335](https://github.com/hrygo/hotplex/issues/335)

## Description

Admin API endpoints (port 9080) are unreachable from the host machine when running in Docker, despite port mapping being configured correctly in docker-compose.yml.

### Steps to Reproduce

1. Run `make docker-up`
2. Access `http://localhost:9080/status` from host
3. Connection refused error

### Expected Behavior

Admin API should be accessible at `http://localhost:9080`

### Actual Behavior

Connection refused

## Labels

`#priority/critical` `#type/bug` `#size/medium` `#status/ready-for-work` `#area/admin`

## Related Issues

- Related to [[Issue-334-admin-api-docker-bind]]
- Blocks [[Issue-340-multi-level-typing]]

## Comments

> [!quote] Latest Comments (3 total)
> - **@hrygo** (2026-03-27): Investigating Docker networking...
> - **@claude** (2026-03-26): Confirmed this is a bind mount issue...
> - **@hrygo** (2026-03-25): Initial triage complete

---

**Sync Info**:
- Last synced: 2026-03-27 12:00:00 UTC
- Sync version: 1
- Source: [GitHub Issue #335](https://github.com/hrygo/hotplex/issues/335)
```

---

## 4. 标签映射规则

### 4.1 GitHub Labels → Obsidian Tags

使用函数提取标签，然后通过 `obsidian property:append` 添加：

```bash
extract_obsidian_tags() {
  local issue=$1
  local tags=()

  # 状态标签
  if [ "$issue.state" = "OPEN" ]; then
    tags+=("issue/open")
  else
    tags+=("issue/closed")
  fi

  # 从 labels 提取
  for label in $issue.labels; do
    case $label in
      priority/*|type/*|status/*|platform/*|area/*)
        tags+=("$label")
        ;;
      good\ first\ issue)
        tags+=("good-first-issue")
        ;;
      help\ wanted)
        tags+=("help-wanted")
        ;;
    esac
  done

  echo "${tags[@]}"
}

# 使用
tags=$(extract_obsidian_tags $issue)
for tag in $tags; do
  obsidian property:append name="tags" value="$tag" file="$note_name"
done
```

---

## 5. 核心函数实现（Bash + Obsidian CLI）

### 5.1 主同步函数

```bash
sync_issues_to_obsidian() {
  local vault_path=$1
  local sync_mode=${2:-incremental}
  local force_full=${3:-false}

  # 1. 检查 Obsidian 是否运行
  if ! obsidian info &>/dev/null; then
    echo "❌ Obsidian is not running. Please open Obsidian first."
    return 1
  fi

  # 2. 获取上次同步时间
  local last_sync=""
  if [ "$sync_mode" = "incremental" ] && [ "$force_full" = false ]; then
    last_sync=$(obsidian property:get name="last_sync_time" file="Obsidian Sync State" 2>/dev/null || echo "")
  fi

  # 3. 获取 issues
  local issues
  if [ -n "$last_sync" ]; then
    echo "📥 Fetching issues updated since $last_sync..."
    issues=$(fetch_updated_issues "$last_sync")
  else
    echo "📥 Fetching all open issues..."
    issues=$(fetch_all_issues)
  fi

  # 4. 处理每个 issue
  local created=0
  local updated=0
  local skipped=0

  for issue in $issues; do
    local note_name="Issue-${issue.number}-${slug(issue.title)}"

    # 检查笔记是否存在
    if obsidian search query="$note_name" limit=1 | grep -q "$note_name"; then
      # 检查是否需要更新
      local current_updated=$(obsidian property:get name="github_updated_at" file="$note_name")
      if [ "$current_updated" != "$issue.updated_at" ]; then
        echo "🔄 Updating $note_name..."
        update_issue_note "$issue"
        ((updated++))
      else
        ((skipped++))
      fi
    else
      echo "✨ Creating $note_name..."
      create_issue_note "$issue"
      ((created++))
    fi
  done

  # 5. 更新同步状态
  local now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  obsidian property:set name="last_sync_time" value="$now" file="Obsidian Sync State"

  # 6. 生成报告
  echo ""
  echo "✅ Sync completed!"
  echo "   - Created: $created"
  echo "   - Updated: $updated"
  echo "   - Skipped: $skipped"
  echo "   - Total: $((created + updated + skipped))"
}
```

### 5.2 创建笔记函数

```bash
create_issue_note() {
  local issue=$1
  local note_name="Issue-${issue.number}-$(slugify "${issue.title}")"

  # 生成 Markdown 内容（不含 frontmatter）
  local content
  content="# Issue #${issue.number}: ${issue.title}\n\n"
  content+="> [!info] Issue Metadata\n"
  content+="> - **Status**: \`#issue/${issue.state}\` | **Priority**: \`#priority/$(extract_priority "$issue")\`\n"
  content+="> - **Type**: \`#type/$(extract_type "$issue")\` | **Size**: \`#size/$(extract_size "$issue")\`\n"
  content+="> - **Created**: ${issue.created_at} | **Updated**: ${issue.updated_at}\n"
  content+="> - **GitHub**: [#${issue.number}](${issue.html_url})\n\n"
  content+="## Description\n\n"
  content+="${issue.body:-_No description provided._}\n\n"
  content+="## Labels\n\n"
  content+="$(generate_label_tags "$issue")\n\n"
  content+="---\n\n"
  content+="**Sync Info**:\n"
  content+="- Last synced: $(date -u +"%Y-%m-%d %H:%M:%S UTC")\n"
  content+="- Source: [GitHub Issue #${issue.number}](${issue.html_url})\n"

  # 创建笔记（使用正确的项目文件夹路径）
  obsidian create \
    name="$note_name" \
    content="$content" \
    path="1-Projects/HotPlex/Issues/" \
    silent

  # 批量设置 properties（优先使用单次调用或最少的调用）
  # 注意：以下示例假设 CLI 支持批量操作或使用最少的调用次数
  obsidian property:set name="title" value="Issue #${issue.number}: ${issue.title}" file="$note_name"
  obsidian property:set name="issue_number" value="${issue.number}" file="$note_name"
  obsidian property:set name="issue_url" value="${issue.html_url}" file="$note_name"
  obsidian property:set name="github_status" value="${issue.state}" file="$note_name"
  obsidian property:set name="github_updated_at" value="${issue.updated_at}" file="$note_name"
  obsidian property:set name="priority" value="$(extract_priority "$issue")" file="$note_name"
  obsidian property:set name="type" value="$(extract_type "$issue")" file="$note_name"
  obsidian property:set name="size" value="$(extract_size "$issue")" file="$note_name"
  obsidian property:set name="area" value="$(extract_area "$issue")" file="$note_name"
  obsidian property:set name="created_at" value="${issue.created_at}" file="$note_name"
  obsidian property:set name="updated_at" value="${issue.updated_at}" file="$note_name"
  obsidian property:set name="cssclasses" value="issue-note" file="$note_name"

  # ✅ 关键：使用 type=list 参数创建 YAML list 格式的 tags
  local tags=$(extract_obsidian_tags "$issue")
  local tags_str=$(IFS=,; echo "$tags")
  obsidian property:set name="tags" value="$tags_str" type=list file="$note_name"

  # 设置 labels 数组（使用 type=list）
  local labels_str=$(IFS=,; echo "${issue.labels}")
  obsidian property:set name="labels" value="$labels_str" type=list file="$note_name"
}
```

**关键改进**：
1. **文件夹路径**：使用 `1-Projects/HotPlex/Issues/` 而不是 `Issues/`
2. **Tags 格式**：使用 `type=list` 参数确保创建 YAML list
3. **批量操作**：避免循环 `property:append`，使用单次 `property:set` + `type=list`

### 5.3 辅助函数

```bash
# Slugify 标题
slugify() {
  local title=$1
  echo "$title" | \
    tr '[:upper:]' '[:lower:]' | \
    sed 's/[^a-z0-9]/-/g' | \
    sed 's/--*/-/g' | \
    sed 's/^-//;s/-$//' | \
    cut -c1-50
}

# 提取优先级
extract_priority() {
  local issue=$1
  for label in ${issue.labels}; do
    case $label in
      priority/critical|priority/high|priority/medium|priority/low)
        echo "${label#priority/}"
        return
        ;;
    esac
  done
  echo "medium"  # 默认
}

# 生成标签 Markdown
generate_label_tags() {
  local issue=$1
  local tags=""
  for label in ${issue.labels}; do
    tags+="\`#${label}\` "
  done
  echo "$tags"
}
```

---

## 6. 同步状态笔记（替代 JSON）

### 6.1 创建同步状态笔记

```bash
init_sync_state() {
  local content
  content="# Obsidian Sync State\n\n"
  content+="## Last Sync\n"
  content+="- **Time**: Never\n"
  content+="- **Mode**: incremental\n"
  content+="- **Issues Processed**: 0\n\n"
  content+="## Statistics\n"
  content+="- **Total Synced**: 0\n"
  content+="- **Open**: 0\n"
  content+="- **Closed**: 0\n"

  obsidian create \
    name="Obsidian Sync State" \
    content="$content" \
    path="Issues/" \
    silent \
    overwrite

  obsidian property:set name="last_sync_time" value="" file="Obsidian Sync State"
  obsidian property:set name="sync_mode" value="incremental" file="Obsidian Sync State"
  obsidian property:set name="total_synced" value=0 file="Obsidian Sync State"
  obsidian property:set name="cssclasses" value="sync-state" file="Obsidian Sync State"
}
```

### 6.2 更新同步状态

```bash
update_sync_state() {
  local total=$1
  local open=$2
  local closed=$3

  local now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  obsidian property:set name="last_sync_time" value="$now" file="Obsidian Sync State"
  obsidian property:set name="total_synced" value="$total" file="Obsidian Sync State"

  # 更新内容中的统计
  obsidian edit file="Obsidian Sync State" content="# Obsidian Sync State

## Last Sync
- **Time**: $now
- **Mode**: incremental
- **Issues Processed**: $total

## Statistics
- **Total Synced**: $total
- **Open**: $open
- **Closed**: $closed
"
}
```

---

## 7. 与现有 Workflow 集成

### 7.1 集成点

在 `references/workflows.md` 的 **Step 5 之后**添加：

```markdown
### Step 6: 同步到 Obsidian（可选）

如果用户配置了 Obsidian 同步：

```bash
# 检查是否启用 Obsidian 同步
if [ "$OBSIDIAN_SYNC_ENABLED" = true ]; then
  sync_issues_to_obsidian "$OBSIDIAN_VAULT_PATH" incremental false
fi
```

### 7.2 新增命令

```bash
# 初始化同步状态
"初始化 Obsidian 同步"

# 增量同步
"同步 issues 到 Obsidian"

# 强制全量同步
"强制全量同步 issues 到 Obsidian"

# 查看同步状态
"查看 Obsidian 同步状态"  # → 打开 "Obsidian Sync State" 笔记

# 搜索已同步 issues
"搜索 Obsidian 中的 issues"
```

---

## 8. 优势对比

### 8.1 代码简洁性

**旧版（文件系统）**：
```python
# 需要手动管理文件路径
note_path = f"{vault_path}/{issues_folder}/Issue-{number}-{slug}.md"

# 需要手动生成 YAML
frontmatter = yaml.dump({...})

# 需要手动写入文件
with open(note_path, 'w') as f:
    f.write(f"---\n{frontmatter}---\n\n{content}")

# 需要手动解析 YAML 更新
with open(note_path, 'r') as f:
    content = f.read()
    # ... 解析 YAML ... 修改 ... 写回
```

**新版（Obsidian CLI）**：
```bash
# Obsidian 自动处理路径
note_name="Issue-$number-$slug"

# Obsidian 自动管理 frontmatter
obsidian create name="$note_name" content="$content" path="Issues/" silent

# 更新 property 一行搞定
obsidian property:set name="priority" value="critical" file="$note_name"
```

### 8.2 错误处理

**旧版**：
```python
try:
    with open(note_path, 'w') as f:
        f.write(content)
except IOError as e:
    print(f"Error writing file: {e}")
```

**新版**：
```bash
# Obsidian CLI 提供明确的错误消息
if ! obsidian create name="$note_name" content="$content"; then
  echo "❌ Failed to create note. Is Obsidian running?"
  return 1
fi
```

---

## 9. 配置示例

### 9.1 首次使用

```bash
# 1. 确保 Obsidian 正在运行
open -a Obsidian

# 2. 初始化同步状态
"初始化 Obsidian 同步"

# 3. 首次全量同步
"全量同步所有 issues 到 Obsidian"

# 4. 查看同步结果
"查看 Obsidian 同步状态"  # 打开 "Obsidian Sync State" 笔记
```

### 9.2 定期同步

```bash
# 每天增量同步
"同步 issues 到 Obsidian"
```

---

## 10. 报告格式（简化）

同步完成后，直接打开 **Obsidian Sync State** 笔记查看状态：

```markdown
# Obsidian Sync State

## Last Sync
- **Time**: 2026-03-27T12:00:00Z
- **Mode**: incremental
- **Issues Processed**: 7

## Statistics
- **Total Synced**: 31
- **Open**: 28
- **Closed**: 3
```

也可以使用 Dataview 查询：

```dataview
TABLE
  github_status as "Status",
  priority as "Priority",
  type as "Type"
FROM "Issues"
WHERE issue_number
SORT updated_at DESC
LIMIT 10
```

---

## 11. 依赖

### 11.1 必需
- **Obsidian CLI** - 已内置在 Claude Code
- **Obsidian App** - 需要运行中

### 11.2 推荐 Obsidian 插件
- **Dataview** - 动态查询
- **Kanban** - 看板视图
- **Graph Analysis** - 关系分析

---

## 12. 故障排查

### 12.1 Obsidian 未运行

```bash
❌ Obsidian is not running. Please open Obsidian first.
```

**解决**：`open -a Obsidian`

### 12.2 Vault 未找到

```bash
❌ Vault not found. Available vaults: MyVault, WorkVault
```

**解决**：`obsidian vault="MyVault" ...`

### 12.3 笔记已存在

```bash
⚠️ Note "Issue-335-..." already exists. Use --overwrite to replace.
```

**解决**：添加 `overwrite` flag 或使用 `property:set` 更新

---

## 13. 性能优化最佳实践

### 13.1 批量属性更新（推荐）

**问题**：逐个设置属性会产生多次 CLI 调用（10+ 次/issue）

**优化方案**：使用批量属性设置

```bash
# ❌ 低效：10+ 次 CLI 调用
obsidian property:set name="title" value="..." file="$note_name"
obsidian property:set name="issue_number" value=335 file="$note_name"
obsidian property:set name="issue_url" value="..." file="$note_name"
# ... 7 more calls

# ✅ 高效：1-2 次 CLI 调用
# 方案 A: 创建时设置属性（如果 CLI 支持）
obsidian create \
  name="$note_name" \
  content="$content" \
  path="Issues/" \
  --property title="Issue #335: ..." \
  --property issue_number=335 \
  --property issue_url="https://..." \
  silent

# 方案 B: 使用 JSON 批量设置（如果 CLI 支持）
obsidian property:set-batch \
  file="$note_name" \
  properties='{
    "title": "Issue #335: ...",
    "issue_number": 335,
    "issue_url": "https://...",
    "priority": "critical",
    "type": "bug"
  }'
```

### 13.2 批量标签操作

**问题**：循环追加标签产生多次 CLI 调用

**优化方案**：一次性设置标签数组

```bash
# ❌ 低效：每个标签一次调用
for tag in $tags; do
  obsidian property:append name="tags" value="$tag" file="$note_name"
done

# ✅ 高效：一次设置所有标签
tags_str=$(IFS=,; echo "${tags[*]}")
obsidian property:set name="tags" value="$tags_str" file="$note_name"
```

### 13.3 避免冗余搜索

**问题**：更新前先搜索检查笔记是否存在

**优化方案**：直接尝试操作，失败时创建

```bash
# ❌ 低效：2 次 CLI 调用（search + update）
if obsidian search query="$note_name" limit=1 | grep -q "$note_name"; then
  update_issue_note "$issue"
else
  create_issue_note "$issue"
fi

# ✅ 高效：1 次 CLI 调用（update 或 create）
if ! obsidian property:set name="github_updated_at" value="${issue.updated_at}" file="$note_name" 2>/dev/null; then
  create_issue_note "$issue"
else
  update_issue_note "$issue"
fi
```

### 13.4 状态文件清理

**问题**：`.issue-state.json` 无限增长

**优化方案**：定期清理旧记录

```bash
# 在每次同步后清理 90 天前的记录
cleanup_old_state() {
  local state_file=".issue-state.json"
  local cutoff_date=$(date -u -v-90d +"%Y-%m-%dT%H:%M:%SZ")

  # 使用 jq 过滤旧记录（如果安装了 jq）
  if command -v jq &> /dev/null; then
    jq --arg cutoff "$cutoff_date" '
      .processed_issues |= with_entries(
        select(.value.updated_at > $cutoff)
      )
    ' "$state_file" > "${state_file}.tmp" && mv "${state_file}.tmp" "$state_file"
  fi
}
```

### 13.5 性能对比

| 操作 | 旧方案 | 优化方案 | 改进 |
|------|--------|----------|------|
| **设置 10 个属性** | 10 次 CLI 调用 | 1-2 次调用 | ↓ 80-90% |
| **设置 5 个标签** | 5 次 CLI 调用 | 1 次调用 | ↓ 80% |
| **检查 + 更新** | 2 次调用 | 1 次调用 | ↓ 50% |
| **100 issues 同步** | ~1700 次调用 | ~200 次调用 | ↓ 88% |

---

## 总结

**使用 Obsidian CLI 的优势**：

✅ **代码简洁** - 减少 60% 代码量（无需手动管理文件/YAML）
✅ **自动刷新** - Obsidian 实时更新，无需 reload
✅ **原生搜索** - 使用 `obsidian search` 而非遍历文件
✅ **错误明确** - CLI 提供清晰的错误消息
✅ **状态可视化** - 使用笔记而非 JSON 存储状态
✅ **跨平台** - 无需关心文件路径分隔符
✅ **扩展性强** - 未来可使用更多 Obsidian CLI 功能

**与旧版兼容**：
- 可选保留 `.issue-obsidian-sync.json` 用于外部工具集成
- 笔记格式完全相同（frontmatter + markdown）
- 命令接口一致

**版本**: v2.0 (Obsidian CLI)
**更新日期**: 2026-03-27
