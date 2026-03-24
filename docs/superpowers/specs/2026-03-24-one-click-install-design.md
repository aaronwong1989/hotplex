# HotPlex 一键安装脚本设计

**版本**: v1.0
**日期**: 2026-03-24
**状态**: 已批准
**参考**: [alibaba/hiclaw install/README.md](https://github.com/alibaba/hiclaw/blob/main/install/README.md)

---

## 1. 概述

### 1.1 目标

提供一键安装脚本，让用户通过单行命令完成 HotPlex 的安装、配置和部署。

### 1.2 核心需求

- **一键体验**: `curl | bash` 直接运行（推荐两步安装：先下载脚本 → 校验 → 执行）
- **双模式支持**: Quick Start（二进制） + Advanced（Docker）
- **全生命周期**: install / uninstall / purge / upgrade / status
- **混合交互**: 默认交互式，支持环境变量非交互模式
- **跨平台**: macOS / Linux（Phase 1），Windows PowerShell（Phase 2）

### 1.3 安全要求（P0 级别）

- **SHA256 校验**: 二进制和脚本必须校验完整性
- **Token 验证**: 调用 Slack `auth.test` API 验证 Token 有效性
- **原子操作**: 使用 `mktemp` + `mv` 避免安装中断导致损坏
- **日志权限**: `chmod 600` 保护敏感信息
- **开机自启**: 自动配置 systemd/launchd

### 1.3 用户故事

| 用户类型 | 需求 | 成功标准 |
|----------|------|----------|
| 新手用户 | 快速体验 HotPlex | 5 分钟内完成安装并运行 |
| 运维人员 | 批量部署多实例 | 脚本可集成 CI/CD |
| 开发者 | 升级/回滚 | 一键升级，保留配置 |

---

## 2. 架构设计

### 2.1 文件结构

```
install/
├── hotplex-install.sh        # 主安装脚本 (macOS/Linux)
├── hotplex-install.ps1       # Windows PowerShell 版本 (Phase 2)
└── README.md                 # 安装文档

# 运行时目录 (~/.hotplex/)
~/.hotplex/
├── bin/hotplexd              # 二进制文件
├── configs/                  # 配置目录
│   ├── base/slack.yaml       # 基础模板
│   └── admin/slack.yaml      # Admin Bot 配置
├── .env                      # 环境变量（从模板生成）
├── install.log               # 安装日志 (chmod 600)
└── systemd/                  # 开机自启配置 (systemd/launchd)
```

### 2.2 脚本模块划分

```bash
hotplex-install.sh
├── 全局变量定义（VERSION、REPO、BIN_DIR）
├── 颜色与日志函数（info、error、success）
├── 系统检测（detect_os、detect_arch、check_dependencies）
├── 安装函数
│   ├── install_binary()      # Quick Start 模式
│   ├── install_docker()      # Advanced 模式
│   └── generate_config()     # 配置生成
├── 卸载函数 (uninstall)
├── 升级函数 (upgrade)
├── 状态检查 (status_check)
└── 主逻辑 (main)
```

### 2.3 依赖清单

**Quick Start（二进制模式）**:
- `curl` / `wget`: 下载二进制
- `jq`: 解析 JSON（版本检查）
- `unzip`: 解压归档（如需要）

**Advanced（Docker 模式）**:
- `docker`: 运行容器
- `docker-compose` 或 `docker compose`: 编排服务

---

## 3. 用户流程

### 3.1 Quick Start 流程（二进制）

```bash
# 1. 用户运行
curl -fsSL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash

# 2. 脚本交互
╭─ HotPlex Quick Start ─────────────────────────╮
│ Select installation mode:                     │
│   [1] Quick Start (Binary)   ← 默认         │
│   [2] Advanced (Docker)                       │
│   [3] Uninstall                               │
╰───────────────────────────────────────────────╯

# 3. 配置收集（交互式）
Enter Slack Bot Token: xoxb-xxx...
Enter Slack App Token: xapp-xxx...
Enter Slack Bot User ID: B12345
Port [8080]: 8080

# 4. 执行安装
✓ Detecting system (macOS arm64)
✓ Downloading hotplexd v0.35.4
✓ Installing to ~/.hotplex/bin/
✓ Generating configuration
✓ Starting daemon
✓ Running health check

╭─ Installation Complete ───────────────────────╮
│ Binary: ~/.hotplex/bin/hotplexd              │
│ Config: ~/.hotplex/.env                      │
│ PID: 12345                                   │
│ Health: http://localhost:8080/health         │
╰───────────────────────────────────────────────╯
```

### 3.2 Advanced 流程（Docker）

```bash
# 选择 [2] Advanced
✓ Detecting Docker
✓ Generating docker-compose.yml
✓ Creating .env-01 (Bot 1)
✓ Starting containers

# 支持多 Bot
Add another bot? [y/N]: y
Bot ID [bot-02]: bot-02
# ... 重复配置 ...
```

### 3.3 非交互模式（CI/CD）

```bash
# 环境变量驱动
export HOTPLEX_NON_INTERACTIVE=1
export HOTPLEX_MODE=binary
export HOTPLEX_SLACK_BOT_TOKEN=xoxb-xxx
export HOTPLEX_SLACK_APP_TOKEN=xapp-xxx
export HOTPLEX_SLACK_BOT_USER_ID=B12345

curl -fsSL ... | bash
```

### 3.4 关键环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `HOTPLEX_NON_INTERACTIVE` | 跳过交互 | `0` |
| `HOTPLEX_MODE` | 安装模式 | `binary` |
| `HOTPLEX_SLACK_BOT_TOKEN` | Slack Bot Token | 必填 |
| `HOTPLEX_SLACK_APP_TOKEN` | Slack App Token | 必填 |
| `HOTPLEX_SLACK_BOT_USER_ID` | Bot User ID | 必填 |
| `HOTPLEX_PORT` | 服务端口 | `8080` |
| `HOTPLEX_DATA_DIR` | 数据目录 | `~/.hotplex` |

---

## 4. 错误处理与恢复

### 4.1 错误分类

**1. 前置检查失败（退出 + 指引）**
```bash
✗ Docker not found
  → Install Docker: https://docs.docker.com/get-docker/

✗ Slack Bot Token invalid format
  → Expected: xoxb-xxx...

✗ Slack Token validation failed
  → Token is invalid or expired. Check your Slack App settings.
  → Run `auth.test` API to verify: https://api.slack.com/methods/auth.test/test
```

**2. 下载失败（重试 3 次）**
```bash
! Download failed (1/3) - Retrying...
! Download failed (2/3) - Retrying...
✓ Download complete (3rd attempt)
```

**3. 配置冲突（询问覆盖）**
```bash
⚠ Configuration already exists at ~/.hotplex/.env
  [1] Overwrite and restart
  [2] Keep existing (skip)
  [3] Backup and overwrite
```

**4. 服务启动失败（回滚）**
```bash
✗ Daemon failed to start
  → Rolling back installation...
  → Binary restored from temporary location
  → Config preserved

  Debug log: ~/.hotplex/install.log (chmod 600)
```

**5. 端口冲突（自动解决）**
```bash
⚠ Port 8080 is already in use
  [1] Kill existing process (PID: 12345)
  [2] Use alternative port (8081)
  [3] Cancel installation
```

### 4.2 日志输出规范

```bash
# 标准输出（用户可见）
[INFO]  Downloading hotplexd v0.35.4...
[OK]    Installation complete
[WARN]  Port 8080 already in use
[ERROR] Docker daemon not running

# 详细日志（文件，chmod 600）
~/.hotplex/install.log
2026-03-24T10:00:00Z [DEBUG] OS: darwin arm64
2026-03-24T10:00:01Z [DEBUG] Download URL: https://github.com/hrygo/hotplex/releases/download/v0.35.4/hotplexd-darwin-arm64
2026-03-24T10:00:05Z [DEBUG] SHA256 verified: abc123...

# 敏感信息打码（日志中）
[DEBUG] Slack Bot Token: xoxb-1234...****
[DEBUG] Slack App Token: xapp-5678...****
```

### 4.3 健康检查

```bash
health_check() {
    local timeout=15
    local start_time=$(date +%s)

    # 1. 二进制可执行
    if ! ~/.hotplex/bin/hotplexd version > /dev/null 2>&1; then
        echo "[ERROR] Binary execution failed"
        return 1
    fi

    # 2. 服务可访问（带超时）
    while true; do
        if curl -sf http://localhost:${PORT}/health | grep -q "ok"; then
            break
        fi

        # 超时检查
        local current_time=$(date +%s)
        if (( current_time - start_time > timeout )); then
            echo "[ERROR] Health check timeout (${timeout}s)"
            return 1
        fi
        sleep 1
    done

    # 3. 进程存活
    if ! pgrep -f hotplexd > /dev/null; then
        echo "[ERROR] Process not found"
        return 1
    fi

    return 0
}
```

---

## 5. 子命令设计

```bash
# 安装（默认）
./hotplex-install.sh              # 交互式
./hotplex-install.sh install      # 显式 install

# 模式选择
./hotplex-install.sh --mode=binary    # Quick Start
./hotplex-install.sh --mode=docker    # Advanced

# 卸载
./hotplex-install.sh uninstall

# 升级
./hotplex-install.sh upgrade

# 状态检查
./hotplex-install.sh status
```

---

## 6. 实现优先级

### Phase 1: 核心功能（P0 安全基线）
- [ ] 系统检测（OS/Arch）
- [ ] **SHA256 校验（安全 P0）**
- [ ] **原子安装操作（安全 P0）**
- [ ] **Token 验证 - Slack auth.test（安全 P0）**
- [ ] **日志权限 chmod 600（安全 P0）**
- [ ] 二进制下载与安装
- [ ] 基础配置生成
- [ ] 健康检查（带超时）
- [ ] **systemd/launchd 开机自启（P0）**

### Phase 2: 交互体验
- [ ] 交互式菜单
- [ ] 环境变量支持
- [ ] 错误重试机制
- [ ] 端口冲突自动解决
- [ ] 敏感信息打码

### Phase 3: 全生命周期
- [ ] 卸载功能（uninstall - 保留配置）
- [ ] 清理功能（purge - 完全清理）
- [ ] 升级功能
- [ ] 状态检查

### Phase 4: 高级功能
- [ ] Docker 模式
- [ ] 多 Bot 支持
- [ ] 日志轮转
- [ ] Windows PowerShell 支持

---

## 7. 验收标准

### 功能验收
- [ ] 一行命令完成安装（Quick Start）
- [ ] 支持非交互模式（CI/CD）
- [ ] 健康检查通过率 100%
- [ ] 卸载后无残留文件（purge 模式）
- [ ] 升级保留用户配置
- [ ] 错误信息清晰可执行

### 安全验收（P0）
- [ ] SHA256 校验失败时阻止安装
- [ ] Token 无效时明确提示并拒绝
- [ ] 日志文件权限为 600
- [ ] 安装中断时无损坏残留（原子操作）
- [ ] 敏感信息在日志中打码

### 体验验收
- [ ] 新手 5 分钟内完成首次安装
- [ ] 错误提示包含可执行建议
- [ ] 开机自动重启服务

---

## 8. 安全实现细节

### 8.1 SHA256 校验流程

```bash
verify_binary() {
    local version=$1
    local arch=$2
    local os=$3

    # 1. 下载 SHA256SUMS 文件
    local checksum_url="https://github.com/hrygo/hotplex/releases/download/v${version}/SHA256SUMS"
    curl -fsSL "$checksum_url" -o /tmp/SHA256SUMS

    # 2. 下载二进制文件到临时位置
    local binary_url="https://github.com/hrygo/hotplex/releases/download/v${version}/hotplexd-${os}-${arch}"
    local temp_binary=$(mktemp /tmp/hotplexd.XXXXXX)
    curl -fsSL "$binary_url" -o "$temp_binary"

    # 3. 验证 SHA256
    local expected_hash=$(grep "hotplexd-${os}-${arch}" /tmp/SHA256SUMS | awk '{print $1}')
    local actual_hash=$(sha256sum "$temp_binary" | awk '{print $1}')

    if [ "$expected_hash" != "$actual_hash" ]; then
        echo "[ERROR] SHA256 mismatch!"
        echo "  Expected: $expected_hash"
        echo "  Actual:   $actual_hash"
        rm -f "$temp_binary" /tmp/SHA256SUMS
        return 1
    fi

    # 4. 校验成功，移动到目标位置
    chmod +x "$temp_binary"
    mv "$temp_binary" "$BIN_DIR/hotplexd"
    rm -f /tmp/SHA256SUMS

    echo "[OK] SHA256 verified"
    return 0
}
```

### 8.2 原子安装操作

```bash
install_binary_atomic() {
    # 1. 下载到临时文件
    local temp_binary=$(mktemp /tmp/hotplexd.XXXXXX)
    local temp_env=$(mktemp /tmp/hotplex-env.XXXXXX)

    # 2. 执行安装（失败则清理）
    if ! download_and_verify "$temp_binary"; then
        rm -f "$temp_binary"
        echo "[ERROR] Installation failed, cleaned up"
        return 1
    fi

    # 3. 生成配置到临时文件
    if ! generate_config "$temp_env"; then
        rm -f "$temp_binary" "$temp_env"
        echo "[ERROR] Config generation failed, cleaned up"
        return 1
    fi

    # 4. 原子移动（成功则覆盖）
    mv "$temp_binary" "$BIN_DIR/hotplexd" || {
        rm -f "$temp_binary" "$temp_env"
        echo "[ERROR] Failed to move binary"
        return 1
    }
    mv "$temp_env" "$DATA_DIR/.env" || {
        # 回滚二进制
        rm -f "$BIN_DIR/hotplexd"
        rm -f "$temp_env"
        echo "[ERROR] Failed to move config, rolled back"
        return 1
    }

    echo "[OK] Atomic installation complete"
    return 0
}
```

### 8.3 Token 验证

```bash
validate_slack_token() {
    local token=$1

    # 1. 格式检查
    if [[ ! "$token" =~ ^xoxb-[0-9]+- ]]; then
        echo "[ERROR] Invalid Slack Bot Token format"
        echo "  Expected: xoxb-XXXXXXXXXXXX-XXXXXXXXXXXX-..."
        return 1
    fi

    # 2. API 验证（调用 auth.test）
    local response=$(curl -sf -X POST "https://slack.com/api/auth.test" \
        -H "Authorization: Bearer $token" \
        -H "Content-Type: application/json")

    if [ $? -ne 0 ]; then
        echo "[ERROR] Slack API unreachable"
        return 1
    fi

    local valid=$(echo "$response" | jq -r '.ok')
    if [ "$valid" != "true" ]; then
        local error=$(echo "$response" | jq -r '.error')
        echo "[ERROR] Token validation failed: $error"
        return 1
    fi

    # 3. 提取并显示用户信息
    local user=$(echo "$response" | jq -r '.user')
    local team=$(echo "$response" | jq -r '.team')
    echo "[OK] Token validated for user '$user' on team '$team'"
    return 0
}
```

### 8.4 日志权限控制

```bash
setup_secure_logging() {
    local log_file="$DATA_DIR/install.log"

    # 创建日志文件
    touch "$log_file"

    # 设置权限（仅所有者可读写）
    chmod 600 "$log_file"

    # 验证权限
    local perms=$(stat -c %a "$log_file" 2>/dev/null || stat -f %Lp "$log_file")
    if [ "$perms" != "600" ]; then
        echo "[WARN] Failed to set log permissions (got: $perms)"
    else
        echo "[OK] Log file secured (chmod 600)"
    fi
}
```

---

## 9. 开机自启配置

### 9.1 systemd（Linux）

```bash
install_systemd() {
    cat > /etc/systemd/system/hotplexd.service <<EOF
[Unit]
Description=HotPlex Daemon
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=$BIN_DIR/hotplexd start
Restart=on-failure
RestartSec=10
Environment=HOTPLEX_DATA_DIR=$DATA_DIR

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    systemctl enable hotplexd
    systemctl start hotplexd

    echo "[OK] systemd service installed and started"
}
```

### 9.2 launchd（macOS）

```bash
install_launchd() {
    local plist_file="$HOME/Library/LaunchAgents/com.hotplex.daemon.plist"

    cat > "$plist_file" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.hotplex.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>$BIN_DIR/hotplexd</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <dict>
        <key>Crashed</key>
        <false/>
        <key>SuccessfulExit</key>
        <false/>
    </dict>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOTPLEX_DATA_DIR</key>
        <string>$DATA_DIR</string>
    </dict>
</dict>
</plist>
EOF

    launchctl load "$plist_file"
    echo "[OK] launchd agent installed and started"
}
```

---

## 10. 参考实现

- [alibaba/hiclaw install/README.md](https://github.com/alibaba/hiclaw/blob/main/install/README.md)
- [Docker install script](https://get.docker.com/)
- [Homebrew install.sh](https://brew.sh/)
