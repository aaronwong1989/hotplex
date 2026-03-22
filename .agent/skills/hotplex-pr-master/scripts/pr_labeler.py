#!/usr/bin/env python3
"""
PR Labeler - 自动标注引擎

基于 PR 内容、文件变更、关联 issue 等维度自动标注 PR
"""

import re
from typing import Dict, List, Optional, Any
from dataclasses import dataclass
from enum import Enum


class Priority(Enum):
    CRITICAL = "priority/critical"
    HIGH = "priority/high"
    MEDIUM = "priority/medium"
    LOW = "priority/low"


class Type(Enum):
    BUG = "type/bug"
    FEATURE = "type/feature"
    ENHANCEMENT = "type/enhancement"
    REFACTOR = "type/refactor"
    DOCS = "type/docs"
    TEST = "type/test"


class Size(Enum):
    SMALL = "size/small"
    MEDIUM = "size/medium"
    LARGE = "size/large"


class Status(Enum):
    DRAFT = "status/draft"
    READY = "status/ready-for-review"
    REVIEW_REQUESTED = "status/review-requested"
    APPROVED = "status/approved"
    BLOCKED = "status/blocked"
    CONFLICTS = "status/conflicts"
    STALE = "status/stale"


class ReviewState(Enum):
    PENDING = "review/pending"
    APPROVED = "review/approved"
    CHANGES_REQUESTED = "review/changes-requested"


@dataclass
class PRLabels:
    priority: str
    type: str
    size: str
    status: str
    review: str


class PRLabeler:
    """PR 自动标注引擎"""

    def __init__(self):
        self.priority_keywords = {
            Priority.CRITICAL: ['p0', 'critical', 'hotfix', 'security', 'blocker'],
            Priority.HIGH: ['p1', 'high', 'important', 'urgent'],
            Priority.MEDIUM: ['p2', 'medium'],
            Priority.LOW: ['p3', 'low', 'minor']
        }

        self.type_keywords = {
            Type.BUG: ['fix', 'bugfix', 'hotfix', 'error', 'fail', 'crash'],
            Type.FEATURE: ['feat', 'feature', 'add', 'implement', 'new'],
            Type.ENHANCEMENT: ['enhance', 'improve', 'optimize', 'perf'],
            Type.REFACTOR: ['refactor', 'simplify', 'clean', 'restructure'],
            Type.DOCS: ['docs', 'documentation', 'readme', 'license'],
            Type.TEST: ['test', 'tests', 'coverage', 'spec']
        }

        self.branch_prefixes = {
            'fix/': Type.BUG,
            'bugfix/': Type.BUG,
            'hotfix/': Type.BUG,
            'feat/': Type.FEATURE,
            'feature/': Type.FEATURE,
            'docs/': Type.DOCS,
            'test/': Type.TEST,
            'tests/': Type.TEST,
            'refactor/': Type.REFACTOR,
            'enhance/': Type.ENHANCEMENT,
            'perf/': Type.ENHANCEMENT
        }

    def analyze_pr(self, pr: Dict[str, Any]) -> PRLabels:
        """
        分析 PR 并返回标签

        Args:
            pr: GitHub PR 对象

        Returns:
            PRLabels 对象
        """
        priority = self._analyze_priority(pr)
        pr_type = self._analyze_type(pr)
        size = self._analyze_size(pr)
        status = self._analyze_status(pr)
        review = self._analyze_review(pr)

        return PRLabels(
            priority=priority,
            type=pr_type,
            size=size,
            status=status,
            review=review
        )

    def _analyze_priority(self, pr: Dict[str, Any]) -> str:
        """分析优先级"""
        title = pr.get('title', '').lower()
        body = pr.get('body', '') or ''
        body_lower = body.lower()

        # 检查显式标记
        for priority, keywords in self.priority_keywords.items():
            for keyword in keywords:
                if keyword in title or keyword in body_lower:
                    return priority.value

        # 检查关联 issue 优先级
        linked_issues = self._extract_linked_issues(body)
        if linked_issues:
            # 继承第一个关联 issue 的优先级
            # 注意：需要额外调用 GitHub API 获取 issue
            pass

        # 默认中等优先级
        return Priority.MEDIUM.value

    def _analyze_type(self, pr: Dict[str, Any]) -> str:
        """分析类型"""
        title = pr.get('title', '').lower()
        body = pr.get('body', '') or ''
        body_lower = body.lower()
        head_branch = pr.get('head', {}).get('ref', '').lower()

        # 1. 检查分支名前缀
        for prefix, pr_type in self.branch_prefixes.items():
            if head_branch.startswith(prefix):
                return pr_type.value

        # 2. 检查标题前缀
        title_prefixes = {
            '[feat]': Type.FEATURE,
            '[fix]': Type.BUG,
            '[docs]': Type.DOCS,
            '[test]': Type.TEST,
            '[refactor]': Type.REFACTOR,
            '[enhance]': Type.ENHANCEMENT
        }

        for prefix, pr_type in title_prefixes.items():
            if prefix in title:
                return pr_type.value

        # 3. 检查关键词
        for pr_type, keywords in self.type_keywords.items():
            for keyword in keywords:
                if keyword in title or keyword in body_lower:
                    return pr_type.value

        # 4. 检查文件变更
        files = pr.get('files', [])
        if files:
            file_paths = [f.get('filename', '') for f in files]

            # 文档
            if all(p.endswith('.md') or p.startswith('docs/') for p in file_paths):
                return Type.DOCS.value

            # 测试
            if all('_test.go' in p or p.startswith('tests/') for p in file_paths):
                return Type.TEST.value

        # 默认为 feature
        return Type.FEATURE.value

    def _analyze_size(self, pr: Dict[str, Any]) -> str:
        """分析规模"""
        additions = pr.get('additions', 0)
        deletions = pr.get('deletions', 0)
        total_lines = additions + deletions

        changed_files = pr.get('changed_files', 0)

        # 基于行数和文件数判断
        if total_lines < 100 and changed_files < 5:
            return Size.SMALL.value
        elif total_lines > 500 or changed_files > 15:
            return Size.LARGE.value
        else:
            return Size.MEDIUM.value

    def _analyze_status(self, pr: Dict[str, Any]) -> str:
        """分析状态"""
        # Draft 状态
        if pr.get('draft', False):
            return Status.DRAFT.value

        # 检查是否 stale (30+ 天未更新)
        from datetime import datetime, timezone, timedelta
        updated_at = datetime.fromisoformat(pr['updated_at'].replace('Z', '+00:00'))
        if datetime.now(timezone.utc) - updated_at > timedelta(days=30):
            return Status.STALE.value

        # 检查是否已请求 review
        if pr.get('requested_reviewers') or pr.get('requested_teams'):
            return Status.REVIEW_REQUESTED.value

        # 默认为 ready
        return Status.READY.value

    def _analyze_review(self, pr: Dict[str, Any]) -> str:
        """分析 review 状态"""
        # 注意：需要调用 GitHub API 获取 reviews
        # 这里返回默认值
        return ReviewState.PENDING.value

    def _extract_linked_issues(self, body: str) -> List[int]:
        """从 PR 描述中提取关联 issue"""
        if not body:
            return []

        # 匹配 "Resolves #123", "Closes #456", "Fixes #789"
        pattern = r'(Resolves|Closes|Fixes)\s+#(\d+)'
        matches = re.findall(pattern, body, re.IGNORECASE)

        return [int(num) for _, num in matches]

    def check_conflicts(self, pr: Dict[str, Any]) -> bool:
        """检查冲突状态"""
        # GitHub API 的 mergeable 字段
        # None: 正在计算
        # True: 可以合并
        # False: 有冲突
        mergeable = pr.get('mergeable')

        if mergeable is False:
            return True
        else:
            return False

    def check_ci_status(self, pr: Dict[str, Any]) -> Dict[str, Any]:
        """
        检查 CI/CD 状态

        注意：需要调用 GitHub Status API
        """
        # 占位符
        return {
            'state': 'unknown',
            'failed_checks': []
        }

    def check_review_state(self, pr: Dict[str, Any]) -> str:
        """
        检查 review 状态

        注意：需要调用 GitHub Reviews API
        """
        # 占位符
        return ReviewState.PENDING.value


def main():
    """示例用法"""
    # 示例 PR
    pr = {
        'number': 345,
        'title': '[feat] Add Admin Webhook API',
        'body': 'This PR implements the Admin Webhook API endpoints.\n\nResolves #340',
        'head': {'ref': 'feat/admin-webhook-api'},
        'draft': False,
        'additions': 450,
        'deletions': 50,
        'changed_files': 12,
        'mergeable': True,
        'updated_at': '2026-03-15T10:30:00Z'
    }

    labeler = PRLabeler()
    labels = labeler.analyze_pr(pr)

    print("PR Labels:")
    print(f"  Priority: {labels.priority}")
    print(f"  Type: {labels.type}")
    print(f"  Size: {labels.size}")
    print(f"  Status: {labels.status}")
    print(f"  Review: {labels.review}")


if __name__ == '__main__':
    main()
