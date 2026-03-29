# HotPlex v1.0.0 错误处理规范

> 版本：v1.0  
> 日期：2026-03-29  
> 状态：完整错误处理规范

---

## 1. 概述

### 1.1 设计原则

- **错误码唯一**：每个错误有唯一错误码，便于追踪
- **错误链**：支持 `errors.Wrap` 保留错误链
- **可恢复性**：区分可恢复错误和不可恢复错误
- **分级处理**：Channel / Brain / Worker / Storage 分层处理

### 1.2 错误分类

| 层级 | 错误前缀 | 说明 |
|------|----------|------|
| Channel | `CH-*` | 渠道层错误 |
| Brain | `BR-*` | 智能层错误 |
| Worker | `WK-*` | 执行层错误 |
| Session | `SE-*` | 会话层错误 |
| Storage | `ST-*` | 存储层错误 |
| Provider | `PR-*` | 提供商错误 |
| System | `SY-*` | 系统错误 |

---

## 2. 错误码定义 (`pkg/errors/errors.go`)

### 2.1 完整错误码表

```go
package errors

// Channel 错误 (CH-*)
const (
    ErrChannelNotFound     Code = "CH-001"  // 渠道未找到
    ErrChannelDisconnected Code = "CH-002"  // 渠道断开
    ErrChannelSendFailed   Code = "CH-003"  // 发送失败
    ErrChannelAuthFailed   Code = "CH-004"  // 认证失败
    ErrChannelRateLimited  Code = "CH-005"  // 限流
    ErrChannelMsgTooLong   Code = "CH-006"  // 消息过长
    ErrChannelInvalidMsg   Code = "CH-007"  // 无效消息
    ErrChannelEventFailed  Code = "CH-008"  // 事件处理失败
    ErrChannelTimeout      Code = "CH-009"  // 超时
    ErrChannelConfigError  Code = "CH-010"  // 配置错误
)

// Brain 错误 (BR-*)
const (
    ErrBrainInitFailed     Code = "BR-001"  // 初始化失败
    ErrBrainProcessFailed  Code = "BR-002"  // 处理失败
    ErrBrainTimeout        Code = "BR-003"  // 超时
    ErrBrainIntentFailed   Code = "BR-004"  // 意图分类失败
    ErrBrainGuardBlocked   Code = "BR-005"  // 被 WAF 拦截
    ErrBrainConfigError    Code = "BR-006"  // 配置错误
    ErrBrainModelNotFound  Code = "BR-007"  // 模型未找到
    ErrBrainInvalidInput   Code = "BR-008"  // 无效输入
)

// Worker 错误 (WK-*)
const (
    ErrWorkerInitFailed    Code = "WK-001"  // 初始化失败
    ErrWorkerStartFailed   Code = "WK-002"  // 启动失败
    ErrWorkerProcessFailed Code = "WK-003"  // 执行失败
    ErrWorkerTimeout       Code = "WK-004"  // 超时
    ErrWorkerAbortFailed   Code = "WK-005"  // 终止失败
    ErrWorkerPoolExhausted Code = "WK-006"  // 进程池耗尽
    ErrWorkerNotFound      Code = "WK-007"  // Worker 未找到
    ErrWorkerStateError    Code = "WK-008"  // 状态错误
    ErrWorkerConfigError   Code = "WK-009"  // 配置错误
    ErrWorkerCLINotFound   Code = "WK-010"  // CLI 未找到
)

// Session 错误 (SE-*)
const (
    ErrSessionNotFound     Code = "SE-001"  // 会话未找到
    ErrSessionCreateFailed Code = "SE-002"  // 创建失败
    ErrSessionUpdateFailed Code = "SE-003"  // 更新失败
    ErrSessionExpired      Code = "SE-004"  // 已过期
    ErrSessionFull        Code = "SE-005"   // 会话数满
    ErrSessionNotActive   Code = "SE-006"   // 非活跃状态
    ErrSessionConfigError  Code = "SE-007"   // 配置错误
)

// Storage 错误 (ST-*)
const (
    ErrStorageInitFailed   Code = "ST-001"  // 初始化失败
    ErrStorageSaveFailed   Code = "ST-002"  // 保存失败
    ErrStorageQueryFailed  Code = "ST-003"  // 查询失败
    ErrStorageNotFound    Code = "ST-004"   // 未找到
    ErrStorageDeleteFailed Code = "ST-005"   // 删除失败
    ErrStorageFull        Code = "ST-006"   // 存储满
    ErrStorageTimeout     Code = "ST-007"   // 超时
    ErrStorageConfigError Code = "ST-008"   // 配置错误
)

// Provider 错误 (PR-*)
const (
    ErrProviderAuthFailed    Code = "PR-001"  // 认证失败
    ErrProviderRateLimited  Code = "PR-002"   // 限流
    ErrProviderQuotaExceeded Code = "PR-003"  // 配额超限
    ErrProviderModelNotFound Code = "PR-004"  // 模型未找到
    ErrProviderRequestFailed Code = "PR-005"  // 请求失败
    ErrProviderTimeout      Code = "PR-006"   // 超时
    ErrProviderInvalidReq   Code = "PR-007"   // 无效请求
    ErrProviderConfigError  Code = "PR-008"   // 配置错误
)

// System 错误 (SY-*)
const (
    ErrInvalidConfig    Code = "SY-001"  // 配置无效
    ErrInvalidInput     Code = "SY-002"  // 输入无效
    ErrInternalError    Code = "SY-003"  // 内部错误
    ErrNotImplemented  Code = "SY-004"  // 未实现
    ErrTimeout         Code = "SY-005"  // 超时
    ErrCancelled       Code = "SY-006"  // 取消
    ErrResourceFull    Code = "SY-007"  // 资源满
    ErrResourceNotFound Code = "SY-008"  // 资源未找到
    ErrPermissionDenied Code = "SY-009"  // 权限拒绝
)
```

---

## 3. 错误类型定义

### 3.1 Error 结构

```go
// Error HotPlex 错误
type Error struct {
    // 错误码
    Code Code
    
    // 错误信息
    Message string
    
    // 原始错误
    Cause error
    
    // 详情
    Details map[string]interface{}
    
    // 请求 ID (用于链路追踪)
    RequestID string
    
    // 时间戳
    Timestamp time.Time
    
    // 堆栈 (调试用)
    Stack string
    
    // 是否可重试
    Retryable bool
    
    // HTTP 状态码 (如果有)
    HTTPStatus int
}

// Error 实现 error 接口
func (e *Error) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap 实现 errors.Unwrap
func (e *Error) Unwrap() error {
    return e.Cause
}

// WithRequestID 添加请求 ID
func (e *Error) WithRequestID(id string) *Error {
    e.RequestID = id
    return e
}

// WithDetails 添加详情
func (e *Error) WithDetails(details map[string]interface{}) *Error {
    e.Details = details
    return e
}

// WithTimestamp 添加时间戳
func (e *Error) WithTimestamp(t time.Time) *Error {
    e.Timestamp = t
    return e
}

// IsRetryable 是否可重试
func (e *Error) IsRetryable() bool {
    return e.Retryable
}

// HTTPStatus 返回 HTTP 状态码
func (e *Error) HTTPStatus() int {
    if e.HTTPStatus > 0 {
        return e.HTTPStatus
    }
    return DefaultHTTPStatus(e.Code)
}

// DefaultHTTPStatus 默认 HTTP 状态码
func DefaultHTTPStatus(code Code) int {
    switch {
    case strings.HasPrefix(string(code), "CH-"):
        return http.StatusBadGateway
    case strings.HasPrefix(string(code), "BR-"):
        return http.StatusBadGateway
    case strings.HasPrefix(string(code), "WK-"):
        return http.StatusServiceUnavailable
    case strings.HasPrefix(string(code), "SE-"):
        return http.StatusNotFound
    case strings.HasPrefix(string(code), "ST-"):
        return http.StatusServiceUnavailable
    case strings.HasPrefix(string(code), "PR-"):
        return http.StatusBadGateway
    default:
        return http.StatusInternalServerError
    }
}
```

### 3.2 错误构造

```go
// New 创建新错误
func New(code Code, msg string) *Error {
    return &Error{
        Code:     code,
        Message:  msg,
        Stack:    string(debug.Stack()),
        Timestamp: time.Now(),
        Retryable: IsRetryableCode(code),
    }
}

// Wrap 包装错误
func Wrap(code Code, err error, msg string) *Error {
    if err == nil {
        return nil
    }
    return &Error{
        Code:     code,
        Message:  msg,
        Cause:    err,
        Stack:    string(debug.Stack()),
        Timestamp: time.Now(),
        Retryable: IsRetryableCode(code),
    }
}

// Wrapf 包装错误 (格式化)
func Wrapf(code Code, err error, format string, args ...interface{}) *Error {
    return Wrap(code, err, fmt.Sprintf(format, args...))
}

// IsRetryableCode 判断错误码是否可重试
func IsRetryableCode(code Code) bool {
    retryableCodes := map[Code]bool{
        ErrChannelRateLimited: true,
        ErrBrainTimeout:       true,
        ErrWorkerTimeout:      true,
        ErrWorkerPoolExhausted: true,
        ErrStorageTimeout:     true,
        ErrProviderRateLimited: true,
        ErrProviderTimeout:    true,
        ErrTimeout:           true,
    }
    return retryableCodes[code]
}
```

---

## 4. 错误处理模式

### 4.1 分层错误处理

```go
// Channel 层错误处理
func (c *FeishuChannel) HandleMessage(ctx context.Context, event *FeishuEvent) error {
    // 解析消息
    msg, err := c.parseMessage(event)
    if err != nil {
        return errors.Wrap(ErrChannelInvalidMsg, err, "failed to parse message")
    }
    
    // 调用 Brain
    output, err := c.brain.Process(ctx, &brain.BrainInput{Message: msg})
    if err != nil {
        // Brain 错误，转为用户友好消息
        return c.sendUserError(msg, errors.Wrap(ErrBrainProcessFailed, err, "brain processing failed"))
    }
    
    // Brain 拦截
    if output.Blocked {
        return c.sendBlockedMessage(msg, output.BlockReason)
    }
    
    // 继续处理...
    return nil
}

// 发送错误给用户
func (c *FeishuChannel) sendUserError(msg *channel.Message, err *errors.Error) error {
    // 根据错误码决定消息内容
    userMsg := c.formatUserError(err)
    return c.Send(ctx, &Response{
        ChannelID: msg.ChannelID,
        UserID:   msg.UserID,
        Content:  userMsg,
    })
}

// 发送拦截消息
func (c *FeishuChannel) sendBlockedMessage(msg *channel.Message, reason string) error {
    return c.Send(ctx, &Response{
        ChannelID: msg.ChannelID,
        UserID:   msg.UserID,
        Content:  fmt.Sprintf("⛔ 消息已被拦截: %s", reason),
    })
}
```

### 4.2 Worker 错误处理

```go
// Worker 执行错误处理
func (w *ClaudeCodeWorker) Run(ctx context.Context, task *Task) (*Result, error) {
    // 获取进程
    proc, err := w.pool.Acquire(ctx)
    if err != nil {
        if errors.Is(err, context.DeadlineExceeded) {
            return nil, errors.New(ErrWorkerPoolExhausted, "process pool exhausted")
        }
        return nil, errors.Wrap(ErrWorkerProcessFailed, err, "failed to acquire process")
    }
    defer w.pool.Release(proc)
    
    // 执行
    result, err := w.runner.Run(ctx, proc, task)
    if err != nil {
        // 判断错误类型
        if errors.Is(err, context.DeadlineExceeded) {
            proc.Kill()
            return &Result{
                Status: ResultStatusTimeout,
                Error: &TaskError{
                    Code:    string(ErrWorkerTimeout),
                    Message: fmt.Sprintf("task exceeded timeout of %v", task.Timeout),
                },
            }, nil
        }
        
        // 可重试错误
        if errors.IsRetryable(err) && task.Retry.Count < task.Retry.Max {
            return w.retry(ctx, task)
        }
        
        return &Result{
            Status: ResultStatusFailed,
            Error: &TaskError{
                Code:    string(ErrWorkerProcessFailed),
                Message: err.Error(),
            },
        }, err
    }
    
    return result, nil
}

// 重试
func (w *ClaudeCodeWorker) retry(ctx context.Context, task *Task) (*Result, error) {
    task.Retry.Count++
    
    delay := task.Retry.InitialDelay * time.Duration(math.Pow(2, float64(task.Retry.Count-1)))
    if delay > task.Retry.MaxDelay {
        delay = task.Retry.MaxDelay
    }
    
    select {
    case <-time.After(delay):
        return w.Run(ctx, task)
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

### 4.3 Storage 错误处理

```go
// Storage 降级处理
type FallbackStorage struct {
    primary   Storage
    secondary Storage
    logger   *slog.Logger
}

func (s *FallbackStorage) Save(ctx context.Context, records ...*Record) error {
    // 尝试 Primary
    if err := s.primary.Save(ctx, records...); err == nil {
        return nil
    } else if !errors.IsRetryable(err) {
        // 非重试错误，直接降级
        s.logger.Warn("primary storage failed, falling back", "error", err)
        return s.secondary.Save(ctx, records...)
    }
    
    // 尝试 Secondary
    if err := s.secondary.Save(ctx, records...); err != nil {
        return errors.Wrap(ErrStorageSaveFailed, err, "both primary and secondary storage failed")
    }
    
    s.logger.Warn("primary storage failed, secondary succeeded")
    return nil
}

// 检查是否可重试
func (s *FallbackStorage) isRetryable(err error) bool {
    var e *Error
    if errors.As(err, &e) {
        return e.Retryable
    }
    return false
}
```

---

## 5. Recovery 机制

### 5.1 Goroutine Panic 恢复

```go
// RecoverHandler 恢复处理器
type RecoverHandler struct {
    logger *slog.Logger
    alerts Alerts
}

func (h *RecoverHandler) Recover(ctx context.Context, name string, r interface{}) error {
    // 记录日志
    h.logger.Error("panic recovered",
        "goroutine", name,
        "panic", r,
        "stack", string(debug.Stack()),
    )
    
    // 发送告警
    h.alerts.Notify("panic", map[string]interface{}{
        "goroutine": name,
        "panic":     fmt.Sprintf("%v", r),
    })
    
    return errors.New(ErrInternalError, fmt.Sprintf("panic in %s: %v", name, r))
}

// SafeGo 安全启动 goroutine
func SafeGo(ctx context.Context, name string, fn func(), recoverHandler *RecoverHandler) {
    go func() {
        defer func() {
            if r := recover(); r != nil {
                recoverHandler.Recover(ctx, name, r)
            }
        }()
        fn()
    }()
}
```

### 5.2 Supervisor 恢复

```go
// Supervisor 进程恢复
func (s *Supervisor) handleFailure(kind string, err error) {
    handle := s.workers[kind]
    
    switch s.policy.Mode {
    case RestartModeNever:
        return
        
    case RestartModeOnFailure:
        s.restartWithBackoff(handle)
        
    case RestartModeBackoff:
        if handle.restarts < s.policy.MaxRestarts {
            s.restartWithBackoff(handle)
        } else {
            s.logger.Error("worker exceeded max restarts",
                "kind", kind,
                "restarts", handle.restarts,
            )
            s.setWorkerDown(handle)
        }
    }
}

func (s *Supervisor) restartWithBackoff(handle *WorkerHandle) {
    delay := s.policy.InitialInterval * time.Duration(math.Pow(2, float64(handle.restarts)))
    if delay > s.policy.MaxRestartInterval {
        delay = s.policy.MaxRestartInterval
    }
    
    handle.restarts++
    handle.state = HandleStateRestarting
    
    time.Sleep(delay)
    
    if err := handle.Worker.Start(context.Background()); err != nil {
        s.logger.Error("worker restart failed",
            "kind", handle.Kind,
            "error", err,
        )
        s.setWorkerDown(handle)
        return
    }
    
    handle.state = HandleStateRunning
    s.events <- SupervisorEvent{
        Type:       EventWorkerRestart,
        WorkerKind: handle.Kind,
        Time:       time.Now(),
    }
}
```

---

## 6. 错误响应格式

### 6.1 API 错误响应

```json
{
  "error": {
    "code": "BR-005",
    "message": "消息包含敏感内容已被拦截",
    "details": {
      "violations": [
        {
          "rule": "profanity",
          "severity": "high",
          "matched": "***"
        }
      ]
    },
    "request_id": "req-123456",
    "timestamp": "2026-03-29T10:30:00Z"
  }
}
```

### 6.2 WebSocket 错误消息

```json
{
  "type": "error",
  "code": "WK-004",
  "message": "任务执行超时",
  "task_id": "task-789",
  "retryable": true
}
```

---

## 7. 错误日志规范

### 7.1 日志级别

| 级别 | 场景 |
|------|------|
| ERROR | 需要人工介入的错误 |
| WARN | 可恢复错误，需要关注 |
| INFO | 正常错误处理流程 |
| DEBUG | 详细错误信息 |

### 7.2 日志格式

```go
// 错误日志
slog.Error("worker execution failed",
    "code", "WK-003",
    "error", err.Error(),
    "task_id", task.ID,
    "session_id", task.SessionID,
    "worker", "claude-code",
    "duration", time.Since(start),
)

// 带错误链的日志
slog.Error("brain processing failed",
    "code", "BR-002",
    "error", err.Error(),          // 自动包含错误链
    "message", "brain processing failed",
    "session_id", sessionID,
    "intent", output.Intent.Kind,
)
```

---

## 8. 告警机制

### 8.1 告警规则

```go
// AlertRule 告警规则
type AlertRule struct {
    Name        string
    Condition   AlertCondition
    Cooldown    time.Duration
    Severity    string
    Message     string
}

// AlertCondition 告警条件
type AlertCondition struct {
    ErrorCode     string      // 错误码
    Count         int         // 触发次数
    Window        time.Duration // 时间窗口
    Consecutive   bool        // 是否连续
}

// 预定义告警规则
var DefaultAlertRules = []AlertRule{
    {
        Name: "high_error_rate",
        Condition: AlertCondition{
            ErrorCode: "WK-*",
            Count:     10,
            Window:    5 * time.Minute,
        },
        Cooldown: 10 * time.Minute,
        Severity: "critical",
        Message:  "Worker 错误率过高",
    },
    {
        Name: "storage_failure",
        Condition: AlertCondition{
            ErrorCode: "ST-*",
            Count:     5,
            Window:    1 * time.Minute,
        },
        Cooldown: 5 * time.Minute,
        Severity: "critical",
        Message:  "存储服务故障",
    },
    {
        Name: "channel_disconnect",
        Condition: AlertCondition{
            ErrorCode: "CH-002",
            Count:     3,
            Window:    10 * time.Minute,
        },
        Cooldown: 30 * time.Minute,
        Severity: "warning",
        Message:  "Channel 频繁断开",
    },
}
```

### 8.2 告警通知

```go
// AlertNotifier 告警通知
type AlertNotifier interface {
    Notify(level string, title string, msg string, details map[string]interface{})
}

// WebhookNotifier Webhook 通知
type WebhookNotifier struct {
    webhookURL string
    client     *http.Client
}

// EmailNotifier 邮件通知
type EmailNotifier struct {
    smtpHost string
    from     string
    to       []string
}
```

---

*文档版本：v1.0 | 最后更新：2026-03-29*
