# OpenCode ACP 协议调研

> 调研日期: 2026-03-30
> opencode 版本: 1.3.3
> 调研方式: 测试脚本 (`scripts/test_opencode_acp.py`) + 手动探索

---

## ⚠️ 重要澄清：ACP ≠ 消息发送接口

opencode 有**两套完全不同的协议**：

| 接口 | 协议 | 用途 |
|------|------|------|
| `opencode acp` | JSON-RPC 2.0 over stdio | 进程生命周期管理（init/session_new） |
| `opencode run --format json` | 每行 JSON 对象 over stdout | **实际的消息执行和流式输出** |

**结论**: ACP 只负责进程初始化和 session 管理，**消息发送和 streaming 输出通过 `opencode run` 实现**。

---

## 1. opencode acp（JSON-RPC 2.0）

### 1.1 initialize ✅

**请求**:
```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{
  "protocolVersion": 1,          // int（非 string）
  "capabilities": {},
  "clientInfo": {"name":"hotplex","version":"0.1.0"}
}}
```

**响应**:
```json
{"jsonrpc":"2.0","id":1,"result":{
  "protocolVersion": 1,
  "agentCapabilities": {
    "loadSession": true,
    "mcpCapabilities": {"http": true, "sse": true},
    "promptCapabilities": {"embeddedContext": true, "image": true},
    "sessionCapabilities": {"fork": {}, "list": {}, "resume": {}}
  },
  "authMethods": [{"id":"opencode-login","name":"Login with opencode"}],
  "agentInfo": {"name":"OpenCode", "version": "1.3.3"}
}}
```

> ⚠️ `initialized` notification 不支持，返回 `-32601 Method not found`

### 1.2 session/new ✅

**请求**:
```json
{"method":"session/new","params":{
  "cwd": ".",        // string（必填）
  "mcpServers": []     // array（必填）
}}
```

**响应**:
```json
{"result":{
  "sessionId": "ses_xxx",
  "models": {
    "currentModelId": "opencode/big-pickle",
    "availableModels": [...]
  },
  "modes": {
    "currentModeId": "Sisyphus (Ultraworker)",
    "availableModes": [...]
  }
}}
```

### 1.3 session/update Notification ✅

`session/new` 后立即收到：
```json
{"method":"session/update","params":{
  "sessionId": "ses_xxx",
  "update": {
    "sessionUpdate": "available_commands_update",
    "availableCommands": [{"name":"init","description":"..."}]
  }
}}
```

### 1.4 所有消息发送方法均不存在 ❌

以下 JSON-RPC 方法均返回 `-32601 Method not found`：
- `session/sendMessage`, `session/message`
- `sampling/createMessage`, `mcp/sampling/createMessage`
- `prompt`, `chat/send`, `invoke/prompt`
- `auth/login`, `oauth/start`
- MCP 标准方法: `prompts/list`, `resources/list`, `tools/list`

**结论**: ACP 不负责消息发送。

---

## 2. opencode run --format json（真正的 Worker 接口）

### 2.1 基本用法

```bash
opencode run "Reply with exactly: OK" --format json --dir /path/to/project
```

### 2.2 Event Types（每行一个 JSON 对象）

#### `step_start` — 执行步骤开始
```json
{
  "type": "step_start",
  "timestamp": 1774854663321,
  "sessionID": "ses_2c26b5585ffe1oNW4eL9At4TTK",
  "part": {
    "id": "prt_xxx",
    "sessionID": "ses_xxx",
    "messageID": "msg_xxx",
    "type": "step-start"
  }
}
```

#### `text` — 文本输出（streaming）
```json
{
  "type": "text",
  "timestamp": 1774854664202,
  "sessionID": "ses_xxx",
  "part": {
    "id": "prt_xxx",
    "sessionID": "ses_xxx",
    "messageID": "msg_xxx",
    "type": "text",
    "text": "\n\nOK",
    "time": {"start": 1774854664201, "end": 1774854664201}
  }
}
```

#### `step_finish` — 执行步骤完成
```json
{
  "type": "step_finish",
  "timestamp": 1774854664208,
  "sessionID": "ses_xxx",
  "part": {
    "id": "prt_xxx",
    "sessionID": "ses_xxx",
    "messageID": "msg_xxx",
    "type": "step-finish",
    "reason": "stop",
    "cost": 0,
    "tokens": {
      "total": 47480,
      "input": 35,
      "output": 21,
      "reasoning": 0,
      "cache": {"read": 47424, "write": 0}
    }
  }
}
```

### 2.3 Session 续持（多轮对话）

```bash
# 继续上次 session
opencode run "What is 2+2?" --format json --continue --dir /path

# 继续指定 session
opencode run "..." --format json --session ses_xxx --dir /path

# Fork session（复制一份继续）
opencode run "..." --format json --fork --session ses_xxx --dir /path
```

> `sessionID` 在多次 `--continue` 调用中保持一致。

### 2.4 其他 Event Types（待探索）

根据 session/new 返回的 modes 推断，可能还有：
- `thinking` — 推理过程（`--thinking` 标志开启）
- `tool_call` / `tool_result` — 工具调用（如果执行了代码修改）
- `error` — 执行错误

---

## 3. opencode serve（HTTP 服务器）

`opencode serve --port N` 启动 web UI 服务器，提供：
- `GET /` — Web UI HTML
- `GET /api/sessions`, `POST /api/chat` 等（需要认证）

**注意**: 所有 HTTP API 端点直接返回 HTML，不支持纯 JSON API（POST 返回 HTML 重定向）。

---

## 4. 错误码汇总

| code | 场景 | 示例 |
|------|------|------|
| `-32601` | Method not found | `session/sendMessage` 等 |
| `-32602` | Invalid params | 缺少必填字段 |

---

## 5. Auth 认证

本地认证状态（`opencode auth list`）:
- Google OAuth ✅
- MiniMax API ✅

`authMethods` 返回 `["Login with opencode"]` 表示 ACP 协议层的认证提示，但不影响 `opencode run` 的本地执行。

---

## 6. HotPlex 集成方案

### 6.1 Worker Adapter 设计

**正确方案**: 使用 `opencode run --format json`，而非 ACP JSON-RPC。

```go
type OpenCodeWorker struct {
    proc     *exec.Cmd
    sessionID string  // 用于 --continue / --session
    dir      string
    stdout   *bufio.Scanner
    mu       sync.Mutex
    running  atomic.Bool
}

func (w *OpenCodeWorker) Execute(ctx context.Context, input string) error {
    w.mu.Lock()
    defer w.mu.Unlock()

    if w.sessionID != "" {
        // 继续上次 session
        args = append(args, "--continue", "--session", w.sessionID)
    }

    w.proc = exec.CommandContext(ctx, "opencode", args...)
    w.proc.Stdout = r, w.proc.Stderr = devnull

    go w.streamEvents(r)  // 解析 step_start/text/step_finish

    return w.proc.Start()
}

func (w *OpenCodeWorker) streamEvents(r io.Reader) {
    scanner := bufio.NewScanner(r)
    for scanner.Scan() {
        var event map[string]any
        json.Unmarshal(scanner.Bytes(), &event)

        switch event["type"] {
        case "step_start":
            emit(state(running))
        case "text":
            emit(messageDelta(event["part"].(map[string]any)["text"].(string)))
        case "step_finish":
            tokens := event["part"].(map[string]any)["tokens"].(map[string]any)
            emit(done(true, stats(tokens)))
        }
    }
}
```

### 6.2 AEP Event 映射

| opencode event | AEP event | 说明 |
|---------------|-----------|------|
| `step_start` | `state(running)` | 执行开始 |
| `text` | `message.delta` | 流式文本 |
| `step_finish.tokens` | `done.stats` | 统计信息 |
| `step_finish.reason=stop` | `done(success:true)` | 正常结束 |
| `step_finish.reason=error` | `error` + `done(false)` | 异常结束 |

### 6.3 session 生命周期

```bash
# Turn 1
opencode run "task" --format json --dir /project
# → sessionID=ses_xxx

# Turn 2+
opencode run "next task" --format json --continue --session ses_xxx --dir /project
# → 同一 session，自动恢复上下文

# 清理
opencode session delete ses_xxx  # 清理 session 文件
```

---

## 7. 下一步

- [ ] 实现 `provider/opencode/worker.go`（基于 `opencode run --format json`）
- [ ] 测试工具调用事件（`opencode run` + 代码修改场景）
- [ ] 测试 `--fork` 实现 session 分叉
- [ ] 测试 error/timeout 场景
- [ ] 探索 `--thinking` 推理输出格式
- [ ] 更新 Worker-Gateway-Design.md 中 opencode 的协议说明

---

## 8. 参考

- `opencode --help`
- `opencode run --help`
- `opencode session --help`
- JSON-RPC 2.0 规范: https://www.jsonrpc.org/specification
