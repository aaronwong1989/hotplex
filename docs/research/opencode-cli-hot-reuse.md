# Claude Code vs OpenCode CLI 热复用对比调研

> 调研日期: 2026-03-30
> opencode 版本: 1.3.3 | Claude Code: 最新版
> 调研方式: 代码分析 (`pool.go`, `session.go`, `claude_provider.go`) + 手动测试

---

## 1. 结论速览

| 维度 | Claude Code CLI | opencode CLI (`opencode run`) | opencode Server (HTTP) |
|------|----------------|-------------------------------|------------------------|
| **进程模型** | 长驻进程 / session | 长驻进程 / session | 单一服务器进程 |
| **会话持久化** | `~/.claude/projects/.../*.jsonl` | opencode 数据库 (`~/.local/share/opencode/`) | opencode 数据库 |
| **热复用 flag** | `--resume <session-id>` | `--continue` / `--session <id>` | HTTP API session 绑定 |
| **多会话支持** | 多进程各自独立 | 多进程各自独立 | 单一进程内多 session |
| **HotPlex 支持** | ✅ `ClaudeCodeProvider` | ❌ 无 CLI Provider | ✅ `OpenCodeServerProvider` |
| **VerifySession** | ✅ 检查 `.jsonl` 文件 | ❌ 未实现 | ✅ HTTP health check |

---

## 2. 进程生命周期对比

### 2.1 Claude Code CLI

```
Turn 1:  hotplex pool
              │
              ▼
         ┌─────────────────────────────────┐
         │  spawn: claude --session-id X  │
         │       (长期运行)                │
         └─────────────────────────────────┘
              │
              ├── stdin: {"type":"user",...}
              │
Turn 2:  hotplex pool ──→ 同一进程
              │               (IsAlive() == true)
              │               stdin: {"type":"user",...}
              │               (--resume X, 不重建进程)
              ▼
         idle timeout → cleanupSession()
```

**关键机制**:
- `SessionPool.GetOrCreateSession()`: 检查 `sm.sessions[sessionID]` 是否存活
- 如果 `sess.IsAlive()` → 直接复用，不重建进程
- Turn 间通过 stdin 发送新的 JSON 消息，保持同一进程
- `CLISessionStarter` 持有 `*exec.Cmd`，进程在 `Session` 对象销毁前一直运行

### 2.2 opencode CLI (`opencode run`)

**当前 HotPlex 缺失该 Provider**，但机制上完全可行：

```
Turn 1:
         spawn: opencode run --format json
              │       (长期运行)
              ▼
         stdin: "Reply with exactly: 001\n"

Turn 2:  pool ──→ 同一进程 (IsAlive())
              │    stdin: "Next task\n" (--continue)

Turn N:  pool ──→ 同一进程
```

**关键 flag**:
- 新建 session：无 `--session-id` flag，session ID 由 opencode 自动生成并从 `step_start` 事件提取
- `--continue`: 继续上次 session（配合 `--session <id>`）
- `--session <id>`: 指定要继续的 session ID
- `--fork`: 叉出独立 session
- `--format json`: 输出 JSON 格式事件流（每行一个 JSON 对象）

**Session 存储**:
```bash
opencode session list
# ses_xxx  | Title | Updated
# ses_yyy  | ...
```

### 2.3 opencode Server (HTTP)

```
opencode serve (单一进程，长期运行)
     │
     ├── session_aaa (in-memory)
     │      └── Turn 1, 2, 3... (同一 HTTP 连接)
     │
     └── session_bbb (in-memory)
            └── Turn 1, 2...
```

HotPlex 的 `OpenCodeServerProvider` 通过 SSE 订阅会话事件，实现多会话管理。

---

## 3. 热复用：两层含义

"热复用"在 HotPlex 架构中有**两个层次**：

### 3.1 Pool 层热复用（进程复用）

```
❌ 冷复用: 每条指令 → spawn 新进程 → 等待退出
   问题: 每次冷启动 ~3-5s（LLM 加载、工具初始化）

✅ 热复用: 进程常驻 → 收到指令 → stdin 发送 → 流式输出
   优势: 无冷启动延迟
```

`pool.go` 的 `GetOrCreateSession`:
```go
// 1. 检查内存中是否有存活的 Session
if sess, ok := sm.sessions[sessionID]; ok {
    if sess.IsAlive() {
        return sess, false, nil  // 热复用！不重建
    }
}
// 2. 启动新进程
sess, err := sm.starter.StartSession(ctx, sessionID, cfg, prompt, nil)
```

### 3.2 Session 层热复用（上下文复用）

```
Claude Code:
  Turn 1: --session-id ses_xxx  →  .jsonl 写入历史
  Turn 2: --resume ses_xxx      →  .jsonl 读取 → LLM 理解上下文

opencode CLI:
  Turn 1: (无 --session-id flag) → opencode 自动生成 ID → DB 写入历史
  Turn 2: --continue --session ses_xxx  →  DB 读取 → LLM 理解上下文
```

**两者效果相同**：客户端（HotPlex）无感知差异，都是同一个 sessionId，CLI 内部自动处理上下文。

---

## 4. Claude Code vs opencode CLI 详细对比

### 4.1 会话存储路径

| | Claude Code | opencode CLI |
|--|------------|-------------|
| **存储位置** | `~/.claude/projects/<workspace-key>/<session-id>.jsonl` | `~/.local/share/opencode/` (数据库) |
| **存储格式** | JSONL（每行一个 event） | SQLite / JSON |
| **workspace key** | 路径转义: `/Users/hzh/.hotplex` → `-Users-hzh--hotplex` | 按目录隔离 |
| **session 列表** | 无 CLI 命令 | `opencode session list` |
| **session 删除** | 删除 `.jsonl` 文件 | `opencode session delete <id>` |

### 4.2 Session 验证（Resume 判定）

```go
// Claude Code: 检查 .jsonl 文件是否存在
func (p *ClaudeCodeProvider) VerifySession(sessionID, workDir string) bool {
    sessionPath := filepath.Join(
        homeDir, ".claude", "projects",
        toWorkspaceKey(cwd),  // 路径转义
        sessionID+".jsonl",
    )
    _, err := os.Stat(sessionPath)
    return err == nil
}
```

opencode CLI 无对应实现（但可通过 `opencode session list` 的输出验证）。

### 4.3 CLI 参数对比

| 场景 | Claude Code | opencode CLI |
|------|------------|-------------|
| 创建新 session | `--session-id <id>` | 无 flag（ID 由 opencode 自动生成） |
| 继续已有 session | `--resume <id>` | `--continue --session <id>` |
| 指定目录 | 无 flag（使用 cwd） | `--dir /path` |
| 指定模型 | `--model claude-sonnet-4-6` | `-m provider/model` |
| 输出格式 | `--output-format stream-json` | `--format json` |
| 指定 agent | `--agent agent-name` | `--agent agent-name` |
| 工具限制 | `--allowed-tools r,w,s` | (需探索) |

### 4.4 Event 格式对比

**Claude Code** (`stream-json`):
```json
{"type":"result","sessionId":"...","result":"...","modelUsage":{...}}
{"type":"error","error":"..."}
```

**opencode CLI** (`--format json`):
```json
{"type":"step_start","sessionID":"...","part":{...}}
{"type":"text","timestamp":...,"part":{"type":"text","text":"..."}}
{"type":"step_finish","timestamp":...,"part":{"type":"step-finish","tokens":{...}}}
```

| 字段 | Claude Code | opencode CLI |
|------|------------|-------------|
| Session ID 字段 | `sessionId` | `sessionID` |
| 文本 delta | `result` / `content` | `part.text` |
| Token 统计 | `modelUsage` | `part.tokens` |
| 工具调用 | `type:"tool_use"` | `part.type:"tool"` |
| 执行结束 | `type:"result"` | `type:"step_finish"` |
| 错误 | `type:"error"` | 待探索 |

---

## 5. HotPlex 当前状态

### 5.1 现有 Providers

| Provider | 模式 | 热复用 | 说明 |
|----------|------|--------|------|
| `ClaudeCodeProvider` | CLI subprocess | ✅ | 完整实现 |
| `OpenCodeServerProvider` | HTTP + SSE | ✅ | 通过 `--attach URL` 连接 |
| `PiProvider` | CLI subprocess | ❌ | 无 session 复用 |

### 5.2 缺失：`OpenCodeCLIProvider`

当前没有 `OpenCodeCLIProvider`，即 HotPlex 不支持直接通过 `opencode run` 驱动 opencode。

**需要实现**:
```go
type OpenCodeCLIProvider struct {
    ProviderBase
    // 与 ClaudeCodeProvider 类似，但:
    // - CLI flag: --continue / --session
    // - Event 解析: 解析 step_start / text / step_finish
    // - VerifySession: 调用 opencode session list 检查
}
```

### 5.3 实现方案（待开发）

```go
func (p *OpenCodeCLIProvider) BuildCLIArgs(sessionID string, opts *ProviderSessionOptions) []string {
    args := []string{
        "run",
        "--format", "json",
    }

    if opts.ResumeSession {
        args = append(args, "--continue", "--session", sessionID)
    } else {
        // 新建 session：opencode 无 --session-id flag
        // → Provider 从 step_start 事件的 sessionID 字段提取并存储
    }

    if opts.WorkDir != "" {
        args = append(args, "--dir", opts.WorkDir)
    }

    return args
}
```

### 5.4 OpenCodeServerProvider vs OpenCodeCLIProvider

| 维度 | Server (HTTP) | CLI |
|------|--------------|-----|
| 进程数 | 1 个 `opencode serve` 服务所有 session | N 个 `opencode run` 进程（每个 session 一个） |
| 部署依赖 | 需要 `opencode serve` 守护进程 | 直接调用 CLI |
| SSE 事件 | ✅ 完整（message.part.updated 等） | ✅ 完整（step_start/text/step_finish） |
| session 管理 | HTTP API | CLI flag + DB |
| hot-multiplexing | ✅（已有实现） | 待实现 |

---

## 6. 核心问题解答

### Q: opencode CLI 支持热复用（一次启动 + 连续指令）吗？

**✅ 支持**。

```
Turn 1:  opencode run "task 1" --format json --dir /project
Turn 2+: opencode run "task 2" --format json --continue --session X --dir /project
```

同一 `--session X` 下：
- session 数据在 opencode DB 中持久化
- CLI 进程在 Turn 间保持运行（pool 层热复用）
- 无需每次重建进程

### Q: Claude Code 和 opencode 的热复用机制有何本质区别？

**没有本质区别**，都是：
1. Pool 层：进程在 Turn 间保持运行
2. Session 层：`--resume` / `--continue` 确保上下文连续

**实现细节差异**：
| | Claude Code | opencode CLI |
|--|------------|-------------|
| Session 数据格式 | JSONL | SQLite |
| 进程间隔离 | 每个 session 一个进程 | 每个 session 一个进程 |
| Session 验证 | 检查 `.jsonl` | 调用 `opencode session list` |

### Q: HotPlex 对 opencode CLI 的热复用支持现状？

**❌ 未实现**。需要新建 `OpenCodeCLIProvider`，参考 `ClaudeCodeProvider` 的实现模式。

### Q: `opencode acp` 和 `opencode run` 如何选择？

| 场景 | 推荐 | 原因 |
|------|------|------|
| HotPlex Worker Adapter | `opencode run` | 完整事件流，完整 token 统计 |
| 进程生命周期管理 | `opencode acp` | JSON-RPC 管理 init/session/fork |
| MCP Server | `opencode mcp` | MCP 协议集成 |

**两者可结合**：用 `opencode acp` 管理进程，用 `opencode run` 执行任务。

---

## 7. 下一步

- [ ] **实现 `OpenCodeCLIProvider`**：参考 `ClaudeCodeProvider`，实现 `BuildCLIArgs` + `ParseEvent`
- [ ] **Event 解析**：实现 `step_start` → `state(running)`, `text` → `message.delta`, `step_finish` → `done`
- [ ] **VerifySession**：调用 `opencode session list` 验证 session 存在
- [ ] **多会话池**：验证多个 `opencode run` 进程可同时运行
- [ ] **错误处理**：测试 `step_finish.reason=error` 场景

---

## 8. 参考

- `pool.go`: `GetOrCreateSession` 详解
- `session.go`: `Session.IsAlive()` 机制
- `claude_provider.go`: `BuildCLIArgs` + `VerifySession`
- `opencode run --help`: CLI flag 完整参考
- `opencode session --help`: session 管理命令
