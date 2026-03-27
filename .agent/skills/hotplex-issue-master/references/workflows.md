# HotPlex Issue 工作流程

## 智能增量管理模式（默认）

**核心优势**：只处理真正需要管理的 issues，大幅提升效率。

### Step 0: 加载状态文件

读取 `.issue-state.json` 获取上次处理记录：

```python
state = load_state('.issue-state.json')
last_incremental_scan = state['last_incremental_scan']
processed_issues = state['processed_issues']
```

### Step 1: 智能过滤 (Smart Filtering)

获取 open issues 并应用智能过滤规则：

```python
# 获取所有 open issues
all_issues = list_issues(owner="hrygo", repo="hotplex", state="OPEN", perPage=100)

# 智能过滤：只保留需要处理的 issues
issues_to_process = [
    issue for issue in all_issues
    if should_process_issue(issue, processed_issues, last_incremental_scan)
]

def should_process_issue(issue, processed_issues, last_scan):
    """判断 issue 是否需要处理"""
    issue_number = issue['number']
    updated_at = parse_datetime(issue['updated_at'])
    created_at = parse_datetime(issue['created_at'])
    now = datetime.now(timezone.utc)

    # 规则1: 新创建的 issues (< 7天)
    if (now - created_at).days < 7:
        return True

    # 规则2: 最近有更新 (< 14天)
    if (now - updated_at).days < 14:
        return True

    # 规则3: 高优先级 (critical/high)
    priority = analyze_priority(issue)
    if priority in ['priority/critical', 'priority/high']:
        return True

    # 规则4: 状态为 needs-triage
    if has_label(issue, 'status/needs-triage'):
        return True

    # 规则5: 从未处理过
    if issue_number not in processed_issues:
        return True

    # 规则6: 有新更新（since last scan）
    if last_scan and updated_at > last_scan:
        return True

    # 其他情况：跳过
    return False
```

**过滤效果示例**：
- 总 open issues: 31
- 需要处理: 12 (新创建 3 + 有更新 4 + 高优先级 3 + needs-triage 2)
- 跳过处理: 19 (已稳定低优先级)

### Step 2: 分析每个 Issue

对每个 issue 进行以下分析：

1. **优先级分析**：
   - 检查 body 中的 P0/P1/P2 标记
   - 关键词：严重程度、影响范围、阻塞
   - 判断：critical/high/medium/low

2. **类型分析**：
   - 检查标题前缀 `[feat]`, `[admin]` 等
   - 关键词：bug, feature, enhancement, docs, test
   - 判断：bug/feature/enhancement/docs/test

3. **规模分析**：
   - 涉及模块数
   - 是否需要架构变更
   - 判断：small/medium/large

4. **状态分析**：
   - 创建时间、更新时间
   - 信息完整性
   - 是否有阻塞依赖
   - 判断：needs-triage/ready-for-work/blocked/stale

5. **可关闭性分析**：
   - 重复、已修复、过时、无效
   - 生成建议列表

### Step 3: 应用标签

使用 GitHub MCP 的 `add_labels_to_issue` API 应用标签：

```python
add_labels_to_issue(
    owner="hrygo",
    repo="hotplex",
    issue_number=issue_number,
    labels=["priority/high", "type/bug", "size/medium", "status/ready-for-work"]
)
```

### Step 4: 生成报告

输出简短的 Markdown 格式报告，包含：
- 扫描模式（增量/全量）
- 总 issue 数 / 需要处理 / 跳过处理
- 标签分布
- 可关闭 issues
- 高优先级 issues

### Step 5: 保存状态文件

更新 `.issue-state.json` 记录处理历史：

```python
def save_state(state_file, processed_issues):
    """保存处理状态"""
    state = {
        'last_incremental_scan': datetime.now(timezone.utc).isoformat(),
        'processed_issues': processed_issues,  # {issue_number: {labels, updated_at}}
        'metadata': {
            'version': '1.1.0',
            'updated_at': datetime.now(timezone.utc).isoformat()
        }
    }

    with open(state_file, 'w') as f:
        json.dump(state, f, indent=2)
```

---

## Stale Issue 清理流程

**VS Code 标准**:
- **60 天**无更新 → 标记 `status/stale`
- **14 天**后无响应 → 自动关闭
- **豁免标签**: `priority/critical`, `priority/high`, `status/in-progress`, `status/blocked`, `type/security`

**执行流程**:
```python
for issue in open_issues:
    if issue.updated_at < 60_days_ago:
        if not has_exempt_labels(issue):
            add_label(issue, 'status/stale')
            comment(issue, "This issue will be closed in 14 days if no activity occurs.")

            if issue.updated_at < 74_days_ago:  # 60 + 14
                close_issue(issue)
                comment(issue, "Closed due to inactivity.")
```

---

## 重复检测流程

**相似度计算**:
```python
similarity = (
    title_similarity * 0.6 +  # 标题权重 60%
    body_similarity * 0.3 +   # 描述权重 30%
    label_similarity * 0.1    # 标签权重 10%
)

if similarity > 0.8:  # 80% 以上视为重复
    mark_as_duplicate(new_issue, original_issue)
```

**操作流程**:
1. 标记 `duplicate`
2. 评论: "Duplicate of #XXX"
3. 生成建议列表（需人工确认）

---

## 优先级动态调整

**评分公式**:
```
priority_score =
    severity * 0.4 +          # 严重程度
    impact * 0.3 +            # 影响范围
    urgency * 0.2 +           # 紧急程度
    community_votes * 0.1 -   # 社区投票
    days_inactive * 0.05      # 时间衰减
```

**调整规则**:
- 👍 > 20 → 自动晋升为 `priority/high` (feature requests)
- 60+ 天无更新 + P3 → 降级为 `priority/low`
- 安全漏洞 (CVSS ≥ 9.0) → 强制 `priority/critical`

---

## 批量操作示例

**批量打标签**:
```python
# 所有包含 "bug" 的 issues 打 type/bug
issues = search_issues("bug in:title state:open")
for issue in issues:
    add_label(issue, 'type/bug')

# 所有 [feat] issues 打 type/feature
issues = filter(lambda i: i.title.startswith('[feat]'), open_issues)
bulk_add_labels(issues, 'type/feature')
```

**批量关闭**:
```python
# 关闭所有 stale + low priority issues
candidates = filter(
    lambda i: has_label(i, 'status/stale') and
              has_label(i, 'priority/low'),
    open_issues
)
bulk_close(candidates, reason="Stale and low priority")
```

---

## 自动化集成

### GitHub Actions 配置

创建 `.github/workflows/issue-triage.yml`:

```yaml
name: Issue Triage
on:
  schedule:
    - cron: '0 0 * * *'  # 每日运行
  issues:
    types: [opened, edited]

jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - name: Auto-label issues
        uses: actions/github-script@v7
        with:
          script: |
            // 基于文件路径自动打标签
            const labeler = require('./.github/labeler.js');
            await labeler.process(context);

      - name: Detect duplicates
        uses: actions/github-script@v7
        with:
          script: |
            // 检测重复 issues
            // ...实现代码

      - name: Mark stale issues
        uses: actions/stale@v9
        with:
          stale-issue-message: 'This issue has been inactive for 60 days.'
          days-before-stale: 60
          days-before-close: 14
          exempt-issue-labels: 'priority/critical,priority/high,status/in-progress'
```

### 自动标签配置

创建 `.github/labeler.yml`:
```yaml
type/docs:
- changed-files:
  - any-glob-to-any-file: ['docs/**/*', '**/*.md']

type/test:
- changed-files:
  - any-glob-to-any-file: ['**/*_test.go', 'tests/**/*']

area/engine:
- changed-files:
  - any-glob-to-any-file: ['internal/engine/**/*']

area/adapter:
- changed-files:
  - any-glob-to-any-file: ['chatapps/**/*', 'provider/**/*']
```

---

## 注意事项

### ⚠️ 重要原则

1. **幂等性**: 重复运行不会重复添加标签
2. **保留人工标记**: 不覆盖已手动添加的标签
3. **人工确认**: 可关闭性建议需人工确认，不自动关闭
4. **豁免机制**: P0/P1/in-progress/security issues 不自动关闭
5. **增量更新**: 支持只处理新 issues 或有更新的 issues

### 🔒 安全边界

- **不自动删除标签**: 只添加，不删除
- **不修改 issue 内容**: 只添加标签和评论
- **不修改优先级**: 除非明确指定（批量操作）
- **需要确认**: 批量关闭、优先级调整等破坏性操作

### 📊 性能考虑

- **API 速率限制**: GitHub API 5000 req/hour
- **批量操作**: 每次最多 100 issues
- **并发控制**: 避免同时运行多个 triage job
- **缓存策略**: 缓存已分析的 issues
