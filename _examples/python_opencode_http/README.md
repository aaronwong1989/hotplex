# OpenCode HTTP 客户端示例 (Python)

本示例展示如何使用 Python 通过 REST API + SSE (Server-Sent Events) 与 HotPlex OpenCode 兼容层进行交互。

## 功能特点

- **SSE 监听**：使用 `requests` 库进行实时事件流
- **会话管理**：RESTful 风格的会话创建和消息发送
- **事件映射**：将 OpenCode 的 "Parts" 转换为可读的控制台输出

## 快速开始

### 1. 安装依赖

```bash
cd _examples/python_opencode_http
pip install requests
```

### 2. 启动 HotPlex 服务器

```bash
go run cmd/hotplexd/main.go
```

### 3. 运行示例

```bash
python client.py
```

## 代码结构

### 1. 监听事件流

```python
def listen_to_events():
    """监听 SSE 全局事件流"""
    response = requests.get(f"{BASE_URL}/global/event", stream=True)

    for line in response.iter_lines():
        if line:
            decoded_line = line.decode('utf-8')
            if decoded_line.startswith('data: '):
                data_str = decoded_line[len('data: '):]
                event = json.loads(data_str)

                # 处理事件
                payload = event.get("payload", {})
                p_type = payload.get("type")
```

### 2. 创建会话

```python
def create_session():
    """通过 OpenCode API 创建新会话"""
    resp = requests.post(f"{BASE_URL}/session")
    session_id = resp.json()["info"]["id"]
    return session_id
```

### 3. 发送提示词

```python
def send_prompt(session_id, prompt):
    """向活动会话发送提示词"""
    url = f"{BASE_URL}/session/{session_id}/message"
    requests.post(url, json={"prompt": prompt})
```

## OpenCode 事件类型

### 消息部分更新

```python
if p_type == "message.part.updated":
    part = props.get("part", {})
    content_type = part.get("type")

    if content_type == "text":
        print(f"\n🤖: {part.get('text')}")
    elif content_type == "reasoning":
        print(f"\n🤔 思考: {part.get('text')}")
    elif content_type == "tool":
        state = part.get("state", {})
        status = state.get("status")

        if status == "running":
            print(f"\n🛠️ 使用工具: {part.get('tool')}")
        elif status == "completed":
            print(f"✅ 工具结果: {state.get('output')[:100]}...")
```

### 服务器事件

```python
elif p_type == "server.connected":
    print("✅ 已连接到 HotPlex OpenCode 服务器")
```

## REST API 端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/session` | POST | 创建新会话 |
| `/session/{id}` | GET | 获取会话信息 |
| `/session/{id}/message` | POST | 发送消息 |
| `/global/event` | GET | SSE 事件流 |

## 使用 cURL 测试

```bash
# 1. 创建会话
curl -X POST http://localhost:8080/session

# 2. 建立事件流（另一个终端）
curl -N http://localhost:8080/global/event

# 3. 发送提示词
curl -X POST http://localhost:8080/session/<id>/message \
     -H "Content-Type: application/json" \
     -d '{"prompt": "写一个 Python 脚本列出文件"}'
```

## 运行要求

- Python 3.8+
- `requests` 库

## 扩展阅读

- [Node.js WebSocket 示例](../node_claude_websocket)
- [Go 基础示例](../go_claude_basic)
- [API 文档](../../docs/server/api_zh.md)
