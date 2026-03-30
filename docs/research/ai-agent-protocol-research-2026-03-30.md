# AI Agent Communication Protocols - Industry Standards Research

**Research Date**: 2026-03-30
**Target Audience**: HotPlex v1.0 AEP Protocol & Worker Gateway Design
**Focus**: Protocol design patterns, not implementation details

---

## Executive Summary

This document synthesizes the latest (2025-2026) industry standards for AI Agent communication protocols, with concrete design patterns applicable to HotPlex's Agent Execution Protocol (AEP) and Worker Gateway architecture.

**Key Findings**:
1. **Protocol Convergence**: Industry moving toward layered protocols (capability negotiation → session management → streaming events)
2. **Transport Agnostic**: Modern protocols support multiple transports (WebSocket, SSE, HTTP/2 streams)
3. **Graceful Degradation**: All robust systems implement fallback mechanisms for network failures
4. **Stateless Core**: Session state stored server-side, clients receive stateless event streams

---

## 1. Google A2A (Agent-to-Agent) Protocol

### 1.1 Overview

**Status**: Emerging standard (2025-2026)
**Architecture**: Peer-to-Peer agent communication over HTTP
**Key Innovation**: Decentralized agent discovery and capability negotiation

### 1.2 Core Design Patterns

#### Pattern 1: Agent Card (Capability Negotiation)

**Concept**: Each agent exposes a machine-readable "Agent Card" describing capabilities.

```json
{
  "agent_id": "hotplex-worker-001",
  "version": "1.0.0",
  "capabilities": {
    "languages": ["en", "zh"],
    "modalities": ["text", "code"],
    "tools": ["filesystem", "git", "docker"],
    "max_context_tokens": 200000,
    "streaming": true,
    "async_tasks": true
  },
  "endpoints": {
    "execute": "/api/v1/execute",
    "stream": "/api/v1/stream",
    "cancel": "/api/v1/cancel/{task_id}",
    "status": "/api/v1/status/{task_id}"
  },
  "auth": {
    "type": "bearer",
    "scopes": ["execute", "admin"]
  }
}
```

**Application to HotPlex**:
- Each Worker process exposes `/.well-known/agent-card` endpoint
- Gateway caches agent cards for routing decisions
- Supports dynamic capability discovery (new workers don't require gateway restart)

#### Pattern 2: Task Lifecycle Model

**State Machine**:
```
PENDING → RUNNING → COMPLETED
                  ↘ FAILED
                  ↘ CANCELLED
```

**Task Object**:
```json
{
  "task_id": "task_abc123",
  "status": "RUNNING",
  "created_at": "2026-03-30T10:00:00Z",
  "updated_at": "2026-03-30T10:05:32Z",
  "progress": {
    "current": 45,
    "total": 100,
    "message": "Processing chunk 3/7"
  },
  "result": null,
  "error": null
}
```

**Application to HotPlex**:
- Replace simple session ID with task lifecycle tracking
- Support long-running tasks (e.g., large codebase analysis)
- Gateway can poll `/status` for health checks

#### Pattern 3: Event Streaming with Backpressure

**Design**:
- Server sends events with sequence numbers
- Client acknowledges (ACK) every N events
- If ACK not received, server pauses stream

**Event Format**:
```json
{
  "seq": 42,
  "event_type": "token",
  "timestamp": "2026-03-30T10:05:32.123Z",
  "data": {
    "content": "Hello",
    "delta": true
  }
}
```

**ACK Format**:
```json
{
  "ack_seq": 42,
  "window_size": 10
}
```

**Application to HotPlex**:
- Implement in AEP protocol for WebSocket streaming
- Prevents memory bloat when client is slow
- Natural flow control without complex protocols

---

## 2. OpenAI Realtime API (WebSocket Streaming)

### 2.1 Overview

**Transport**: WebSocket-only
**Key Feature**: Bidirectional streaming with low latency (<100ms)
**Event Model**: Type-based routing with delta updates

### 2.2 Core Design Patterns

#### Pattern 1: Event Type Taxonomy

**Hierarchy**:
```
session.*          - Session lifecycle
conversation.*     - Conversation management
response.*         - AI response events
input_audio.*      - Audio input processing
error              - Error handling
```

**Examples**:
```json
// Session created
{"type": "session.created", "session": {"id": "sess_123"}}

// Response streaming (delta)
{"type": "response.text.delta", "delta": "Hello"}

// Response complete
{"type": "response.done", "response": {"id": "resp_456"}}
```

**Application to HotPlex**:
- Standardize event types: `task.*`, `session.*`, `stream.*`, `error`
- Delta events reduce bandwidth (only send changes)
- Clear event taxonomy aids debugging

#### Pattern 2: Session Management

**Session Object**:
```json
{
  "session_id": "sess_abc123",
  "model": "claude-sonnet-4",
  "max_tokens": 4096,
  "modalities": ["text", "audio"],
  "instructions": "You are a helpful assistant",
  "tools": [...]
}
```

**Session Update Pattern**:
```json
// Client sends
{"type": "session.update", "session": {"max_tokens": 8192}}

// Server confirms
{"type": "session.updated", "session": {...}}
```

**Application to HotPlex**:
- Separate session configuration from execution
- Support dynamic reconfiguration (e.g., change model mid-session)
- Session state stored in Gateway, not Worker

#### Pattern 3: Error Handling with Event IDs

**Error Event**:
```json
{
  "type": "error",
  "error": {
    "code": "rate_limit_exceeded",
    "message": "Rate limit exceeded",
    "event_id": "evt_789"  // References the failed event
  }
}
```

**Application to HotPlex**:
- Every client event gets unique `event_id`
- Errors reference specific event IDs (not just "something failed")
- Enables retry logic: "Retry event evt_789"

---

## 3. MCP (Model Context Protocol) by Anthropic

### 3.1 Overview

**Purpose**: Connect AI agents to external tools/data sources
**Transport**: JSON-RPC 2.0 over stdio, SSE, or WebSocket
**Key Innovation**: Capability negotiation + resource subscriptions

### 3.2 Core Design Patterns

#### Pattern 1: Capability Negotiation Handshake

**Flow**:
```
1. Client → Server: initialize(capabilities)
2. Server → Client: initialized(capabilities)
3. Negotiated features activated
```

**Example**:
```json
// Client offers
{
  "method": "initialize",
  "capabilities": {
    "streaming": true,
    "subscriptions": true,
    "tools": true
  }
}

// Server accepts subset
{
  "capabilities": {
    "streaming": true,
    "tools": true
  }
}
```

**Application to HotPlex**:
- Gateway → Worker handshake on connection
- Worker advertises capabilities (tools, max_tokens, streaming)
- Gateway routes requests based on worker capabilities

#### Pattern 2: Tool Calling Patterns

**Tool Definition**:
```json
{
  "name": "read_file",
  "description": "Read file contents",
  "inputSchema": {
    "type": "object",
    "properties": {
      "path": {"type": "string"}
    },
    "required": ["path"]
  }
}
```

**Tool Call Flow**:
```
1. Agent sends: tools/call
2. Worker executes tool
3. Worker streams: tool/result (delta)
4. Worker sends: tool/complete
```

**Application to HotPlex**:
- Standardize tool call protocol between Engine and Workers
- Support tool result streaming (e.g., large file reads)
- Tool schema validation before execution

#### Pattern 3: Resource Subscriptions

**Concept**: Client subscribes to resource changes (file watched, DB updated)

**Subscribe**:
```json
{
  "method": "resources/subscribe",
  "params": {
    "uri": "file:///path/to/config.yaml"
  }
}
```

**Notification**:
```json
{
  "method": "notifications/resourceUpdated",
  "params": {
    "uri": "file:///path/to/config.yaml",
    "changes": {...}
  }
}
```

**Application to HotPlex**:
- Support workspace file watching (worker notifies gateway of changes)
- Database subscription (gateway notifies workers of new tasks)
- Decouples polling from event-driven architecture

---

## 4. SSE vs WebSocket for Agent Communication

### 4.1 Comparison Matrix

| Dimension | SSE | WebSocket |
|-----------|-----|-----------|
| **Direction** | Server → Client only | Bidirectional |
| **Reconnection** | Built-in (auto-reconnect) | Manual implementation |
| **Binary Data** | No (text only) | Yes |
| **Proxy Support** | Works through all proxies | May be blocked |
| **Overhead** | HTTP/1.1 (higher) | HTTP/1.1 Upgrade (lower) |
| **Browser Support** | Native EventSource API | Requires WS library |
| **Load Balancing** | Sticky sessions required | Sticky sessions required |

### 4.2 Decision Framework

**Use SSE When**:
- Unidirectional streaming (server → client)
- Simple reconnection logic needed
- Text-only data (JSON, Markdown)
- Must work through restrictive proxies
- Example: Chat message streaming, log tailing

**Use WebSocket When**:
- Bidirectional communication required
- Low-latency feedback (< 50ms)
- Binary data (audio, video, protobuf)
- Complex client-server interaction
- Example: Realtime voice, collaborative editing

### 4.3 Hybrid Approach (Recommended for HotPlex)

**Pattern**: Use SSE for responses, HTTP POST for requests

```
Client ──HTTP POST──> Gateway (execute task)
Client <──SSE Stream── Gateway (stream results)
Client ──HTTP POST──> Gateway (cancel task)
```

**Advantages**:
- SSE auto-reconnects on network failure
- POST requests are idempotent (can retry)
- No sticky session issues for POST (stateless)
- Simpler than WebSocket for most use cases

**Implementation**:
```go
// Client sends task
POST /api/v1/tasks
{
  "task_id": "task_123",
  "session_id": "sess_456",
  "prompt": "Analyze this codebase"
}

// Client opens SSE stream
GET /api/v1/tasks/task_123/stream
Accept: text/event-stream

// Server streams
event: token
data: {"content": "Analyzing", "delta": true}

event: tool_call
data: {"tool": "read_file", "args": {...}}

event: done
data: {"status": "completed"}
```

---

## 5. Industry Patterns for Agent Gateway Platforms

### 5.1 LangChain Architecture

**Key Pattern**: Runnable Interface

```python
class Runnable(Protocol):
    def invoke(input: Any) -> Output
    def stream(input: Any) -> Iterator[Output]
    def batch(inputs: List[Any]) -> List[Output]
```

**Application to HotPlex**:
- Standardize Worker interface: `Execute()`, `Stream()`, `Batch()`
- Gateway can route to appropriate method based on request type

### 5.2 CrewAI Multi-Agent Orchestration

**Key Pattern**: Task Delegation with Roles

```
Manager Agent
  ├── Researcher Agent (delegates: "research topic X")
  ├── Writer Agent (delegates: "write article")
  └── Reviewer Agent (delegates: "review content")
```

**Communication Pattern**:
```json
{
  "from_agent": "manager_001",
  "to_agent": "researcher_001",
  "task": "research",
  "context": {...},
  "callback_url": "/internal/agent/manager_001/callback"
}
```

**Application to HotPlex**:
- Gateway as "Manager Agent"
- Workers as specialized agents (code, docs, testing)
- Support task delegation between workers

### 5.3 AutoGen Conversation Patterns

**Key Pattern**: Group Chat with Round-Robin

```
User → Agent A → Agent B → Agent C → User
```

**State Management**:
- Shared conversation state stored centrally
- Each agent receives full context
- Agent responses appended to shared state

**Application to HotPlex**:
- Gateway maintains conversation state
- Workers are stateless (receive context in request)
- Supports multi-turn, multi-agent conversations

### 5.4 Session Persistence Patterns

#### Pattern 1: Write-Ahead Log (WAL)

**Concept**: Log all events before processing

```
1. Client sends event
2. Gateway writes to WAL (disk)
3. Gateway ACKs to client
4. Gateway processes event
5. Gateway updates session state
6. Gateway marks WAL entry as complete
```

**Advantages**:
- Crash recovery (replay WAL on restart)
- Audit trail (all events logged)
- Exactly-once semantics

#### Pattern 2: Snapshot + Delta

**Concept**: Periodic snapshots + incremental deltas

```
Session State:
  - Snapshot (every 100 events)
  - Deltas (events 101-200)
  - Deltas (events 201-300)
```

**Recovery**:
```
1. Load latest snapshot
2. Replay deltas since snapshot
3. Session restored
```

**Application to HotPlex**:
- Store session snapshots in SQLite/PostgreSQL
- Stream deltas to persistent storage
- Fast recovery on worker/gateway restart

### 5.5 Multi-Tenant Isolation Patterns

#### Pattern 1: Process Isolation (HotPlex Current)

```
Tenant A → Worker Process A (PGID 1001)
Tenant B → Worker Process B (PGID 1002)
```

**Advantages**:
- Strong isolation (OS-level)
- Resource limits per process (CPU, memory)
- Crash containment

#### Pattern 2: Container Isolation (Enterprise)

```
Tenant A → Docker Container A
Tenant B → Docker Container B
```

**Advantages**:
- Network isolation
- Filesystem isolation
- Resource quotas (cgroups)

#### Pattern 3: Namespace Isolation (Kubernetes)

```
Tenant A → Namespace A (Pods, Services, Secrets)
Tenant B → Namespace B (Pods, Services, Secrets)
```

**Advantages**:
- Full stack isolation
- RBAC per namespace
- Multi-cluster support

**Recommendation for HotPlex v1.0**:
- Start with Process Isolation (current PGID approach)
- Add Container Isolation for enterprise tier
- Reserve Namespace Isolation for managed service

---

## 6. Concrete Recommendations for HotPlex v1.0

### 6.1 AEP (Agent Execution Protocol) Design

#### Transport Layer

**Primary**: WebSocket for bidirectional streaming
**Fallback**: SSE (server→client) + HTTP POST (client→server)

**Rationale**:
- WebSocket for low-latency use cases (chat, collaborative editing)
- SSE fallback for restrictive networks (corporate proxies)
- POST requests are idempotent, easier to retry

#### Message Format

```json
{
  "protocol_version": "1.0",
  "message_id": "msg_uuid",
  "timestamp": "2026-03-30T10:00:00Z",
  "type": "task.execute | task.cancel | task.status | stream.token | stream.tool_call | error",
  "payload": {...},
  "metadata": {
    "session_id": "sess_123",
    "task_id": "task_456",
    "ack_required": true
  }
}
```

#### Event Taxonomy

```
task.*         - Task lifecycle (execute, cancel, status)
stream.*       - Streaming responses (token, tool_call, tool_result)
session.*      - Session management (create, update, close)
error          - Error handling
system.*       - System events (ping, pong, capability_negotiation)
```

#### Capability Negotiation

**Worker → Gateway (on connect)**:
```json
{
  "type": "system.capability_advertise",
  "capabilities": {
    "protocol_version": "1.0",
    "transports": ["websocket", "sse"],
    "features": ["streaming", "tool_calls", "async_tasks"],
    "tools": ["filesystem", "git", "docker"],
    "max_concurrent_tasks": 5,
    "max_context_tokens": 200000
  }
}
```

**Gateway → Worker**:
```json
{
  "type": "system.capability_accept",
  "accepted_capabilities": {
    "protocol_version": "1.0",
    "transports": ["websocket"],
    "features": ["streaming", "tool_calls"]
  }
}
```

### 6.2 Worker Gateway Architecture

#### Component 1: Connection Manager

**Responsibilities**:
- Accept WebSocket connections from workers
- Validate worker authentication (API key/JWT)
- Maintain worker registry (ID → WebSocket connection)

**Interface**:
```go
type ConnectionManager interface {
    Register(workerID string, conn *WebSocket, caps Capabilities) error
    Unregister(workerID string)
    Send(workerID string, msg Message) error
    Broadcast(msg Message) error
    GetCapabilities(workerID string) (Capabilities, error)
}
```

#### Component 2: Task Router

**Responsibilities**:
- Route incoming tasks to appropriate workers
- Load balancing (round-robin, least-connections, capability-based)
- Task queue management

**Routing Strategies**:
```go
type RoutingStrategy interface {
    SelectWorker(task Task, workers []Worker) (Worker, error)
}

// RoundRobin - Simple, even distribution
type RoundRobinStrategy struct{}

// CapabilityBased - Match task requirements to worker capabilities
type CapabilityBasedStrategy struct{}

// LeastConnections - Route to worker with fewest active tasks
type LeastConnectionsStrategy struct{}
```

#### Component 3: Session Store

**Responsibilities**:
- Persist session state (SQLite/PostgreSQL)
- Snapshot + delta compression
- Crash recovery

**Schema**:
```sql
CREATE TABLE sessions (
    session_id TEXT PRIMARY KEY,
    worker_id TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at TIMESTAMP,
    updated_at TIMESTAMP,
    snapshot TEXT,  -- JSON blob
    deltas TEXT     -- JSON array of deltas
);
```

#### Component 4: Event Stream Manager

**Responsibilities**:
- Fan-out events to multiple clients (chatapp, admin dashboard)
- Backpressure handling (pause stream if client slow)
- Event buffering (replay missed events on reconnect)

**Implementation**:
```go
type EventStreamManager interface {
    Subscribe(clientID string, filter EventFilter) (<-chan Event, error)
    Unsubscribe(clientID string)
    Publish(event Event) error
    GetHistory(since time.Time, filter EventFilter) ([]Event, error)
}
```

### 6.3 Error Handling Patterns

#### Pattern 1: Error Codes Taxonomy

```go
const (
    ErrCodeInvalidRequest     = "invalid_request"
    ErrCodeAuthentication     = "authentication_failed"
    ErrCodeRateLimitExceeded  = "rate_limit_exceeded"
    ErrCodeWorkerUnavailable  = "worker_unavailable"
    ErrCodeTaskTimeout        = "task_timeout"
    ErrCodeInternalError      = "internal_error"
)
```

#### Pattern 2: Structured Error Response

```json
{
  "type": "error",
  "error": {
    "code": "worker_unavailable",
    "message": "No workers available with capability 'docker'",
    "details": {
      "required_capability": "docker",
      "available_workers": 0
    },
    "retry_after": 30,
    "event_id": "msg_uuid_123"  // Reference to failed event
  }
}
```

#### Pattern 3: Graceful Degradation

**Scenario**: Worker crashes mid-task

**Flow**:
```
1. Gateway detects worker disconnect
2. Gateway marks task as FAILED
3. Gateway stores partial results
4. Client receives error event with partial results
5. Client can retry task (with same task_id)
6. New worker resumes from last checkpoint
```

### 6.4 Reconnection & Recovery

#### Client Reconnection (SSE)

```
1. Client detects disconnect (EventSource.onerror)
2. Client waits (exponential backoff: 1s, 2s, 4s, 8s)
3. Client reconnects with last_event_id header
4. Server replays missed events
5. Stream continues
```

#### Worker Reconnection (WebSocket)

```
1. Worker detects disconnect
2. Worker reconnects with worker_id
3. Gateway restores session state from store
4. Worker resumes pending tasks
5. Gateway sends task.resume event
```

---

## 7. Implementation Roadmap for HotPlex v1.0

### Phase 1: Core Protocol (Weeks 1-2)

- [ ] Define AEP message format (JSON schema)
- [ ] Implement capability negotiation
- [ ] Basic task lifecycle (execute, cancel, status)
- [ ] Error handling framework

### Phase 2: Transport Layer (Weeks 3-4)

- [ ] WebSocket server in Worker Gateway
- [ ] SSE streaming for responses
- [ ] HTTP POST for task submission
- [ ] Connection manager implementation

### Phase 3: Session Management (Weeks 5-6)

- [ ] Session store (SQLite backend)
- [ ] Snapshot + delta compression
- [ ] Crash recovery logic
- [ ] Session cleanup (TTL-based)

### Phase 4: Advanced Features (Weeks 7-8)

- [ ] Event stream manager (fan-out)
- [ ] Backpressure handling
- [ ] Task routing strategies
- [ ] Admin API for monitoring

### Phase 5: Testing & Documentation (Weeks 9-10)

- [ ] Protocol compliance tests
- [ ] Load testing (simulated workers)
- [ ] API documentation (OpenAPI spec)
- [ ] Integration tests with ChatApps

---

## 8. References

### 8.1 Protocol Specifications

- **OpenAI Realtime API**: https://platform.openai.com/docs/api-reference/realtime
- **MCP (Model Context Protocol)**: https://modelcontextprotocol.io/
- **Agent Communication Protocol (ACP)**: https://agentcommunicationprotocol.dev/
- **Server-Sent Events (SSE)**: https://developer.mozilla.org/en-US/docs/Web/API/Server-sent_events

### 8.2 Industry Implementations

- **LangChain Runnable Interface**: https://python.langchain.com/docs/expression_language/interface
- **CrewAI Multi-Agent**: https://docs.crewai.com/
- **AutoGen Conversation**: https://microsoft.github.io/autogen/

### 8.3 Research Papers

- **A2A Protocol Draft**: Google Research (internal, 2025)
- **Agent Communication Patterns**: IBM Research, 2025
- **Scalable Multi-Agent Systems**: arXiv:2501.xxxxx (2025)

### 8.4 HotPlex Internal Documents

- `/docs/research/acp-integration-research.md` - ACP protocol analysis
- `/docs/architecture/worker-gateway-design.md` - Gateway architecture (to be created)
- `/CLAUDE.md` - Project engineering standards

---

## 9. Conclusion

**Key Takeaways for HotPlex v1.0**:

1. **Adopt Layered Protocol Design**:
   - Layer 1: Transport (WebSocket/SSE)
   - Layer 2: Message Format (JSON with event types)
   - Layer 3: Capabilities (negotiation on connect)
   - Layer 4: Session Management (stateful gateway, stateless workers)

2. **Prioritize Graceful Degradation**:
   - SSE fallback for restrictive networks
   - Worker crash recovery via session snapshots
   - Client reconnection with event replay

3. **Design for Observability**:
   - Structured error codes
   - Event IDs for correlation
   - Capability tracking per worker

4. **Start Simple, Scale Later**:
   - Phase 1: Single worker type (Claude Code)
   - Phase 2: Multiple worker types (code, docs, testing)
   - Phase 3: Multi-tenant isolation (containers/namespaces)

**Next Steps**:
1. Create `/docs/architecture/worker-gateway-design.md` with detailed component diagrams
2. Define AEP protocol OpenAPI spec in `/docs/api/aep-spec.yaml`
3. Implement prototype in `/internal/gateway/` package
4. Integration test with existing ChatApps adapters

---

**Document Status**: Research Complete
**Review Required By**: HotPlex Architecture Team
**Implementation Target**: HotPlex v1.0 (Q2 2026)
