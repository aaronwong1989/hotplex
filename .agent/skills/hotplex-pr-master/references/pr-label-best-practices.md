# PR Label Best Practices

本文档定义了 HotPlex 项目 PR 标签的最佳实践，基于 Kubernetes, React, VS Code 等大型开源项目的经验。

## 优先级标签 (Priority Labels)

### P0 - Critical

**判断标准**：
- 阻塞生产发布
- 安全漏洞修复（CVSS ≥ 9.0）
- 数据丢失风险
- 核心功能完全不可用

**示例**：
```
- Hotfix: Fix authentication bypass vulnerability
- Critical: Fix data corruption in session pool
- Blocker: Fix gateway crash on startup
```

**处理优先级**：
- 立即响应（< 1 小时）
- 最高优先级 review
- 立即合并（跳过常规流程）

---

### P1 - High

**判断标准**：
- Sprint 核心功能
- 重要 bugfix（严重影响用户体验）
- 关键性能优化
- 重要功能缺失

**示例**：
```
- [feat] Add Admin API endpoints
- Fix: Slack streaming writer state error
- [perf] Optimize session pool GC performance
```

**处理优先级**：
- 24 小时内响应
- 高优先级 review
- Sprint 内合并

---

### P2 - Medium

**判断标准**：
- 常规功能开发
- 一般 bugfix
- 代码优化
- 文档更新

**示例**：
```
- [feat] Add typing indicator timeout config
- Fix: Incorrect log format
- [docs] Update API documentation
- [refactor] Simplify error handling
```

**处理优先级**：
- 3 天内响应
- 常规 review 流程
- 2 周内合并

---

### P3 - Low

**判断标准**：
- Nice-to-have 功能
- 代码清理
- 非紧急优化
- 改进建议

**示例**：
```
- [chore] Update dependencies
- [refactor] Rename variable for clarity
- [docs] Fix typo in README
- [test] Add edge case test
```

**处理优先级**：
- 1 周内响应
- 低优先级 review
- 长期规划合并

---

## 类型标签 (Type Labels)

### type/bug

**判断依据**：
1. **分支名前缀**：`fix/`, `bugfix/`, `hotfix/`
2. **标题关键词**：Fix, Bugfix, Hotfix, 修复
3. **描述内容**：描述了异常行为、错误、失败

**示例**：
```
- Fix: Session leak in pool cleanup
- Hotfix: Fix Docker container startup failure
- Bugfix: Incorrect error message
```

---

### type/feature

**判断依据**：
1. **分支名前缀**：`feat/`, `feature/`
2. **标题关键词**：Add, Implement, New, 新增, 实现
3. **描述内容**：添加新功能、新能力

**示例**：
```
- [feat] Add Admin Webhook API
- Implement multi-level typing indicator
- Add support for custom prompts
```

---

### type/enhancement

**判断依据**：
1. **分支名前缀**：`enhance/`, `improve/`, `perf/`
2. **标题关键词**：Improve, Enhance, Optimize, 优化, 改进
3. **描述内容**：改进现有功能、性能提升

**示例**：
```
- Improve session pool performance
- Enhance error messages
- Optimize WebSocket message handling
```

---

### type/refactor

**判断依据**：
1. **分支名前缀**：`refactor/`
2. **标题关键词**：Refactor, 重构, Simplify, Clean up
3. **描述内容**：代码重构、清理、无功能变更

**示例**：
```
- Refactor: Extract session management logic
- Simplify error handling
- Clean up unused code
```

---

### type/docs

**判断依据**：
1. **文件变更**：仅修改 `docs/`, `*.md`, `LICENSE`
2. **分支名前缀**：`docs/`, `doc/`
3. **标题关键词**：Docs, Documentation, README, 文档

**示例**：
```
- [docs] Update API documentation
- Fix typo in README
- Add architecture diagram
```

---

### type/test

**判断依据**：
1. **文件变更**：仅修改 `*_test.go`, `tests/`
2. **分支名前缀**：`test/`, `tests/`
3. **标题关键词**：Test, 测试, Coverage

**示例**：
```
- [test] Add unit tests for session pool
- Increase test coverage for brain package
- Fix flaky test
```

---

## 规模标签 (Size Labels)

### size/small

**判断标准**（满足任一）：
- 变更行数 < 100
- 变更文件 < 5
- 单模块影响

**典型场景**：
- 单文件修改
- 简单 bugfix
- 文档更新
- 配置调整

**预期工作量**：< 1 天

---

### size/medium

**判断标准**（满足任一）：
- 变更行数 100-500
- 变更文件 5-15
- 多模块影响

**典型场景**：
- 新功能（中等复杂度）
- 多文件修改
- 重构
- 性能优化

**预期工作量**：1-3 天

---

### size/large

**判断标准**（满足任一）：
- 变更行数 > 500
- 变更文件 > 15
- 架构变更
- 多子系统影响

**典型场景**：
- 大型功能
- 架构重构
- 多平台适配
- 核心模块重写

**预期工作量**：> 3 天

---

## 状态标签 (Status Labels)

### status/draft

**判断标准**：
- PR 为 Draft 状态
- 尚未准备好 review
- 标题包含 [WIP], [Draft]

**自动操作**：
- 不请求 review
- 不触发 CI（可选）

---

### status/ready-for-review

**判断标准**：
- PR 为非 Draft 状态
- 填写了完整的描述
- 通过了初步检查

**自动操作**：
- 自动请求 reviewers
- 触发 CI/CD

---

### status/review-requested

**判断标准**：
- 已请求 reviewers
- Review 状态为 PENDING

**自动操作**：
- 发送通知给 reviewers
- 追踪 review 时间

---

### status/approved

**判断标准**：
- 获得至少 1 个 APPROVED review
- 无 CHANGES_REQUESTED review

**自动操作**：
- 准备合并
- 可启用 auto-merge

---

### status/blocked

**判断标准**：
- CI/CD 失败
- 缺少必要 approval
- 依赖其他 PRs
- 技术问题

**自动操作**：
- 阻塞合并
- 发送提醒

---

### status/conflicts

**判断标准**：
- GitHub API `mergeable = false`
- 与 base branch 冲突

**自动操作**：
- 提醒 rebase
- 阻塞合并

---

### status/stale

**判断标准**：
- 30+ 天无更新
- 长期无 review

**自动操作**：
- 发送提醒
- 建议 close 或 rebase

---

## Review 标签

### review/pending

**判断标准**：
- 无 review 或 review 状态为 PENDING
- 等待 reviewers 响应

**SLA**：
- P0: < 1 小时
- P1: < 24 小时
- P2: < 3 天
- P3: < 1 周

---

### review/approved

**判断标准**：
- 至少 1 个 APPROVED review
- 无 CHANGES_REQUESTED review

**后续操作**：
- 标注 `status/approved`
- 准备合并

---

### review/changes-requested

**判断标准**：
- 存在 CHANGES_REQUESTED review

**后续操作**：
- 等待 author 修改
- 追踪修改进度

---

## 自动标注规则

### 规则优先级

1. **显式标记** > 隐式推断
   - P0/P1/P2/P3 标记优先级最高
   - 标题前缀 `[feat]` 优先级高于内容分析

2. **Issue 关联** > PR 内容
   - 如果 PR 关联 issue，继承 issue 的优先级和类型

3. **代码分析** > 文本分析
   - 文件变更路径优先于描述内容
   - 变更行数准确度高于预估

### 判断流程

```
1. 检查显式标记（P0/P1, [feat], etc.）
   ↓
2. 检查关联 Issue（继承标签）
   ↓
3. 分析分支名（fix/feat/docs）
   ↓
4. 分析文件变更（路径、行数）
   ↓
5. 分析描述内容（关键词）
   ↓
6. 应用标签
```

---

## 最佳实践

### 1. 保持标签体系简洁

- ✅ **推荐**：`priority/high`, `type/feature`, `size/medium`
- ❌ **避免**：`urgent`, `important`, `backend`, `frontend`（过于主观或细粒度）

### 2. 避免标签重叠

- ✅ **清晰**：`type/bug` vs `type/feature`
- ❌ **重叠**：`type/bugfix` vs `type/bug`

### 3. 保持一致性

- 与 Issue 标签体系保持一致
- 遵循 Kubernetes/VS Code 等大型项目惯例

### 4. 自动化优先

- 能自动标注的不手动标注
- 减少人工判断负担

### 5. 定期回顾

- 每月回顾标签使用情况
- 清理无用标签
- 优化判断规则

---

## 参考资源

- [GitHub Labels Guide](https://docs.github.com/en/issues/using-labels-and-milestones-to-track-work/managing-labels)
- [Kubernetes Labels](https://github.com/kubernetes/kubernetes/labels)
- [VS Code Issue Triage](https://github.com/microsoft/vscode/wiki/Issue-Triage)
- [Angular Commit Convention](https://github.com/angular/angular/blob/master/CONTRIBUTING.md#type)
