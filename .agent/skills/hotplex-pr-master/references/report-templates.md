# PR 分析报告模板

本文档提供 HotPlex PR 管理大师的完整报告模板。

## 简化版报告

适用于快速查看和日常管理。

```markdown
# HotPlex PR 分析报告

**分析时间**: 2026-03-22
**总 PR 数**: 15 | **已标注**: 15

## 标签分布
- **优先级**: Critical (1), High (5), Medium (7), Low (2)
- **类型**: Feature (6), Bug (3), Enhancement (2), Docs (2), Test (1), Refactor (1)
- **规模**: Small (4), Medium (8), Large (3)
- **状态**: Ready (10), Draft (2), Approved (2), Blocked (1)
- **Review**: Pending (8), Approved (5), Changes-Requested (2)

## CI/CD 状态
✅ 通过: 12 | ❌ 失败: 2 | ⏳ 进行中: 1

## 阻塞 PRs (需处理)
1. **#345** - CI 失败 (tests failed)
2. **#338** - 与 main 冲突，需要 rebase
3. **#335** - 长期未 review (15天)

## 高优先级 PRs
1. **#345** - Fix critical security vulnerability [P0, review/approved]
2. **#342** - Add Admin API endpoints [P1, review/pending]
3. **#340** - Implement multi-level typing indicator [P1, review/pending]
```

---

## 完整版报告

适用于周期性回顾和团队会议。

```markdown
# HotPlex PR 管理报告

**生成时间**: 2026-03-22
**报告周期**: 过去 30 天

## 📊 概览

- **Open PRs**: 15
- **Merged PRs**: 25 (过去30天)
- **New PRs**: 30 (过去30天)
- **平均合并时间**: 3.5 天
- **首次 Review 时间**: 1.2 天

## 🏷️ 标签分布

### 优先级
- Critical: 1 (6.7%)
- High: 5 (33.3%)
- Medium: 7 (46.7%)
- Low: 2 (13.3%)

### 类型
- Feature: 6 (40%)
- Bug: 3 (20%)
- Enhancement: 2 (13.3%)
- Docs: 2 (13.3%)
- Test: 1 (6.7%)
- Refactor: 1 (6.7%)

### 状态
- Ready for Review: 10 (66.7%)
- Draft: 2 (13.3%)
- Approved: 2 (13.3%)
- Blocked: 1 (6.7%)

### Review 状态
- Pending: 8 (53.3%)
- Approved: 5 (33.3%)
- Changes-Requested: 2 (13.3%)

## 🔄 CI/CD 状态

- ✅ 通过: 12 (80%)
- ❌ 失败: 2 (13.3%)
- ⏳ 进行中: 1 (6.7%)

## ⚠️ 阻塞 PRs

1. **#345** - CI 失败 (tests failed) [P0, size/small]
2. **#338** - 与 main 冲突，需要 rebase [P1, size/medium]
3. **#335** - Review changes requested [P1, size/large]

## 🔴 长期未 Review (7+ 天)

1. **#340** - 10 天未 review [P1, size/large]
2. **#336** - 8 天未 review [P2, size/medium]

## 💡 建议操作

### 立即行动
1. 修复 #345 的 CI 失败
2. Rebase #338 解决冲突
3. Review 长期未处理的 PRs (#340, #336)

### 自动化建议
1. 启用 auto-merge for approved PRs
2. 设置 branch protection rules（要求 CI 通过）
3. 配置 stale PR 清理（30+ 天）

### 流程改进
1. 设置 PR 模板 - 标准化描述格式
2. 添加 mandatory reviewers - 加速 review
3. 启用 Dependabot auto-merge - 依赖更新自动化
```

---

## 趋势分析报告

适用于月度/季度回顾。

```markdown
# HotPlex PR 趋势分析

**分析周期**: 2026 Q1 (1月-3月)
**生成时间**: 2026-03-31

## 📈 关键指标

| 指标 | Q1 平均 | 1月 | 2月 | 3月 | 趋势 |
|------|---------|-----|-----|-----|------|
| 新建 PRs | 30 | 25 | 32 | 33 | ↗️ +32% |
| 合并 PRs | 28 | 24 | 29 | 31 | ↗️ +29% |
| 平均合并时间 | 3.5 天 | 4.2 | 3.8 | 2.5 | ↘️ -40% |
| 首次 Review | 1.2 天 | 1.8 | 1.4 | 0.4 | ↘️ -78% |
| PR 积压数 | 15 | 20 | 16 | 9 | ↘️ -55% |

## 🎯 效率提升

### Review 效率
- 首次 Review 时间从 1.8 天降至 0.4 天（**78% 提升**）
- 平均 Review 轮次从 2.5 降至 1.8

### 合并效率
- 平均合并时间从 4.2 天降至 2.5 天（**40% 提升**）
- 一天内合并的 PRs 占比从 20% 提升至 45%

### PR 质量
- 一次通过 CI 的 PRs 占比从 65% 提升至 85%
- 需要多轮修改的 PRs 占比从 35% 降至 20%

## 🏷️ 标签趋势

### 优先级分布变化
| 优先级 | 1月 | 2月 | 3月 | 变化 |
|--------|-----|-----|-----|------|
| P0 Critical | 5% | 6% | 4% | → 持平 |
| P1 High | 25% | 28% | 30% | ↗️ +20% |
| P2 Medium | 50% | 48% | 52% | → 持平 |
| P3 Low | 20% | 18% | 14% | ↘️ -30% |

**解读**：高优先级 PRs 增加，低优先级减少，说明团队更聚焦关键任务。

### 类型分布变化
| 类型 | Q1 占比 | 趋势 |
|------|---------|------|
| Feature | 45% | ↗️ 新功能开发活跃 |
| Bug | 25% | ↘️ 质量提升 |
| Enhancement | 15% | → 稳定 |
| Refactor | 10% | ↗️ 技术债清理 |
| Docs | 5% | → 文档持续更新 |

## ⚠️ 问题识别

### 瓶颈 PRs（长期阻塞）
1. **#340** - 10+ 天未合并（依赖 review）
2. **#336** - 8+ 天未合并（等待作者更新）

### 反复 Request Changes 的 PRs
1. **#335** - 3 轮修改（需求不清晰）
2. **#332** - 2 轮修改（测试覆盖不足）

### CI 失败率高的 PRs
- 失败率：15%（目标 < 10%）
- 主要原因：测试覆盖不足（60%）、Lint 错误（30%）、其他（10%）

## 💡 改进建议

### 流程优化
1. **Review SLA**：设置 24 小时首次 review 目标
2. **PR 模板**：标准化描述格式，减少反复沟通
3. **Auto-merge**：已批准 + CI 通过的 PRs 自动合并

### 质量提升
1. **Pre-commit hooks**：本地运行 lint/test，减少 CI 失败
2. **测试覆盖要求**：新功能必须 > 80% 覆盖率
3. **代码审查指南**：提供 review checklist

### 自动化增强
1. **Dependabot auto-merge**：依赖更新自动合并
2. **Stale PR 清理**：30+ 天未更新自动提醒
3. **智能分配 reviewers**：基于历史和专长自动分配

## 🎯 Q2 目标

| 指标 | Q1 实际 | Q2 目标 | 提升 |
|------|---------|---------|------|
| 首次 Review 时间 | 0.4 天 | 0.2 天 | -50% |
| 平均合并时间 | 2.5 天 | 1.5 天 | -40% |
| PR 积压数 | 9 | < 5 | -44% |
| CI 一次通过率 | 85% | 90% | +6% |
```

---

## 使用场景

| 场景 | 推荐模板 |
|------|---------|
| 日常管理 | 简化版报告 |
| 周会/月会 | 完整版报告 |
| 季度回顾 | 趋势分析报告 |
| 新成员 Onboarding | 完整版报告 + 标签体系说明 |

---

**维护者**: HotPlex Team
**最后更新**: 2026-03-22
