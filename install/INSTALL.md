# HotPlex Agent 安装指南

本文档面向 AI Agent 工具，用于自动执行 HotPlex 系统安装。文档包含完整的安装流程、参数说明和错误处理。

## 目录

- [前置条件](#前置条件)
- [快速安装](#快速安装)
- [安装模式](#安装模式)
- [参数详解](#参数详解)
- [配置说明](#配置说明)
- [安装后验证](#安装后验证)
- [卸载方法](#卸载方法)
- [故障排除](#故障排除)

---

## 前置条件

### 系统要求

| 平台 | 架构 | 支持状态 |
|------|------|---------|
| Linux | amd64, arm64 | ✅ 支持 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) | ✅ 支持 |
| Windows | WSL2 | ✅ 支持 |

### 必需依赖

**二进制模式**:
- `curl` 或 `wget`
- `tar`
- `jq` (用于 JSON 解析)
- `sha256sum` / `shasum` (用于校验)

**Docker 模式**:
- `docker` (版本 ≥ 20.10)
- `docker compose` 或 `docker-compose`

### 权限要求

- 安装到系统目录 (`/usr/local/bin`): 需要 `sudo` 权限
- 安装到用户目录 (`~/.local/bin`): 普通用户权限
- 配置 systemd 服务: 需要 `sudo` 权限

---

## 快速安装

### 标准安装 (Linux/macOS)

```bash
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash
```

### 指定版本

```bash
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- -v v0.35.0
```

### Docker 模式安装

```bash
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- docker
```

### 非交互式安装 (Agent 专用)

```bash
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- \
  --non-interactive \
  --slack-bot-token "xoxb-xxx" \
  --slack-app-token "xapp-xxx" \
  --slack-bot-user-id "U123456" \
  --github-token "ghp_xxx"
```

---

## 安装模式

### 1. Binary 模式 (默认)

直接下载并安装 hotplexd 二进制文件到本地系统。

```bash
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- --mode binary
```

### 2. Docker 模式

使用 Docker 容器运行 HotPlex。

```bash
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- --mode docker
```

或使用子命令:

```bash
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- docker
```

Docker 模式会生成:
- `docker-compose.yml` - 容器编排配置
- `.env` - 环境变量配置
- `data/` - 数据目录
- `projects/` - 项目目录
- `logs/` - 日志目录

管理命令:

```bash
cd ~/.hotplex
docker compose up -d      # 启动
docker compose logs -f    # 查看日志
docker compose down       # 停止
docker compose restart    # 重启
```

### 3. 配置仅生成模式

仅生成配置文件，不执行实际安装。

```bash
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- --config-only
```

---

## 参数详解

### 安装选项

| 参数 | 说明 | 默认值 | 必填 |
|------|------|--------|------|
| `-v, --version <ver>` | 指定安装版本 | latest | 否 |
| `-d, --dir <path>` | 安装目录 | `/usr/local/bin` | 否 |
| `--mode <mode>` | 安装模式: binary/docker | `binary` | 否 |
| `-c, --config-only` | 仅生成配置 | false | 否 |
| `-f, --force` | 强制覆盖安装 | false | 否 |
| `-n, --dry-run` | 预览模式 (不执行) | false | 否 |
| `-q, --quiet` | 静默模式 | false | 否 |
| `-V, --verbose` | 详细输出 | false | 否 |
| `-h, --help` | 显示帮助 | - | 否 |

### 跳过选项

| 参数 | 说明 |
|------|------|
| `--skip-health-check` | 跳过安装后健康检查 |
| `--skip-autostart` | 跳过 systemd/launchd 配置 |
| `--skip-verify` | 跳过 SHA256 校验 |
| `--skip-wizard` | 跳过安装向导 |

### Slack 配置参数 (非交互式安装必填)

| 参数 | 环境变量 | 说明 | 示例 |
|------|----------|------|------|
| `--slack-bot-token` | `HOTPLEX_SLACK_BOT_TOKEN` | Slack Bot Token | `xoxb-xxx` |
| `--slack-app-token` | `HOTPLEX_SLACK_APP_TOKEN` | Slack App Token | `xapp-xxx` |
| `--slack-bot-user-id` | `HOTPLEX_SLACK_BOT_USER_ID` | Bot User ID | `U123456` |
| `--slack-primary-owner` | `HOTPLEX_SLACK_PRIMARY_OWNER` | 主管理员 User ID | `U123456` |

### 其他配置参数

| 参数 | 环境变量 | 说明 | 默认值 |
|------|----------|------|--------|
| `--github-token` | `GITHUB_TOKEN` | GitHub Token | - |
| `--port` | `HOTPLEX_PORT` | 服务端口 | `8080` |
| `--admin-port` | `HOTPLEX_ADMIN_PORT` | 管理端口 | `9080` |
| `--data-dir` | `HOTPLEX_DATA_DIR` | 数据目录 | `~/.hotplex` |

---

## 配置说明

### 配置文件位置

HotPlex 按以下顺序查找配置文件:

1. `--env-file` 参数指定路径
2. `HOTPLEX_ENV_FILE` 环境变量
3. 当前目录 `.env`
4. `~/.config/hotplex/.env` (XDG 标准路径)
5. `~/.hotplex/.env`

### 必需配置项

安装后需在配置文件中设置以下项:

```bash
# API 安全密钥 (必填，生产环境)
HOTPLEX_API_KEY=your-secure-api-key-min-32-chars

# Slack Bot 配置 (必填)
HOTPLEX_SLACK_PRIMARY_OWNER=UXXXXXXXXXX    # 主管理员的 Slack User ID
HOTPLEX_SLACK_BOT_USER_ID=UXXXXXXXXXX       # Bot 的 User ID
HOTPLEX_SLACK_BOT_TOKEN=xoxb-your-bot-token # Bot Token
HOTPLEX_SLACK_APP_TOKEN=xapp-your-app-token # App Level Token

# GitHub Token (用于 Git 操作，可选)
GITHUB_TOKEN=ghp_your-token

# 服务端口 (可选)
HOTPLEX_PORT=8080
HOTPLEX_ADMIN_PORT=9080
```

### 获取 Slack 配置的方法

1. **Bot Token**: Slack App 设置 → OAuth & Permissions → Bot User OAuth Token
2. **App Token**: Slack App 设置 → Basic Information → App-Level Tokens
3. **Bot User ID**: 在 Slack 中 @ 机器人 → 查看用户档案 → ID
4. **Primary Owner**: 主管理员的 Slack User ID (格式: U开头的字符串)

---

## 安装后验证

### 1. 检查安装

```bash
# 检查二进制是否存在
which hotplexd
hotplexd version

# 检查服务状态
hotplexd status
```

### 2. 健康检查

```bash
# HTTP 健康检查
curl http://localhost:8080/health

# 完整诊断
hotplexd doctor
```

### 3. 启动服务

```bash
# 使用默认配置启动
hotplexd start

# 使用指定配置启动
hotplexd start --env-file ~/.config/hotplex/.env
```

### 4. 验证日志

```bash
# 查看安装日志
cat ~/.hotplex/install.log

# 查看服务日志 (journald)
journalctl -u hotplexd -f
```

---

## 多 Bot 支持

HotPlex 支持在一台服务器上运行多个独立的 Bot 实例，适合复杂组织结构。

### 交互式配置多 Bot

```bash
# 在安装菜单中选择 "8) 多 Bot 配置"
./hotplex-install.sh
```

### Docker 模式多 Bot

```bash
# 安装时启用多 Bot
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- --mode docker

# 在安装菜单中选择 "8) 多 Bot 配置" 添加更多 Bot
```

### 多 Bot 配置文件

每个 Bot 有独立的配置文件:

```
~/.hotplex/
├── .env                 # 主配置 (可选)
├── bot-01.env          # Bot 1 配置
├── bot-02.env          # Bot 2 配置
├── bot-03.env          # Bot 3 配置
└── docker-compose.yml  # Docker 编排 (多 Bot 时)
```

### Bot 配置结构

```bash
# bot-01.env 示例
BOT_NAME=bot-01
HOTPLEX_PORT=8080
HOTPLEX_SLACK_BOT_TOKEN=xoxb-bot1-token
HOTPLEX_SLACK_BOT_USER_ID=U001
HOTPLEX_MESSAGE_STORE_SQLITE_PATH=/app/data/chatapp_messages_01.db
```

---

## 日志管理

### 日志位置

| 类型 | 路径 |
|------|------|
| 安装日志 | `~/.hotplex/install.log` |
| 服务日志 | `~/.hotplex/hotplexd.log` |
| Docker 日志 | `~/.hotplex/logs/` |

### 配置日志轮转

```bash
# 交互式配置
./hotplex-install.sh
# 选择 "9) 日志管理" → "1) 配置日志轮转"
```

默认规则:
- 每天轮转
- 保留 5 份
- 大小超过 10MB 触发轮转
- 自动压缩旧日志

### 手动清理日志

```bash
# 在安装菜单中选择 "9) 日志管理" → "2) 手动清理日志"
./hotplex-install.sh
```

或手动清理:

```bash
# 清空主日志
: > ~/.hotplex/hotplexd.log

# 删除所有日志文件
find ~/.hotplex/logs -name "*.log" -delete
```

---

## 卸载方法

### 自动卸载

```bash
# 使用安装脚本卸载
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- --uninstall
```

### 手动卸载

```bash
# 停止服务
hotplexd stop

# 删除二进制
sudo rm /usr/local/bin/hotplexd

# 删除配置 (可选)
rm -rf ~/.config/hotplex
rm -rf ~/.hotplex

# 删除 systemd 服务 (Linux)
sudo rm /etc/systemd/system/hotplexd.service
sudo systemctl daemon-reload

# 删除 launchd 服务 (macOS)
launchctl unload ~/Library/LaunchAgents/com.hotplexd.plist
rm ~/Library/LaunchAgents/com.hotplexd.plist
```

---

## 故障排除

### 常见错误

#### 1. "curl: command not found"

```bash
# Debian/Ubuntu
sudo apt-get install curl

# macOS
brew install curl
```

#### 2. "sha256sum: command not found"

```bash
# Debian/Ubuntu
sudo apt-get install coreutils

# macOS (shasum 支持 -a 256)
shasum -a 256 hotplexd
```

#### 3. "jq: command not found"

```bash
# Debian/Ubuntu
sudo apt-get install jq

# macOS
brew install jq
```

#### 4. "Permission denied" 安装到 /usr/local/bin

```bash
# 使用 sudo
curl ... | sudo bash

# 或安装到用户目录
curl ... | bash -s -- -d ~/.local/bin
```

#### 5. "Slack Token validation failed"

- 检查 Token 格式是否正确 (xoxb- 开头)
- 确认 Token 未过期或被撤销
- 验证 Slack App 已启用必要的 OAuth Scopes
- 使用 Slack API 测试: `curl -H "Authorization: Bearer <token>" https://slack.com/api/auth.test`

#### 6. "Port already in use"

```bash
# 查看端口占用
lsof -i :8080

# 使用其他端口
curl ... | bash -s -- --port 8081
```

### 调试模式

```bash
# 启用详细输出
curl ... | bash -s -- -V

# 干运行模式 (仅预览，不执行)
curl ... | bash -s -- -n
```

### 获取诊断信息

```bash
# 运行诊断命令
hotplexd doctor

# 收集日志
tar -czf hotplex-debug.tar.gz ~/.hotplex/*.log
```

---

## Agent 执行检查清单

使用本文档执行安装时，Agent 应按以下顺序检查:

### 安装前

- [ ] 检测操作系统和架构
- [ ] 检查必需命令 (curl, tar, jq, sha256sum)
- [ ] 检查权限 (sudo/普通用户)
- [ ] 验证网络连接 (curl -I https://github.com)

### 安装中

- [ ] 下载安装脚本
- [ ] 校验脚本 SHA256 (可选)
- [ ] 执行安装脚本
- [ ] 收集用户提供的配置值
- [ ] 生成配置文件

### 安装后

- [ ] 验证二进制安装: `which hotplexd && hotplexd version`
- [ ] 验证配置生成: `ls -la ~/.config/hotplex/`
- [ ] 检查端口可用: `lsof -i :8080`
- [ ] 执行健康检查: `curl http://localhost:8080/health`
- [ ] 配置开机自启 (systemd/launchd)

---

## 参考链接

- [设计文档](https://github.com/hrygo/hotplex/blob/main/docs/superpowers/specs/2026-03-24-one-click-install-design.md)
- [Releases 页面](https://github.com/hrygo/hotplex/releases)
- [Slack API 文档](https://api.slack.com/)
- [Docker 安装指南](https://docs.docker.com/get-docker/)
