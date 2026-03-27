# OpenCode Server Makefile 快速参考

## 🚀 快速开始

```bash
# 1. 首次设置
make opencode-password    # 生成密码
make opencode-verify      # 验证依赖
make opencode-start       # 启动服务

# 2. 检查状态
make opencode-status      # 健康检查
make opencode-config      # 查看配置

# 3. 查看日志
make opencode-logs        # 实时日志
```

## 📋 命令清单

### 基础管理

| 命令 | 说明 | 示例 |
|------|------|------|
| `opencode-config` | 显示配置 | `make opencode-config` |
| `opencode-verify` | 验证依赖 | `make opencode-verify` |
| `opencode-password` | 生成密码 | `make opencode-password` |
| `opencode-start` | 启动服务 | `make opencode-start` |
| `opencode-stop` | 停止服务 | `make opencode-stop` |
| `opencode-restart` | 重启服务 | `make opencode-restart` |
| `opencode-status` | 检查状态 | `make opencode-status` |
| `opencode-logs` | 查看日志 | `make opencode-logs` |

### 高级功能

| 命令 | 说明 | 示例 |
|------|------|------|
| `opencode-test` | 运行测试 | `make opencode-test` |
| `opencode-logs-truncate` | 轮转日志 | `make opencode-logs-truncate` |
| `opencode-with-hotplex` | 联动启动 | `make opencode-with-hotplex` |
| `opencode-docker-integrate` | Docker 集成 | `make opencode-docker-integrate` |

## 🔧 环境变量

```bash
# 基础配置
OPENCODE_PORT=4096                    # 服务端口
OPENCODE_BINARY=opencode              # 二进制文件名
OPENCODE_DEBUG=false                  # Debug 模式

# HotPlex 集成
HOTPLEX_OPEN_CODE_SERVER_URL=http://127.0.0.1:4096
HOTPLEX_OPEN_CODE_PASSWORD=your-password
HOTPLEX_OPEN_CODE_PORT=4096
```

## 📁 文件位置

| 文件 | 路径 | 说明 |
|------|------|------|
| 日志文件 | `~/.hotplex/logs/opencode-server.log` | 服务日志 |
| PID 文件 | `~/.hotplex/.opencode-server.pid` | 进程 ID |
| 密码文件 | `~/.hotplex/.opencode-password` | Basic Auth 密码 |
| 配置文件 | `configs/chatapps/slack.yaml` | HotPlex 配置 |
| 环境变量 | `.env` | 环境变量 |

## 🎯 常见任务

### 任务 1：首次配置

```bash
# 生成密码
make opencode-password

# 复制显示的密码，添加到 .env
echo "HOTPLEX_OPEN_CODE_PASSWORD=<password>" >> .env

# 启动服务
make opencode-start

# 验证
make opencode-status
```

### 任务 2：日常开发

```bash
# 启动 OpenCode + HotPlex
make opencode-with-hotplex

# 或分开启动
make opencode-start
make run

# 查看日志
make opencode-logs
```

### 任务 3：故障排查

```bash
# 检查配置
make opencode-config

# 验证依赖
make opencode-verify

# 检查状态
make opencode-status

# 查看日志
make opencode-logs

# 运行测试
make opencode-test
```

### 任务 4：Docker 部署

```bash
# 查看集成指南
make opencode-docker-integrate

# 手动添加配置到 docker-compose.yml
# 启动服务
make docker-up
```

### 任务 5：日志管理

```bash
# 查看实时日志
make opencode-logs

# 轮转日志（保留最近 1000 行）
make opencode-logs-truncate

# 查看日志大小
du -h ~/.hotplex/logs/opencode-server.log
```

## 🔍 故障排查

### 问题：端口被占用

```bash
# 检查端口占用
lsof -i :4096

# 使用不同端口
OPENCODE_PORT=4097 make opencode-start
```

### 问题：密码认证失败

```bash
# 检查密码文件
cat ~/.hotplex/.opencode-password

# 重新生成密码
make opencode-password

# 更新 .env
# 手动编辑 .env，更新 HOTPLEX_OPEN_CODE_PASSWORD
```

### 问题：服务无法启动

```bash
# 验证依赖
make opencode-verify

# 查看日志
cat ~/.hotplex/logs/opencode-server.log

# 检查二进制
which opencode
opencode --version
```

### 问题：HotPlex 连接失败

```bash
# 检查 OpenCode 状态
make opencode-status

# 检查配置
make opencode-config

# 测试连接
curl -u admin:$(cat ~/.hotplex/.opencode-password) \
  http://127.0.0.1:4096/
```

## 📚 相关文档

- [快速入门指南](./opencode-server-quickstart.md)
- [配置规范](./opencode-server-provider-spec.md)
- [优化总结](./opencode-server-makefile-optimization.md)
- [配置示例](../../configs/examples/opencode-server-provider.yaml)

## 🆘 获取帮助

```bash
# 查看所有 OpenCode 命令
make help | grep -A 20 "OpenCode Server"

# 查看特定命令的帮助
make opencode-config
make opencode-verify
```
