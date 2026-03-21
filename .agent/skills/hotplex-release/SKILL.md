---
name: hotplex-release
description: Use when releasing a new version of HotPlex (any version bump: patch/minor/major, git tag, GitHub release, or CI verification)
version: 0.2.0
---

# HotPlex Release Skill

Complete release automation: version bump → changelog → git commit → tag → push → CI verify → GitHub release.

## When to Use

**Triggered by:** "发布版本", "创建 release", "版本升级", "打 tag", "发布新版本", "bump version"

**NOT for:** Regular commits, non-release git operations

## Quick Reference

| Operation | Command |
|-----------|---------|
| Current version | `grep 'Version = ' hotplex.go` |
| Last tag | `git describe --tags --abbrev=0` |
| Commits since last tag | `git log --oneline $(git describe --tags --abbrev=0)..HEAD` |
| CI status | `gh run list --limit 3` |
| Create release | `gh release create vX.Y.Z --title "vX.Y.Z" --notes-file CHANGELOG.md` |

## Release Workflow

### Step 1: 确定发布参数

用户未指定发布类型时，明确询问：
- **patch** (v0.31.9 → v0.32.0): Bug fixes
- **minor** (v0.31.9 → v0.32.0): New features
- **major** (v0.31.9 → v1.0.0): Breaking changes

### Step 2: 前置验证

**必须通过后才继续**：
```bash
# 确保工作区干净
git status

# 运行测试
go test ./...
go test -race ./...

# Lint
go vet ./...
```

### Step 3: 版本号递增

**必须更新全部 5 个文件，缺一不可：**

| # | File | Field |
|---|------|-------|
| 1 | `hotplex.go` | `Version = "vX.Y.Z"` |
| 2 | `Makefile:64` | `VERSION = X.Y.Z` |
| 3 | `CHANGELOG.md` | Header entry |
| 4 | `CLAUDE.md:3` | `vX.Y.Z` |
| 5 | `AGENT.md:3` | `vX.Y.Z` |

**版本计算：**
- **patch**: `x.y.Z+1`
- **minor**: `x.y+1.0`
- **major**: `x+1.0.0`

### Step 4: 生成 CHANGELOG

从 `git log` 自动获取自上次 tag 以来的所有提交：

```bash
git log --oneline $(git describe --tags --abbrev=0)..HEAD
```

按 Conventional Commits 归类到 `### Features` / `### Bug Fixes` / `### Maintenance`：

```markdown
## [vX.Y.Z] - YYYY-MM-DD

### Features
- feat(scope): description (PR #)

### Bug Fixes
- fix(scope): description (PR #)

### Maintenance
- chore(scope): description (PR #)
```

### Step 5: Git 操作

```bash
git add -A
git commit -m "chore(release): bump version to vX.Y.Z"
git push origin main
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z
```

### Step 6: CI 验证

**Tag push 后立即验证 CI：**

```bash
gh run list --limit 3
```

- **running**: 等待完成（轮询直到 success 或 failure）
- **failure**: 诊断原因，修复后重新运行 CI
- **success**: 继续发布

**CI 失败常见原因：**
- `go vet ./...` 或 `go test -race ./...` 失败
- Linter 报错
- 未更新的版本引用导致构建不一致

### Step 7: GitHub Release（Minor/Major 推荐）

```bash
gh release create vX.Y.Z \
  --title "Release vX.Y.Z" \
  --notes-file CHANGELOG.md \
  --target main
```

## Minor/Major 发布额外要求

### 文档防腐检查

发布 `minor` 或 `major` 时必须执行：

1. **检查源码包 README**：`*/README.md` — 新模块是否已更新
2. **检查 API 文档**：`docs-site/**/*.md` — API 变更是否同步
3. **检查配置文档**：`docs/*.md` — 配置说明是否完整

如发现文档未更新，提示用户后再继续。

### NotebookLM 同步

```bash
Skill(tool="hotplex-notebooklm")
```

同步内容：新增/修改的源码 README、API 文档、架构设计文档。

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| 版本号不一致 | 验证 5 个文件全部更新 |
| 跳过 CI 验证 | Tag push 后必须等待 CI 通过 |
| CHANGELOG 为空 | 从 `git log` 自动生成 |
| 未 push 就 tag | 确保 `git push origin main` 先完成 |
| 测试未跑就发布 | 必须 `go test ./...` 通过 |

## Error Handling

| Scenario | Action |
|----------|--------|
| 工作区不干净 | 提示用户 stash 或 commit |
| CI 失败 | 诊断原因 → 修复 → 重新验证 |
| GitHub Release 失败 | 检查 tag 是否已推送，尝试手动创建 |
| 版本号冲突 | 检查远程是否有相同 tag |
| CHANGELOG 生成失败 | 从 `git log` 手动归类 |

## Output Template

发布完成后报告：

```
✅ 版本发布完成！
- 版本号: vX.Y.Z
- Commit: <sha>
- Release: <url>
- CI: ✅ passed
```
