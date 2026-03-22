# GitHub Actions 配置示例

本文档提供了 HotPlex PR 管理的 GitHub Actions 自动化配置示例。

## PR Triage Workflow

创建 `.github/workflows/pr-triage.yml`:

```yaml
name: PR Triage
on:
  pull_request:
    types: [opened, edited, synchronize, ready_for_review, review_requested]
  pull_request_review:
    types: [submitted]
  schedule:
    - cron: '0 */6 * * *'  # 每 6 小时运行一次

jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Auto-label PRs
        uses: actions/github-script@v7
        with:
          script: |
            // 自动标注
            const labeler = require('./.github/pr-labeler.js');
            await labeler.process(context);

      - name: Check conflicts
        uses: actions/github-script@v7
        with:
          script: |
            // 冲突检测
            const conflicts = require('./.github/conflict-checker.js');
            await conflicts.check(context);

      - name: Review reminder
        uses: actions/github-script@v7
        with:
          script: |
            // Review 提醒
            const reminder = require('./.github/review-reminder.js');
            await reminder.check(context);

      - name: CI status monitor
        uses: actions/github-script@v7
        with:
          script: |
            // CI/CD 监控
            const monitor = require('./.github/ci-monitor.js');
            await monitor.check(context);
```

---

## 自动标签配置

创建 `.github/pr-labeler.yml`:

```yaml
type/docs:
- changed-files:
  - any-glob-to-any-file: ['docs/**/*', '**/*.md', 'LICENSE']

type/test:
- changed-files:
  - any-glob-to-any-file: ['**/*_test.go', 'tests/**/*']

type/feature:
- head-branch: ['^feat/', '^feature/']

type/bug:
- head-branch: ['^fix/', '^bugfix/', '^hotfix/']

type/refactor:
- head-branch: ['^refactor/']

area/engine:
- changed-files:
  - any-glob-to-any-file: ['internal/engine/**/*', 'brain/**/*']

area/adapter:
- changed-files:
  - any-glob-to-any-file: ['chatapps/**/*', 'provider/**/*']

area/security:
- changed-files:
  - any-glob-to-any-file: ['internal/security/**/*']
```

---

## Branch Protection Rules

创建 `.github/settings.yml`（需要 GitHub Settings App）:

```yaml
branches:
  - name: main
    protection:
      required_pull_request_reviews:
        required_approving_review_count: 1
        dismiss_stale_reviews: true
        require_code_owner_reviews: true
      required_status_checks:
        strict: true
        contexts:
          - ci/test
          - ci/lint
          - ci/build
      enforce_admins: true
      restrictions: null
```

### CODEOWNERS 配置

创建 `.github/CODEOWNERS`:

```
# Engine
/internal/engine/ @hrygo/engine-team
/brain/ @hrygo/engine-team

# Adapters
/chatapps/ @hrygo/adapter-team
/provider/ @hrygo/adapter-team

# Security
/internal/security/ @hrygo/security-team

# Documentation
/docs/ @hrygo/docs-team
**/*.md @hrygo/docs-team
```

---

## Auto-merge Workflow

创建 `.github/workflows/auto-merge.yml`:

```yaml
name: Auto Merge
on:
  pull_request_review:
    types: [submitted]

jobs:
  auto-merge:
    runs-on: ubuntu-latest
    if: github.event.review.state == 'approved'
    steps:
      - name: Check mergeability
        uses: actions/github-script@v7
        with:
          script: |
            const pr = await github.pulls.get({
              owner: context.repo.owner,
              repo: context.repo.repo,
              pull_number: context.issue.number
            });

            // 检查条件
            const canMerge =
              !pr.data.draft &&
              pr.data.mergeable &&
              pr.data.state === 'open' &&
              !pr.data.labels.some(l => l.name === 'status/blocked');

            if (canMerge) {
              // 等待 5 分钟给其他 reviewer 时间
              await new Promise(r => setTimeout(r, 5 * 60 * 1000));

              // 再次检查是否仍可合并
              const updated = await github.pulls.get({
                owner: context.repo.owner,
                repo: context.repo.repo,
                pull_number: context.issue.number
              });

              if (updated.data.state === 'open') {
                await github.pulls.merge({
                  owner: context.repo.owner,
                  repo: context.repo.repo,
                  pull_number: context.issue.number,
                  merge_method: 'squash'
                });
              }
            }
```

---

## Stale PR 清理

创建 `.github/workflows/stale-prs.yml`:

```yaml
name: Stale PRs
on:
  schedule:
    - cron: '0 0 * * *'  # 每日运行

jobs:
  stale:
    runs-on: ubuntu-latest
    steps:
      - name: Mark stale PRs
        uses: actions/stale@v9
        with:
          repo-token: ${{ secrets.GITHUB_TOKEN }}
          stale-pr-message: 'This PR has been inactive for 30 days.'
          days-before-stale: 30
          days-before-close: 14
          exempt-pr-labels: 'priority/critical,priority/high,status/in-progress,status/blocked'
```

---

## PR Labeler 脚本

创建 `.github/pr-labeler.js`:

```javascript
module.exports = {
  async process(context) {
    const { github, payload } = context;
    const pr = payload.pull_request;

    // 标签分析逻辑
    const labels = analyzePR(pr);

    // 应用标签
    if (labels.length > 0) {
      await github.issues.addLabels({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: pr.number,
        labels: labels
      });
    }
  }
};

function analyzePR(pr) {
  const labels = [];

  // 类型分析
  if (pr.head.ref.startsWith('feat/')) {
    labels.push('type/feature');
  } else if (pr.head.ref.startsWith('fix/')) {
    labels.push('type/bug');
  } else if (pr.head.ref.startsWith('docs/')) {
    labels.push('type/docs');
  } else if (pr.head.ref.startsWith('test/')) {
    labels.push('type/test');
  }

  // 规模分析
  const totalChanges = pr.additions + pr.deletions;
  if (totalChanges < 100) {
    labels.push('size/small');
  } else if (totalChanges > 500) {
    labels.push('size/large');
  } else {
    labels.push('size/medium');
  }

  return labels;
}
```

---

## Conflict Checker 脚本

创建 `.github/conflict-checker.js`:

```javascript
module.exports = {
  async check(context) {
    const { github, payload } = context;
    const pr = payload.pull_request;

    // 检查冲突
    if (pr.mergeable === false) {
      // 添加冲突标签
      await github.issues.addLabels({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: pr.number,
        labels: ['status/conflicts']
      });

      // 评论提醒
      await github.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: pr.number,
        body: '⚠️ This PR has conflicts with the base branch. Please rebase.'
      });
    } else {
      // 移除冲突标签
      try {
        await github.issues.removeLabel({
          owner: context.repo.owner,
          repo: context.repo.repo,
          issue_number: pr.number,
          name: 'status/conflicts'
        });
      } catch (e) {
        // 标签不存在，忽略
      }
    }
  }
};
```

---

## Review Reminder 脚本

创建 `.github/review-reminder.js`:

```javascript
const { formatDistanceToNow, parseISO } = require('date-fns');

module.exports = {
  async check(context) {
    const { github } = context;
    const owner = context.repo.owner;
    const repo = context.repo.repo;

    // 获取所有 open PRs
    const prs = await github.pulls.list({
      owner,
      repo,
      state: 'open',
      per_page: 100
    });

    for (const pr of prs.data) {
      // 跳过 Draft
      if (pr.draft) continue;

      // 检查创建时间
      const createdAt = parseISO(pr.created_at);
      const daysOld = Math.floor((Date.now() - createdAt) / (1000 * 60 * 60 * 24));

      // 超过 7 天未 review
      if (daysOld >= 7) {
        const reviews = await github.pulls.listReviews({
          owner,
          repo,
          pull_number: pr.number
        });

        // 如果没有任何 review
        if (reviews.data.length === 0) {
          // 评论提醒
          const message = `🔔 This PR has been waiting for review for ${daysOld} days.\n\ncc: ${pr.requested_reviewers.map(r => `@${r.login}`).join(' ')}`;

          await github.issues.createComment({
            owner,
            repo,
            issue_number: pr.number,
            body: message
          });
        }
      }
    }
  }
};
```

---

## CI Monitor 脚本

创建 `.github/ci-monitor.js`:

```javascript
module.exports = {
  async check(context) {
    const { github, payload } = context;
    const pr = payload.pull_request;

    // 获取 commit status
    const status = await github.repos.getCombinedStatusForRef({
      owner: context.repo.owner,
      repo: context.repo.repo,
      ref: pr.head.sha
    });

    if (status.data.state === 'failure') {
      // 添加阻塞标签
      await github.issues.addLabels({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: pr.number,
        labels: ['status/blocked']
      });

      // 获取失败详情
      const failedChecks = status.data.statuses
        .filter(s => s.state === 'failure')
        .map(s => s.context);

      // 评论提醒
      const message = `❌ CI Failed:\n${failedChecks.map(c => `- ${c}`).join('\n')}`;

      await github.issues.createComment({
        owner: context.repo.owner,
        repo: context.repo.repo,
        issue_number: pr.number,
        body: message
      });
    } else if (status.data.state === 'success') {
      // 移除阻塞标签
      try {
        await github.issues.removeLabel({
          owner: context.repo.owner,
          repo: context.repo.repo,
          issue_number: pr.number,
          name: 'status/blocked'
        });
      } catch (e) {
        // 标签不存在，忽略
      }
    }
  }
};
```

---

## 使用说明

1. **安装依赖**：
   ```bash
   npm install date-fns
   ```

2. **创建配置文件**：
   - 复制上述配置到 `.github/` 目录
   - 根据项目需求调整参数

3. **启用 GitHub Actions**：
   - 确保仓库已启用 Actions
   - 推送配置文件到 main 分支

4. **测试**：
   - 创建测试 PR
   - 观察自动标注和检查

---

**维护者**: HotPlex Team
**最后更新**: 2026-03-22
