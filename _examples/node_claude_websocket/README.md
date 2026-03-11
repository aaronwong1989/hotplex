# Claude WebSocket 客户端示例 (Node.js)

本示例展示如何使用 WebSocket 连接与 HotPlex 服务器进行实时交互。

## 文件说明

| 文件 | 说明 |
|------|------|
| `client.js` | 快速入门 - 最简示例，约 50 行代码 |
| `enterprise_client.js` | 企业级客户端 - 生产就绪特性 |

## 快速开始

### 1. 启动 HotPlex 服务器

```bash
go run cmd/hotplexd/main.go
```

### 2. 运行客户端

```bash
cd _examples/node_claude_websocket
npm install

# 快速入门
node client.js

# 企业级示例
node enterprise_client.js
```

## client.js - 快速入门

### 连接与发送请求

```javascript
const ws = new WebSocket("ws://localhost:8080/ws/v1/agent");

ws.on("open", () => {
    ws.send(JSON.stringify({
        type: "execute",
        session_id: "quick-start-demo",
        prompt: "Say 'Hello from HotPlex!'",
        work_dir: process.cwd()
    }));
});

ws.on("message", (data) => {
    const msg = JSON.parse(data);
    switch (msg.event) {
        case "thinking":
            process.stdout.write(".");
            break;
        case "answer":
            process.stdout.write(msg.data.event_data);
            break;
        case "completed":
            console.log("\nDone!");
            ws.close();
            break;
    }
});
```

## enterprise_client.js - 企业级特性

### 核心功能

- **自动重连**：指数退避算法
- **错误处理**：完善的异常恢复
- **结构化日志**：可配置日志级别
- **心跳检测**：连接健康监控
- **请求超时**：超时管理
- **优雅关闭**：SIGINT/SIGTERM 支持
- **指标收集**：延迟、成功率、重连次数
- **进度回调**：流式事件处理

### 使用方式

```javascript
const { HotPlexClient } = require('./enterprise_client');

const client = new HotPlexClient({
    url: 'ws://localhost:8080/ws/v1/agent',
    sessionId: 'my-session',
    logLevel: 'info',
    reconnect: { enabled: true, maxAttempts: 5 }
});

await client.connect();

const result = await client.execute('List files', {
    systemPrompt: 'You are a helpful assistant.',
    onProgress: (event) => {
        if (event.type === 'answer') {
            process.stdout.write(event.data);
        }
    }
});

console.log(result);
await client.disconnect();
```

## WebSocket 协议

### 请求格式

```json
{
    "type": "execute",
    "session_id": "session-123",
    "prompt": "你的提示词",
    "work_dir": "/path/to/workdir",
    "instructions": "可选的特殊指令",
    "request_id": 123
}
```

### 响应格式

```json
{
    "event": "answer",
    "data": {
        "event_type": "answer",
        "event_data": "响应内容",
        "meta": {
            "tool_name": "bash",
            "status": "running"
        }
    },
    "request_id": 123
}
```

### 事件类型

| 事件 | 说明 |
|------|------|
| `thinking` | 模型正在思考 |
| `tool_use` | 工具调用开始 |
| `tool_result` | 工具调用结果 |
| `answer` | 流式文本响应 |
| `completed` | 执行完成 |
| `error` | 错误发生 |
| `stopped` | 请求已停止 |

### 其他请求类型

```json
// 停止执行
{ "type": "stop", "session_id": "session-123" }

// 获取统计
{ "type": "stats", "session_id": "session-123" }

// 获取版本
{ "type": "version" }
```

## 企业级客户端 API

### HotPlexClient 配置

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `url` | WebSocket 服务器地址 | 必填 |
| `sessionId` | 会话标识符 | 自动生成 |
| `logLevel` | 日志级别 (debug/info/warn/error) | info |
| `reconnect` | 重连配置 | 禁用 |
| `timeout` | 请求超时时间 | 60000ms |

### 方法

| 方法 | 说明 |
|------|------|
| `connect()` | 建立 WebSocket 连接 |
| `execute(prompt, options)` | 执行提示词 |
| `stop(sessionId)` | 停止会话 |
| `stats(sessionId)` | 获取统计信息 |
| `disconnect()` | 断开连接 |

## 系统提示词注入

通过 WebSocket 请求的 `system_prompt` 字段传递：

```javascript
const result = await client.execute('List files', {
    // BaseSystemPrompt - Engine 级别
    systemPrompt: 'You are a Go expert. Provide concise answers.',

    // TaskInstructions - Session 级别
    instructions: 'Always use Go 1.25+ features.',

    onProgress: (event) => { /* ... */ }
});
```

### 对比说明

| 字段 | 级别 | 说明 |
|------|------|------|
| `system_prompt` | Engine | 会话全程生效的身份定义 |
| `instructions` | Session | 每个 turn 追加的指令 |

详细说明请参考 [Claude 生命周期示例](../go_claude_lifecycle)。

## 运行要求

- Node.js 18+
- HotPlex 服务器运行中

## 扩展阅读

- [Python HTTP 示例](../python_opencode_http)
- [Go 基础示例](../go_claude_basic)
- [API 文档](../../docs/server/api_zh.md)
