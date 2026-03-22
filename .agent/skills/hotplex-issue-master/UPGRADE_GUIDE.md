# HotPlex Issue Master v1.1.0 - 升级指南

## 🎉 新增功能

### 1. 智能增量管理

**核心优势**：只处理真正需要管理的 issues，大幅提升效率。

**工作原理**：
- 维护 `.issue-state.json` 状态文件
- 自动跳过已稳定且低优先级的 issues
- 智能过滤规则：
  - ✅ 新创建 issues (< 7天)
  - ✅ 最近有更新 (< 14天)
  - ✅ 高优先级 (critical/high)
  - ✅ 状态为 needs-triage
  - ✅ 从未处理过
  - ❌ 已稳定低优先级

**使用示例**：
```python
# 默认：增量模式（推荐）
labeler = IssueLabelerV2()
issues_to_process = [
    issue for issue in all_issues
    if labeler.should_process_issue(issue)
]

# 结果：31个issues中只处理12个，跳过19个
```

### 2. 自适应标注

**核心优势**：保留用户手动标签，只补充缺失维度。

**工作原理**：
- 检测已有标签（priority/*, type/*, size/*, status/*)
- 如果已有某维度的标签 → 跳过该维度
- 如果缺失某维度 → 补充推荐标签

**使用示例**：
```python
# Issue 已有手动标签 ["priority/high", "type/bug"]
# 自动分析推荐: priority/low, type/feature, size/small, status/needs-triage
# 实际应用: size/small, status/needs-triage (保留已有的 high/bug)
labels = labeler.analyze_issue(issue, preserve_existing=True)
```

### 3. 特殊类型识别

**改进**：RFC/epic 不再误判为 bug

```python
# v1.0.0: [RFC] xxx → 识别为 type/bug ❌
# v1.1.0: [RFC] xxx → 保留原有标签（epic/rfc） ✅
```

## 📖 使用方法

### 基础用法（增量模式）

```python
from labeler_v2 import IssueLabelerV2

# 1. 初始化（自动加载状态文件）
labeler = IssueLabelerV2(state_file='.issue-state.json')

# 2. 获取所有 issues
all_issues = get_all_issues()

# 3. 智能过滤
issues_to_process = [
    issue for issue in all_issues
    if labeler.should_process_issue(issue, force=False)
]

# 4. 分析并应用标签
保留已有标签)
    applied_labels = labeler.analyze_issue(issue, preserve_existing=True)

    # 应用标签到 GitHub
    apply_labels(issue['number'], applied_labels)

    # 更新状态文件
    labeler.update_processed_state(issue, applied_labels)

print(f"处理了 {len(issues_to_process)} 个 issues")
```

### 强制全量扫描

```python
# 强制处理所有 issues（忽略智能过滤）
issues_to_process = [
    issue for issue in all_issues
    if labeler.should_process_issue(issue, force=True)
]
```

### 不保留已有标签

```python
# 覆盖已有标签（谨慎使用）
labels = labeler.analyze_issue(issue, preserve_existing=False)
```

## 🗂 状态文件格式

`.issue-state.json`:
```json
{
  "last_incremental_scan": "2026-03-22T10:30:00+00:00",
  "processed_issues": {
    "335": {
      "labels": {
        "priority": "priority/critical",
        "type": "type/bug",
        "size": "size/small",
        "status": "status/blocked"
      },
      "processed_at": "2026-03-22T10:25:00+00:00",
      "updated_at": "2026-03-21T23:21:50Z"
    }
  },
  "metadata": {
    "version": "1.1.0",
    "created_at": "2026-03-22T10:00:00+00:00",
    "updated_at": "2026-03-22T10:30:00+00:00"
  }
}
```

## 🔄 迁移指南

**从 v1.0.0 迁移到 v1.1.0**:

1. **更新导入**:
   ```python
   # 旧版本
   from labeler import IssueLabeler

   # 新版本
   from labeler_v2 import IssueLabelerV2
   ```

2. **添加智能过滤**:
   ```python
   # 旧版本：处理所有 issues
   for issue in issues:
       labels = labeler.analyze_issue(issue)

   # 新版本：智能过滤
   for issue in issues:
       if labeler.should_process_issue(issue):
           labels = labeler.analyze_issue(issue, preserve_existing=True)
           labeler.update_processed_state(issue, labels)
   ```

3. **状态文件自动创建**:
   - 首次运行自动创建 `.issue-state.json`
   - 无需手动迁移

## 📊 性能对比

**v1.0.0（全量扫描）**:
- 31个 issues → 处理31个 → 31次 API 调用

**v1.1.0（增量模式）**:
- 31个 issues → 智能过滤12个 → 12次 API 调用
- **效率提升**: 61% ↓

## ⚙️ 向后兼容性

- ✅ `labeler.py` (v1.0.0) 仍然可用
- ✅ 新旧版本可共存
- ✅ 状态文件可选（默认使用，可禁用）

## 🐛 已知限制

1. **状态文件依赖**: 需要写入权限到 `.issue-state.json`
2. **时间戳精度**: GitHub API 返回的 `updated_at` 可能精确到秒级
3. **标签检测**: 只检测标准 slash 套餐命名空间（priority/*, type/*, size/*, status/*）

## 🔮 未来计划

- [ ] 支持自定义过滤规则
- [ ] 机器学习优化标签推荐
- [ ] 集成 GitHub Actions 自动化
- [ ] Web UI 可视化管理

---

**版本**: v1.1.0
**发布时间**: 2026-03-22
**维护者**: hotplex Team
