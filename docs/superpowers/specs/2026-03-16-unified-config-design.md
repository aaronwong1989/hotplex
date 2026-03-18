# HotPlex 统一配置方案设计

**日期**: 2026-03-16
**状态**: Draft
**作者**: Claude (基于用户需求讨论)

---

## 1. 概述

统一 HotPlex 所有运行模式的配置方案，确保二进制进程、macOS 服务、Docker 容器行为一致。

### 1.1 核心原则

1. **SSOT (Single Source of Truth)**: `./configs/base/` 是基础配置的唯一来源
2. **路径一致性**: 所有模式统一使用 `~/.hotplex/` 作为配置根目录
3. **实例隔离**: 每个 bot 实例拥有独立的数据目录
4. **Seed 模式**: 共享配置通过 seed 机制分发

### 1.2 运行模式

| 模式 | 配置路径 | 说明 |
|------|----------|------|
| 宿主二进制 | `~/.hotplex/` | admin bot |
| 宿主服务 | `~/.hotplex/` | admin bot (launchd) |
| Docker 容器 | `/home/hotplex/.hotplex/` | 挂载实例目录 |

---

## 2. 目录结构

### 2.1 代码仓库

```
./configs/
├── base/                              # SSOT: 基础模板
│   ├── server.yaml
│   ├── slack.yaml
│   ├── slack_capabilities.yaml
│   └── feishu.yaml
│
└── admin/                             # admin bot 实例配置
    ├── slack.yaml                     # inherits: ./base/slack.yaml
    └── server.yaml

./docker/matrix/configs/
├── bot-01/                            # bot-01 实例配置
│   ├── base/                          # (sync 时从 ./configs/base/ 复制)
│   ├── slack.yaml                     # inherits: ./base/slack.yaml
│   └── server.yaml
├── bot-02/
│   ├── base/
│   ├── slack.yaml
│   └── server.yaml
└── bot-03/
    ├── base/
    ├── slack.yaml
    └── server.yaml
```

### 2.2 宿主机 (~/.hotplex/)

```
~/.hotplex/
│
├── seed/                              # 共享模板 (./configs/base/ 同步)
│   ├── server.yaml
│   ├── slack.yaml
│   └── slack_capabilities.yaml
│
├── configs/                           # admin bot 实例
│   ├── base/                          # (从 seed 复制)
│   ├── slack.yaml                     # inherits: ./base/slack.yaml
│   └── server.yaml
│
├── chatapp_messages.db                # admin bot 消息存储
├── projects/                          # admin bot 项目目录
└── sessions/                          # admin bot 会话数据

├── instances/
│   ├── U0AHRCL1KCM/                   # bot-01 完整实例
│   │   ├── configs/
│   │   │   ├── base/                  # (sync 时复制)
│   │   │   ├── slack.yaml             # inherits: ./base/slack.yaml
│   │   │   └── server.yaml
│   │   ├── .env                       # 实例凭证
│   │   ├── chatapp_messages.db        # 实例消息存储
│   │   ├── claude/                    # 实例 Claude 配置
│   │   ├── projects/                  # 实例项目目录
│   │   ├── sessions/                  # 实例会话数据
│   │   └── storage/                   # 实例存储
│   │
│   ├── U0AJVRH4YF6/                   # bot-02 (同结构)
│   │   └── ...
│   │
│   └── U0AL7H8UU75/                   # bot-03 (同结构)
│       └── ...
```

---

## 3. 同步命令

### 3.0 强制规则：重启前必须同步

> **宪法级规则**：所有重启操作前必须先执行同步。

| 重启命令 | 必须先执行 |
|----------|-----------|
| `make restart` | `make sync` |
| `make service-restart` | `make sync` |
| `make docker-restart` | 自动包含 `docker-sync`（无需手动） |

**设计理由**：
1. **SSOT 保证**：确保运行时配置始终与代码仓库一致
2. **避免配置漂移**：防止本地配置与源码不同步导致难以调试的问题
3. **显式操作**：用户明确知道配置会被更新

**违反后果**：
- 运行时可能使用过期配置
- 继承链断裂导致启动失败
- 行为与预期不符

### 3.1 make sync

同步 admin bot 配置：

```bash
# 1. 同步基础模板到 seed
cp -r ./configs/base/* ~/.hotplex/seed/

# 2. 同步 admin 实例配置
cp -r ./configs/admin/* ~/.hotplex/configs/

# 3. 复制 base 到 admin configs (用于继承)
cp -r ./configs/base ~/.hotplex/configs/base/

# 4. 同步 .env 凭证 (admin bot 专用)
cp .env ~/.hotplex/.env
```

### 3.2 make docker-sync

同步 Docker 实例配置：

```bash
# 对每个实例:
for instance in bot-01 bot-02 bot-03; do
    ID=$(grep "^HOTPLEX_BOT_ID=" docker/matrix/.env-${instance#bot-} | cut -d= -f2)

    # 1. 复制 base 模板
    cp -r ./configs/base ~/.hotplex/instances/$ID/configs/base/

    # 2. 复制实例特定配置
    cp -r ./docker/matrix/configs/$instance/* ~/.hotplex/instances/$ID/configs/
done
```

---

## 4. Docker 配置

### 4.1 docker-compose.yml

```yaml
services:
  hotplex-01:
    extends:
      file: common.yml
      service: hotplex-base
    container_name: hotplex-01
    ports: [ "127.0.0.1:18080:8080" ]
    env_file:
      - ~/.hotplex/instances/U0AHRCL1KCM/.env
    volumes:
      # 实例完整隔离
      - ~/.hotplex/instances/U0AHRCL1KCM:/home/hotplex/.hotplex:rw
      # Claude 配置文件 (只读，来自宿主机)
      - ${HOME}/.claude/settings.json:/home/hotplex/.claude/settings.json:ro
      - ${HOME}/.claude/skills:/home/hotplex/.claude/skills:ro
      # Per-instance Claude 状态 (named Docker volume)
      - hotplex-matrix-claude-01:/home/hotplex/.claude:rw
    environment:
      HOTPLEX_BOT_ID: U0AHRCL1KCM
    labels:
      - "hotplex.bot.id=U0AHRCL1KCM"
```

### 4.2 common.yml

```yaml
services:
  hotplex-base:
    environment:
      # 统一配置路径 - 使用现有环境变量
      HOTPLEX_SERVER_CONFIG: /home/hotplex/.hotplex/configs/server.yaml
      HOTPLEX_CHATAPPS_CONFIG_DIR: /home/hotplex/.hotplex/configs
      # 消息存储
      HOTPLEX_MESSAGE_STORE_SQLITE_PATH: /home/hotplex/.hotplex/chatapp_messages.db
      # 项目目录
      HOTPLEX_PROJECTS_DIR: /home/hotplex/projects
      # 其他配置...
    volumes:
      # Claude 配置文件 (只读，来自宿主机)
      - ${HOME}/.claude/settings.json:/home/hotplex/.claude/settings.json:ro
      - ${HOME}/.claude/skills:/home/hotplex/.claude/skills:ro
      # Go 模块缓存 (shared named volume, safe to share)
      - hotplex-matrix-go-mod:/home/hotplex/go/pkg/mod:rw
      # Per-instance build cache 在 docker-compose.yml 各服务中单独挂载
```

**注意**: `hotplex-matrix-go-build` 已改为 per-instance，在 `docker-compose.yml` 各服务定义中挂载。

---

## 5. 配置继承

### 5.1 实例 slack.yaml 示例

```yaml
# ~/.hotplex/instances/U0AHRCL1KCM/configs/slack.yaml
inherits: ./base/slack.yaml

# Optional: Override system_prompt for bot-specific identity
# system_prompt: |
#   You are Bot-01, a senior Go backend engineer...

security:
  permission:
    bot_user_id: U0AHRCL1KCM
```

### 5.2 admin bot slack.yaml 示例

```yaml
# ~/.hotplex/configs/slack.yaml
inherits: ./base/slack.yaml

# Optional: Override system_prompt for admin-specific identity
# system_prompt: |
#   You are the Admin Bot...

security:
  permission:
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
```

---

## 6. 环境变量文件 (.env) 规范

### 6.1 文件定位

| 文件 | 用途 | 路径约定 |
|------|------|----------|
| `~/.hotplex/.env` | admin bot 专属凭证 | `~/.hotplex/` (root) |
| `./.env.example` | Docker 实例模板 | `instances/${HOTPLEX_BOT_ID}/` |
| `./.env` | 本地开发临时 | 不提交 (gitignored) |
| `docker/matrix/.env-NN` | Docker 实例凭证 | `instances/${HOTPLEX_BOT_ID}/` |

### 6.2 路径差异

**admin bot** (`~/.hotplex/.env`):
```bash
# admin bot 使用 root 目录
HOTPLEX_MESSAGE_STORE_SQLITE_PATH=~/.hotplex/chatapp_messages.db
HOTPLEX_PROJECTS_DIR=~/.hotplex/projects
```

**Docker 实例** (`docker/matrix/.env-NN`):
```bash
# Docker 实例使用 instances/$ID/ 目录
HOTPLEX_MESSAGE_STORE_SQLITE_PATH=~/.hotplex/instances/${HOTPLEX_BOT_ID}/chatapp_messages.db
HOTPLEX_PROJECTS_DIR=~/.hotplex/instances/${HOTPLEX_BOT_ID}/projects
```

### 6.3 加载优先级

hotplexd 按以下顺序加载 `.env`:
1. `~/.hotplex/.env` (admin bot 首选)
2. `./.env` (当前目录，fallback)
3. `$XDG_CONFIG_HOME/hotplex/.env` (XDG fallback)

---

## 7. 隔离 vs 共享矩阵

| 资源 | admin bot | Docker bot | 说明 |
|------|-----------|------------|------|
| `configs/base/` | 共享 (seed) | 内嵌副本 | 模板，SSOT |
| `configs/slack.yaml` | 独立 | 独立 | 包含 `bot_user_id` |
| `.env` | `~/.hotplex/.env` | `instances/$ID/.env` | 凭证，必须隔离 |
| `chatapp_messages.db` | `~/.hotplex/` | `instances/$ID/` | 消息存储，隔离 |
| `claude/` | 宿主机 `~/.claude/` | 实例独立 | MCP 配置 |
| `projects/` | 独立 | 独立 | 代码库副本，隔离 |
| `sessions/` | 独立 | 独立 | 会话数据，隔离 |

---

## 8. 安全注意事项

### 7.1 敏感信息分离

| 文件类型 | 内容 | 是否可提交 |
|----------|------|------------|
| `configs/bot-*/slack.yaml` | `bot_user_id`, `system_prompt` | ✅ 可提交 |
| `configs/bot-*/server.yaml` | 服务配置，无敏感信息 | ✅ 可提交 |
| `.env` | `HOTPLEX_SLACK_BOT_TOKEN`, `HOTPLEX_SLACK_APP_TOKEN` | ❌ 不可提交 |
| `.env-*` | 凭证文件 | ❌ 不可提交 |

**原则**：敏感信息（token, api key）只存在于 `.env` 文件中，配置文件不含敏感信息。

### 7.2 .gitignore 配置

```gitignore
# 实例凭证 - 不可提交
docker/matrix/.env-*
!docker/matrix/.env.example

# 用户自定义配置 - 不同步到上游
docker/matrix/configs/bot-*/
```

### 7.3 README 说明

在 `docker/matrix/configs/` 添加 README，说明：
1. 配置继承方案
2. 实例配置目录结构
3. **提示用户不要将此目录同步到上游仓库**

---

## 9. 实例创建工具 (add-bot.sh)

### 8.1 概述

使用 `docker/matrix/add-bot.sh` 交互式创建新 bot 实例，生成全套配置文件。

**约定大于配置**：提供合理默认值，用户只需输入必要信息。

### 8.2 Makefile 命令

```makefile
add-bot: ## @docker 交互式创建新 bot 实例
	@./docker/matrix/add-bot.sh
```

### 8.3 交互流程

```
╭──────────────────────────────────────────────────────────────────╮
│  🤖 HotPlex Matrix: Add New Bot Instance                         │
╰──────────────────────────────────────────────────────────────────╯

🔍 Step 1: Discovering Environment...
  ✓ Bot Index: 04
  ✓ Target Port: 18083
  ✓ Generated API Key: 2fead6aca...

⌨️  Step 2: Required Configuration
  Input BOT_USER_ID (Slack User ID): UXXXXXXXXXX
  Input SLACK_BOT_TOKEN (xoxb-...): xoxb-************
  Input SLACK_APP_TOKEN (xapp-...): xapp-************

⚙️  Step 3: Optional Configuration (Enter to use default)
  Bot Name [bot-04]:
  Primary Owner [U0AHCF4DPK2]:
  System Prompt Role (go/frontend/devops/custom) [go]:
  Log Level [debug/info]: info

📋 Step 4: Summary
  Bot ID:        UXXXXXXXXXX
  Bot Name:      bot-04
  Port:          18083
  Config Dir:    docker/matrix/configs/bot-04/
  Env File:      ~/.hotplex/instances/UXXXXXXXXXX/.env

  Files to create:
    ✓ docker/matrix/configs/bot-04/slack.yaml
    ✓ docker/matrix/configs/bot-04/server.yaml
    ✓ docker/matrix/configs/bot-04/base/ (from ./configs/base/)
    ✓ ~/.hotplex/instances/UXXXXXXXXXX/.env
    ✓ docker/matrix/.env-04 (symlink to instances/.env)
    ✓ docker/matrix/docker-compose.yml (add service entry)

🚀 Proceed? [Y/n]: y
  ✓ Created config files
  ✓ Created .env file
  ✓ Added docker-compose service
  ✓ Synced to ~/.hotplex/instances/

✅ Bot instance created! Run with: make docker-up
```

### 8.4 生成的文件结构

```
代码仓库:
docker/matrix/configs/bot-04/
├── base/                     # 从 ./configs/base/ 复制
│   ├── server.yaml
│   ├── slack.yaml
│   └── slack_capabilities.yaml
├── slack.yaml               # inherits: ./base/slack.yaml
└── server.yaml              # inherits: ./base/server.yaml

宿主机:
~/.hotplex/instances/UXXXXXXXXXX/
├── .env                     # 凭证文件
├── configs/                 # (docker-sync 同步)
├── claude/
├── projects/
├── sessions/
└── storage/
```

### 8.5 默认值表

| 配置项 | 默认值 | 来源 |
|--------|--------|------|
| Bot Index | 递增 (01, 02, ...) | 自动检测 |
| Port | 18080 + index - 1 | 自动检测 |
| API Key | 随机 64 字符 hex | 自动生成 |
| Bot Name | `bot-$(index)` | 约定 |
| Primary Owner | 从 `.env-01` 继承 | 继承 |
| System Prompt | 根据 role 选择模板 | 模板 |
| Log Level | `info` | 约定 |

### 8.6 System Prompt 角色

| Role | System Prompt 模板 |
|------|-------------------|
| `go` | Go 后端专家 (默认) |
| `frontend` | React/Next.js 前端专家 |
| `devops` | Docker/K8s DevOps 专家 |
| `custom` | 用户自定义 (编辑器打开) |

---

## 10. 实施清单

### 9.1 配置目录
- [x] 创建 `./configs/admin/` 目录结构
- [x] 更新 `docker/matrix/configs/bot-*/` 结构（添加 base/ 子目录）

### 9.2 Makefile
- [x] 添加 `sync` 目标（admin bot 配置同步）
- [x] 更新 `docker-sync` 目标（实例配置同步）
- [x] 添加 `add-bot` 目标（链接 add-bot.sh）

### 9.3 add-bot.sh 脚本
- [x] 重构交互流程（约定大于配置）
- [x] 支持创建全套配置文件（slack.yaml, server.yaml, base/）
- [x] 支持创建 .env 文件
- [x] 支持自动添加 docker-compose 服务
- [x] 支持角色选择（go/frontend/devops/custom）

### 9.4 Docker 配置
- [x] 更新 `docker/matrix/docker-compose.yml`
- [x] 更新 `docker/matrix/common.yml`

### 9.5 文档
- [x] 添加 `docker/matrix/configs/README.md`
- [x] 更新 `.gitignore`

### 9.6 角色模板 (2026-03 新增)
- [x] 创建 `configs/templates/roles/` 目录
- [x] 添加 go.yaml (Go 后端工程师)
- [x] 添加 frontend.yaml (React/Next.js 前端工程师)
- [x] 添加 devops.yaml (Docker/K8s DevOps 工程师)
- [x] 添加 custom.yaml (用户自定义)

---

## 11. 风险评估

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 配置覆盖 | 数据丢失 | 使用 `cp -n` (no-clobber) 或备份机制 |
| 路径不一致 | 运行时错误 | 统一使用环境变量 `HOTPLEX_*` |
| 继承断裂 | 配置缺失 | 启动时检查继承文件是否存在 |

---

## 12. 参考资料

- [HotPlex CLAUDE.md](../../CLAUDE.md)
- [Configuration Layering](../../memory/MEMORY.md)
- [Uber Go Style Guide](../rules/uber-go-style-guide.md)
