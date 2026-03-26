# OpenCode Server Provider — 实现规格 (终版)

> **原则**: 整洁架构 · DRY · SOLID
> **Issue**: [#356](https://github.com/hrygo/hotplex/issues/356) · **方案**: Server Provider (`opencode serve`)
> **上游**: [anomalyco/opencode](https://github.com/anomalyco/opencode) · [SDK Types](https://github.com/anomalyco/opencode/blob/dev/packages/sdk/js/src/gen/types.gen.ts)

> [!IMPORTANT]
> **战略决策**: OpenCode 放弃 CLI subprocess 方案，直接采用 Server Provider。
> 现有 `opencode_provider.go` 标记废弃。ACP Provider/Server 作为远期规划。

---

## 1. 架构

### 1.1 目标架构

```
┌─────────────────────────────────────────────────┐
│  Engine (Application Layer)                      │
│  ┌───────────────────────────────────────┐       │
│  │  Provider Interface (Domain Contract) │       │
│  └──────────┬────────────────────────────┘       │
│             │ DIP                                 │
│  ┌──────────┴────────────────────────────┐       │
│  │  OpenCodeServerProvider               │       │
│  │  ├─ ParseEvent()   (事件解析)         │       │
│  │  ├─ DetectTurnEnd() (Turn 检测)       │       │
│  │  └─ HTTPTransport  (通信层 · DI)      │       │
│  │       ├─ POST /session/:id/message    │       │
│  │       ├─ GET /event (SSE)             │       │
│  │       └─ 断连重连                     │       │
│  └───────────────────────────────────────┘       │
│                                                   │
│  ┌───────────────────┐  ┌────────────────────┐   │
│  │ ClaudeCodeProvider│  │ PiProvider         │   │
│  │ (CLI subprocess)  │  │ (CLI subprocess)   │   │
│  └───────────────────┘  └────────────────────┘   │
└─────────────────────────────────────────────────┘
```

### 1.2 设计决策

| 决策            | 选择           | 理由                              |
| --------------- | -------------- | --------------------------------- |
| OpenCode CLI    | ❌ 废弃         | 冷启动慢、无 session 管理、无 SSE |
| Server Provider | ✅ 唯一方案     | 零冷启动、完整 API、实时事件流    |
| Provider 接口   | 不修改         | OCP — 后向兼容 Claude/Pi          |
| Transport 抽象  | ✅ 新增         | SRP — 通信 ≠ 解析                 |
| Pool 适配       | SessionStarter | DIP — CLI/HTTP 策略分离           |

---

## 2. 共享类型 — `provider/opencode_types.go` [NEW]

> 与 `opencode_http.go`（出站）共用，消除重复。

```go
package provider

import "encoding/json"

// ── Part 类型 ──
const (
    OCPartText       = "text"
    OCPartReasoning  = "reasoning"
    OCPartTool       = "tool"
    OCPartStepStart  = "step-start"
    OCPartStepFinish = "step-finish"
    OCPartAgent      = "agent"
    OCPartSubtask    = "subtask"
)

// ── SSE Event 类型 ──
const (
    OCEventMessagePartUpdated = "message.part.updated"
    OCEventMessageUpdated     = "message.updated"
    OCEventSessionStatus      = "session.status"
    OCEventSessionIdle        = "session.idle"
    OCEventSessionError       = "session.error"
    OCEventPermissionUpdated  = "permission.updated"
)

// ── Part 结构 ──
type OCPart struct {
    ID        string `json:"id"`
    SessionID string `json:"sessionID"`
    MessageID string `json:"messageID"`
    Type      string `json:"type"`

    // text / reasoning
    Text string `json:"text,omitempty"`

    // tool
    CallID string       `json:"callID,omitempty"`
    Tool   string       `json:"tool,omitempty"`
    State  *OCToolState `json:"state,omitempty"`

    // step-finish
    Reason string    `json:"reason,omitempty"`
    Cost   float64   `json:"cost,omitempty"`
    Tokens *OCTokens `json:"tokens,omitempty"`
}

type OCToolState struct {
    Status string         `json:"status"`
    Input  map[string]any `json:"input,omitempty"`
    Output string         `json:"output,omitempty"`
    Title  string         `json:"title,omitempty"`
    Error  string         `json:"error,omitempty"`
}

type OCTokens struct {
    Input     int32   `json:"input"`
    Output    int32   `json:"output"`
    Reasoning int32   `json:"reasoning"`
    Cache     OCCache `json:"cache"`
}

type OCCache struct {
    Read  int32 `json:"read"`
    Write int32 `json:"write"`
}

// ── Event 结构 ──
type OCEvent struct {
    Type       string          `json:"type"`
    Properties json.RawMessage `json:"properties"`
}

type OCGlobalEvent struct {
    Directory string  `json:"directory"`
    Payload   OCEvent `json:"payload"`
}

type OCPartUpdateProps struct {
    Part  OCPart `json:"part"`
    Delta string `json:"delta,omitempty"`
}

type OCSessionStatusProps struct {
    SessionID string         `json:"sessionID"`
    Status    OCSessionState `json:"status"`
}

type OCSessionState struct {
    Type    string `json:"type"`
    Attempt int    `json:"attempt,omitempty"`
}

type OCSessionErrorProps struct {
    SessionID string   `json:"sessionID,omitempty"`
    Error     *OCError `json:"error,omitempty"`
}

type OCError struct {
    Name string         `json:"name"`
    Data map[string]any `json:"data"`
}

// ── Session / Message ──
type OCSession struct {
    ID        string      `json:"id"`
    ProjectID string      `json:"projectID"`
    Directory string      `json:"directory"`
    ParentID  string      `json:"parentID,omitempty"`
    Title     string      `json:"title"`
    Version   string      `json:"version"`
    Time      OCTimeStamp `json:"time"`
}

type OCTimeStamp struct {
    Created int64 `json:"created"`
    Updated int64 `json:"updated"`
}

type OCAssistantMessage struct {
    ID         string   `json:"id"`
    SessionID  string   `json:"sessionID"`
    Role       string   `json:"role"`
    ParentID   string   `json:"parentID"`
    ModelID    string   `json:"modelID"`
    ProviderID string   `json:"providerID"`
    Cost       float64  `json:"cost"`
    Tokens     OCTokens `json:"tokens"`
    Finish     string   `json:"finish,omitempty"`
    Error      *OCError `json:"error,omitempty"`
}

type OCUserMessage struct {
    ID        string          `json:"id"`
    SessionID string          `json:"sessionID"`
    Role      string          `json:"role"`
    Agent     string          `json:"agent"`
    Model     OCModelRef      `json:"model"`
    System    string          `json:"system,omitempty"`
    Tools     map[string]bool `json:"tools,omitempty"`
}

type OCModelRef struct {
    ProviderID string `json:"providerID"`
    ModelID    string `json:"modelID"`
}
```

---

## 3. Transport 层

### 3.1 接口 — `provider/transport.go` [NEW]

```go
type Transport interface {
    Connect(ctx context.Context, cfg TransportConfig) error
    Send(ctx context.Context, sessionID string, message map[string]any) error
    Events() <-chan string
    CreateSession(ctx context.Context, title string) (string, error)
    DeleteSession(ctx context.Context, sessionID string) error
    RespondPermission(ctx context.Context, sessionID, permissionID, response string) error
    Health(ctx context.Context) error
    Close() error
}

type TransportConfig struct {
    Endpoint string
    Env      map[string]string
    WorkDir  string
}
```

### 3.2 HTTPTransport — `provider/transport_http.go` [NEW]

> 本节为 HTTPTransport 完整实现。SSE 断连重连的详细展开见 §13。

**核心能力**: REST 调用 + SSE 流 + 指数退避重连 + Basic Auth

```go
type HTTPTransport struct {
    baseURL    string
    restClient *http.Client  // REST 请求 (30s timeout)
    sseClient  *http.Client // SSE 长连接 (无 timeout)
    password   string
    events     chan string
    cancel     context.CancelFunc
    logger     *slog.Logger
    mu         sync.Mutex
    connected  bool
}

// ── SSE 断连重连 ──
func (t *HTTPTransport) streamSSE(ctx context.Context) {
    backoff := []time.Duration{1*time.Second, 2*time.Second, 5*time.Second, 10*time.Second}
    for attempt := 0; ; attempt++ {
        if ctx.Err() != nil { return }
        err := t.connectAndStream(ctx)
        if ctx.Err() != nil { return }
        delay := backoff[min(attempt, len(backoff)-1)]
        t.logger.Warn("SSE reconnecting", "attempt", attempt, "delay", delay, "error", err)
        time.Sleep(delay)
    }
}

func (t *HTTPTransport) connectAndStream(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/event", nil)
    req.Header.Set("Accept", "text/event-stream")
    if t.password != "" {
        req.SetBasicAuth("opencode", t.password)
    }
    resp, err := t.client.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: ") {
            select {
            case t.events <- strings.TrimPrefix(line, "data: "):
            default: // buffer full, drop
            }
        }
    }
    return scanner.Err()
}

// ── REST 方法 ──
func (t *HTTPTransport) Send(ctx context.Context, sessionID string, msg map[string]any) error { ... }
func (t *HTTPTransport) CreateSession(ctx context.Context, title string) (string, error) { ... }
func (t *HTTPTransport) DeleteSession(ctx context.Context, id string) error { ... }
func (t *HTTPTransport) Health(ctx context.Context) error { ... }

// ── Permission 响应 ──
func (t *HTTPTransport) RespondPermission(ctx context.Context, sessionID, permID, response string) error { ... }
```

---

## 4. Provider 实现 — `provider/opencode_server_provider.go` [NEW]

### 4.1 构造 + Plugin 注册

```go
const ProviderTypeOpenCodeServer ProviderType = "opencode-server"

type OpenCodeServerProvider struct {
    ProviderBase
    transport     *HTTPTransport
    opts          ProviderConfig
    promptBuilder *PromptBuilder
    sessions      sync.Map // ProviderSessionID → OC SessionID
}

func NewOpenCodeServerProvider(cfg ProviderConfig, logger *slog.Logger) (*OpenCodeServerProvider, error) {
    ocCfg := cfg.OpenCode
    if ocCfg == nil { ocCfg = &OpenCodeConfig{} }
    url := ocCfg.ServerURL
    if url == "" {
        port := 4096
        if ocCfg.Port > 0 { port = ocCfg.Port }
        url = fmt.Sprintf("http://127.0.0.1:%d", port)
    }

    return &OpenCodeServerProvider{
        ProviderBase: ProviderBase{
            meta: ProviderMeta{
                Type:        ProviderTypeOpenCodeServer,
                DisplayName: "OpenCode (Server)",
                BinaryName:  "opencode",
                InstallHint: "brew install anomalyco/tap/opencode",
                Features: ProviderFeatures{
                    SupportsResume:      true,
                    SupportsStreamJSON:  true,
                    SupportsSSE:         true,
                    SupportsHTTPAPI:     true,
                    SupportsSessionID:   true,
                    MultiTurnReady:      true,
                },
            },
            logger: logger.With("provider", "opencode-server"),
        },
        transport: &HTTPTransport{
            baseURL:    url,
            restClient: &http.Client{Timeout: 30 * time.Second},
            sseClient:  &http.Client{},
            password:   ocCfg.Password,
            events:     make(chan string, 256),
            logger:     logger.With("component", "oc_transport"),
        },
        opts:          cfg,
        promptBuilder: NewPromptBuilder(false),
    }, nil
}

// Plugin 注册
func init() { RegisterPlugin(&openCodeServerPlugin{}) }
type openCodeServerPlugin struct{}
func (p *openCodeServerPlugin) Type() ProviderType { return ProviderTypeOpenCodeServer }
func (p *openCodeServerPlugin) New(cfg ProviderConfig, logger *slog.Logger) (Provider, error) {
    return NewOpenCodeServerProvider(cfg, logger)
}
func (p *openCodeServerPlugin) Meta() ProviderMeta { ... }
```

### 4.2 Provider 适配方法

```go
func (p *OpenCodeServerProvider) ValidateBinary() (string, error) {
    if err := p.transport.Health(context.Background()); err != nil {
        return "", fmt.Errorf("opencode server unreachable at %s: %w", p.transport.baseURL, err)
    }
    return p.transport.baseURL, nil
}

func (p *OpenCodeServerProvider) BuildCLIArgs(_ string, _ *ProviderSessionOptions) []string {
    return nil // Server 模式无 CLI 子进程
}

func (p *OpenCodeServerProvider) BuildInputMessage(prompt, taskInstructions string) (map[string]any, error) {
    body := map[string]any{
        "parts": []map[string]any{{"type": OCPartText, "text": p.promptBuilder.Build(prompt, taskInstructions)}},
    }
    if cfg := p.opts.OpenCode; cfg != nil {
        if m := cfg.Model; m != "" {
            if parts := strings.SplitN(m, "/", 2); len(parts) == 2 {
                body["model"] = map[string]string{"providerID": parts[0], "modelID": parts[1]}
            }
        }
        if a := cfg.Agent; a != "" { body["agent"] = a }
    }
    return body, nil
}
```

### 4.3 事件解析

```go
func (p *OpenCodeServerProvider) ParseEvent(line string) ([]*ProviderEvent, error) {
    var global OCGlobalEvent
    if err := json.Unmarshal([]byte(line), &global); err != nil {
        return nil, fmt.Errorf("parse SSE event: %w", err)
    }
    return p.mapEvent(global.Payload)
}

func (p *OpenCodeServerProvider) mapEvent(evt OCEvent) ([]*ProviderEvent, error) {
    switch evt.Type {
    case OCEventMessagePartUpdated:
        var props OCPartUpdateProps
        json.Unmarshal(evt.Properties, &props)
        return p.mapPart(props.Part, props.Delta)

    case OCEventMessageUpdated:
        var props struct{ Info OCAssistantMessage `json:"info"` }
        json.Unmarshal(evt.Properties, &props)
        if props.Info.Finish != "" {
            return []*ProviderEvent{{Type: EventTypeResult, RawType: evt.Type,
                Metadata: &ProviderEventMeta{
                    InputTokens: props.Info.Tokens.Input, OutputTokens: props.Info.Tokens.Output,
                    CacheReadTokens: props.Info.Tokens.Cache.Read, CacheWriteTokens: props.Info.Tokens.Cache.Write,
                    TotalCostUSD: props.Info.Cost,
                }}}, nil
        }
        return nil, nil

    case OCEventSessionIdle:
        return []*ProviderEvent{{Type: EventTypeResult, RawType: evt.Type}}, nil

    case OCEventSessionStatus:
        var props OCSessionStatusProps
        json.Unmarshal(evt.Properties, &props)
        switch props.Status.Type {
        case "busy":
            return []*ProviderEvent{{Type: EventTypeSystem, Status: "running"}}, nil
        case "retry":
            return []*ProviderEvent{{Type: EventTypeSystem, Status: "retrying",
                Content: fmt.Sprintf("Retrying (attempt %d)", props.Status.Attempt)}}, nil
        }
        return nil, nil

    case OCEventSessionError:
        var props OCSessionErrorProps
        json.Unmarshal(evt.Properties, &props)
        msg := "unknown error"
        if props.Error != nil { msg = props.Error.Name }
        return []*ProviderEvent{{Type: EventTypeError, Error: msg, IsError: true}}, nil

    case OCEventPermissionUpdated:
        var perm struct{ ID, Type, SessionID, Title string }
        json.Unmarshal(evt.Properties, &perm)
        return []*ProviderEvent{{
            Type: EventTypePermissionRequest, ToolName: perm.Title, ToolID: perm.ID,
            Content: fmt.Sprintf("[Permission] %s: %s", perm.Type, perm.Title),
        }}, nil

    default:
        return nil, nil
    }
}
```

### 4.4 Part → ProviderEvent 映射

```go
func (p *OpenCodeServerProvider) mapPart(part OCPart, delta string) ([]*ProviderEvent, error) {
    switch part.Type {
    case OCPartText:
        c := delta; if c == "" { c = part.Text }
        return []*ProviderEvent{{Type: EventTypeAnswer, Content: c}}, nil

    case OCPartReasoning:
        c := delta; if c == "" { c = part.Text }
        return []*ProviderEvent{{Type: EventTypeThinking, Content: c}}, nil

    case OCPartTool:
        if part.State == nil { return nil, nil }
        switch part.State.Status {
        case "pending", "running":
            return []*ProviderEvent{{Type: EventTypeToolUse,
                ToolName: part.Tool, ToolID: part.CallID,
                ToolInput: part.State.Input, Status: "running"}}, nil
        case "completed":
            return []*ProviderEvent{{Type: EventTypeToolResult,
                ToolName: part.Tool, ToolID: part.CallID,
                Content: part.State.Output, Status: "success"}}, nil
        case "error":
            return []*ProviderEvent{{Type: EventTypeToolResult,
                ToolName: part.Tool, ToolID: part.CallID,
                Error: part.State.Error, IsError: true, Status: "error"}}, nil
        }

    case OCPartStepStart:
        return []*ProviderEvent{{Type: EventTypeStepStart}}, nil

    case OCPartStepFinish:
        meta := &ProviderEventMeta{TotalCostUSD: part.Cost}
        if t := part.Tokens; t != nil {
            meta.InputTokens, meta.OutputTokens = t.Input, t.Output
            meta.CacheReadTokens, meta.CacheWriteTokens = t.Cache.Read, t.Cache.Write
        }
        return []*ProviderEvent{{Type: EventTypeStepFinish, Metadata: meta}}, nil
    }
    return nil, nil
}

func (p *OpenCodeServerProvider) DetectTurnEnd(e *ProviderEvent) bool {
    return e != nil && (e.Type == EventTypeResult || e.Type == EventTypeError)
}
```

### 4.5 Session 管理

```go
// Session ID 映射: ProviderSessionID (SHA1) → OC Session ID (server 分配)
func (p *OpenCodeServerProvider) getOrCreateOCSession(ctx context.Context, providerSessionID string) (string, error) {
    if id, ok := p.sessions.Load(providerSessionID); ok { return id.(string), nil }
    ocID, err := p.transport.CreateSession(ctx, providerSessionID)
    if err != nil { return "", err }
    p.sessions.Store(providerSessionID, ocID)
    return ocID, nil
}

func (p *OpenCodeServerProvider) CleanupSession(id string, _ string) error {
    if ocID, ok := p.sessions.LoadAndDelete(id); ok {
        return p.transport.DeleteSession(context.Background(), ocID.(string))
    }
    return nil
}

func (p *OpenCodeServerProvider) VerifySession(id string, _ string) bool {
    _, ok := p.sessions.Load(id)
    return ok && p.transport.Health(context.Background()) == nil
}
```

---

## 5. Pool 适配 — `internal/engine/session_starter.go` [NEW]

> [!IMPORTANT]
> `pool.go:startSession()` 是 180+ 行 OS 进程生命周期管理方法。引入 `SessionStarter` 策略接口解耦。

> [!CAUTION]
> **Session.WriteInput() 写 stdin，nil stdin 会 panic**（`session_test.go:1153` 已确认）。
> **Session.IsAlive() 检查 cmd.Process，nil cmd 返回 false**，Pool 会误判 HTTP session 已死。
> 必须为 Session 注入 I/O 抽象。

> **编辑注记**: 本节侧重接口定义与实现代码。§11 从问题背景与 Pool 集成视角描述同一设计，两节互为补充。

```go
// ── Session I/O 抽象 ──

type SessionIO interface {
    WriteInput(msg map[string]any) error
    IsAlive() bool
    Close() error
}

// CLISessionIO — 原有 stdin/stdout 模式
type CLISessionIO struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    stderr io.ReadCloser
}

func (io *CLISessionIO) WriteInput(msg map[string]any) error {
    data, _ := json.Marshal(msg)
    _, err := io.stdin.Write(append(data, '\n'))
    return err
}

func (io *CLISessionIO) IsAlive() bool {
    return io.cmd != nil && io.cmd.Process != nil && sys.IsProcessAlive(io.cmd.Process)
}

// HTTPSessionIO — HTTP Transport 模式
type HTTPSessionIO struct {
    transport provider.Transport
    sessionID string
}

func (io *HTTPSessionIO) WriteInput(msg map[string]any) error {
    return io.transport.Send(context.Background(), io.sessionID, msg)
}

func (io *HTTPSessionIO) IsAlive() bool {
    return io.transport.Health(context.Background()) == nil
}

// ── SessionStarter 策略接口 ──

type SessionStarter interface {
    StartSession(ctx context.Context, cfg StartSessionConfig) (*Session, error)
}

// CLISessionStarter — 原有 startSession 逻辑提取（Claude/Pi 使用）
type CLISessionStarter struct { ... }

// HTTPSessionStarter — Server Provider 使用
type HTTPSessionStarter struct {
    transport provider.Transport
    provider  provider.Provider
}

func (s *HTTPSessionStarter) StartSession(ctx context.Context, cfg StartSessionConfig) (*Session, error) {
    ocSessionID, err := s.transport.CreateSession(ctx, cfg.SessionID)
    if err != nil { return nil, err }

    sess := &Session{
        ID:                cfg.SessionID,
        ProviderSessionID: ocSessionID,
        Config:            cfg.Config,
        Status:            SessionStatusReady, // 立即 Ready — 无冷启动
        CreatedAt:         time.Now(),
        LastActive:        time.Now(),
        statusChange:      make(chan SessionStatus, 10),
        logger:            cfg.Logger,
        io:                &HTTPSessionIO{transport: s.transport, sessionID: ocSessionID},
    }
    go s.consumeSSE(sess)
    return sess, nil
}

func (s *HTTPSessionStarter) consumeSSE(sess *Session) {
    for line := range s.transport.Events() {
        if cb := sess.GetCallback(); cb != nil {
            _ = cb("raw_line", line)
        }
    }
    sess.SetStatus(SessionStatusDead)
    if cb := sess.GetCallback(); cb != nil {
        _ = cb("runner_exit", nil)
    }
}
```

---

## 6. 配置

```go
type OpenCodeConfig struct {
    // 保留字段（后向兼容 — 勿删除，避免 YAML 反序列化异常）
    UseHTTPAPI bool   `json:"use_http_api,omitempty" koanf:"use_http_api"` // Deprecated: Server 模式始终使用 HTTP
    PlanMode   bool   `json:"plan_mode,omitempty" koanf:"plan_mode"`
    Provider   string `json:"provider,omitempty" koanf:"provider"`
    Model      string `json:"model,omitempty" koanf:"model"`
    Port       int    `json:"port,omitempty" koanf:"port"`
    // 新增
    ServerURL string `json:"server_url,omitempty" koanf:"server_url"`
    Agent     string `json:"agent,omitempty" koanf:"agent"`
    Password  string `json:"password,omitempty" koanf:"password"`
}
```

```yaml
provider:
  type: opencode-server
  opencode:
    server_url: "http://127.0.0.1:4096"
    agent: "build"
    model: "anthropic/claude-sonnet-4-20250514"
```

---

## 7. Docker Sidecar

```bash
# docker-entrypoint.sh
start_opencode_server() {
    command -v opencode &>/dev/null || return 1
    local port="${OPENCODE_SERVER_PORT:-4096}"
    opencode serve --port "$port" --hostname 127.0.0.1 &
    for i in $(seq 1 60); do
        curl -sf "http://127.0.0.1:$port/" >/dev/null 2>&1 && return 0
        sleep 0.5
    done
    return 1
}
```

---

## 8. 错误映射

> **编辑注记**: 本节为错误映射速查表。完整代码实现（含 Go 代码）见 §16。

| OpenCode Error             | HotPlex 处理           | ProviderEventType |
| -------------------------- | ---------------------- | ----------------- |
| `ProviderAuthError`        | API key 无效 提示       | `error`           |
| `UnknownError`             | 透传 message           | `error`           |
| `MessageOutputLengthError` | 输出过长               | `error`           |
| `MessageAbortedError`      | 用户主动取消           | `result`          |
| `APIError` (retryable)     | 等待自动重试           | `system`          |
| `APIError` (fatal)         | 带 statusCode          | `error`           |

---

## 9. 文件变更清单

| 操作         | 文件                                        | 说明                          | 原则            |
| ------------ | ------------------------------------------- | ----------------------------- | --------------- |
| **[NEW]**    | `provider/transport.go`                     | Transport 接口                | SRP · ISP · DIP |
| **[NEW]**    | `provider/transport_http.go`                | HTTPTransport (SSE+REST+重连) | SRP             |
| **[NEW]**    | `provider/opencode_types.go`                | 共享类型                      | DRY             |
| **[NEW]**    | `provider/opencode_server_provider.go`      | Server Provider + Plugin      | OCP · LSP       |
| **[NEW]**    | `provider/opencode_server_provider_test.go` | 单元测试                      | —               |
| **[NEW]**    | `internal/engine/session_starter.go`        | SessionStarter 策略           | SRP · DIP       |
| **[MODIFY]** | `provider/provider.go`                      | `ProviderTypeOpenCodeServer`  | OCP             |
| **[MODIFY]** | `internal/server/opencode_http.go`          | 引用共享类型                  | DRY             |
| **[MODIFY]** `internal/engine/pool.go`                   | 注入 SessionStarter           | DIP             |
| **[MODIFY]** `docker/docker-entrypoint.sh`               | sidecar 启动                  | —               |
| **[DELETE]** | `provider/opencode_provider.go`            | 删除                          | —               |

---

## 10. 验证计划

```bash
# 单元测试
go test ./provider/ -run TestOCPartMarshal -v
go test ./provider/ -run TestServerProviderParseEvent -v
go test ./provider/ -run TestMapPartToEvents -v
go test ./provider/ -run TestHTTPTransport -v

# 集成测试
opencode serve --port 4096 &
curl -s -X POST http://localhost:4096/session -d '{"title":"test"}' | jq .id
curl -N http://localhost:4096/event &
curl -s -X POST http://localhost:4096/session/{id}/message \
  -d '{"parts":[{"type":"text","text":"hello"}]}'
```

---

## 11. Pool 集成 — SessionStarter 策略

> **编辑注记**: 本节从问题背景与集成视角描述 SessionStarter 设计，与 §5 从接口定义视角的描述互为补充。

### 11.1 问题

`pool.go:startSession()` 是 180+ 行的 OS 进程生命周期管理方法（`exec.Command`, stdin/stdout pipe, PGID, Job Handle, Marker Store, `ReadStdout()` goroutine 等）。Server Provider 的 Session 创建逻辑完全不同——HTTP 请求 + SSE 消费。无法用单条 `if` 旁路。

### 11.2 方案 — `internal/engine/session_starter.go` [NEW]

```go
// SessionStarter 策略接口
type SessionStarter interface {
    StartSession(ctx context.Context, cfg StartSessionConfig) (*Session, error)
}

type StartSessionConfig struct {
    SessionID         string
    ProviderSessionID string
    Config            SessionConfig
    Prompt            string
    Logger            *slog.Logger
    IsResuming        bool
}
```

**CLISessionStarter** — 原有 `startSession` 逻辑提取（Claude/Pi 使用）:

```go
type CLISessionStarter struct {
    cliPath     string
    provider    provider.Provider
    opts        EngineOptions
    markerStore persistence.SessionMarkerStore
}

func (s *CLISessionStarter) StartSession(ctx context.Context, cfg StartSessionConfig) (*Session, error) {
    // 原 pool.go:287-469 全部逻辑搬入此处
    // exec.Command, pipe setup, PGID, ReadStdout()...
}
```

**HTTPSessionStarter** — Server Provider 使用:

```go
type HTTPSessionStarter struct {
    transport provider.Transport
    provider  provider.Provider
}

func (s *HTTPSessionStarter) StartSession(ctx context.Context, cfg StartSessionConfig) (*Session, error) {
    ocSessionID, err := s.transport.CreateSession(ctx, cfg.SessionID)
    if err != nil { return nil, err }

    sess := &Session{
        ID:                cfg.SessionID,
        ProviderSessionID: ocSessionID,
        Config:            cfg.Config,
        Status:            SessionStatusReady, // 立即 Ready — 无冷启动
        CreatedAt:         time.Now(),
        LastActive:        time.Now(),
        statusChange:      make(chan SessionStatus, 10),
        logger:            cfg.Logger,
        // cmd, stdin, stdout, stderr 均为 nil
    }

    go s.consumeSSE(sess)
    return sess, nil
}

func (s *HTTPSessionStarter) consumeSSE(sess *Session) {
    for line := range s.transport.Events() {
        if cb := sess.GetCallback(); cb != nil {
            _ = cb("raw_line", line)
        }
    }
    sess.SetStatus(SessionStatusDead)
    if cb := sess.GetCallback(); cb != nil {
        _ = cb("runner_exit", nil)
    }
}
```

**Pool 适配**:

```go
// pool.go 修改
type SessionPool struct {
    // ...existing fields...
    starter SessionStarter // 注入 CLISessionStarter 或 HTTPSessionStarter
}

func (sm *SessionPool) startSession(ctx context.Context, sessionID string, cfg SessionConfig, prompt string) (*Session, error) {
    providerSessionID := ... // 现有 SHA1 逻辑不变
    return sm.starter.StartSession(ctx, StartSessionConfig{
        SessionID:         sessionID,
        ProviderSessionID: providerSessionID,
        Config:            cfg,
        Prompt:            prompt,
        IsResuming:        sm.markerStore.Exists(providerSessionID),
    })
}
```

---

## 12. Session ID 三层映射

```
HotPlex SessionID (Slack user+channel hash)
    ↓ pool.buildCLIArgs() — SHA1 确定性生成
Provider Session ID (deterministic UUID)
    ↓ HTTPTransport.CreateSession()
OpenCode Session ID (server 分配)
    ↓ HTTP API 调用时使用
POST /session/{OC_Session_ID}/message
```

```go
// OpenCodeServerProvider 中的 Session 映射管理
func (p *OpenCodeServerProvider) getOrCreateOCSession(
    ctx context.Context, providerSessionID string,
) (string, error) {
    if id, ok := p.sessions.Load(providerSessionID); ok {
        return id.(string), nil
    }
    ocID, err := p.transport.CreateSession(ctx, providerSessionID)
    if err != nil { return "", err }
    p.sessions.Store(providerSessionID, ocID)
    return ocID, nil
}
```

---

## 13. SSE 断连重连 · 详细展开

> **编辑注记**: 本节为 §3.2 `streamSSE` / `connectAndStream` 的完整实现版本，补充了 `attempt` 退避计数重置逻辑。

```go
func (t *HTTPTransport) streamSSE(ctx context.Context) {
    backoff := []time.Duration{1*time.Second, 2*time.Second, 5*time.Second, 10*time.Second}
    attempt := 0

    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        err := t.connectAndStream(ctx)
        if ctx.Err() != nil {
            return
        }

        delay := backoff[min(attempt, len(backoff)-1)]
        t.logger.Warn("SSE connection lost, reconnecting",
            "attempt", attempt, "delay", delay, "error", err)
        time.Sleep(delay)
        attempt++
    }
}

func (t *HTTPTransport) connectAndStream(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", t.baseURL+"/event", nil)
    req.Header.Set("Accept", "text/event-stream")
    if t.password != "" {
        req.SetBasicAuth("opencode", t.password)
    }

    resp, err := t.client.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()

    t.mu.Lock()
    t.connected = true
    t.mu.Unlock()

    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "data: ") {
            jsonLine := strings.TrimPrefix(line, "data: ")
            select {
            case t.events <- jsonLine:
                attempt = 0 // 收到数据，重置退避计数
            default:
                t.logger.Warn("SSE event buffer full, dropping event")
            }
        }
    }

    t.mu.Lock()
    t.connected = false
    t.mu.Unlock()

    return scanner.Err()
}
```

---

## 14. Constructor + Plugin 注册

### 14.1 Constructor

> HTTPTransport 定义见 §3.2。

```go
func NewOpenCodeServerProvider(cfg ProviderConfig, logger *slog.Logger) (*OpenCodeServerProvider, error) {
    if logger == nil {
        logger = slog.Default()
    }

    ocCfg := cfg.OpenCode
    if ocCfg == nil {
        ocCfg = &OpenCodeConfig{}
    }

    serverURL := ocCfg.ServerURL
    if serverURL == "" {
        port := 4096
        if ocCfg.Port > 0 { port = ocCfg.Port }
        serverURL = fmt.Sprintf("http://127.0.0.1:%d", port)
    }

    transport := &HTTPTransport{
        baseURL:  serverURL,
        client:   &http.Client{Timeout: 30 * time.Second},
        password: ocCfg.Password,
        events:   make(chan string, 256),
        logger:   logger.With("component", "oc_transport"),
    }

    meta := ProviderMeta{
        Type:        ProviderTypeOpenCodeServer,
        DisplayName: "OpenCode (Server Mode)",
        BinaryName:  "opencode",
        InstallHint: "brew install anomalyco/tap/opencode",
        Features: ProviderFeatures{
            SupportsResume:      true,
            SupportsStreamJSON:  true,
            SupportsSSE:         true,
            SupportsHTTPAPI:     true,
            SupportsSessionID:   true,
            MultiTurnReady:      true,
        },
    }

    return &OpenCodeServerProvider{
        ProviderBase: ProviderBase{
            meta:   meta,
            logger: logger.With("provider", "opencode-server"),
        },
        transport:     transport,
        opts:          cfg,
        promptBuilder: NewPromptBuilder(false),
    }, nil
}
```

### 14.2 Plugin 注册

```go
type openCodeServerPlugin struct{}

func (p *openCodeServerPlugin) Type() ProviderType {
    return ProviderTypeOpenCodeServer
}

func (p *openCodeServerPlugin) New(cfg ProviderConfig, logger *slog.Logger) (Provider, error) {
    return NewOpenCodeServerProvider(cfg, logger)
}

func (p *openCodeServerPlugin) Meta() ProviderMeta {
    return ProviderMeta{
        Type:        ProviderTypeOpenCodeServer,
        DisplayName: "OpenCode (Server Mode)",
        BinaryName:  "opencode",
        InstallHint: "brew install anomalyco/tap/opencode",
    }
}

func init() {
    RegisterPlugin(&openCodeServerPlugin{})
}
```

---

## 15. Permission 完整流程

### 15.1 解析

```go
case OCEventPermissionUpdated:
    var perm struct {
        ID        string         `json:"id"`
        Type      string         `json:"type"`
        SessionID string         `json:"sessionID"`
        Title     string         `json:"title"`
        Metadata  map[string]any `json:"metadata"`
    }
    json.Unmarshal(evt.Properties, &perm)
    return []*ProviderEvent{{
        Type:     EventTypePermissionRequest,
        RawType:  evt.Type,
        ToolName: perm.Title,
        ToolID:   perm.ID,
        Content:  fmt.Sprintf("[Permission] %s: %s", perm.Type, perm.Title),
        Metadata: &ProviderEventMeta{
            Status: "pending",
        },
    }}, nil
```

### 15.2 响应（新增到 Transport）

```go
func (t *HTTPTransport) RespondPermission(
    ctx context.Context, sessionID, permissionID, response string,
) error {
    url := fmt.Sprintf("%s/session/%s/permissions/%s",
        t.baseURL, sessionID, permissionID)
    body, _ := json.Marshal(map[string]any{"response": response})
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    req.Header.Set("Content-Type", "application/json")
    if t.password != "" {
        req.SetBasicAuth("opencode", t.password)
    }
    resp, err := t.client.Do(req)
    if err != nil { return err }
    defer resp.Body.Close()
    if resp.StatusCode >= 400 {
        return fmt.Errorf("permission respond failed: HTTP %d", resp.StatusCode)
    }
    return nil
}
```

---

## 16. 错误类型映射

| OpenCode Error Name            | HotPlex 处理           | ProviderEventType |
| ------------------------------ | ---------------------- | ----------------- |
| `ProviderAuthError`            | API key 无效 提示       | `error`           |
| `UnknownError`                | 透传 message           | `error`           |
| `MessageOutputLengthError`     | 输出过长               | `error`           |
| `MessageAbortedError`          | 用户主动取消           | `result`          |
| `APIError` (isRetryable=true)  | 等待自动重试           | `system`          |
| `APIError` (isRetryable=false) | 带 statusCode          | `error`           |

```go
func (p *OpenCodeServerProvider) mapOCError(ocErr *OCError) *ProviderEvent {
    if ocErr == nil {
        return &ProviderEvent{Type: EventTypeError, Error: "unknown"}
    }
    switch ocErr.Name {
    case "MessageAbortedError":
        return &ProviderEvent{Type: EventTypeResult, Content: "aborted by user"}
    case "APIError":
        if retryable, ok := ocErr.Data["isRetryable"].(bool); ok && retryable {
            return &ProviderEvent{Type: EventTypeSystem, Status: "retrying",
                Content: fmt.Sprintf("API error (retryable): %v", ocErr.Data["message"])}
        }
        return &ProviderEvent{Type: EventTypeError, IsError: true,
            Error: fmt.Sprintf("API error %v: %v", ocErr.Data["statusCode"], ocErr.Data["message"])}
    default:
        msg := fmt.Sprintf("%s: %v", ocErr.Name, ocErr.Data["message"])
        return &ProviderEvent{Type: EventTypeError, Error: msg, IsError: true}
    }
}
```

---

## 17. 远期规划

| Phase         | 内容                              | 时间线 |
| ------------- | --------------------------------- | ------ |
| ✅ **Phase 1** | Server Provider 方案              | 当前   |
| 🔮 Phase 2     | ACP Provider (`transport_acp.go`) | 远期   |
| 🔮 Phase 3     | HotPlex ACP Server                | 远期   |
