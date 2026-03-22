---
name: hotplex-doc-sync
description: >
  HotPlex 文档防腐同步工具。当用户提到以下场景时必须激活：
  - "更新文档"、"同步文档"、"文档防腐"、"文档一致性检查"
  - "check docs"、"sync docs"、"update documentation"
  - 发布 minor/major 版本前的文档审查
  - "哪些文档过时了"、"文档和代码不一致"
  此 skill 会全面扫描项目文档与实际实现，找出差异并更新。
---

# HotPlex Doc Sync — 文档防腐同步

## 核心原则

文档是代码的门面。过时的文档比没有文档更危险——它会误导开发者，让他们沿着错误的路径走。本 skill 的职责是**确保文档永远反映代码的真实状态**。

## 工作流程

### Step 1: 建立文档清单

首先建立项目文档清单，按优先级分类：

**第一优先级（项目核心文档，每次发布必须检查）：**
```
hotplex.go                    # Version 常量
Makefile                      # VERSION 变量
CLAUDE.md                     # Project Status 版本号
AGENT.md                      # Project Status 版本号
CHANGELOG.md                  # 变更记录
README.md                     # 根 README
ARCHITECTURE.md               # 架构文档
```

**第二优先级（模块级 README，每个 internal/*/README.md）：**
```
internal/engine/README.md
internal/config/README.md
internal/security/README.md
internal/server/README.md
internal/persistence/README.md
internal/secrets/README.md
internal/telemetry/README.md
internal/strutil/README.md
internal/sys/README.md
internal/panicx/README.md
provider/README.md
plugins/storage/README.md
brain/README.md
chatapps/README.md
```

**第三优先级（指南和参考文档）：**
```
docs/configuration.md / docs/configuration_zh.md
docs/providers/*.md
docs/observability-guide.md
docs/quick-start.md
docs-site/reference/protocol.md
```

### Step 2: 版本一致性检查

验证所有版本引用是否一致。执行：

```bash
grep 'Version = ' hotplex.go          # 获取当前版本
grep 'VERSION\s*?=' Makefile          # Makefile 版本
grep 'v[0-9]\+\.[0-9]\+' CLAUDE.md    # CLAUDE.md 版本
grep 'v[0-9]\+\.[0-9]\+' AGENT.md     # AGENT.md 版本
grep '^## \[v' CHANGELOG.md | head -1  # CHANGELOG 最新版本
```

**常见不一致场景：**
- `hotplex.go` 是真相来源，其他文件应与之对齐
- CHANGELOG 顶部版本号必须与 `hotplex.go` 完全匹配
- 如果发现不一致 → 立即修复

### Step 3: 模块 README 一致性检查

对每个 `*/README.md`，验证：

1. **包名和导入路径**是否正确
2. **导出的类型/函数/常量**是否在代码中存在
3. **结构描述**是否与实际目录结构匹配

检查方法：
```bash
# 获取模块的导出
grep -r '^type \|^func \|^var \|^const ' --include="*.go" <module>/

# 获取 README 声称的内容
grep -E '\*\*|##|`' <module>/README.md
```

**典型问题：**
- README 提到某个 package 但实际 import path 不同
- README 列出的子目录已被删除或重命名
- README 的示例代码使用了旧 API

### Step 4: API 和配置文档检查

**配置文档**（docs/configuration*.md）：
```bash
# 获取实际配置结构
grep -r 'type.*Config\|YAMLTag\|json:' --include="*.go" internal/config/

# 获取文档中的配置项
grep -E '^###|^##|^\s*-\s|\|' docs/configuration*.md
```

验证：
- 文档中的每个配置项在代码中有对应字段
- 类型描述正确（string vs int vs bool）
- 默认值正确

**Provider 文档**（docs/providers/*.md）：
```bash
# 获取实际 provider 导出
grep -r '^func \|^type ' --include="*.go" provider/

# 对比文档中的 API surface
```

### Step 5: 变更驱动的文档更新

在 minor/major 发布时，根据本次提交的变更范围更新相关文档：

```bash
git log --oneline <last-tag>..HEAD
```

对于每个 commit：
1. 判断涉及哪个模块
2. 检查该模块的 README 是否需要更新
3. 检查 docs/ 下是否有相关指南需要更新
4. 检查 docs-site/ 下是否有需要同步的内容

### Step 6: 更新决策

| 场景 | 行动 |
|------|------|
| 版本号不一致 | 立即修复 |
| README 中的导出与代码不符 | 修复 README |
| 文档提到已删除的功能 | 标记 `[DEPRECATED]` 或删除 |
| 配置项文档缺失 | 添加新配置项文档 |
| 新模块缺少 README | 创建 README |

**更新原则：**
- 保持文档风格一致
- 如果不确定，宁可保守（只更新确定错误的部分）
- 涉及架构变更的重大更新，提议后再行动

## 输出格式

检查完成后，输出报告：

```markdown
# 📋 文档一致性报告

## ✅ 版本一致性
- hotplex.go: vX.Y.Z
- Makefile: X.Y.Z
- CLAUDE.md: vX.Y.Z
- AGENT.md: vX.Y.Z
- CHANGELOG: vX.Y.Z
结论: [一致/不一致 + 详情]

## ⚠️ 模块文档问题
| 模块 | 问题 | 建议修复 |
|------|------|----------|
| internal/engine | README 提到不存在的 package | 删除引用 |

## 📝 待更新文档
- docs/providers/claudecode.md: 新增 API 未记录
```

## 自动化检查点

在发布流程中，此 skill 应在以下时间点执行：

1. **发布前审查**（minor/major）：全面检查所有三类文档
2. **发布后确认**（patch）：快速检查版本一致性 + CHANGELOG
3. **PR 审查时**（可选）：检查修改的代码是否有对应文档更新

## 快速命令参考

```bash
# 版本一致性
grep 'Version = ' hotplex.go && grep 'VERSION\s*?=' Makefile

# 模块列表
ls -d internal/*/ provider/ plugins/*/ brain/ chatapps/

# 所有 README
find . -name 'README.md' -not -path './docs-site/node_modules/*' | sort

# 最新 CHANGELOG 条目
head -5 CHANGELOG.md
```
