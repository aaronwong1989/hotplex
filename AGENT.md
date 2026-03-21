# 🤖 HotPlex: AI Agent Engineering Protocol

**Project Status**: v0.32.2 | **Core Role**: High-performance AI Agent Control Plane (Cli-as-a-Service).
This document defines the operational boundaries and technical DNA for AI agents working on **hotplex**.

---

## 1. System Philosophy & DNA
- **Cli-as-a-Service**: Bridge high-power AI CLIs (Claude Code, OpenCode) into production-ready interactive services.
- **Persistence**: Eliminate spin-up overhead via long-lived, isolated process sessions.
- **Tech Stack**: Go 1.25 | WebSocket Gateway | Regex WAF | PGID Isolation.

---

## Quick Start

```bash
make build        # 构建 hotplexd 守护进程
make test         # 运行单元测试
make test-race    # 运行竞态检测测试
make run          # 构建并前台运行
make lint         # 运行 golangci-lint

# Docker 部署
make docker-build # 构建镜像
make docker-up    # 启动服务
make docker-logs  # 查看日志
make docker-down  # 停止服务

# 系统服务管理 (systemd/launchd)
make service-install   # 安装为系统服务
make service-start     # 启动服务
make service-stop      # 停止服务
make service-status    # 检查服务状态
make service-uninstall # 卸载服务

# 测试覆盖率
make coverage      # 生成覆盖率报告
make coverage-html # 生成 HTML 覆盖率报告
```

**CLI 命令** (hotplexd):
```bash
# 启动守护进程
hotplexd start --config=/path/to/config.yaml

# 会话管理
hotplexd session list              # 列出所有会话
hotplexd session kill <session-id> # 终止会话
hotplexd session logs <session-id> # 查看会话日志

# 诊断
hotplexd status   # 运行时状态 (Admin API)
hotplexd doctor   # 全面诊断检查
hotplexd config validate <path>  # 验证配置
hotplexd version  # 显示版本
```

环境配置：复制 `.env.example` 到 `.env` 并填写凭证。

---

## 2. Engineering Standards (AI Directive)

### 2.1 Technical Constraints
| Category        | Mandatory Guideline                                                                             |
| :-------------- | :---------------------------------------------------------------------------------------------- |
| **Concurrency** | Use `sync.RWMutex` for `SessionPool`. `defer mu.Unlock()` immediately. Zero Deadlock tolerance. |
| **Isolation**   | Spawn with PGID (`Setpgid: true`). Terminate via `-PGID` (Kill process group, not just PID).    |
| **State**       | `internal/engine/pool.go` is the **State Owner**. No redundant mapping in other layers.         |
| **Errors**      | Never `panic()`. Return explicit errors. Wrap with `%w`. Use `log/slog` with context.           |

### 2.2 Go Style & Integrity
- **Uber Style**: Follow [Uber Go Style Guide](.agent/rules/uber-go-style-guide.md). Verified interface compliance is mandatory.
- **Linter Signal**: Linter errors (e.g., `unused`) signify **incomplete integration**. **Link it, don't delete it.**
- **Testing**: Features require unit tests. Mock heavy I/O (use echo/cat). `go test -race` must pass.

---

## 3. Security Boundary Protocol
1. **WAF Bypass Forbidden**: No `Stdin` input shall reach the engine without `internal/security/detector.go:CheckInput()`.
2. **Capability Governance**: Prefer native CLI tool restrictions (`AllowedTools`) over manual path interception.
3. **Sandbox Hygiene**: Ensure CLI is initialized in its specific `WorkDir`. Avoid `sh -c` unless sanitized.

---

## 4. Architectural Map (Navigation for Agents)

- **Entrypoints**: `hotplex.go` (Public SDK), `client.go` (Interface), `cmd/hotplexd/` (Daemon + CLI).
- **Orchestration**: `engine/runner.go` (I/O Multiplexer & Singleton).
- **Intelligence**: `brain/` (Native Brain - orchestration, routing, memory compression).
- **Adapters (ACL Layer)**:
    - `provider/`: Translates CLI protocols (Claude/OpenCode).
    - `chatapps/`: Translates social platforms (Slack/TG/Ding). `engine_handler.go` is the bridge.
- **Internal Core (Stability)**:
    - `internal/engine/`: `pool.go` (Pool/GC), `session.go` (Piping/PGID).
    - `internal/server/`: WebSocket & HTTP Gateway implementations.
    - `internal/security/`: `detector.go` (Regex WAF).
    - `internal/persistence/`: `marker.go` (Session durability).
    - `internal/secrets/`: Secrets provider (API key management).
    - `internal/telemetry/`: OpenTelemetry integration.
    - `internal/admin/`: Admin API server (port 9080) with session management, diagnostics, and config validation.
- **Systems**: `internal/sys/` (OS Signals), `internal/config/` (Watchers), `internal/strutil/` (High-perf utils).
- **Domain**: `types/` & `event/` (The "Universal Language" of the system).
- **Plugins**: `plugins/storage/` (Message persistence backends: SQLite, PostgreSQL).

---

## 5. Integrity & Multi-Agent Safety (CRITICAL)

### 5.1 Git Multi-Agent Protocol
In shared development, destructive commands destroy others' work.
- **STRICTLY FORBIDDEN**: `git checkout -- .`, `git reset --hard`, `git restore .`, `git clean -fd`.
- **MANDATORY CHECK**: Run `git status` before any git operation. 
- **SAFE ACTION**: Use `git checkout HEAD -- <specific-path>` or `git stash` for maintenance.
- **COMMIT FREQUENCY**: Commit/Push atomic, independent units of work often to "claim" progress.

### 5.2 Destructive Action Workflow
1. Run `git status` + `git diff --staged`.
2. Review all files; identify if changes belong to your current session.
3. If "dirty" files from other sessions exist, **request explicit user confirmation** before any broad git action.

---

## 6. AI File Editing Lifecycle (Zero-Corruption Rules)

The edit tool tracks file state. Sequential edits without re-reading cause duplicates/corruption.
1. **Read-Before-Edit**: Always `view_file` before editing, even if recently accessed. LINE#ID references must be fresh.
2. **One Edit Per Turn**: Maximum one logical block `edit`/`replace` call per response to prevent race conditions.
3. **Verify-After-Edit**: Immediately `view_file` the affected area. Confirm logic and formatting.
4. **Recovery**: If edit produces unexpected results, **STOP**, `view_file` fresh state, and `git checkout` if corrupted.

---

## 7. Action Execution Protocol
1. **Acknowledge**: State the technical plan briefly.
2. **Safety Check**: Check `git status` and architectural constraints in this document.
3. **Atomic Execution**: Write code in verifiable steps.
4. **Validation**: `go build ./...` and `go test` must pass before task completion.
5. **PR Creation**: **MANDATORY** to include `Resolves #<issue-id>` or `Refs #<issue-id>` in PR description body. This links the PR to the issue and enables automatic closure on merge.

## 7.1 Diagnostic-First Debugging Protocol

**Before diving into fixes, gather full context first.**

When encountering Docker/config issues:
1. Run `docker ps --format '{{.Names}}: {{.Status}}'` to check actual container state
2. Run `docker logs <container>` to see real error output
3. Do NOT assume root cause from symptoms alone

**Common misdiagnosis patterns:**
- Bind mount conflict → 实际是 health check 过早确认
- Gateway runs as root → 需要实际检查进程用户
- GitHub token 过期 → 实际是 host 环境变量覆盖了容器配置

**Verify before fix:**
- Docker 问题：先诊断，不要直接假设
- Git 操作：先 `git status`，确认当前分支和未提交内容
- Token 问题：确认是容器内还是 host 泄露

## 8. Gotchas & Lessons Learned

### Docker Operations
- **正确的 Makefile targets**：`docker-build-all`（不是 `build-docker-all`）、`docker-up`、`docker-down`、`docker-restart`
- **Health check 前等待**：restart 后等待 ~15 秒再确认健康状态
- **失败排查顺序**：(1) .env 文件是否存在，(2) health check grace period，(3) $$ 在 Makefile 宏中的转义
- **Volume 命名**：使用 `hotplex-*` 前缀（如 `hotplex-matrix-standalone`）
- **环境变量隔离**：host shell 变量（~/.zshrc/~/.zprofile）可能泄漏进 Docker Compose 变量替换，覆盖 token 设置

### Configuration Pitfalls
- **Shell Default Syntax**: Go's `os.ExpandEnv` does NOT support shell-style defaults (`${VAR:-default}`). Use `${VAR}` only.
  - ❌ `${HOTPLEX_SLACK_BOT_USER_ID:-}` → Treated as literal variable name
  - ✅ `${HOTPLEX_SLACK_BOT_USER_ID}` → Works correctly

### Configuration Layering
- **Priority**: `.env` (凭证/敏感值) → YAML 配置 → `inherits` 父配置 → 默认值
- **bot_user_id**: Each bot MUST have unique `bot_user_id` in .env, otherwise session IDs collide
- **message_store**: 结构定义在 YAML，敏感路径使用 `${VAR}` 环境变量
  ```yaml
  message_store:
    enabled: true
    backend: sqlite
    path: ${HOTPLEX_MESSAGE_STORE_PATH}  # 从 .env 读取
  ```

### Configuration Inheritance (inherits)
- Use `inherits: ./path/to/parent.yaml` to inherit parent configuration
- Child config overrides parent's fields with the same name
- Supports relative paths
- Circular inheritance will cause an error
- Example:
  ```yaml
  # configs/chatapps/slack-prod.yaml
  inherits: ./slack-base.yaml
  ai:
    system_prompt: "Production prompt"  # Override parent
  ```

### GitHub Token Configuration
- Token 跨文件同步：更新所有文件（.env、common.yml、add-bot.sh、env_file 引用）
- 容器环境隔离：验证容器使用独立环境，host shell 变量可能覆盖配置
- 过期排查：检查 ~/.zshrc/~/.zprofile 是否存在泄漏的环境变量

### Codebase Conventions
- Go 文件使用 `fmt.Errorf` + 小写错误消息
- **跨平台 shell 脚本**：避免 awk/sed GNU 特性，使用纯 shell 或 POSIX 兼容语法（macOS 不兼容 GNU awk）
- 权限设置：`chmod 755` 用于目录，禁止 `chmod 1777`
- **Defer 验证**：始终验证 deferred 函数是否执行，资源是否释放
- Makefile 中 `$$` 是 `$` 的转义（用于 shell 变量）

---

## 9. Release Workflow

Use `Skill(hotplex-release)` for complete release workflow (version bump → changelog → git commit → tag → CI verify → GitHub release).

---

**Mission Directive for AI Agents**: Extend HotPlex without compromising its structural density or safety. **Analyze twice, integrate once.**
