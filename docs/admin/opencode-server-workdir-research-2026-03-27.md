# OpenCode Server 工作目录机制调研报告

**日期**: 2026-03-27
**问题**: OpenCode Server 模式下工作目录未正确设置

---

## 🔍 调研结果总结

### 关键发现：OpenCode Server 使用 HTTP Header 指定工作目录

**OpenCode Server 的目录机制**：

根据官方文档和源码分析，OpenCode Server 支持通过以下两种方式指定工作目录：

1. **HTTP Header**: `x-opencode-directory: /path/to/project`
2. **Query Parameter**: `?directory=/path/to/project`

---

## 📚 官方文档证据

### 来源 1: [跟着OpenCode学智能体设计和开发6：服务器API](https://qixinbo.github.io/2026/01/21/opencode-6/)

> The API server implements a **directory-based context middleware** that extracts the working directory from query parameters or the `x-opencode-directory` header, enabling **multi-project support within a single server instance**.

**关键点**：
- ✅ Server 实现了 **directory-based context middleware**
- ✅ 从 **query parameters** 或 **`x-opencode-directory` header** 提取工作目录
- ✅ 允许 **单个服务器实例支持多个项目**

### 来源 2: [GitHub Issue #14595](https://github.com/anomalyco/opencode/issues/14595)

```typescript
// server.ts - OpenCode Server 的实现
const raw = c.req.query("directory") || c.req.header("x-opencode-directory");
```

**关键点**：
- ✅ 优先级：query parameter > HTTP header
- ✅ Server 使用这个值设置 `Instance.project`

---

## 🔧 OpenCode Server API 端点

### GET /project/current

获取当前活动项目信息：

```http
GET /project/current
x-opencode-directory: /path/to/project
```

**响应示例**：
```json
{
  "id": "project-1",
  "name": "My Project",
  "path": "/path/to/project"
}
```

### GET /find/file

查找文件（支持指定目录）：

```http
GET /find/file?query=helpers&type=file&directory=/custom/path
```

---

## ❌ HotPlex 的问题

### 当前实现（有缺陷）

检查 `/Users/huangzhonghui/hotplex/provider/transport_http.go`：

```go
// Line 140-148: SSE 连接
func (t *HTTPTransport) connectAndStream(ctx context.Context, attempt *int) error {
	url := fmt.Sprintf("%s/event", t.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
	}
	// ❌ 缺少: req.Header.Set("x-opencode-directory", workDir)
```

```go
// Line 275-297: 发送消息
func (t *HTTPTransport) Send(ctx context.Context, sessionID string, message map[string]any) error {
	url := fmt.Sprintf("%s/session/%s/prompt_async", t.baseURL, sessionID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
	}
	// ❌ 缺少: req.Header.Set("x-opencode-directory", workDir)
```

```go
// Line 326-339: 创建会话
func (t *HTTPTransport) CreateSession(ctx context.Context, title string) (string, error) {
	url := t.baseURL + "/session"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if t.password != "" {
		req.SetBasicAuth("opencode", t.password)
	}
	// ❌ 缺少: req.Header.Set("x-opencode-directory", workDir)
```

### 问题根源

1. **HTTPTransport 没有 WorkDir 字段**
   - `HTTPTransportConfig` 只包含 `Endpoint`, `Password`, `Logger`, `Timeout`
   - 没有 `WorkDir` 字段

2. **所有 HTTP 请求都缺少 `x-opencode-directory` header**
   - `connectAndStream()` - SSE 连接
   - `Send()` - 发送消息
   - `CreateSession()` - 创建会话

3. **结果**：OpenCode Server 使用默认工作目录（启动服务器的目录）
   - 而不是 HotPlex 配置的 `/Users/huangzhonghui/.hotplex/projects/hotplex`

---

## ✅ 修复方案

### Step 1: 添加 WorkDir 到 HTTPTransportConfig

```go
// provider/transport.go
type HTTPTransportConfig struct {
	Endpoint string
	Password string
	Logger   *slog.Logger
	Timeout  time.Duration
	WorkDir  string // 🎯 新增：工作目录
}
```

### Step 2: 在 HTTPTransport 中存储 WorkDir

```go
type HTTPTransport struct {
	baseURL    string
	restClient *http.Client
	sseClient  *http.Client
	password   string
	workDir    string        // 🎯 新增
	logger     *slog.Logger
	// ...
}

func NewHTTPTransport(cfg HTTPTransportConfig) *HTTPTransport {
	return &HTTPTransport{
		baseURL:    strings.TrimSuffix(cfg.Endpoint, "/"),
		password:   cfg.Password,
		workDir:    cfg.WorkDir, // 🎯 新增
		// ...
	}
}
```

### Step 3: 在所有 HTTP 请求中添加 Header

```go
func (t *HTTPTransport) connectAndStream(ctx context.Context, attempt *int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	// ...
	if t.workDir != "" {
		req.Header.Set("x-opencode-directory", t.workDir) // 🎯 新增
	}
	// ...
}

func (t *HTTPTransport) Send(ctx context.Context, sessionID string, message map[string]any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	// ...
	if t.workDir != "" {
		req.Header.Set("x-opencode-directory", t.workDir) // 🎯 新增
	}
	// ...
}

func (t *HTTPTransport) CreateSession(ctx context.Context, title string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	// ...
	if t.workDir != "" {
		req.Header.Set("x-opencode-directory", t.workDir) // 🎯 新增
	}
	// ...
}
```

### Step 4: 在 OpenCodeServerProvider 中传递 WorkDir

```go
// provider/opencode_server_provider.go
func NewOpenCodeServerProvider(cfg ProviderConfig, logger *slog.Logger) (*OpenCodeServerProvider, error) {
	// ...
	return &OpenCodeServerProvider{
		ProviderBase: ProviderBase{
			meta:   openCodeServerMeta,
			logger: logger.With("provider", "opencode-server"),
		},
		transport: NewHTTPTransport(HTTPTransportConfig{
			Endpoint: url,
			Password: ocCfg.Password,
			Logger:   logger.With("component", "oc_transport"),
			Timeout:  30 * time.Second,
			WorkDir:  cfg.Engine.WorkDir, // 🎯 新增：从配置中获取工作目录
		}),
		// ...
	}, nil
}
```

---

## 📊 修复后的预期行为

1. **HotPlex 配置**：
   ```yaml
   engine:
     work_dir: ${HOTPLEX_PROJECTS_DIR}/hotplex
   ```

2. **展开后**：
   ```
   work_dir: /Users/huangzhonghui/.hotplex/projects/hotplex
   ```

3. **HTTP 请求**：
   ```http
   POST /session
   x-opencode-directory: /Users/huangzhonghui/.hotplex/projects/hotplex
   ```

4. **OpenCode Server 行为**：
   - ✅ 接收到 header
   - ✅ 设置 `Instance.project` = `/Users/huangzhonghui/.hotplex/projects/hotplex`
   - ✅ 在正确的目录中执行 Git 操作、文件读写等

---

## 🔗 相关资源

### 官方文档
- [OpenCode Config](https://opencode.ai/docs/config/)
- [OpenCode Server](https://opencode.ai/docs/server/)
- [OpenCode CLI](https://opencode.ai/docs/cli/)

### 技术文章
- [跟着OpenCode学智能体设计和开发6：服务器API](https://qixinbo.github.io/2026/01/21/opencode-6/)
- [GitHub Issue #14595: Background Task Session Lookup Fails Across Project Contexts](https://github.com/anomalyco/opencode/issues/14595)

### 相关工具
- [Open-Switch](https://github.com/fengshao1227/Open-Switch) - OpenCode 配置管理工具
- [opencode-sdk-go](https://pkg.go.dev/github.com/sst/opencode-sdk-go) - Go SDK

---

## 🎯 下一步行动

1. **实现修复**：按照上述方案修改代码
2. **测试验证**：
   - 发送测试消息
   - 检查 OpenCode Server 是否使用正确的工作目录
   - 验证 Git 操作、文件读写是否在正确目录执行
3. **日志验证**：
   - 检查 OpenCode Server 日志，确认收到 `x-opencode-directory` header
   - 验证 `Instance.project` 设置正确

---

**结论**：HotPlex 的 OpenCode Server Provider 缺少关键的 `x-opencode-directory` header，导致 OpenCode Server 使用默认工作目录。修复方案已明确，需要立即实施。
