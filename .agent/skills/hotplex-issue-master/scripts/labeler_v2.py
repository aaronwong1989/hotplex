#!/usr/bin/env python3
"""
HotPlex Issue 智能标注工具 v1.1.0

基于 GitHub issue 管理最佳实践，自动分析和标记 HotPlex 项目的 issues。
支持增量管理、智能过滤、自适应标注。
"""

import json
import os
from datetime import datetime, timezone
from typing import Dict, Tuple


class IssueLabelerV2:
    """Issue 标注器 v1.1.0 - 支持增量管理"""

    def __init__(self, state_file: str = '.issue-state.json'):
        self.state_file = state_file
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
        self.state = self.load_state()

    def load_state(self) -> Dict:
        """加载状态文件"""
        if os.path.exists(self.state_file):
            with open(self.state_file, 'r') as f:
                return json.load(f)
        return {
            'last_incremental_scan': None,
            'processed_issues': {},
            'metadata': {
                'version': '1.1.0',
                'created_at': datetime.now(timezone.utc).isoformat(),
                'updated_at': datetime.now(timezone.utc).isoformat()
            }
        }

    def save_state(self):
        """保存状态文件"""
        self.state['metadata']['updated_at'] = datetime.now(timezone.utc).isoformat()
        with open(self.state_file, 'w') as f:
            json.dump(self.state, f, indent=2)

    def should_process_issue(self, issue: Dict, force: bool = False) -> bool:
        """
        智能判断 issue 是否需要处理

        Args:
            issue: GitHub issue 对象
            force: 是否强制处理（忽略智能过滤规则）

        Returns:
            bool: 是否需要处理
        """
        if force:
            return True

        issue_number = issue['number']
        updated_at = self._parse_datetime(issue['updated_at'])
        created_at = self._parse_datetime(issue['created_at'])
        now = datetime.now(timezone.utc)

        # 规则1: 新创建的 issues (< 7天)
        if (now - created_at).days < 7:
            return True

        # 规则2: 最近有更新 (< 14天)
        if (now - updated_at).days < 14:
            return True

        # 规则3: 高优先级
        priority = self.analyze_priority(issue)
        if priority in [self.labels['priority']['critical'], self.labels['priority']['high']]:
            return True

        # 规则4: 状态为 needs-triage
        existing_labels = [l['name'] if isinstance(l, dict) else l for l in issue.get('labels', [])]
        if 'status/needs-triage' in existing_labels:
            return True

        # 规则5: 从未处理过
        if str(issue_number) not in self.state['processed_issues']:
            return True

        # 规则6: 有新更新（since last scan）
        if self.state['last_incremental_scan']:
            last_scan = self._parse_datetime(self.state['last_incremental_scan'])
            if updated_at > last_scan:
                return True

        # 其他情况：跳过
        return False

    def _parse_datetime(self, dt_str: str) -> datetime:
        """解析 ISO 格式时间字符串"""
        return datetime.fromisoformat(dt_str.replace('Z', '+00:00'))

    def analyze_priority(self, issue: Dict) -> str:
        """分析优先级"""
        body = issue.get('body', '').lower() if issue.get('body') else ''
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
        body = issue.get('body', '').lower() if issue.get('body') else ''

        # 🆕 特殊类型处理：RFC/Epic 永远不是 bug
        if '[RFC]' in title or '[Epic]' in title or title.startswith('📋'):
            return self.labels['type']['feature']

        # 检查标题前缀
        if '[feat]' in title or '[admin]' in title:
            return self.labels['type']['feature']

        if '[docs]' in title:
            return self.labels['type']['docs']

        if '[test]' in title:
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
        body = issue.get('body', '').lower() if issue.get('body') else ''

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
        if '[feat]' in title:
            return self.labels['size']['medium']

        # 默认小规模
        return self.labels['size']['small']

    def analyze_status(self, issue: Dict) -> str:
        """分析状态"""
        created_at = self._parse_datetime(issue['created_at'])
        updated_at = self._parse_datetime(issue['updated_at'])
        now = datetime.now(timezone.utc)

        # 检查是否 stale (60+ 天无更新)
        if (now - updated_at).days > 60:
            return self.labels['status']['stale']

        # 检查是否 blocked
        body = issue.get('body', '').lower() if issue.get('body') else ''
        if any(kw in body for kw in ['blocked', '阻塞', '依赖', 'depends on', '等待']):
            return self.labels['status']['blocked']

        # 检查信息完整性
        if len(body) < 100:
            return self.labels['status']['needs-triage']

        # 检查是否有清晰的描述和复现步骤
        if not any(kw in body for kw in ['步骤', 'step', '复现', 'reproduce', '预期', 'expected', '实际', 'actual']):
            return self.labels['status']['needs-triage']

        return self.labels['status']['ready-for-work']

    def analyze_issue(self, issue: Dict, preserve_existing: bool = True) -> Dict[str, str]:
        """
        分析单个 issue（支持保留已有标签）

        Args:
            issue: GitHub issue 对象
            preserve_existing: 是否保留已有标签（默认 True）

        Returns:
            Dict 包含需要添加的标签（不包含已有标签）
        """
        existing_labels = [l['name'] if isinstance(l, dict) else l for l in issue.get('labels', [])]

        result = {}

        # 分析各维度
        for category in ['priority', 'type', 'size', 'status']:
            analysis_method = getattr(self, f'analyze_{category}')
            if not analysis_method:
                continue

            recommended_label = analysis_method(issue)

            # 如果保留已有标签，检查是否已存在该维度的标签
            if preserve_existing:
                has_category_label = any(label.startswith(category + '/') for label in existing_labels)
                if has_category_label:
                    # 已有该维度的标签，跳过
                    continue

            # 否则，添加推荐标签
            result[category] = recommended_label

        return result

    def check_closeability(self, issue: Dict) -> Tuple[bool, str]:
        """
        检查 issue 是否可关闭

        Returns:
            (can_close, reason): 是否可关闭及原因
        """
        body = issue.get('body', '').lower() if issue.get('body') else ''
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
        updated_at = self._parse_datetime(issue['updated_at'])
        now = datetime.now(timezone.utc)

        if (now - updated_at).days > 60:
            priority = self.analyze_priority(issue)
            # 只有低优先级的 stale issues 才建议关闭
            if priority == self.labels['priority']['low']:
                return True, 'stale'

        # 无效检测
        invalid_signals = ['invalid', '无效', '无法复现', 'cannot reproduce', '信息不足', 'needs more info']
        if any(signal in body for signal in invalid_signals):
            # 检查是否有 7+ 天无响应
            created_at = self._parse_datetime(issue['created_at'])
            if (now - created_at).days > 7:
                return True, 'invalid'

        return False, ''

    def update_processed_state(self, issue: Dict, applied_labels: Dict[str, str]):
        """更新已处理状态"""
        issue_number = str(issue['number'])
        self.state['processed_issues'][issue_number] = {
            'labels': applied_labels,
            'processed_at': datetime.now(timezone.utc).isoformat(),
            'updated_at': issue['updated_at']
        }
        self.state['last_incremental_scan'] = datetime.now(timezone.utc).isoformat()
        self.save_state()


def main():
    """测试函数"""
    labeler = IssueLabelerV2()

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
        'updated_at': '2026-03-21T23:21:50Z',
        'labels': []
    }

    print("=== 测试 1: 智能过滤 ===")
    should = labeler.should_process_issue(test_issue)
    print(f"应该处理: {should}")

    print("\n=== 测试 2: 分析标签 ===")
    labels = labeler.analyze_issue(test_issue, preserve_existing=True)
    for category, label in labels.items():
        print(f"  {category}: {label}")

    print("\n=== 测试 3: 可关闭性检查 ===")
    can_close, reason = labeler.check_closeability(test_issue)
    print(f"可关闭: {can_close}")
    if can_close:
        print(f"原因: {reason}")


if __name__ == '__main__':
    main()
