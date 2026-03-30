# OpenCodeServerProvider vs OpenCodeCLIProvider 深度对比

> 调研日期: 2026-03-30
> opencode 版本: 1.3.3
> 假设两者均已完整实现

---

## 0. 前提假设

两者**最终功能完全等价**：
- 都使用同一个 opencode CLI 底层
- 都支持 session 持久化和上下文复用
- 都输出相同的 event 语义（text/tool/step 等）

差异在于**进程架构**和**集成方式**，而非能力边界。

---

## 1. 进程架构对比

### 1.1 OpenCodeServerProvider（HTTP + SSE）

```
┌─────────────────────────────────────────────────────┐
│         opencode serve（单一进程，长期运行）            │
│                                                     │
│   Session A (in-memory) ←── SSE ──→ HotPlex Client  │
│   Session B (in-memory) ←── SSE ──→ HotPlex Client  │
│   Session C (in-memory) ←── SSE ──→ HotPlex Client  │
│                        HTTP POST /api/chat           │
└─────────────────────────────────────────────────────┘
         ▲                              ▲
         │                              │
    共享进程内存                   HTTP 连接（无状态）
```

- **进程数**: 1 个（`opencode serve`）
- **所有 session**: 共享同一进程的内存空间
- **传输协议**: HTTP（SSE 流式推送）
- **HotPlex 侧**: 持有 `HTTPTransport`，通过 REST API 管理 session

### 1.2 OpenCodeCLIProvider（CLI Subprocess）

```
┌──────────────────┐   stdin/stdout    ┌──────────────────┐
│  opencode run    │ ←──────────────→  │  HotPlex pool.go │
│  (Session A)     │                   │                  │
│  --continue -s A │                   │  IsAlive() = true│
└──────────────────┘                   └──────────────────┘

┌──────────────────┐   stdin/stdout    ┌──────────────────┐
│  opencode run    │ ←──────────────→  │  HotPlex pool.go │
│  (Session B)     │                   │                  │
│  --continue -s B │                   │  IsAlive() = true│
└──────────────────┘                   └──────────────────┘
```

- **进程数**: N 个（每个 session 一个 `opencode run` 进程）
- **所有 session**: 进程级完全隔离
- **传输协议**: stdio 管道（bufio.Scanner 逐行解析）
- **HotPlex 侧**: 持有 `*exec.Cmd`，通过 pool.go 管理进程生命周期

---

## 2. 多维度对比

### 2.1 资源使用

| 维度 | Server | CLI |
|------|--------|-----|
| **进程数** | 1 | N（= session 数） |
| **内存占用** | 固定（≈ 1× opencode 内存） | 线性（≈ N× opencode 内存） |
| **文件描述符** | 2 + 2N（SSE + HTTP keepalive） | 2N（stdin/stdout pipes） |
| **CPU 开销（idle）** | 1 个进程 minimal | N 个进程 minimal |
| **CPU 开销（active）** | 共享调度 | 独立调度 |

**定量估算**（假设单 opencode 进程 ~100MB 内存）：

| 并发 session | Server 内存 | CLI 内存 | 差异 |
|------------|-----------|---------|------|
| 5 | ~120MB | ~500MB | CLI 多 4× |
| 20 | ~120MB | ~2GB | CLI 多 16× |
| 100 | ~120MB | ~10GB | CLI 多 80× |

> **但注意**: 对于 HotPlex 的实际规模（每个 bot 通常 1-5 个并发 session），差距在数百 MB 量级——在现代服务器上完全可以接受。

### 2.2 启动延迟

| 场景 | Server | CLI |
|------|--------|-----|
| **冷启动（daemon 未运行）** | 需要先启动 serve | 即时 spawn |
| **Turn 1（session 新建）** | ~0ms（daemon 已在运行） | `exec.Command` spawn ~50-200ms |
| **Turn 2+（session 复用）** | ~0ms | ~0ms（进程已在 pool 中） |

Server 的优势：**Turn 1 的零延迟**。Daemon 预热后，所有新建 session 都是即时连接。

CLI 的代价：**每次新建 session 触发一次 `exec.Command` 开销**。但 HotPlex 的 pool.go 会尽量保持 session 存活（idle timeout 前复用），实际新建 session 的频率远低于总请求量。

### 2.3 故障隔离

| 故障场景 | Server | CLI |
|---------|--------|-----|
| **单 session 崩溃** | 仅该 session 挂，其他不受影响 | 仅该进程挂，其他不受影响 |
| **opencode serve 进程崩溃** | ⚠️ **全部 session 丢失** | N/A |
| **HotPlex daemon 重启** | 需要重连 serve（HTTP） | CLI 进程被 pool 清理重建 |
| **网络抖动（HTTP）** | 需要重连 | N/A（stdio pipe） |
| **OOM Killer** | serve 死 → 全灭 | 仅一个 session 进程死 |

**CLI 的关键优势**: 进程级硬隔离。一个 session 的 panic/OOM/Memory spike **绝不波及其他 session**。

Server 的致命弱点：`opencode serve` 是单点故障。一旦该进程崩溃，**所有 session 的上下文在内存中全部丢失**（除非 opencode DB 做了持久化，但重新连接后 SSE 流无法自动恢复）。

### 2.4 错误恢复

| 场景 | Server 恢复路径 | CLI 恢复路径 |
|------|--------------|------------|
| Session 执行出错 | HTTP 错误码 → 发送新请求 | 进程异常退出 → pool 清理 → 新建 session |
| serve 崩溃 | 需要外部 watchdog 重启 serve | N/A |
| 网络中断（SSE） | 需要重连 → 丢失中间事件 | N/A（stdio 无网络层） |
| Session 超时 | API 级别 timeout | pool idle timeout → cleanup |

**CLI 更简单**: 所有错误都通过"进程退出"这一统一信号表达。pool.go 的 `IsAlive()` 检查 + `cleanupSession()` 清理 + 下一请求重建，是完整可靠的恢复链。

**Server 更复杂**: HTTP 有网络层（重试/超时/连接管理），SSE 有事件订阅管理，错误类型更多（网络错误 vs API 错误 vs session 错误）。

### 2.5 安全性

| 维度 | Server | CLI |
|------|--------|-----|
| **进程隔离** | ❌ 共享同一进程 | ✅ 每个 session 独立进程 |
| **工作目录隔离** | serve 在固定目录运行，session 通过 API 参数隔离 | ✅ `--dir` flag，每个进程指定独立 workDir |
| **环境变量隔离** | 共享 serve 的 env | ✅ 每个进程独立 env（可注入不同 env） |
| **网络暴露** | ⚠️ HTTP 端口（127.0.0.1:4096 默认） | ✅ 无网络（stdio） |
| **认证** | `OPENCODE_SERVER_PASSWORD` | 无（本地 pipe） |

**CLI 的安全优势**：
1. **工作目录硬隔离**: 不同 session 的 workDir 在 OS 级别隔离，无法相互访问文件
2. **无网络攻击面**: stdio pipe 不暴露任何端口
3. **环境变量隔离**: 每个进程可注入不同的 `API_KEY` 等凭证

**Server 的安全劣势**：
1. 所有 session 共享同一 serve 进程的 uid/权限
2. HTTP 端口需要 `OPENCODE_SERVER_PASSWORD`（但默认无密码）
3. session 间通过 serve 内部状态隔离（非进程级）

### 2.6 与 HotPlex 架构契合度

| HotPlex 组件 | Server | CLI |
|------------|--------|-----|
| `pool.go` | ⚠️ 需改造（HTTPTransport 不是进程管理） | ✅ 原生契合（`GetOrCreateSession` 管理进程） |
| `Session.IsAlive()` | ⚠️ 检查 HTTP 连接状态 | ✅ 检查 `*exec.Cmd.Process` 是否存活 |
| `CLISessionStarter` | ❌ 不适用 | ✅ 直接复用 |
| `HTTPSessionStarter` | ✅ 已有实现 | ❌ 不适用 |
| `sys.KillProcessGroup` | ❌ 无 PGID 概念 | ✅ SIGTERM PGID 清理 |
| `VerifySession` | 检查 HTTP health | 检查 `opencode session list` |

**HotPlex 的设计哲学是进程隔离**：
- `pool.go` 是为 subprocess 模型设计的
- `Session.cmd` + `sys.KillProcessGroup` 是核心清理机制
- `Session.IsAlive()` 依赖 `*exec.Cmd.Process`

强行将 Server 塞入这个框架会导致：
1. `HTTPSessionStarter` 是独立于 `CLISessionStarter` 的另一套逻辑
2. `Session.io` 接口需要额外抽象
3. `VerifySession` 无法复用现有模式

### 2.7 运维复杂度

| 维度 | Server | CLI |
|------|--------|-----|
| **daemon 管理** | 需要自启动 + watchdog | 无（HotPlex 按需 spawn） |
| **日志聚合** | 单一日志源 | N 个 stderr 流（需汇总） |
| **端口占用** | 1 个 HTTP 端口 | 0 个端口 |
| **版本升级** | 重启 serve | N 个进程逐个更新 |
| **配置管理** | serve URL + password | `--dir` + env + model 参数 |
| **进程监控** | 监控 serve 存活 | 监控 pool 中的 N 个进程 |

**Server 运维代价**: 需要额外管理 `opencode serve` 守护进程（自启动脚本、watchdog、健康检查）。

**CLI 运维代价**: 更多的子进程（但 pool.go 的 cleanup loop 已经处理）。

---

## 3. 功能能力对比（无差异项）

以下功能在两者中**完全等价**：

| 功能 | Server | CLI |
|------|--------|-----|
| Session 持久化 | ✅ opencode DB | ✅ opencode DB（同一 DB） |
| `opencode run` 核心能力 | ✅（serve 内部调用） | ✅ |
| tool 调用 | ✅ | ✅ |
| streaming 输出 | ✅ SSE | ✅ stdout pipe |
| token/cost 统计 | ✅ | ✅ |
| session fork | ✅ | ✅ `--fork` |
| 模型选择 | ✅ | ✅ `-m` |
| MCP servers | ✅ | ✅ `--mcpServers` |
| `--thinking` | ✅ | ✅ |

---

## 4. 规模敏感性分析

### 4.1 小规模（1-10 并发 session）

**推荐: CLI**

- 内存差距: ~100MB vs ~1GB，可接受
- 进程隔离优势明显（一个 session 崩溃不影响其他）
- 与 Claude Code Provider 模式统一
- 无额外运维负担（pool.go 原生管理）

### 4.2 中规模（10-50 并发 session）

**倾向: CLI**

- 内存差距: ~1GB vs ~5GB，服务器可接受
- 故障隔离价值 > 资源开销
- pool.go 的进程管理在此规模下完全胜任

### 4.3 大规模（50+ 并发 session）

**倾向: Server**（但 HotPlex 极少到达此规模）

- 内存节省显著（50 个 session: ~5GB vs ~100MB）
- 统一的 opencode serve 进程更易监控
- 但: 失去进程隔离，serve 是单点

> **现实**: HotPlex 的定位是"个人 AI 助手"或"团队 bot"，预期并发 session 数量通常在 1-20 量级。Server 的资源节省优势在实践中不重要，但其单点故障和运维复杂度是真实成本。

---

## 5. 深度技术差异

### 5.1 Event 流本质

**Server (SSE)**:
```
HotPlex                    opencode serve                    opencode 核心
   │                             │                                │
   │──── POST /api/chat ─────────→│                                │
   │                             │────── opencode run ───────────→│
   │                             │←───── SSE events ──────────────│
   │←── SSE stream ──────────────│                                │
   │                             │                                │
```
两层抽象：`HotPlex` ↔ `serve` ↔ `opencode`。中间多一个进程，多一层故障点。

**CLI**:
```
HotPlex pool.go              opencode run（直接）              opencode 核心
   │                                  │                               │
   │──── stdin: JSON ────────────────→│                               │
   │←── stdout: JSON lines ───────────│                               │
   │                                  │                               │
```
单层抽象：`HotPlex` ↔ `opencode run`。直接的 stdio pipe，无中间人。

### 5.2 HotPlex 内部的架构复杂度

**Server 需要引入**:
```go
// transport_http.go - 单独的 HTTP 传输层
type HTTPTransport struct {
    client  *http.Client
    baseURL string
    sessions map[string]Subscription  // SSE 订阅管理
}

// 新增 SessionStarter 实现
type HTTPSessionStarter struct { ... }  // 已有，区别于 CLISessionStarter

// Event 映射层
func (p *OpenCodeServerProvider) mapEvent(evt OCSSEEvent) []*ProviderEvent  // 已有
```

**CLI 复用现有**:
```go
// 复用 CLISessionStarter 几乎全部逻辑
type OpenCodeCLIProvider struct {  // 新增 provider
    ProviderBase
}
func (p *OpenCodeCLIProvider) BuildCLIArgs(...) []string  // 新写（类似 ClaudeCodeProvider）
func (p *OpenCodeCLIProvider) ParseEvent(line string) []*ProviderEvent  // 新写（解析 step_start/text/step_finish）
```

**结论**: CLI Provider 与 `ClaudeCodeProvider` 共用 `CLISessionStarter`，代码复用度高，引入的新代码更少。

### 5.3 Session 生命周期边界

**Server 模式下**，`opencode serve` 的生命周期与 HotPlex daemon **完全解耦**：
- HotPlex 重启 → serve 仍运行 → 重连即可
- 但 serve 崩溃 → 需要外部 watchdog 恢复

**CLI 模式下**，session 进程生命周期由 `pool.go` 完全掌控：
- HotPlex daemon 重启 → 所有 CLI 进程被清理
- pool idle timeout → `sys.KillProcessGroup` 清理

**哪种更好？**

对于 HotPlex 这类需要"即用即走"的服务，**CLI 模式更透明**：
- 所有资源（进程、内存）都是 HotPlex 可见的
- 没有"后台还有 serve 在跑但 pool 已经忘记它"的状态歧义

### 5.4 调试可观测性

| 调试维度 | Server | CLI |
|---------|--------|-----|
| **看进程列表** | `ps aux \| grep opencode` → 仅 serve | `ps aux \| grep opencode` → 所有 N 个进程 |
| **看 session 日志** | serve 的 stderr（统一） | 每个进程的 stderr（分散） |
| **trace 一个 session** | HTTP 请求日志 | stdio pipe 数据流 |
| **模拟故障** | kill serve | kill 单个 opencode run 进程 |

CLI 的日志分散问题可通过 `Session.logFile`（pool.go 已有 `logFile *os.File`）解决——每个 session 的 stderr 重定向到独立日志文件。

---

## 6. 结论与建议

### 6.1 核心结论

```
OpenCodeServerProvider  =  更好的资源效率  +  更强的运维复杂度  +  更弱的故障隔离
OpenCodeCLIProvider     =  更好的架构契合  +  更简单的错误模型  +  更好的进程隔离
```

**对于 HotPlex 来说，CLI 模式更优**，原因按重要性排序：

1. **架构契合**: HotPlex 的 pool.go + Session + CLISessionStarter 是为 subprocess 设计的。CLI Provider 直接复用，Server Provider 需要引入独立的 HTTPSessionStarter 体系。
2. **故障隔离**: 每个 session 独立进程，一个崩溃不波及其他。这与 HotPlex 作为多租户 ChatApp 桥接器的定位完全一致。
3. **安全隔离**: 工作目录和环境变量在 OS 级别隔离，session 间无任何共享状态。
4. **运维简单**: 不需要管理 `opencode serve` 守护进程，HotPlex 按需启动/清理子进程。
5. **代码复用**: 可大量复用 `ClaudeCodeProvider` 的实现模式（`BuildCLIArgs`/`ParseEvent`/`VerifySession` 框架相同）。

### 6.2 Server 适合的场景

Server Provider 适合以下场景，如需补充支持：

- **边缘网关**: opencode serve 作为独立服务，多个 HotPlex 实例共享连接
- **轻量客户端**: 嵌入式环境，内存受限，无法启动多个进程
- **企业集中管控**: 所有 session 日志在 serve 端统一收集

### 6.3 推荐实施路径

```
Phase 1: 实现 OpenCodeCLIProvider
  - 参考 ClaudeCodeProvider（相同的 BuildCLIArgs 模式）
  - 解析 step_start / text / step_finish event
  - 复用 CLISessionStarter，零新增架构负担

Phase 2（可选）: 补充 OpenCodeServerProvider
  - 针对特定部署场景（边缘网关等）
  - 利用已有的 HTTPSessionStarter 基础设施
```

---

## 7. 参考架构

### 7.1 OpenCodeCLIProvider 核心接口

```go
type OpenCodeCLIProvider struct {
    ProviderBase
    promptBuilder *PromptBuilder
}

func (p *OpenCodeCLIProvider) BuildCLIArgs(sessionID string, opts *ProviderSessionOptions) []string {
    args := []string{
        "run",
        "--format", "json",
    }

    if opts.WorkDir != "" {
        args = append(args, "--dir", opts.WorkDir)
    }

    if opts.Model != "" {
        args = append(args, "-m", opts.Model)
    }

    if opts.ResumeSession {
        // 热复用：继续已有 session
        args = append(args, "--continue", "--session", sessionID)
    } else {
        // 新建 session：opencode 自动生成 sessionID，无法自定义
        // → Provider 从 step_start 事件的 sessionID 字段提取 ID
    }

    return args
}

func (p *OpenCodeCLIProvider) ParseEvent(line string) ([]*ProviderEvent, error) {
    var raw map[string]any
    if err := json.Unmarshal([]byte(line), &raw); err != nil {
        return nil, err
    }

    switch raw["type"] {
    case "step_start":
        return []*ProviderEvent{{Type: EventTypeStepStart}}, nil
    case "text":
        part := raw["part"].(map[string]any)
        return []*ProviderEvent{{
            Type:    EventTypeAnswer,
            Content: part["text"].(string),
        }}, nil
    case "step_finish":
        part := raw["part"].(map[string]any)
        tokens := part["tokens"].(map[string]any)
        return []*ProviderEvent{{
            Type: EventTypeResult,
            Metadata: &ProviderEventMeta{
                InputTokens:  int32(tokens["input"].(float64)),
                OutputTokens: int32(tokens["output"].(float64)),
            },
        }}, nil
    }
    return nil, nil
}

func (p *OpenCodeCLIProvider) VerifySession(sessionID, workDir string) bool {
    // 调用 opencode session list，检查 sessionID 是否存在
    out, err := exec.Command("opencode", "session", "list").Output()
    if err != nil {
        return false
    }
    return strings.Contains(string(out), sessionID)
}
```

### 7.2 与 ClaudeCodeProvider 的差异

| 方法 | Claude Code | OpenCode CLI |
|------|-----------|-------------|
| `BuildCLIArgs` | `--resume` / `--session-id` | `--continue --session` / 新建无 ID flag |
| `ParseEvent` | 解析 `result`/`tool_use`/`error` | 解析 `step_start`/`text`/`step_finish` |
| `VerifySession` | 检查 `.jsonl` 文件存在 | 检查 `opencode session list` 输出 |
| `BuildInputMessage` | `{"type":"user","message":{...}}` | `{"parts":[{"type":"text","text":"..."}]}` |
| `CleanupSession` | 删除 `.jsonl` 文件 | `opencode session delete` |
