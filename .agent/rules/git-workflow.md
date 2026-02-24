# Git Workflow (HotPlex)

## 1. Fast Path (gh-cli)
```bash
gh issue create -t "[type] desc" -b "body"       # Create Issue
git checkout -b <type>/<id>-desc               # New Branch
git commit -m "<type>(scope): <desc> (Refs #ID)" # Commit (Atomic)
gh pr create --fill                            # Create PR (Resolves #ID)
```

## 2. Commit Standards
Strictly follow **Conventional Commits**: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `wip`.

- **Atomic**: One commit per independent logic unit (e.g., one function/test).
- **Frequent**: Commit or `git add` every 50 lines.
- **Checkpoint**: Use `wip:` for temporary saves. AI must ask to commit if >3 files changed.

## 3. Safety First (Anti-Loss)
> [!CAUTION]
> Unstaged changes are fragile. AI MUST run `git status` before any destructive command.

- **Forbidden (Solo)**: `checkout .`, `reset --hard`, `clean -fd`.
- **Dirty Tree**: If unstaged work exists, Agent **MUST** ask to `stash` or `commit` before switching branches or rebasing.
- **Recovery**: Use IDE **Timeline** or `git fsck --lost-found` for mishaps.

## 4. Pull Requests
- **Branch**: `<type>/<issue-id>-short-desc`
- **Body**: Must link issue (`Resolves #123`). Briefly list changes and test plan.
- **Main**: **Locked.** PR only.

## 5. Release
1. Update `CHANGELOG.md`.
2. `git tag -a vX.Y.Z -m "Release vX.Y.Z" && git push origin vX.Y.Z`
3. `gh release create vX.Y.Z --generate-notes`
