# HotPlex Issue 管理大师 - 技能创建报告

## 🎉 创建成功！

已成功创建 **HotPlex Issue 管理大师** 技能（hotplex-issue-master）

## 📁 技能结构

```
hotplex-issue-master/
├── SKILL.md                          # 主技能文档 (17KB)
├── scripts/
│   └── labeler.py                   # 核心标注引擎
└── references/
    └── label-best-practices.md      # 最佳实践参考
```

## ✨ 核心功能

### 1. 自动标注 (Auto-Labeling)
- **优先级**: critical/high/medium/low
- **类型**: bug/feature/enhancement/docs/test
- **规模**: small/medium/large
- **状态**: needs-triage/ready-for-work/blocked/stale

### 2. 生命周期管理
- 重复检测（相似度 > 80%）
- Stale issue 清理（60+ 天）
- 自动关闭建议（需确认）

### 3. 优先级动态调整
- Severity × Impact × Urgency 矩阵
- 社区投票权重
- 时间衰减机制

### 4. 批量操作
- 批量打标签
- 批量关闭/重新开放
- 批量分配

### 5. 分析报告
- Issue 趋势分析
- 瓶颈识别
- 效率指标（平均解决时间、首次响应时间）

## 🚀 使用方法

```bash
# 基础命令
"分析所有 issues 并打标签"
"检测重复 issues"
"清理 stale issues"
"生成 issue 分析报告"

# 高级命令
"只分析 P0 和 P1 的 issues"
"自动关闭 stale 且低优先级的 issues"
"分析过去 30 天的 issue 趋势"
```

## 📊 技术实现

### 标签体系
基于 **Kubernetes, React, VS Code** 等大型开源项目的最佳实践：
- Slash 前缀命名空间（`type/`, `priority/`, `status/`, `area/`, `size/`）
- 颜色编码系统（温度渐变、饱和色、交通灯）
- 最小标签集（~25个）

### 判断标准
- **优先级**: P0/P1/P2 标记 + 关键词分析 + 严重程度评估
- **类型**: 标题前缀 + 关键词匹配 + 描述分析
- **规模**: 模块数 + 架构变更 + 预估工作量
- **状态**: 创建/更新时间 + 信息完整性 + 阻塞依赖

### 自动化集成
- GitHub Actions 配置示例
- `.github/labeler.yml` 配置
- `actions/stale` workflow

## 📖 参考资料

- **VS Code Issue Triage**: https://github.com/microsoft/vscode/wiki/Automated-Issue-Triaging
- **Kubernetes Labels**: https://github.com/kubernetes/kubernetes/labels
- **GitHub Actions Stale**: https://github.com/actions/stale

## ✅ 测试状态

- [x] 核心标注引擎测试通过
- [x] 优先级分析正常
- [x] 类型识别正常
- [x] 规模估算正常
- [x] 状态分析正常
- [x] 可关闭性检测正常

## 🎯 下一步

1. **测试运行**: 在真实 HotPlex issues 上测试
2. **优化调整**: 根据实际效果调整判断标准
3. **自动化集成**: 设置 GitHub Actions 自动化
4. **文档完善**: 添加更多使用示例

## 📝 版本信息

- **版本**: v1.0.0
- **创建时间**: 2026-03-22
- **维护者**: HotPlex Team
- **基于**: GitHub issue 管理最佳实践（Kubernetes/React/VS Code）

---

**技能已准备就绪！开始使用它来管理你的 HotPlex issues 吧！** 🚀
