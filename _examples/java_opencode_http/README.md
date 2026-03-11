# Java HTTP & WebSocket 客户端示例

本目录包含 Java 客户端示例，演示如何使用 HotPlex 的 HTTP REST API 和 WebSocket 接口。

## 示例文件

### 1. SimpleClient.java
简单的 HTTP 客户端示例，展示：
- REST 会话创建与消息发送
- SSE 事件流监听
- 系统提示词注入

### 2. HotPlexWsClient.java
企业级 WebSocket 客户端，具备：
- 自动重连（指数退避）
- 错误处理与恢复
- 优雅停机
- 指标收集

## 快速开始

### HTTP 客户端

```bash
# 启动 hotplexd 服务器
go run cmd/hotplexd/main.go

# 编译并运行
cd _examples/java_opencode_http
javac SimpleClient.java
java SimpleClient
```

### WebSocket 客户端

```bash
# 编译
javac HotPlexWsClient.java

# 运行
java com.hotplex.example.HotPlexWsClient
```

## 作为库使用

```java
import com.hotplex.example.HotPlexWsClient;

HotPlexWsClient client = new HotPlexWsClient(
    "ws://localhost:8080/ws/v1/agent",
    "my-session"
);

client.connect();

String result = client.execute(
    "List files in current directory",
    "You are a helpful assistant."
);

System.out.println(result);
client.disconnect();
```

## 依赖

- Java 11+
- 无需外部依赖（使用标准库）
