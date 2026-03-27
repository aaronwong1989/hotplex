# HotPlex Issue 标签体系

## 标签维度总览

HotPlex 采用 7 维度 34 标签标准，基于 Kubernetes、React、VS Code 最佳实践。

---

## 1. 优先级 (Priority)

| 标签 | 判断标准 | 示例信号 |
|------|---------|---------|
| `priority/critical` | 阻塞核心功能、安全漏洞、数据丢失 | P0, 阻塞, 生产故障, 安全漏洞 |
| `priority/high` | 严重影响用户体验、频繁出现 | P1, 用户体验严重受影响 |
| `priority/medium` | 中等影响、有 workaround | P2, 需要修复但不紧急 |
| `priority/low` | 小问题、nice-to-have | P3, 改进建议 |

**判断逻辑**：
1. 检查 body 中的 P0/P1/P2/P3 标记
2. 关键词：严重程度、影响范围、阻塞
3. 安全相关 → critical
4. 用户报告的生产问题 → high/medium

---

## 2. 类型 (Type)

| 标签 | 判断标准 |
|------|---------|
| `type/bug` | 标题包含 bug/error/fail/错误/问题，或描述了异常行为 |
| `type/feature` | 标题包含 [feat]/feature/新功能，或请求新能力 |
| `type/enhancement` | 标题包含 enhancement/优化/改进，或改进现有功能 |
| `type/docs` | 文档相关、README、API 文档 |
| `type/test` | 测试相关、测试覆盖、测试用例 |
| `type/refactor` | 标题包含 refactor/重构，或代码重构 |
| `type/security` | 安全相关、CVE、安全漏洞、权限问题 |

**判断逻辑**：
1. 检查标题前缀：`[feat]`, `[admin]`, `[docs]`, `[test]`, `[refactor]`
2. 关键词匹配：bug, feature, enhancement, docs, test, refactor, security
3. 根据描述内容判断

---

## 3. 规模 (Size)

| 标签 | 预估工作量 | 判断标准 |
|------|----------|---------|
| `size/small` | < 1天 | 单文件修改、简单配置、文档更新 |
| `size/medium` | 1-3天 | 多文件修改、新功能、重构 |
| `size/large` | > 3天 | 架构变更、多模块影响、复杂功能 |

**判断逻辑**：
1. 涉及模块数（单模块 vs 多模块）
2. 是否需要架构变更
3. 是否涉及多个子系统（engine, adapter, security 等）
4. 关键词：重构、架构、多平台

---

## 4. 状态 (Status)

| 标签 | 判断标准 |
|------|---------|
| `status/needs-triage` | 新 issue，需要进一步评估 |
| `status/ready-for-work` | 信息完整，可以开始工作 |
| `status/blocked` | 依赖其他 issues/外部因素 |
| `status/in-progress` | 正在处理中 |
| `status/stale` | 60+ 天无更新，可能过时 |

**状态流转**：
```
[新建] → needs-triage
         ↓ (信息完整)
   ready-for-work ←→ blocked
         ↓ (开始工作)
   in-progress
         ↓ (完成)
   closed → (45天后) locked
```

---

## 5. 平台 (Platform)

| 标签 | 说明 |
|------|------|
| `platform/slack` | Slack 平台相关 |
| `platform/telegram` | Telegram 平台相关 |
| `platform/feishu` | 飞书平台相关 |
| `platform/discord` | Discord 平台相关 |

---

## 6. 模块 (Area)

| 标签 | 说明 |
|------|------|
| `area/engine` | 核心引擎 (internal/engine) |
| `area/adapter` | 平台适配器 (chatapps/) |
| `area/provider` | AI Provider 集成 (provider/) |
| `area/security` | 安全模块 (internal/security) |
| `area/admin` | Admin API (internal/admin) |
| `area/brain` | Native Brain 路由 (brain/) |

---

## 7. 特殊标签 (Special)

| 标签 | 说明 |
|------|------|
| `good first issue` | 适合新手，简单问题 |
| `help wanted` | 需要社区帮助 |
| `epic` | 大型功能，包含多个子 issues |
| `wontfix` | 不会修复 |
| `duplicate` | 重复 issue |

---

## 标签最佳实践

### 避免标签冲突
- ✅ 一个 issue 只有一个 priority 标签
- ✅ 一个 issue 只有一个 type 标签
- ✅ 一个 issue 只有一个 size 标签
- ✅ 可以有多个 platform/area 标签

### 标签组合示例
```
# Bug 优先级最高
priority/critical, type/bug, size/medium, platform/slack, area/adapter

# 新功能请求
priority/high, type/feature, size/large, area/engine

# 文档更新
priority/low, type/docs, size/small
```

---

## 可关闭性判断

**自动识别可关闭的 issues**：
- **重复 (duplicate)**：描述中提到"重复"、"已有 issue"
- **已修复 (fixed)**：描述中提到"已修复"、"fixed in PR #"
- **过时 (stale)**：60+ 天无更新且优先级低
- **无效 (invalid)**：信息不足、无法复现、用户错误

**操作**：生成建议列表，不自动关闭，需人工确认

---

**参考**：
- [Kubernetes Labels](https://github.com/kubernetes/kubernetes/labels)
- [VS Code Issue Triage](https://github.com/microsoft/vscode/wiki/Automated-Issue-Triaging)
- [GitHub Actions Labeler](https://github.com/actions/labeler)
