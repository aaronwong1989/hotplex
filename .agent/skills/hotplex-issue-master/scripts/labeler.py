#!/usr/bin/env python3
"""
HotPlex Issue 自动标注工具

基于 GitHub issue 管理最佳实践，自动分析和标记 HotPlex 项目的 issues。
"""

import re
from datetime import datetime, timedelta
from typing import Dict, List, Tuple


class IssueLabeler:
    """Issue 标注器"""

    def __init__(self):
        self.labels = {
            'priority': {
                'critical': 'priority/critical',
                'high': 'priority/high',
                'medium': 'priority/medium',
                'low': 'priority/low'
            },
            'type': {
                'bug': 'type/bug',
                'feature': 'type/feature',
                'enhancement': 'type/enhancement',
                'docs': 'type/docs',
                'test': 'type/test'
            },
            'size': {
                'small': 'size/small',
                'medium': 'size/medium',
                'large': 'size/large'
            },
            'status': {
                'needs-triage': 'status/needs-triage',
                'ready-for-work': 'status/ready-for-work',
                'blocked': 'status/blocked',
                'stale': 'status/stale'
            }
        }

    def analyze_priority(self, issue: Dict) -> str:
        """分析优先级"""
        body = issue.get('body', '').lower()
        title = issue.get('title', '').lower()

        # 检查 P0/P1/P2 标记
        if 'p0' in body or 'p0' in title:
            return self.labels['priority']['critical']

        if 'p1' in body or 'p1' in title:
            # 进一步判断严重程度
            if any(kw in body for kw in ['严重', '阻塞', 'critical', 'blocker', '生产故障']):
                return self.labels['priority']['critical']
            return self.labels['priority']['high']

        if 'p2' in body or 'p2' in title:
            return self.labels['priority']['medium']

        # 关键词判断
        critical_keywords = ['安全漏洞', 'security', '数据丢失', 'data loss', '生产故障', 'production']
        high_keywords = ['严重', 'severe', '阻塞', 'block', '核心功能', 'core feature']
        medium_keywords = ['中等', 'medium', 'workaround', '影响体验', 'affects experience']

        if any(kw in body or kw in title for kw in critical_keywords):
            return self.labels['priority']['critical']

        if any(kw in body or kw in title for kw in high_keywords):
            return self.labels['priority']['high']

        if any(kw in body or kw in title for kw in medium_keywords):
            return self.labels['priority']['medium']

        return self.labels['priority']['low']

    def analyze_type(self, issue: Dict) -> str:
        """分析类型"""
        title = issue.get('title', '')
        body = issue.get('body', '').lower()

        # 🆕 特殊类型处理：RFC/Epic 永远不是 bug
        if title.startswith('[RFC]') or '[RFC]' in title:
            return self.labels['type']['enhancement']
        if title.startswith('[Epic]') or '[Epic]' in title or title.startswith('📋 [Meta]'):
            return self.labels['type']['feature']

        # 检查标题前缀
        if title.startswith('[feat]') or '[feat]' in title:
            return self.labels['type']['feature']

        if title.startswith('[admin]') or '[admin]' in title:
            return self.labels['type']['feature']

        if title.startswith('[docs]') or '[docs]' in title:
            return self.labels['type']['docs']

        if title.startswith('[test]') or '[test]' in title:
            return self.labels['type']['test']

        # 关键词判断
        bug_keywords = ['bug', 'error', 'fail', '错误', '问题', '异常', 'crash', 'fix']
        feature_keywords = ['feature', '新功能', '新增', 'add', 'implement', '支持']
        enhancement_keywords = ['enhancement', '优化', '改进', 'improve', 'refactor', '性能']
        docs_keywords = ['docs', '文档', 'readme', 'documentation']
        test_keywords = ['test', '测试', 'coverage']

        title_lower = title.lower()

        if any(kw in title_lower or kw in body for kw in bug_keywords):
            return self.labels['type']['bug']

        if any(kw in title_lower or kw in body for kw in feature_keywords):
            return self.labels['type']['feature']

        if any(kw in title_lower or kw in body for kw in enhancement_keywords):
            return self.labels['type']['enhancement']

        if any(kw in title_lower or kw in body for kw in docs_keywords):
            return self.labels['type']['docs']

        if any(kw in title_lower or kw in body for kw in test_keywords):
            return self.labels['type']['test']

        return self.labels['type']['enhancement']  # 默认

    def analyze_size(self, issue: Dict) -> str:
        """分析规模"""
        title = issue.get('title', '')
        body = issue.get('body', '').lower()

        # 大型特征关键词
        large_keywords = ['架构', 'architecture', '重构', 'refactor', '多模块', 'multi-module',
                         '多平台', 'multi-platform', '重大功能', 'major feature']
        # 中型特征关键词
        medium_keywords = ['新增功能', 'add feature', '新功能', 'new feature', '改进', 'improve']

        if any(kw in title.lower() or kw in body for kw in large_keywords):
            return self.labels['size']['large']

        if any(kw in title.lower() or kw in body for kw in medium_keywords):
            # 检查是否涉及多个文件/模块
            if '模块' in body or 'module' in body or '多个' in body:
                return self.labels['size']['large']
            return self.labels['size']['medium']

        # 根据标题前缀判断
        if title.startswith('[feat]'):
            return self.labels['size']['medium']

        # 默认小规模
        return self.labels['size']['small']

    def analyze_status(self, issue: Dict) -> str:
        """分析状态"""
        created_at = datetime.fromisoformat(issue['created_at'].replace('Z', '+00:00'))
        updated_at = datetime.fromisoformat(issue['updated_at'].replace('Z', '+00:00'))
        now = datetime.now(updated_at.tzinfo)

        # 检查是否 stale (60+ 天无更新)
        if (now - updated_at).days > 60:
            return self.labels['status']['stale']

        # 检查是否 blocked
        body = issue.get('body', '').lower()
        if any(kw in body for kw in ['blocked', '阻塞', '依赖', 'depends on', '等待']):
            return self.labels['status']['blocked']

        # 检查信息完整性
        # 如果 body 很短或缺少关键信息，标记为 needs-triage
        if len(body) < 100:
            return self.labels['status']['needs-triage']

        # 检查是否有清晰的描述和复现步骤
        if not any(kw in body for kw in ['步骤', 'step', '复现', 'reproduce', '预期', 'expected', '实际', 'actual']):
            return self.labels['status']['needs-triage']

        return self.labels['status']['ready-for-work']

    def check_closeability(self, issue: Dict) -> Tuple[bool, str]:
        """
        检查 issue 是否可关闭

        Returns:
            (can_close, reason): 是否可关闭及原因
        """
        body = issue.get('body', '').lower()
        title = issue.get('title', '').lower()

        # 重复检测
        duplicate_signals = ['duplicate', '重复', '已有 issue', 'already reported', 'same as #']
        if any(signal in body or signal in title for signal in duplicate_signals):
            return True, 'duplicate'

        # 已修复检测
        fixed_signals = ['fixed', '已修复', 'resolved', 'closed by pr', 'fixed in #']
        if any(signal in body or signal in title for signal in fixed_signals):
            return True, 'fixed'

        # 过时检测
        updated_at = datetime.fromisoformat(issue['updated_at'].replace('Z', '+00:00'))
        now = datetime.now(updated_at.tzinfo)

        if (now - updated_at).days > 60:
            priority = self.analyze_priority(issue)
            # 只有低优先级的 stale issues 才建议关闭
            if priority == self.labels['priority']['low']:
                return True, 'stale'

        # 无效检测
        invalid_signals = ['invalid', '无效', '无法复现', 'cannot reproduce', '信息不足', 'needs more info']
        if any(signal in body for signal in invalid_signals):
            # 检查是否有 7+ 天无响应
            created_at = datetime.fromisoformat(issue['created_at'].replace('Z', '+00:00'))
            if (now - created_at).days > 7:
                return True, 'invalid'

        return False, ''

    def analyze_issue(self, issue: Dict) -> Dict[str, str]:
        """
        分析单个 issue

        Returns:
            Dict 包含所有标签
        """
        return {
            'priority': self.analyze_priority(issue),
            'type': self.analyze_type(issue),
            'size': self.analyze_size(issue),
            'status': self.analyze_status(issue)
        }


def main():
    """测试函数"""
    labeler = IssueLabeler()

    # 测试用例
    test_issue = {
        'number': 335,
        'title': 'Admin API endpoints unreachable from host machine',
        'body': '''## 问题描述

**严重程度**: P0 (阻塞运维)
**影响范围**: 所有 HotPlex 容器 (hotplex-01/02/03)

### 症状
- Admin API 端口 (19080/19081/19082) 从宿主机无法访问
- `curl http://localhost:19080/admin/v1/stats` 连接失败
''',
        'created_at': '2026-03-21T23:21:50Z',
        'updated_at': '2026-03-21T23:21:50Z'
    }

    labels = labeler.analyze_issue(test_issue)
    print("分析结果:")
    for category, label in labels.items():
        print(f"  {category}: {label}")

    can_close, reason = labeler.check_closeability(test_issue)
    print(f"\n可关闭: {can_close}")
    if can_close:
        print(f"原因: {reason}")


if __name__ == '__main__':
    main()
