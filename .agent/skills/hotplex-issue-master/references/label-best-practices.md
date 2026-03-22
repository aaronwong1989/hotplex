# GitHub Issue 管理最佳实践

基于 Kubernetes, React, VS Code 等大型开源项目的实际实践总结。

## 1. 标签分类体系 (Label Taxonomy)

### 命名约定

**推荐使用 slash 前缀命名空间** (Kubernetes 风格):
- `type/bug`, `type/feature`, `type/enhancement`
- `priority/critical`, `priority/high`, `priority/medium`, `priority/low`
- `status/needs-triage`, `status/ready-for-work`, `status/blocked`
- `area/engine`, `area/adapter`, `area/security`
- `size/small`, `size/medium`, `size/large`

**替代方案**: colon 前缀 (React 风格):
- `Type: Bug`, `Priority: High`, `Status: In Progress`

### 颜色编码

| 类别 | 颜色方案 | 示例 |
|------|---------|------|
| **优先级** | 温度渐变 (红→黄→蓝) | critical=#b60205, high=#d93f0b, medium=#fbca04, low=#0e8a16 |
| **类型** | 饱和色 | bug=#d73a4a, feature=#006b75, enhancement=#a2eeef |
| **区域** | 柔和色 | engine=#bfd4f2, adapter=#d4c5f9, security=#bfe3c6 |
| **状态** | 交通灯色系 | needs-triage=#fbca04, ready-for-work=#0e8a16, blocked=#e99695 |
| **规模** | 渐变灰 | small=#c5def5, medium=#81c3e7, large=#1d76db |

### 推荐最小标签集 (~25个)

**类型 (5)**: `type/bug`, `type/feature`, `type/enhancement`, `type/docs`, `type/test`
**优先级 (4)**: `priority/critical`, `priority/high`, `priority/medium`, `priority/low`
**状态 (5)**: `status/needs-triage`, `status/ready-for-work`, `status/in-progress`, `status/blocked`, `status/stale`
**规模 (3)**: `size/small`, `size/medium`, `size/large`
**区域 (8)**: 根据项目结构定制 (e.g., `area/engine`, `area/adapter`, `area/security`, `area/cli`, `area/config`, `area/docker`, `area/docs`, `area/testing`)

---

## 2. 优先级判定框架

### Severity x Impact x Urgency 矩阵

```
优先级 = f(Severity, Impact, Urgency)

Severity (严重程度):
  - Critical: 系统崩溃、数据丢失、安全漏洞
  - High: 核心功能不可用、严重性能下降
  - Medium: 功能受限但有 workaround
  - Low: 小问题、UI瑕疵

Impact (影响范围):
  - Widespread: 所有用户/生产环境
  - Limited: 部分用户/特定场景
  - Isolated: 个别用户/边缘场景

Urgency (紧急程度):
  - Immediate: 需要立即修复
  - Soon: 本周/本月内修复
  - Eventually: 计划修复但无明确时间
```

**优先级判定表**:

| 组合 | 优先级 |
|------|--------|
| Critical + Widespread + Immediate | **priority/critical** |
| High + Widespread + Soon | **priority/high** |
| Medium + Limited + Eventually | **priority/medium** |
| Low + Isolated + Eventually | **priority/low** |

### 社区投票机制 (VS Code 实践)

**Feature Request 自动晋升流程**:
1. 新 feature request → 标记 `status/needs-community-input`
2. 60天内获得 **20+ 👍** → 晋升为 `priority/high` + `status/ready-for-work`
3. 60天内未达标 → 自动关闭 + 评论说明

**热讨论保护**: 20+ 评论 → 不自动关闭，由维护者人工决策

### 安全漏洞优先级

使用 **CVSS v3.1** 评分:
- **9.0-10.0** (Critical) → `priority/critical` + 24小时内响应
- **7.0-8.9** (High) → `priority/high` + 7天内响应
- **4.0-6.9** (Medium) → `priority/medium` + 30天内响应
- **0.1-3.9** (Low) → `priority/low` + 下个版本修复

---

## 3. Issue 生命周期

### 完整状态机

```
[新建] → status/needs-triage
          ↓
    (人工/自动评估)
          ↓
   status/ready-for-work ←→ status/blocked (依赖其他 issue/PR)
          ↓                     ↓
   status/in-progress      (解除阻塞后返回)
          ↓
   [关闭] → status/fixed / status/duplicate / status/wontfix
          ↓
   (可选) status/verified (用户确认修复)
          ↓
   (45天后) status/locked (防止僵尸回复)
```

### Stale Issue 管理

**GitHub Actions 配置** (`actions/stale@v9`):

```yaml
name: Mark stale issues
on:
  schedule:
  - cron: "0 0 * * *"  # 每天运行

jobs:
  stale:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/stale@v9
      with:
        repo-token: ${{ secrets.GITHUB_TOKEN }}
        stale-issue-message: 'This issue has been inactive for 60 days. It will be closed in 14 days if no further activity occurs.'
        stale-issue-label: 'status/stale'
        days-before-stale: 60
        days-before-close: 14
        exempt-issue-labels: 'priority/critical,priority/high,status/in-progress,status/blocked,type/security'
        remove-stale-when-updated: true
```

**关键参数**:
- **60 天**标记为 stale (VS Code 标准)
- **14 天**后关闭 (允许时间窗口)
- **豁免标签**: P0/P1/in-progress/blocked/security 不自动标记

---

## 4. 自动化最佳实践

### 4.1 自动标签 (Auto-labeler)

**配置文件**: `.github/labeler.yml`

```yaml
# 根据文件路径自动应用标签
type/docs:
- changed-files:
  - any-glob-to-any-file: ['docs/**/*', '**/*.md', 'README*']

type/test:
- changed-files:
  - any-glob-to-any-file: ['**/*_test.go', 'tests/**/*']

area/engine:
- changed-files:
  - any-glob-to-any-file: ['internal/engine/**/*', 'engine/**/*']

area/adapter:
- changed-files:
  - any-glob-to-any-file: ['chatapps/**/*', 'provider/**/*']

area/security:
- changed-files:
  - any-glob-to-any-file: ['internal/security/**/*', '.github/workflows/*']
```

### 4.2 ML-Based Triage (VS Code 实践)

**架构**:
1. **两个 ML 模型**:
   - Model A: Issue → Feature Area → Assignee (通过 feature area 映射)
   - Model B: Issue → Assignee (直接映射，当 Model A 置信度不足时使用)

2. **置信度阈值**: **0.75** (75% 准确率最低要求)
   - 高于阈值 → 自动分配
   - 低于阈值 → 保留给 inbox tracker

3. **训练频率**: 每月重训练一次 (使用最新 issue 数据)

4. **人工干预**: 维护者可在 classifier config 中调整个人/区域阈值

### 4.3 Regex Labeler

**用途**: 检测无效/不完整 issues

```yaml
- name: Run Clipboard Labeler
  uses: ./actions/regex-labeler
  with:
    label: "status/needs-more-info"
    mustNotMatch: "^We have written the needed data into your clipboard"
    comment: "It looks like you're using the VS Code Issue Reporter but did not paste the text."
```

### 4.4 Bot 设计三原则

1. **Never Overwrite Manual Labels**: 人工标记 > 自动标记
2. **Always Use Grace Periods**: stale/close 都要有缓冲期
3. **Maintain Exempt Label Lists**: 维护豁免标签列表

---

## 5. 可关闭性信号 (Close-ability)

### 决策树

```
Issue 是否可关闭?
│
├─ 是否重复? (标题/描述提到"重复"、"已有 issue")
│  └─ YES → 关闭 + 标记 `status/duplicate` + 评论指向原 issue
│
├─ 是否已修复? (描述/PR 提到 "fixed in PR #", "已修复")
│  └─ YES → 关闭 + 标记 `status/fixed` + 关联 PR
│
├─ 是否过时? (60+ 天无更新 AND (P3/low OR 无社区兴趣))
│  └─ YES → 关闭 + 标记 `status/stale` + 评论说明
│
├─ 是否无效? (信息不足 AND 7+ 天无响应)
│  └─ YES → 关闭 + 标记 `status/needs-more-info` + 评论说明
│
├─ 是否 wontfix? (超出范围/设计决策/用户错误)
│  └─ YES → 关闭 + 标记 `status/wontfix` + 评论解释原因
│
└─ NO → 保持开放
```

### VS Code Auto-Close Labels

使用 `*` 前缀标记可自动关闭的 issues:
- `*duplicate` - 重复
- `*not-reproducible` - 无法复现
- `*out-of-scope` - 超出范围
- `*caused-by-extension` - 由扩展引起
- `*as-designed` - 设计如此
- `*wontfix` - 不予修复
- `*off-topic` - 离题

### Inactivity 阈值对比

| 项目 | Stale 天数 | Close 天数 | 豁免标签 |
|------|-----------|-----------|---------|
| **VS Code** | 60 | 14 (after stale) | P0/P1/in-progress/security |
| **Kubernetes** | 90 | 30 (after stale) | priority/critical/high |
| **React** | 30 | 7 (after stale) | status/in-progress |
| **HotPlex (建议)** | **60** | **14** | priority/critical/high, status/in-progress/blocked, type/security |

### Duplicate Detection 模式

**关键词**:
- "重复"、"duplicate"、"dup"
- "已有 issue"、"already reported"、"existing issue #"
- "same as #", "related to #"

**操作**:
1. 搜索类似 issues (标题相似度 > 80%)
2. 评论: "Duplicate of #XXX"
3. 关闭 + 标记 `status/duplicate`

### Wontfix Criteria

**合理关闭 wontfix 的场景**:
1. **超出范围**: 与项目目标不符
2. **设计决策**: 有意为之的行为
3. **用户错误**: 误用/配置错误
4. **扩展责任**: 由第三方扩展引起
5. **技术债务**: 修复成本 >> 收益

**必须做的事**:
- 评论解释原因
- 提供替代方案 (如果有)
- 礼貌但坚定

---

## 6. 实施建议

### 阶段 1: 基础标签 (1-2 周)
1. 创建 25 个最小标签集
2. 手动为所有 open issues 打标签
3. 编写标签使用文档

### 阶段 2: 基础自动化 (3-4 周)
1. 设置 `actions/stale` workflow
2. 创建 `.github/labeler.yml` (基于文件路径)
3. 添加 issue/PR 模板

### 阶段 3: 高级自动化 (2-3 月)
1. 实现 regex labeler (检测无效 issues)
2. 设置社区投票机制 (feature requests)
3. 探索 ML-based triage (如果 issue 量 > 500)

---

## 参考资料

- [Kubernetes Labels](https://github.com/kubernetes/kubernetes/labels) - 205 labels, hierarchical slash-prefix
- [React Labels](https://github.com/facebook/react/labels) - 71 labels, colon-prefix
- [VS Code Automated Issue Triaging](https://github.com/microsoft/vscode/wiki/Automated-Issue-Triaging) - 最详细的自动化实践
- [GitHub Actions: Stale](https://github.com/actions/stale) - 官方 stale bot
- [Probot: Auto-Merge](https://github.com/probot/auto-merge) - 自动合并 bot
