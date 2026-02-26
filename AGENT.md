# 🤖 HotPlex: AI Agent Engineering Protocol (v1.0)

Greetings, Agent. You are operating on **hotplex**, a high-performance **AI Agent Control Plane**. As its creator, I expect you to uphold the highest standards of systems engineering and concurrency safety.

---

## 1. The HotPlex Philosophy
- **Cli-as-a-Service**: We don't build LLMs; we bridge high-power AI CLIs (Claude Code, OpenCode) into long-lived, multi-tenant interactive services.
- **Stateful Persistence**: hotplex eliminates shell spin-up overhead by maintaining persistent, isolated sessions.
- **Technical Stack**: Go 1.24 | WebSocket Gateway | OS Process Group Isolation.

---

## 2. Core Engineering Principles

### 2.1 Concurrency & State Ownership
- **The Pool is Truth**: `internal/engine/pool.go` is the exclusive owner of session states. Never maintain shadow maps in other layers.
- **Thread Safety**: Always use `sync.RWMutex` when accessing the Pool. `defer mu.Unlock()` must follow locking immediately.
- **Zero Deadlock Tolerance**: Avoid calling external hooks or callbacks within a locked section.

### 2.2 Lifecycle & OS Isolation
- **PGID Enforcement**: All spawned processes *must* belong to a Process Group ID (`SysProcAttr{Setpgid: true}`).
- **Zombie Prevention**: Signals (SIGKILL) must be sent to the **negative PGID** (`-PGID`) to clean up the entire process tree (Node.js/Python orphans).

### 2.3 Error & Logging
- **No Panic**: Never use `panic()` in production logic. Return explicit, wrapped errors: `fmt.Errorf("context: %w", err)`.
- **Structured Slog**: Use `log/slog` with context (e.g., `session_id`, `user_id`).

---

## 3. Architecture Protocol (Navigation Map)

| Layer          | Responsibility                           | Key Files                                                  |
| :------------- | :--------------------------------------- | :--------------------------------------------------------- |
| **Public API** | Entry points & SDK interfaces            | `hotplex.go`, `client.go`, `cmd/hotplexd/`                 |
| **Engine**     | Session orchestration & I/O multiplexing | `engine/runner.go`                                         |
| **Providers**  | CLI protocol adapters (Anti-Corruption)  | `provider/`, `provider/factory.go`                         |
| **ChatApps**   | Social platform adapters (Slack/TG/Ding) | `chatapps/`, `chatapps/engine_handler.go`                  |
| **Internal**   | State owner (Pool), PGID logic, WAF      | `internal/engine/pool.go`, `internal/security/detector.go` |
| **Gateways**   | WebSocket & OpenCode HTTP translation    | `internal/server/`                                         |
| **Domain**     | Universal types & Event protocols        | `types/`, `event/`                                         |

---

## 4. Security & Safety Boundaries

### 4.1 Input/Output Governance
- **WAF Enforcement**: All `Stdin` content *must* pass through `internal/security/detector.go:CheckInput()`. 
- **Tool Restrictions**: Prioritize native CLI governance (`AllowedTools`) over redundant path interception.

### 4.2 Multi-Agent GIT Protocol (HIGHEST PRIORITY)
In a shared local development environment, destructive commands can destroy others' work.
- **Forbidden**: `git checkout -- .`, `git reset --hard`, `git restore .`. **Never** execute these without explicit user confirmation.
- **Targeted Action**: Only restore specific files: `git checkout HEAD -- <path>`.
- **Stashing**: Use `git stash` to protect uncommitted work before maintenance.
- **Micro-Commits**: Commit after every logically independent task.

---

## 5. Development Strategy

### 5.1 Testing & Integrity
- **Test-Driven Refinement**: New features require unit tests. Mock I/O (use echo/cat) to avoid spawning heavy CLIs during CI.
- **Linter as Signal**: Linter errors (e.g., `unused`) signify missing integration. **Fix by linking**, never by lazy deletion.
- **Race Detection**: Always run `go test -race ./...` before submitting.

### 5.2 File Editing Workflow
To prevent duplicate content or corruption:
1. **Read-Before-Edit**: Always refresh your view of the file line numbers.
2. **One Edit Per Turn**: Perform one logical block update, then verify.
3. **Verify-After-Edit**: Re-read the file to ensure the patch applied correctly.

---

## 6. Resources
- **Code Style**: See [.agent/rules/uber-go-style-guide.md](.agent/rules/uber-go-style-guide.md) for critical Go rules.
- **Reference**: High-fidelity implementation patterns can be found in `/Users/huangzhonghui/openclaw`.

---

AI Agent, your mission is to extend HotPlex without compromising its structural integrity. **Analyze twice, code once.**
