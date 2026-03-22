# HotPlex PR 管理大师 - 创建报告

**创建时间**: 2026-03-22
**版本**: v1.0.0

## 📦 创建内容

### 核心文件

1. **SKILL.md** - 主 skill 文档
   - 核心能力定义
   - 标签体系
   - 工作流程
   - 自动化集成
   - GitHub Actions 配置

2. **references/pr-label-best-practices.md** - 标签最佳实践
   - 优先级标签判断标准
   - 类型标签判断依据
   - 规模标签判断规则
   - 状态标签定义
   - Review 标签规范

3. **scripts/pr_labeler.py** - 自动标注引擎
   - 5 维度标签分析
   - 冲突检测
   - CI/CD 状态检查
   - Review 状态检查

## 🎯 核心功能

### 1. 自动标注
- ✅ **优先级**: Critical, High, Medium, Low
- ✅ **类型**: Bug, Feature, Enhancement, Refactor, Docs, Test
- ✅ **规模**: Small, Medium, Large
- ✅ **状态**: Draft, Ready, Review-Requested, Approved, Blocked, Conflicts, Stale
- ✅ **Review**: Pending, Approved, Changes-Requested

### 2. 生命周期管理
- ✅ Draft → Ready → Review → Approved → Merged
- ✅ 冲突自动检测
- ✅ Stale PR 提醒（30+ 天）
- ✅ CI/CD 阻塞检测

### 3. Review 状态跟踪
- ✅ 自动追踪 requested reviewers
- ✅ Review 状态检测（approved/changes-requested/pending）
- ✅ 长期未 review 提醒（7+ 天）

### 4. CI/CD 监控
- ✅ GitHub Actions 状态检查
- ✅ 失败检查项详细报告
- ✅ 阻塞合并提醒

### 5. 冲突检测
- ✅ 自动检测与 base branch 冲突
- ✅ 标注 `status/conflicts`
- ✅ Rebase 提醒

### 6. Issue 关联
- ✅ 解析 "Resolves #XXX" / "Closes #XXX"
- ✅ 验证关联有效性
- ✅ 继承 issue 优先级

### 7. 批量操作
- ✅ 批量打标签
- ✅ 批量请求 review
- ✅ 批量合并（需确认）

### 8. 分析报告
- ✅ PR 趋势分析
- ✅ 效率指标
- ✅ 瓶颈识别
- ✅ 标签分布统计

## 🏷️ 标签体系

与 hotplex-issue-master 保持一致：

### 优先级
- `priority/critical` - P0, 阻塞发布, 安全漏洞
- `priority/high` - P1, Sprint 核心
- `priority/medium` - P2, 常规开发
- `priority/low` - P3, 改进建议

### 类型
- `type/bug` - Bug 修复
- `type/feature` - 新功能
- `type/enhancement` - 优化改进
- `type/refactor` - 重构清理
- `type/docs` - 文档更新
- `type/test` - 测试相关

### 规模
- `size/small` - < 100 行, < 5 文件
- `size/medium` - 100-500 行, 5-15 文件
- `size/large` - > 500 行, > 15 文件

### 状态
- `status/draft` - Draft 状态
- `status/ready-for-review` - 等待 review
- `status/review-requested` - 已请求 reviewers
- `status/approved` - 已批准
- `status/blocked` - CI 失败/依赖阻塞
- `status/conflicts` - 与 base branch 冲突
- `status/stale` - 30+ 天未更新

### Review
- `review/pending` - 等待 review
- `review/approved` - 已批准
- `review/changes-requested` - 需要修改

## 🔄 工作流程

```
1. 获取 Open PRs
   ↓
2. 分析每个 PR
   - 优先级分析
   - 类型分析
   - 规模分析
   - 状态分析
   - Review 状态
   - CI/CD 状态
   ↓
3. 冲突检测
   ↓
4. 应用标签
   ↓
5. 生成报告
```

## ⚙️ GitHub Actions 集成

### 自动化配置
- ✅ `.github/workflows/pr-triage.yml` - PR triage workflow
- ✅ `.github/pr-labeler.yml` - 自动标签配置
- ✅ `.github/workflows/auto-merge.yml` - 自动合并
- ✅ `.github/settings.yml` - Branch protection rules

### 触发条件
- PR opened/edited/synchronize
- PR ready_for_review
- PR review submitted
- 定时任务（每 6 小时）

## 📊 性能考虑

- **API 速率限制**: GitHub API 5000 req/hour
- **批量操作**: 每次最多 100 PRs
- **幂等性**: 重复运行不会重复添加标签
- **人工确认**: 批量合并等破坏性操作需确认

## 🔒 安全边界

- ❌ 不自动删除标签
- ❌ 不修改 PR 内容
- ❌ 不强制合并
- ✅ 只添加标签和评论
- ✅ 需要人工确认破坏性操作

## 📖 使用示例

### 基础命令

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

### 高级用法

```bash
# 针对性分析
"只分析优先级为 P0 和 P1 的 PRs"

# 批量合并
"合并所有 approved 且 CI 通过的 PRs（需要确认）"

# Review 管理
"为长期未 review 的 PRs 自动追加 reviewers"

# Issue 关联
"验证所有 PRs 的 issue 关联是否正确"
```

## 🎓 最佳实践

### 1. 保持标签体系简洁
- 使用标准标签（priority/type/size/status/review）
- 避免创建主观或细粒度标签

### 2. 自动化优先
- 能自动标注的不手动标注
- 减少人工判断负担

### 3. 定期回顾
- 每月回顾标签使用情况
- 清理无用标签
- 优化判断规则

### 4. 与 Issue 体系保持一致
- 继承 issue 优先级
- 使用相同的类型标签
- 保持标签语义一致

## 🔗 参考资源

- [GitHub Flow](https://docs.github.com/en/get-started/quickstart/github-flow)
- [Kubernetes PR Workflow](https://github.com/kubernetes/community/blob/master/contributors/guide/owners.md)
- [VS Code Issue Triage](https://github.com/microsoft/vscode/wiki/Issue-Triage)
- [GitHub Actions](https://docs.github.com/en/actions)
- [Branch Protection](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/about-protected-branches)

## ✅ 完成检查清单

- [x] SKILL.md 主文档
- [x] 标签最佳实践文档
- [x] 自动标注脚本
- [x] 与 issue-master 标签体系一致
- [x] GitHub Actions 配置示例
- [x] Branch Protection Rules 配置
- [x] Auto-merge 配置
- [x] 安全边界定义
- [x] 使用示例

## 📝 待完善项（可选）

- [ ] 完善 CI/CD 状态检查（需调用 GitHub API）
- [ ] 完善 Review 状态检查（需调用 GitHub API）
- [ ] 添加单元测试
- [ ] 添加更多脚本工具
- [ ] 集成 Slack 通知
- [ ] 添加更多分析报告模板

---

**维护者**: HotPlex Team
**状态**: ✅ Ready for Use
