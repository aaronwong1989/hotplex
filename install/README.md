# HotPlex 安装脚本

## 快速开始

```bash
# 交互式安装 (推荐)
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash

# 指定版本
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash -s -- -v v0.35.0

# 非交互模式 (CI/CD)
export HOTPLEX_NON_INTERACTIVE=1
export HOTPLEX_SLACK_BOT_TOKEN=xoxb-xxx
export HOTPLEX_SLACK_APP_TOKEN=xapp-xxx
export HOTPLEX_SLACK_BOT_USER_ID=UXXXXXXXX
curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install/hotplex-install.sh | bash
```

## 子命令

| 命令 | 说明 |
|------|------|
| `install` | 安装 (默认) |
| `uninstall` | 卸载 |
| `upgrade` | 升级 |
| `status` | 状态检查 |

## 选项

| 选项 | 说明 |
|------|------|
| `-v, --version` | 指定版本 (默认: 最新) |
| `-d, --dir` | 安装目录 (默认: /usr/local/bin) |
| `-p, --port` | 服务端口 (默认: 8080) |
| `-n, --non-interactive` | 非交互模式 |
| `-f, --force` | 强制重新安装 |
| `--skip-health-check` | 跳过健康检查 |
| `--skip-autostart` | 跳过开机自启配置 |
| `--skip-verify` | 跳过 SHA256 校验 |
| `-V, --verbose` | 详细输出 |

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `HOTPLEX_NON_INTERACTIVE` | 非交互模式 | `0` |
| `HOTPLEX_SLACK_BOT_TOKEN` | Slack Bot Token | 必填 |
| `HOTPLEX_SLACK_APP_TOKEN` | Slack App Token | 必填 |
| `HOTPLEX_SLACK_BOT_USER_ID` | Bot User ID | 必填 |
| `HOTPLEX_PORT` | 服务端口 | `8080` |

## 安全特性 (P0)

- **SHA256 校验**: 二进制和脚本完整性校验
- **Token 验证**: Slack auth.test API 验证
- **原子操作**: mktemp + mv 避免安装中断
- **日志权限**: chmod 600 保护敏感信息
- **开机自启**: systemd / launchd 自动配置

## 架构

```
install/
├── hotplex-install.sh   # 主安装脚本 (macOS/Linux)
├── hotplex-install.ps1  # Windows (Phase 2)
└── README.md           # 本文档
```

## 参考

- [设计文档](../docs/superpowers/specs/2026-03-24-one-click-install-design.md)
- [安装指南](../INSTALL.md)
