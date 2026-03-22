# HotPlex PR 管理大师 🚀

**全方位 GitHub Pull Request 智能管理工具**

## 快速开始

```bash
# 分析所有 PRs 并打标签
"分析所有 PRs 并打标签"

# 检查 CI/CD 状态
"检查所有 PRs 的 CI 状态"

# 检测冲突
"检测需要 rebase 的 PRs"

# Review 提醒
"提醒超过 7 天未 review 的 PRs"

# 生成分析报告
"生成 PR 分析报告"
```

## 核心功能

- ✅ **自动标注** - 5 维度标签（优先级、类型、规模、状态、Review）
- ✅ **生命周期管理** - Draft → Ready → Review → Approved → Merged
- ✅ **Review 状态跟踪** - 自动追踪 reviewers 和 review 状态
- ✅ **CI/CD 监控** - 自动检查 GitHub Actions 状态
- ✅ **冲突检测** - 自动检测与 base branch 的冲突
- ✅ **Issue 关联** - 解析并验证关联的 issues
- ✅ **批量操作** - 批量打标签、合并、关闭
- ✅ **分析报告** - PR 趋势、效率指标、瓶颈识别

## 文档

- **SKILL.md** - 完整使用文档
- **references/pr-label-best-practices.md** - 标签最佳实践
- **references/github-actions-examples.md** - GitHub Actions 配置示例
- **scripts/pr_labeler.py** - 自动标注引擎

## 与 Issue 管理大师对比

| 功能 | Issue Master | PR Master |
|------|-------------|-----------|
| 自动标注 | ✅ | ✅ |
| 生命周期管理 | ✅ | ✅ |
| 重复检测 | ✅ | N/A |
| 冲突检测 | N/A | ✅ |
| Review 跟踪 | N/A | ✅ |
| CI/CD 监控 | N/A | ✅ |
| Issue 关联 | N/A | ✅ |

## 标签体系（与 Issue Master 一致）

### 优先级
- `priority/critical` - P0, 阻塞发布
- `priority/high` - P1, Sprint 核心
- `priority/medium` - P2, 常规开发
- `priority/low` - P3, 改进建议

### 类型
- `type/bug`, `type/feature`, `type/enhancement`
- `type/refactor`, `type/docs`, `type/test`

### 规模
- `size/small` (< 100 行, < 5 文件)
- `size/medium` (100-500 行, 5-15 文件)
- `size/large` (> 500 行, > 15 文件)

### 状态
- `status/draft`, `status/ready-for-review`, `status/review-requested`
- `status/approved`, `status/blocked`, `status/conflicts`, `status/stale`

### Review
- `review/pending`, `review/approved`, `review/changes-requested`

---

**版本**: v1.0.0
**维护者**: HotPlex Team
**创建时间**: 2026-03-22
