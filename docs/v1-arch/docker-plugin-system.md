# HotPlex v1.0.0 Docker 容器隔离插件系统

> 版本：v1.0-final  
> 日期：2026-03-29  
> 状态：**已重写**（原 Go .so 方案已废弃）

---

## 一、设计决策

### 1.1 为什么选择 Docker 容器隔离

| 方案 | 优点 | 缺点 | 结论 |
|------|------|------|------|
| Go .so | 零容器开销 | Go 官方明确声明 **不保证 ABI 稳定性** | ❌ **废弃** |
| RPC 进程隔离 | 低延迟 | 插件必须用 Go | ⚠️ 备选 |
| **Docker 容器隔离** | 完全隔离、任意语言、安全加固、与 OpenClaw 对齐 | 额外资源开销 | ✅ **采用** |

**参考：** OpenClaw 同样使用 Docker 容器作为插件隔离边界。

### 1.2 架构概览

```
┌─────────────────────────────────────────────────────────────────┐
│                      HotPlex Host (Main Process)                 │
│                                                                 │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │                   Plugin Gateway                         │   │
│   │            (Unix Domain Socket / HTTP)                  │   │
│   └─────────────────────────────────────────────────────────┘   │
│                              │                                 │
│                              │ Docker API                      │
│                              ▼                                 │
│   ┌────────────┐  ┌────────────┐  ┌────────────┐             │
│   │  Dingtalk  │  │  Custom    │  │  Wechat    │             │
│   │  Channel   │  │   Brain    │  │  Channel   │             │
│   │  Container │  │  Container │  │  Container │             │
│   └────────────┘  └────────────┘  └────────────┘             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 二、ContainerPlugin 接口

### 2.1 接口定义

```go
// pkg/plugin/container.go

package plugin

import (
    "context"
    "time"
)

// ContainerConfig 容器配置
type ContainerConfig struct {
    // 镜像
    Image string
    
    // 容器名称
    Name string
    
    // 环境变量
    Env map[string]string
    
    // 挂载点
    Mounts []Mount
    
    // 网络模式
    NetworkMode string  // "bridge" | "host" | "none"
    
    // 资源限制
    Resources ContainerResources
    
    // Linux capabilities
    Capabilities []string
    
    // 安全选项
    SecurityOpt []string
    
    // 只读根文件系统
    ReadOnlyRoot bool
    
    // 工作目录
    WorkDir string
}

// Mount 挂载
type Mount struct {
    Source string
    Target string
    Type   string  // "bind" | "volume"
    ReadOnly bool
}

// ContainerResources 资源限制
type ContainerResources struct {
    CPU        float64  // CPU 限制（如 0.5）
    Memory     string   // 内存限制（如 "512m"）
    MemorySwap string   // swap 限制（如 "1g"）
    Pids       int64    // PID 限制
}

// ContainerPlugin Docker 容器插件接口
type ContainerPlugin interface {
    Plugin  // 内嵌基类
    
    // 容器配置
    ContainerConfig() *ContainerConfig
    
    // 容器生命周期
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Restart(ctx context.Context) error
    
    // 健康检查
    Health(ctx context.Context) *ContainerHealth
    
    // 指标
    Stats(ctx context.Context) (*ContainerStats, error)
}

// ContainerHealth 容器健康状态
type ContainerHealth struct {
    Status       string
    Message      string
    RestartCount int
    StartedAt    time.Time
}

// ContainerStats 容器统计
type ContainerStats struct {
    CPUPercent    float64
    MemoryUsage   uint64
    MemoryLimit   uint64
    NetworkRX     uint64
    NetworkTX     uint64
    PidsCurrent   int64
}
```

### 2.2 内置插件配置

```go
// 内置 Docker 插件配置

var BuiltinContainerPlugins = map[string]*ContainerConfig{
    "claude-code": {
        Image: "hotplex/claude-code:latest",
        Resources: ContainerResources{
            CPU:    1.0,
            Memory: "1g",
            Pids:   64,
        },
        Capabilities: []string{
            "CAP_CHOWN",
            "CAP_DAC_OVERRIDE",
            "CAP_FOWNER",
            "CAP_SETUID",
        },
        SecurityOpt: []string{
            "no-new-privileges:true",
            "seccomp=runtime/default",
        },
        ReadOnlyRoot: true,
        WorkDir: "/workspace",
    },
    "open-code": {
        Image: "hotplex/open-code:latest",
        Resources: ContainerResources{
            CPU:    1.0,
            Memory: "1g",
            Pids:   64,
        },
        Capabilities: []string{
            "CAP_CHOWN",
            "CAP_DAC_OVERRIDE",
            "CAP_FOWNER",
            "CAP_SETUID",
        },
        SecurityOpt: []string{
            "no-new-privileges:true",
            "seccomp=runtime/default",
        },
        ReadOnlyRoot: true,
        WorkDir: "/workspace",
    },
}
```

---

## 三、ContainerRegistry 注册表

### 3.1 镜像注册表

```go
// pkg/plugin/registry.go

package plugin

import "context"

// ContainerImage 容器镜像
type ContainerImage struct {
    Name       string   // 镜像名
    Tag        string   // 镜像标签
    Digest     string   // 镜像 Digest
    SizeBytes  int64    // 镜像大小
    PullPolicy string   // "always" | "if-not-present" | "never"
}

// ContainerRegistry 插件镜像注册表
type ContainerRegistry interface {
    // 注册镜像
    Register(ctx context.Context, image *ContainerImage) error
    
    // 列出镜像
    List(ctx context.Context) ([]*ContainerImage, error)
    
    // 获取镜像
    Get(ctx context.Context, name string) (*ContainerImage, error)
    
    // 删除镜像
    Remove(ctx context.Context, name string) error
    
    // 拉取镜像
    Pull(ctx context.Context, name string) error
    
    // 推送镜像
    Push(ctx context.Context, name string) error
}

// FileRegistry 基于文件的注册表（开发环境）
type FileRegistry struct {
    path string  // 配置目录
}

// DockerHubRegistry Docker Hub 注册表
type DockerHubRegistry struct {
    namespace string  // "hotplex"
}

// HarborRegistry Harbor 私有仓库注册表
type HarborRegistry struct {
    URL      string
    Project  string
    Username string
    Password string
}
```

### 3.2 插件注册

```go
// 插件注册

type PluginRegistration struct {
    Kind      Kind              // 插件类型
    Name      string            // 插件名称
    Config    *ContainerConfig  // 容器配置
    Factory   PluginFactory     // 工厂函数（可选，用于配置传递）
}

// globalRegistry 全局插件注册表
var globalContainerRegistry = NewContainerRegistry()

// RegisterContainerPlugin 注册容器插件
func RegisterContainerPlugin(reg PluginRegistration) error {
    return globalContainerRegistry.Register(reg.Kind, reg.Name, reg.Config)
}

// 内置插件注册
func RegisterBuiltinContainerPlugins() {
    // Worker
    RegisterContainerPlugin(PluginRegistration{
        Kind:   KindWorker,
        Name:   "claude-code",
        Config: BuiltinContainerPlugins["claude-code"],
    })
    RegisterContainerPlugin(PluginRegistration{
        Kind:   KindWorker,
        Name:   "open-code",
        Config: BuiltinContainerPlugins["open-code"],
    })
}
```

---

## 四、插件生命周期管理

### 4.1 Manager

```go
// pkg/plugin/manager.go

package plugin

import (
    "context"
    "sync"
    "time"
)

// Manager 插件管理器
type Manager struct {
    loader    *DockerLoader
    containers map[string]*ContainerInstance
    mu         sync.RWMutex
    stopCh     chan struct{}
}

// ContainerInstance 容器实例
type ContainerInstance struct {
    ID       string
    Name     string
    Config   *ContainerConfig
    client   DockerClient
    plugin   ContainerPlugin
    startedAt time.Time
}

// NewManager 创建管理器
func NewManager(loader *DockerLoader) *Manager {
    return &Manager{
        loader:     loader,
        containers: make(map[string]*ContainerInstance),
        stopCh:     make(chan struct{}),
    }
}

// Load 加载插件
func (m *Manager) Load(ctx context.Context, kind Kind, name string, cfg map[string]interface{}) (ContainerPlugin, error) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    key := containerKey(kind, name)
    
    // 检查是否已加载
    if c, ok := m.containers[key]; ok {
        return c.plugin, nil
    }
    
    // 创建容器
    instance, err := m.loader.Load(ctx, kind, name, cfg)
    if err != nil {
        return nil, err
    }
    
    m.containers[key] = instance
    return instance.plugin, nil
}

// Start 启动插件
func (m *Manager) Start(ctx context.Context, kind Kind, name string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    key := containerKey(kind, name)
    c, ok := m.containers[key]
    if !ok {
        return ErrPluginNotLoaded
    }
    
    return c.plugin.Start(ctx)
}

// Stop 停止插件
func (m *Manager) Stop(ctx context.Context, kind Kind, name string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    key := containerKey(kind, name)
    c, ok := m.containers[key]
    if !ok {
        return nil
    }
    
    return c.plugin.Stop(ctx)
}

// StopAll 停止所有插件
func (m *Manager) StopAll(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for _, c := range m.containers {
        if err := c.plugin.Stop(ctx); err != nil {
            // 记录错误但继续
            slog.Error("failed to stop plugin", "name", c.Name, "error", err)
        }
    }
    
    return nil
}

// Unload 卸载插件
func (m *Manager) Unload(kind Kind, name string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    key := containerKey(kind, name)
    delete(m.containers, key)
    
    return nil
}

// containerKey 生成容器 key
func containerKey(kind Kind, name string) string {
    return string(kind) + "." + name
}
```

### 4.2 生命周期钩子

```go
// Lifecycle 生命周期接口
type Lifecycle interface {
    OnStart(ctx context.Context) error
    OnStop(ctx context.Context) error
    OnReload(cfg map[string]interface{}) error
}

// 容器启动流程
func (m *Manager) startContainer(ctx context.Context, c *ContainerInstance) error {
    // 1. 预检
    if err := m.preStartCheck(c); err != nil {
        return err
    }
    
    // 2. 拉取镜像（如果需要）
    if err := m.ensureImage(ctx, c.Config.Image); err != nil {
        return err
    }
    
    // 3. 创建容器
    containerID, err := m.createContainer(ctx, c)
    if err != nil {
        return err
    }
    c.ID = containerID
    
    // 4. 启动容器
    if err := m.client.StartContainer(ctx, containerID); err != nil {
        return err
    }
    
    // 5. 等待健康
    if err := m.waitForHealthy(ctx, c); err != nil {
        // 健康检查失败，停止容器
        m.client.StopContainer(ctx, containerID)
        return err
    }
    
    // 6. 调用 OnStart 钩子
    if l, ok := c.plugin.(Lifecycle); ok {
        if err := l.OnStart(ctx); err != nil {
            return err
        }
    }
    
    return nil
}

// preStartCheck 预检
func (m *Manager) preStartCheck(c *ContainerInstance) error {
    // 验证资源配置
    if c.Config.Resources.Memory != "" {
        // 验证内存格式
    }
    // ...
    return nil
}
```

---

## 五、网络通信协议

### 5.1 Unix Domain Socket 通信

```go
// 插件通信使用 Unix Domain Socket

const (
    DefaultSocketPath = "/var/run/hotplex/plugin.sock"
)

// PluginServer 插件服务端（运行在容器内）
type PluginServer struct {
    listener net.Listener
    handler  PluginHandler
}

// PluginHandler 请求处理
type PluginHandler interface {
    HandleMessage(ctx context.Context, msg *PluginMessage) (*PluginResponse, error)
}

// PluginMessage 插件消息
type PluginMessage struct {
    Type    string                 // "invoke" | "stream" | "control"
    Method  string                 // 方法名
    Params  map[string]interface{}  // 参数
    TraceID string                 // 追踪 ID
}

// PluginResponse 插件响应
type PluginResponse struct {
    Status  int                    // HTTP 状态码
    Data    map[string]interface{} // 响应数据
    Error   *PluginError           // 错误
    TraceID string                 // 追踪 ID
}

// StartServer 启动插件服务
func (s *PluginServer) StartServer(ctx context.Context, socketPath string) error {
    listener, err := net.Listen("unix", socketPath)
    if err != nil {
        return err
    }
    s.listener = listener
    
    // 设置权限（仅允许 owner 读写）
    os.Chmod(socketPath, 0600)
    
    return s.serve(ctx)
}

// serve 处理请求
func (s *PluginServer) serve(ctx context.Context) error {
    for {
        conn, err := s.listener.Accept()
        if err != nil {
            return err
        }
        
        go s.handleConn(ctx, conn)
    }
}
```

### 5.2 gRPC 协议

```go
// proto/plugin.proto

syntax = "proto3";

package hotplex.plugin;

option go_package = "github.com/huangzhonghui/hotplex/pkg/plugin/proto";

// PluginService 插件服务
service PluginService {
    // 同步调用
    rpc Invoke(InvokeRequest) returns (InvokeResponse);
    
    // 流式调用
    rpc Stream(StreamRequest) returns (stream StreamResponse);
    
    // 健康检查
    rpc Health(HealthRequest) returns (HealthResponse);
    
    // 指标
    rpc Stats(StatsRequest) returns (StatsResponse);
}

message InvokeRequest {
    string method = 1;
    map<string, string> params = 2;
    string trace_id = 3;
}

message InvokeResponse {
    int32 status = 1;
    bytes data = 2;
    string error = 3;
    string trace_id = 4;
}

message StreamRequest {
    string method = 1;
    map<string, string> params = 2;
    bool stream = 3;
}

message StreamResponse {
    bytes data = 1;
    bool done = 2;
}

message HealthRequest {}
message HealthResponse {
    string status = 1;
}

message StatsRequest {}
message StatsResponse {
    double cpu_percent = 1;
    uint64 memory_usage = 2;
}
```

---

## 六、安全策略

### 6.1 Docker 容器安全配置

```go
// 安全配置

// Worker 安全配置（严格）
var WorkerSecurityConfig = &ContainerConfig{
    // 网络：无
    NetworkMode: "none",
    
    // 资源限制
    Resources: ContainerResources{
        CPU:        1.0,
        Memory:     "512m",
        MemorySwap: "512m",  // 禁用 swap
        Pids:       64,
    },
    
    // Capabilities：仅保留最小集
    Capabilities: []string{
        "CAP_CHOWN",           // 文件所有权
        "CAP_DAC_OVERRIDE",    // 忽略 DAC 权限
        "CAP_FOWNER",          // 忽略文件所有者
        "CAP_SETUID",          // 设置用户 ID
    },
    
    // 安全选项
    SecurityOpt: []string{
        "no-new-privileges:true",        // 禁止提权
        "seccomp=runtime/default",        // 默认 seccomp
    },
    
    // 只读根文件系统
    ReadOnlyRoot: true,
    
    // 允许的写目录
    Mounts: []Mount{
        {Source: "hotplex-workspace", Target: "/workspace", Type: "volume", ReadOnly: false},
        {Source: "/tmp/hotplex", Target: "/tmp/hotplex", Type: "bind", ReadOnly: false},
    },
}

// Channel 安全配置（中等）
var ChannelSecurityConfig = &ContainerConfig{
    // 网络：仅允许必要出站
    NetworkMode: "bridge",
    
    // 资源限制
    Resources: ContainerResources{
        CPU:    0.5,
        Memory: "256m",
        Pids:   32,
    },
    
    // Capabilities
    Capabilities: []string{
        "CAP_NET_BIND_SERVICE",  // 绑定低端口
    },
    
    SecurityOpt: []string{
        "no-new-privileges:true",
    },
    
    ReadOnlyRoot: true,
}
```

### 6.2 Seccomp 白名单

```go
// seccomp_default.go

package seccomp

// DefaultSyscalls 默认允许的系统调用
var DefaultSyscalls = []string{
    // 文件操作
    "read", "write", "open", "close", "stat", "fstat",
    "lstat", "poll", "lseek", "mprotect", "mmap", "brk",
    "rt_sigaction", "rt_sigprocmask", "ioctl", "pread64",
    "pwrite64", "readv", "writev", "access", "pipe",
    
    // 进程
    "sched_yield", "madvise", "dup", "dup2", "pause",
    "nanosleep", "getitimer", "alarm", "setitimer",
    
    // 网络
    "socket", "connect", "bind", "listen", "accept",
    
    // 内存
    "shmget", "shmat", "shmctl",
}

// 文件系统只读白名单
var ReadOnlySyscalls = []string{
    "read", "stat", "lstat", "fstat",
}
```

### 6.3 能力限制

```go
// capabilities.go

// DefaultCapabilities 默认 capabilities
var DefaultCapabilities = map[string][]string{
    // Worker：最严格
    "worker": {
        Keep: []string{
            "CAP_CHOWN",
            "CAP_DAC_OVERRIDE",
            "CAP_FOWNER",
            "CAP_FSETID",
            "CAP_SETGID",
            "CAP_SETUID",
            "CAP_SETFCAP",
        },
        Drop: []string{
            "CAP_NET_RAW",      // 禁止 raw socket
            "CAP_NET_ADMIN",    // 禁止网络管理
            "CAP_SYS_ADMIN",    // 禁止系统管理
            "CAP_SYS_MODULE",   // 禁止加载模块
            "CAP_SYS_RAWIO",    // 禁止裸 I/O
            "CAP_SYS_PTRACE",   // 禁止调试
            "CAP_SYS_TIME",     // 禁止时间修改
            "CAP_SYS_BOOT",     // 禁止重启
            "CAP_LEASE",        // 禁止租约
            "CAP_SYSLOG",       // 禁止 syslog
            "CAP_SETPCAP",      // 禁止修改 capabilities
        },
    },
    
    // Channel：中等
    "channel": {
        Keep: []string{
            "CAP_NET_BIND_SERVICE",
        },
        Drop: []string{
            "CAP_NET_RAW",
            "CAP_NET_ADMIN",
            "CAP_SYS_ADMIN",
        },
    },
}

// GetCapabilities 获取 capabilities
func GetCapabilities(kind string) []string {
    caps := DefaultCapabilities[kind]
    if caps == nil {
        return []string{}  // 全部丢弃
    }
    
    var result []string
    for _, c := range caps.Keep {
        result = append(result, "--cap-add="+c)
    }
    for _, c := range caps.Drop {
        result = append(result, "--cap-drop="+c)
    }
    return result
}
```

---

## 七、资源限制

### 7.1 资源限制配置

```go
// resources.go

// ResourceLimits 资源限制
type ResourceLimits struct {
    // CPU
    CPU    float64  // 如 0.5 表示 0.5 个 CPU
    CPUPeriod int64  // CPU 周期（默认 100000）
    CPUQuota  int64  // CPU 配额（默认等于 period）
    
    // 内存
    Memory     string  // 如 "512m"
    MemorySwap string  // 如 "512m"（禁用 swap）
    
    // PIDs
    PidsLimit int64  // PID 上限
    
    // 磁盘
    DiskReadBps   int64  // 磁盘读速率
    DiskWriteBps  int64  // 磁盘写速率
    DiskReadIops  int64  // 磁盘读 IOPS
    DiskWriteIops int64  // 磁盘写 IOPS
    
    // 网络
    NetworkRxBps  int64  // 网络接收速率
    NetworkTxBps  int64  // 网络发送速率
}

// Validate 验证资源限制
func (r *ResourceLimits) Validate() error {
    if r.Memory != "" {
        if _, err := parseMemory(r.Memory); err != nil {
            return err
        }
    }
    // ...
    return nil
}

// ToDockerArgs 转换为 Docker 参数
func (r *ResourceLimits) ToDockerArgs() []string {
    var args []string
    
    if r.CPU > 0 {
        args = append(args, "--cpus="+strconv.FormatFloat(r.CPU, 'f', 2, 64))
    }
    
    if r.Memory != "" {
        args = append(args, "-m="+r.Memory)
    }
    
    if r.PidsLimit > 0 {
        args = append(args, "--pids-limit="+strconv.FormatInt(r.PidsLimit, 10))
    }
    
    return args
}
```

### 7.2 磁盘配额

```go
// disk_quota.go

// DiskQuota 磁盘配额
type DiskQuota struct {
    Size  string  // 总大小，如 "10g"
}

// Apply 应用磁盘配额（通过 docker volume）
func (d *DiskQuota) Apply(volumeName string) error {
    // 使用 devicemapper 或 vfs 存储驱动
    // 生产环境建议使用 overlay2 + 独立 volume
    return nil
}
```

---

## 八、镜像管理

### 8.1 镜像拉取策略

```go
// image.go

// PullPolicy 拉取策略
type PullPolicy string

const (
    PullAlways         PullPolicy = "always"          // 始终拉取
    PullIfNotPresent   PullPolicy = "if-not-present" // 不存在时拉取
    PullNever          PullPolicy = "never"          // 从不拉取
)

// ImageManager 镜像管理器
type ImageManager struct {
    client      DockerClient
    pullPolicy  PullPolicy
    registry    ContainerRegistry
}

// Pull 拉取镜像
func (m *ImageManager) Pull(ctx context.Context, image string) error {
    switch m.pullPolicy {
    case PullAlways:
        return m.client.PullImage(ctx, image)
    case PullIfNotPresent:
        if m.client.ImageExists(ctx, image) {
            return nil
        }
        return m.client.PullImage(ctx, image)
    case PullNever:
        if m.client.ImageExists(ctx, image) {
            return nil
        }
        return ErrImageNotFound
    }
    return nil
}
```

### 8.2 镜像预热

```go
// preload.go

// PreloadImages 预热镜像
func (m *Manager) PreloadImages(ctx context.Context) error {
    images := []string{
        "hotplex/claude-code:latest",
        "hotplex/open-code:latest",
    }
    
    for _, image := range images {
        slog.Info("preloading image", "image", image)
        if err := m.imageManager.Pull(ctx, image); err != nil {
            slog.Warn("failed to preload image", "image", image, "error", err)
            // 不阻塞，继续
        }
    }
    
    return nil
}
```

---

## 九、插件开发示例

### 9.1 钉钉 Channel 插件

```dockerfile
# plugins/channel-dingtalk/Dockerfile

FROM alpine:3.19

RUN apk add --no-cache \
    ca-certificates \
    tzdata

# 复制插件二进制
COPY dingtalk-channel /usr/local/bin/

# 复制 Unix Socket 通信库
COPY libhotplex-plugin.so /usr/local/lib/

ENV PLUGIN_KIND=channel
ENV PLUGIN_NAME=dingtalk
ENV PLUGIN_VERSION=1.0.0

ENTRYPOINT ["dingtalk-channel"]

# 健康检查
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1
```

```go
// plugins/channel-dingtalk/main.go

package main

import (
    "context"
    "log/slog"
    "net"
    "net/http"
    
    "github.com/huangzhonghui/hotplex/pkg/plugin"
)

type DingtalkChannel struct {
    plugin.Base
    appKey    string
    appSecret string
    server    *PluginServer
}

func NewDingtalkChannel(cfg map[string]interface{}) (*DingtalkChannel, error) {
    return &DingtalkChannel{
        Base: plugin.Base{
            Name:    "dingtalk",
            Version: "1.0.0",
            Kind:    plugin.KindChannel,
        },
        appKey:    cfg["app_key"].(string),
        appSecret: cfg["app_secret"].(string),
    }, nil
}

func (c *DingtalkChannel) Initialize(ctx context.Context, cfg map[string]interface{}) error {
    c.appKey = cfg["app_key"].(string)
    c.appSecret = cfg["app_secret"].(string)
    
    // 启动 Unix Socket 服务
    c.server = NewPluginServer("/var/run/hotplex/dingtalk.sock")
    c.server.RegisterHandler(c)
    
    return c.server.Start(ctx)
}

func (c *DingtalkChannel) Close() error {
    return c.server.Stop()
}

// PluginHandler 实现

func (c *DingtalkChannel) HandleMessage(ctx context.Context, msg *plugin.PluginMessage) (*plugin.PluginResponse, error) {
    switch msg.Method {
    case "send":
        return c.handleSend(ctx, msg)
    case "health":
        return c.handleHealth(ctx)
    default:
        return nil, &plugin.PluginError{Code: "UNKNOWN_METHOD", Message: msg.Method}
    }
}

//go:build plugin
func main() {}
```

### 9.2 编译和部署

```bash
# 编译插件镜像
docker build -t hotplex/channel-dingtalk:1.0.0 ./plugins/channel-dingtalk/

# 推送到私有仓库
docker push registry.example.com/hotplex/channel-dingtalk:1.0.0

# 注册到 HotPlex
hotplex plugin register \
    --kind channel \
    --name dingtalk \
    --image registry.example.com/hotplex/channel-dingtalk:1.0.0
```

---

## 十、故障排除

### 10.1 常见问题

| 问题 | 原因 | 解决方案 |
|------|------|----------|
| 容器启动失败 | 镜像不存在 | 检查 `docker images`，手动 `docker pull` |
| 健康检查失败 | 插件服务未就绪 | 增加 `--start-period`，检查日志 |
| 无法连接 Socket | 权限问题 | 检查 Socket 文件权限（0600）|
| 资源超限被 Kill | 内存限制过低 | 调高 Memory 限制 |
| 网络不通 | 网络模式错误 | 检查 `NetworkMode` 配置 |

### 10.2 调试命令

```bash
# 查看容器日志
docker logs hotplex-dingtalk-xxx

# 进入容器调试
docker exec -it hotplex-dingtalk-xxx /bin/sh

# 查看资源使用
docker stats hotplex-dingtalk-xxx

# 检查容器状态
docker inspect hotplex-dingtalk-xxx
```

---

*文档版本：v1.0-final | 最后更新：2026-03-29*
