# Worker/Adapter Abstraction Patterns for CLI-based AI Agents

**Research Date**: 2026-03-30
**Focus**: Go patterns for process management, transport abstraction, and lifecycle control
**Application**: HotPlex Worker Adapter Layer

---

## Executive Summary

This document synthesizes 2025-2026 best practices for managing CLI-based AI agent processes in Go, drawing from Kubernetes pod lifecycle patterns, Docker container management, OCI runtime specifications, and Go concurrency primitives. The patterns are adapted for HotPlex's architecture where long-lived CLI processes (Claude Code, OpenCode) are wrapped as persistent workers.

**Key Insight**: Treat CLI processes like Kubernetes pods with explicit lifecycle phases, health probes, and graceful termination. Use context.Context as the universal cancellation signal carrier.

---

## 1. Worker Lifecycle Management

### 1.1 Lifecycle Phases (Kubernetes-Inspired)

Adopt Kubernetes pod phase model for worker state machine:

```go
type WorkerPhase string

const (
    PhasePending   WorkerPhase = "Pending"   // Starting up, not ready
    PhaseRunning   WorkerPhase = "Running"   // Active and serving requests
    PhaseSucceeded WorkerPhase = "Succeeded" // Exited cleanly (exit 0)
    PhaseFailed    WorkerPhase = "Failed"    // Exited with error (non-zero)
    PhaseUnknown   WorkerPhase = "Unknown"   // State cannot be determined
)

type WorkerState struct {
    Phase         WorkerPhase
    StartedAt     time.Time
    FinishedAt    *time.Time
    ExitCode      *int
    Restarts      int
    LastProbeTime time.Time
    Ready         bool // Readiness gate
}
```

### 1.2 Graceful Shutdown Sequence

**Pattern**: SIGTERM → Grace Period → SIGKILL (Kubernetes default: 30s)

```go
type Worker struct {
    cmd       *exec.Cmd
    cancel    context.CancelFunc
    done      chan struct{}
    state     WorkerState
    mu        sync.RWMutex
}

func (w *Worker) Shutdown(gracePeriod time.Duration) error {
    w.mu.Lock()
    defer w.mu.Unlock()

    // 1. Cancel context (propagates to all goroutines)
    w.cancel()

    // 2. Send SIGTERM (graceful stop)
    if w.cmd.Process != nil {
        if err := w.cmd.Process.Signal(syscall.SIGTERM); err != nil {
            return fmt.Errorf("send SIGTERM: %w", err)
        }
    }

    // 3. Wait for graceful exit or force kill
    select {
    case <-w.done:
        return nil // Exited gracefully
    case <-time.After(gracePeriod):
        if w.cmd.Process != nil {
            return w.cmd.Process.Kill() // SIGKILL
        }
        return nil
    }
}
```

**HotPlex Application**:
- Default grace period: 30s (configurable via `graceful_shutdown_timeout`)
- Use SIGTERM for Claude Code/OpenCode (both handle SIGTERM gracefully)
- Fall back to SIGKILL only if grace period expires

### 1.3 Health Checks (Probes)

**Three Probe Types** (from Kubernetes):

1. **Startup Probe**: Wait for CLI initialization (e.g., Claude Code startup time)
2. **Liveness Probe**: Detect deadlocks/frozen processes
3. **Readiness Probe**: Check if worker can accept requests

```go
type ProbeConfig struct {
    InitialDelay   time.Duration
    Period         time.Duration
    Timeout        time.Duration
    FailureThreshold int
    SuccessThreshold int
}

type Prober interface {
    Probe(ctx context.Context) error
}

// Example: HTTP health probe
type HTTPProbe struct {
    url    string
    client *http.Client
}

func (p *HTTPProbe) Probe(ctx context.Context) error {
    req, err := http.NewRequestWithContext(ctx, "GET", p.url, nil)
    if err != nil {
        return err
    }
    resp, err := p.client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("health check failed: status %d", resp.StatusCode)
    }
    return nil
}

// Example: Process liveness probe (check if PID exists)
type ProcessProbe struct {
    pid int
}

func (p *ProcessProbe) Probe(ctx context.Context) error {
    process, err := os.FindProcess(p.pid)
    if err != nil {
        return err
    }
    // On Unix, FindProcess always succeeds; need to signal to check
    err = process.Signal(syscall.Signal(0))
    if err != nil {
        return fmt.Errorf("process not running: %w", err)
    }
    return nil
}
```

**HotPlex Application**:
- **Startup Probe**: Wait for first successful JSON-RPC response (initial delay: 5s, period: 1s, timeout: 10s)
- **Liveness Probe**: Check process existence + periodic "ping" to CLI (period: 30s)
- **Readiness Probe**: Check if stdin/stdout pipes are healthy (period: 10s)

### 1.4 Restart Policies

```go
type RestartPolicy string

const (
    RestartAlways    RestartPolicy = "Always"    // Always restart on exit
    RestartOnFailure RestartPolicy = "OnFailure" // Restart only on non-zero exit
    RestartNever     RestartPolicy = "Never"     // Never restart
)

type RestartManager struct {
    policy     RestartPolicy
    maxRetries int
    backoff    BackoffStrategy
}

// Exponential backoff: 10s, 20s, 40s, ... max 300s (Kubernetes pattern)
type ExponentialBackoff struct {
    initial    time.Duration
    max        time.Duration
    multiplier float64
    attempt    int
}

func (b *ExponentialBackoff) Next() time.Duration {
    delay := time.Duration(float64(b.initial) * math.Pow(b.multiplier, float64(b.attempt)))
    if delay > b.max {
        delay = b.max
    }
    b.attempt++
    return delay
}

func (m *RestartManager) ShouldRestart(exitCode int) bool {
    switch m.policy {
    case RestartAlways:
        return true
    case RestartOnFailure:
        return exitCode != 0
    case RestartNever:
        return false
    default:
        return false
    }
}
```

---

## 2. Transport Abstraction

### 2.1 Unified Transport Interface

Abstract stdio, HTTP/SSE, and WebSocket behind common interface:

```go
type Transport interface {
    // Send sends a message to the CLI process
    Send(ctx context.Context, msg []byte) error

    // Receive receives a message from the CLI process
    Receive(ctx context.Context) ([]byte, error)

    // Close closes the transport
    Close() error

    // SetErrorHandler registers error handler for async errors
    SetErrorHandler(handler func(error))
}

// StdioTransport wraps os/exec pipes
type StdioTransport struct {
    stdin  io.WriteCloser
    stdout io.Reader
    stderr io.Reader
    mu     sync.Mutex
}

func (t *StdioTransport) Send(ctx context.Context, msg []byte) error {
    t.mu.Lock()
    defer t.mu.Unlock()

    // Write with context deadline awareness
    done := make(chan error, 1)
    go func() {
        _, err := t.stdin.Write(msg)
        done <- err
    }()

    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (t *StdioTransport) Receive(ctx context.Context) ([]byte, error) {
    buf := make([]byte, 4096)
    n, err := t.stdout.Read(buf)
    if err != nil {
        return nil, err
    }
    return buf[:n], nil
}
```

### 2.2 Line-Based Framing (for JSON-RPC)

```go
type LineFramer struct {
    scanner *bufio.Scanner
    writer  *bufio.Writer
}

func NewLineFramer(rwc io.ReadWriteCloser) *LineFramer {
    return &LineFramer{
        scanner: bufio.NewScanner(rwc),
        writer:  bufio.NewWriter(rwc),
    }
}

func (f *LineFramer) ReadFrame() ([]byte, error) {
    if !f.scanner.Scan() {
        if err := f.scanner.Err(); err != nil {
            return nil, err
        }
        return nil, io.EOF
    }
    return f.scanner.Bytes(), nil
}

func (f *LineFramer) WriteFrame(data []byte) error {
    if _, err := f.writer.Write(data); err != nil {
        return err
    }
    if _, err := f.writer.Write([]byte("\n")); err != nil {
        return err
    }
    return f.writer.Flush()
}
```

**HotPlex Application**:
- Claude Code and OpenCode both use JSON Lines over stdio
- Use `LineFramer` for consistent framing across transports
- Abstract WebSocket upgrades behind `Transport` interface (future-proofing)

---

## 3. Protocol Parsing

### 3.1 JSON Lines Streaming Parser

```go
type JSONLinesParser struct {
    decoder *json.Decoder
}

func NewJSONLinesParser(r io.Reader) *JSONLinesParser {
    return &JSONLinesParser{
        decoder: json.NewDecoder(r),
    }
}

func (p *JSONLinesParser) ParseNext(ctx context.Context) (map[string]any, error) {
    type contextReader struct {
        r io.Reader
        ctx context.Context
    }

    // Wrap read with context
    done := make(chan error, 1)
    var result map[string]any

    go func() {
        err := p.decoder.Decode(&result)
        done <- err
    }()

    select {
    case err := <-done:
        return result, err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

### 3.2 Stream Buffering with Backpressure

```go
type BufferConfig struct {
    Size         int           // Buffer size (messages)
    FlushTimeout time.Duration // Timeout before forcing flush
}

type StreamBuffer struct {
    config    BufferConfig
    buffer    chan []byte
    transport Transport
    mu        sync.Mutex
}

func (b *StreamBuffer) Write(data []byte) error {
    select {
    case b.buffer <- data:
        return nil
    default:
        return fmt.Errorf("buffer full (backpressure)")
    }
}

func (b *StreamBuffer) FlushLoop(ctx context.Context) {
    ticker := time.NewTicker(b.config.FlushTimeout)
    defer ticker.Stop()

    batch := make([][]byte, 0, b.config.Size)

    for {
        select {
        case <-ctx.Done():
            return
        case msg := <-b.buffer:
            batch = append(batch, msg)
            if len(batch) >= b.config.Size {
                b.flushBatch(batch)
                batch = batch[:0]
            }
        case <-ticker.C:
            if len(batch) > 0 {
                b.flushBatch(batch)
                batch = batch[:0]
            }
        }
    }
}
```

**HotPlex Application**:
- Buffer size: 100 messages
- Flush timeout: 100ms (balances latency vs throughput)
- Backpressure: Return error to caller if buffer full (client should retry)

---

## 4. Capability-Based Design

### 4.1 Interface Segregation

Break monolithic adapter interface into fine-grained capabilities:

```go
// Core capability (all adapters must implement)
type TransportCapability interface {
    Send(ctx context.Context, msg []byte) error
    Receive(ctx context.Context) ([]byte, error)
}

// Optional capabilities
type HealthCheckCapability interface {
    HealthCheck(ctx context.Context) error
}

type StreamingCapability interface {
    Stream(ctx context.Context, handler func([]byte) error) error
}

type SessionCapability interface {
    Resume(sessionID string) error
    Suspend() error
}

// Runtime capability detection
func GetCapabilities(adapter any) []string {
    var caps []string

    if _, ok := adapter.(HealthCheckCapability); ok {
        caps = append(caps, "health_check")
    }
    if _, ok := adapter.(StreamingCapability); ok {
        caps = append(caps, "streaming")
    }
    if _, ok := adapter.(SessionCapability); ok {
        caps = append(caps, "session")
    }

    return caps
}
```

### 4.2 Feature Flags with Compile-Time Verification

```go
type Adapter struct {
    supportsStreaming bool
}

// Compile-time capability verification
var _ TransportCapability = (*Adapter)(nil)
var _ HealthCheckCapability = (*Adapter)(nil)

// Optional interface implementation
func (a *Adapter) HealthCheck(ctx context.Context) error {
    // Implementation
    return nil
}
```

**HotPlex Application**:
- Claude Code: `TransportCapability` + `StreamingCapability` + `SessionCapability`
- OpenCode: `TransportCapability` + `HealthCheckCapability`
- Use capability detection in `WorkerPool` to route requests appropriately

---

## 5. Process Isolation

### 5.1 Process Groups (Unix PGID)

```go
func startWithPGID(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
    cmd := exec.CommandContext(ctx, name, args...)

    // Create new process group (isolate from parent)
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true, // Set process group ID = PID
    }

    // Start process
    if err := cmd.Start(); err != nil {
        return nil, err
    }

    return cmd, nil
}

func killProcessGroup(pgid int) error {
    // Kill entire process group (negative PID = PGID)
    return syscall.Kill(-pgid, syscall.SIGKILL)
}

func terminateGracefully(cmd *exec.Cmd, gracePeriod time.Duration) error {
    if cmd.Process == nil {
        return nil
    }

    // 1. Send SIGTERM to process group
    pgid, err := syscall.Getpgid(cmd.Process.Pid)
    if err != nil {
        return err
    }
    if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
        return err
    }

    // 2. Wait for graceful exit or force kill
    done := make(chan error, 1)
    go func() {
        done <- cmd.Wait()
    }()

    select {
    case err := <-done:
        return err // Exited gracefully
    case <-time.After(gracePeriod):
        return killProcessGroup(pgid)
    }
}
```

### 5.2 Resource Limits (Unix setrlimit)

```go
type ResourceLimits struct {
    MaxCPU    time.Duration // CPU time limit
    MaxMemory int64         // Memory limit (bytes)
    MaxFiles  int64         // Open file descriptors
}

func setResourceLimits(cmd *exec.Cmd, limits ResourceLimits) {
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,
        Rlimit: &syscall.Rlimit{
            Cur: limits.MaxFiles,
            Max: limits.MaxFiles,
        },
    }
}
```

### 5.3 Timeout Enforcement with Context

```go
func executeWithTimeout(ctx context.Context, timeout time.Duration, cmd *exec.Cmd) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    if err := cmd.Start(); err != nil {
        return err
    }

    done := make(chan error, 1)
    go func() {
        done <- cmd.Wait()
    }()

    select {
    case err := <-done:
        return err
    case <-ctx.Done():
        // Context cancelled or timeout exceeded
        if cmd.Process != nil {
            cmd.Process.Kill()
        }
        return ctx.Err()
    }
}
```

**HotPlex Application**:
- **PGID Isolation**: Already implemented in `internal/engine/session.go`
- **Execution Timeout**: Use `context.WithTimeout` (configurable via `execution_timeout`)
- **Memory Limits**: (Future) Use cgroups on Linux for memory limiting

---

## 6. Error Classification

### 6.1 Exit Code Taxonomy

```go
type ExitCode int

const (
    ExitSuccess         ExitCode = 0
    ExitGeneralError    ExitCode = 1
    ExitMisuse          ExitCode = 2   // Command syntax error
    ExitUnimplemented   ExitCode = 127 // Command not found
)

type ProcessError struct {
    Code    ExitCode
    Message string
    Cause   error
}

func (e *ProcessError) Error() string {
    return fmt.Sprintf("process exited with code %d: %s", e.Code, e.Message)
}

func (e *ProcessError) Unwrap() error {
    return e.Cause
}

// Error classification
func ClassifyError(err error) errorType {
    if err == nil {
        return ErrorTypeNone
    }

    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        code := ExitCode(exitErr.ExitCode())

        switch {
        case code == 0:
            return ErrorTypeNone
        case code == 137:
            return ErrorTypeOOM // SIGKILL (128 + 9)
        case code == 139:
            return ErrorTypeSegfault // SIGSEGV (128 + 11)
        case code == 143:
            return ErrorTypeTerminated // SIGTERM (128 + 15)
        case code >= 128:
            return ErrorTypeSignal
        default:
            return ErrorTypeApplication
        }
    }

    if errors.Is(err, context.DeadlineExceeded) {
        return ErrorTypeTimeout
    }

    return ErrorTypeUnknown
}

type errorType int

const (
    ErrorTypeNone errorType = iota
    ErrorTypeApplication
    ErrorTypeTimeout
    ErrorTypeOOM
    ErrorTypeSegfault
    ErrorTypeTerminated
    ErrorTypeSignal
    ErrorTypeUnknown
)
```

### 6.2 Error Recovery Strategy

```go
type RecoveryStrategy int

const (
    RecoveryNone     RecoveryStrategy = iota // No recovery
    RecoveryRestart                          // Restart worker
    RecoveryBackoff                          // Restart with backoff
    RecoveryFailover                         // Switch to backup worker
)

func GetRecoveryStrategy(errorType errorType) RecoveryStrategy {
    switch errorType {
    case ErrorTypeOOM, ErrorTypeSegfault:
        return RecoveryBackoff // Memory issues → backoff
    case ErrorTypeTimeout:
        return RecoveryRestart // Timeout → immediate restart
    case ErrorTypeTerminated:
        return RecoveryNone    // Graceful termination → no restart
    default:
        return RecoveryRestart
    }
}
```

**HotPlex Application**:
- Classify errors in `internal/engine/session.go` after process exit
- Use recovery strategy to decide if worker should restart
- Log error type + exit code for observability

---

## 7. Session Resumption

### 7.1 Checkpoint Pattern (Tmux-Inspired)

```go
type SessionState struct {
    ID        string
    CreatedAt time.Time
    WorkDir   string
    Env       []string
    Command   string
    Args      []string

    // Conversation state
    ConversationHistory []Message
    LastMessageID       string

    // Process state
    PID     int
    PGID    int
    Stdin   []byte // Unprocessed stdin buffer
    Stdout  []byte // Unprocessed stdout buffer
}

type CheckpointManager interface {
    Save(state *SessionState) error
    Load(sessionID string) (*SessionState, error)
    Delete(sessionID string) error
}

// File-based checkpoint (simple)
type FileCheckpoint struct {
    dir string
}

func (c *FileCheckpoint) Save(state *SessionState) error {
    data, err := json.Marshal(state)
    if err != nil {
        return err
    }

    path := filepath.Join(c.dir, state.ID+".json")
    return os.WriteFile(path, data, 0644)
}

func (c *FileCheckpoint) Load(sessionID string) (*SessionState, error) {
    path := filepath.Join(c.dir, sessionID+".json")
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var state SessionState
    if err := json.Unmarshal(data, &state); err != nil {
        return nil, err
    }

    return &state, nil
}
```

### 7.2 Resumption Flow

```go
func (w *Worker) Resume(checkpoint *SessionState) error {
    // 1. Restore environment
    cmd := exec.Command(checkpoint.Command, checkpoint.Args...)
    cmd.Dir = checkpoint.WorkDir
    cmd.Env = checkpoint.Env

    // 2. Create process group
    cmd.SysProcAttr = &syscall.SysProcAttr{
        Setpgid: true,
    }

    // 3. Start process
    if err := cmd.Start(); err != nil {
        return err
    }

    // 4. Replay conversation history (if CLI supports it)
    if w.supportsReplay {
        for _, msg := range checkpoint.ConversationHistory {
            if err := w.Send(context.Background(), msg.Data); err != nil {
                return err
            }
        }
    }

    // 5. Flush buffered I/O
    if len(checkpoint.Stdin) > 0 {
        w.stdin.Write(checkpoint.Stdin)
    }

    return nil
}
```

**HotPlex Application**:
- **Checkpoint**: Save session state on every message (conversation history + last message ID)
- **Resume**: On worker restart, replay last N messages (configurable)
- **Hot Reload**: Use checkpoint for zero-downtime restarts (future feature)

---

## 8. Integrated Worker Pool Pattern

### 8.1 Pool with Lifecycle Management

```go
type WorkerPool struct {
    workers   map[string]*Worker
    jobs      chan Job
    results   chan Result
    ctx       context.Context
    cancel    context.CancelFunc
    wg        sync.WaitGroup
    mu        sync.RWMutex

    // Lifecycle hooks
    probes    map[string]Prober
    restart   *RestartManager
    checkpoint CheckpointManager
}

func NewWorkerPool(ctx context.Context, size int) *WorkerPool {
    ctx, cancel := context.WithCancel(ctx)
    return &WorkerPool{
        workers:   make(map[string]*Worker),
        jobs:      make(chan Job, size),
        results:   make(chan Result, size),
        ctx:       ctx,
        cancel:    cancel,
        probes:    make(map[string]Prober),
    }
}

func (p *WorkerPool) Start() {
    for i := 0; i < cap(p.jobs); i++ {
        p.wg.Add(1)
        go p.workerLoop(i)
    }
}

func (p *WorkerPool) workerLoop(id int) {
    defer p.wg.Done()

    for {
        select {
        case <-p.ctx.Done():
            return
        case job := <-p.jobs:
            result := p.executeJob(job)
            p.results <- result
        }
    }
}

func (p *WorkerPool) executeJob(job Job) Result {
    worker, err := p.getOrCreateWorker(job.SessionID)
    if err != nil {
        return Result{Error: err}
    }

    // Execute with timeout
    ctx, cancel := context.WithTimeout(p.ctx, job.Timeout)
    defer cancel()

    result, err := worker.Execute(ctx, job.Payload)
    if err != nil {
        // Classify error and apply recovery strategy
        errorType := ClassifyError(err)
        strategy := GetRecoveryStrategy(errorType)

        if strategy != RecoveryNone {
            go p.restartWorker(job.SessionID, strategy)
        }

        return Result{Error: err}
    }

    return Result{Data: result}
}

func (p *WorkerPool) Shutdown() {
    p.cancel() // Cancel all workers

    done := make(chan struct{})
    go func() {
        p.wg.Wait()
        close(done)
    }()

    select {
    case <-done:
        return
    case <-time.After(30 * time.Second):
        // Force kill remaining workers
        p.mu.RLock()
        for _, w := range p.workers {
            if w.cmd.Process != nil {
                w.cmd.Process.Kill()
            }
        }
        p.mu.RUnlock()
    }
}
```

---

## 9. HotPlex-Specific Recommendations

### 9.1 Current Architecture Gaps

| Area | Current State | Recommended Enhancement |
|------|---------------|------------------------|
| **Lifecycle** | Basic start/stop | Add explicit phases (Pending/Running/Failed) + probes |
| **Health Checks** | None | Implement startup/liveness/readiness probes |
| **Error Handling** | Basic exit code logging | Classify errors (OOM/segfault/timeout) + recovery strategies |
| **Session Resumption** | None | Implement checkpoint manager for conversation history |
| **Restart Policies** | None | Add RestartOnFailure with exponential backoff |

### 9.2 Implementation Roadmap

**Phase 1: Lifecycle & Health Checks (Priority: High)**
1. Add `WorkerState` with phases to `internal/engine/session.go`
2. Implement `ProcessProbe` for liveness checks
3. Add graceful shutdown with 30s grace period

**Phase 2: Error Classification (Priority: Medium)**
1. Create `internal/engine/errors.go` with error taxonomy
2. Classify exit codes in `session.go:Wait()` method
3. Add recovery strategy selection logic

**Phase 3: Session Resumption (Priority: Medium)**
1. Create `internal/persistence/checkpoint.go`
2. Implement file-based checkpoint manager
3. Add replay logic for conversation history

**Phase 4: Advanced Features (Priority: Low)**
1. Resource limits via cgroups (Linux only)
2. Hot reload with zero-downtime restarts
3. Distributed worker pool (multi-node)

### 9.3 Configuration Schema

```yaml
workers:
  lifecycle:
    graceful_shutdown_timeout: 30s

  health_checks:
    startup_probe:
      initial_delay: 5s
      period: 1s
      timeout: 10s
      failure_threshold: 30

    liveness_probe:
      period: 30s
      timeout: 5s
      failure_threshold: 3

    readiness_probe:
      period: 10s
      timeout: 2s
      failure_threshold: 1

  restart_policy: OnFailure
  restart_backoff:
    initial: 10s
    max: 300s
    multiplier: 2.0
    max_retries: 5

  resource_limits:
    execution_timeout: 5m
    max_memory: 1GB
    max_files: 1024

  checkpoint:
    enabled: true
    interval: 1m
    max_history: 100
```

---

## 10. Anti-Patterns to Avoid

### 10.1 Don't Use `exec.Command` Without Context

❌ **Bad**:
```go
cmd := exec.Command("claude", "code")
cmd.Run() // No timeout, no cancellation
```

✅ **Good**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
cmd := exec.CommandContext(ctx, "claude", "code")
cmd.Run()
```

### 10.2 Don't Kill Individual Processes in Groups

❌ **Bad**:
```go
cmd.Process.Kill() // Leaves child processes orphaned
```

✅ **Good**:
```go
pgid, _ := syscall.Getpgid(cmd.Process.Pid)
syscall.Kill(-pgid, syscall.SIGKILL) // Kill entire group
```

### 10.3 Don't Ignore Exit Codes

❌ **Bad**:
```go
if err := cmd.Run(); err != nil {
    log.Printf("error: %v", err) // Swallows exit code
}
```

✅ **Good**:
```go
if err := cmd.Run(); err != nil {
    var exitErr *exec.ExitError
    if errors.As(err, &exitErr) {
        log.Printf("exit code: %d", exitErr.ExitCode())
        // Classify and handle appropriately
    }
}
```

---

## 11. References

- **Kubernetes Pod Lifecycle**: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/
- **Docker Container Restart Policies**: https://docs.docker.com/engine/containers/restart/
- **Go Context Package**: https://pkg.go.dev/context
- **Go os/exec Package**: https://pkg.go.dev/os/exec
- **OCI Runtime Specification**: https://github.com/opencontainers/runtime-spec
- **Uber Go Style Guide**: https://github.com/uber-go/guide

---

## Appendix: Quick Reference

### Exit Code Reference (Linux)

| Code | Meaning | HotPlex Action |
|------|---------|----------------|
| 0 | Success | No restart |
| 1 | General error | Restart (OnFailure) |
| 2 | Misuse | No restart (config error) |
| 9 | SIGKILL | OOM → Backoff restart |
| 11 | SIGSEGV | Segfault → Backoff restart |
| 15 | SIGTERM | Graceful stop → No restart |
| 127 | Command not found | No restart (config error) |
| 137 | SIGKILL (128+9) | OOM → Backoff restart |
| 139 | SIGSEGV (128+11) | Segfault → Backoff restart |
| 143 | SIGTERM (128+15) | Graceful stop → No restart |

### Graceful Shutdown Checklist

- [ ] Cancel context (stops all goroutines)
- [ ] Send SIGTERM to process group
- [ ] Wait for graceful exit (30s timeout)
- [ ] Send SIGKILL if timeout expires
- [ ] Save checkpoint (if session resumption enabled)
- [ ] Clean up resources (pipes, temp files)
- [ ] Update worker state to PhaseSucceeded/PhaseFailed

---

**Document Version**: 1.0
**Last Updated**: 2026-03-30
**Maintainer**: HotPlex Core Team
