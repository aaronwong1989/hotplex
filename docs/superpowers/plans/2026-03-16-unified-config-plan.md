# HotPlex 统一配置方案实施计划

> **Status: ✅ COMPLETED (2026-03-16)**
> 
> All implementation tasks have been completed as of 2026-03-16.

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 统一所有运行模式的配置方案，实现 SSOT 配置源、实例隔离、便捷的实例创建工具

**Architecture:** 基于 seed 模式的配置分发，admin bot 直接使用 `~/.hotplex/`，Docker 实例使用 `~/.hotplex/instances/$BOT_ID/`，通过 `inherits` 实现 YAML 配置继承

**Tech Stack:** Bash, YAML, Makefile, Docker Compose

**Spec:** `docs/superpowers/specs/2026-03-16-unified-config-design.md`

---

## File Structure

```
创建/修改的文件:

configs/
├── base/                    # 现有，不变
└── admin/                   # 新建
    ├── slack.yaml
    └── server.yaml

docker/matrix/
├── configs/
│   ├── README.md            # 新建
│   ├── bot-01/              # 修改：添加 base/ 子目录
│   ├── bot-02/              # 修改：添加 base/ 子目录
│   └── bot-03/              # 修改：添加 base/ 子目录
├── add-bot.sh               # 重构
├── common.yml               # 修改
└── docker-compose.yml       # 修改

Makefile                      # 添加 sync, add-bot 目标
.gitignore                    # 添加配置目录忽略规则
```

---

## Chunk 1: 配置目录结构 (configs/admin/)

**目标:** 创建 admin bot 的配置目录结构，作为宿主机运行的配置模板

### Task 1.1: 创建 configs/admin/ 目录

**Files:**
- Create: `configs/admin/slack.yaml`
- Create: `configs/admin/server.yaml`

- [ ] **Step 1: 创建 admin slack.yaml**

```yaml
# configs/admin/slack.yaml
# Admin bot configuration - inherits from base template

inherits: ../base/slack.yaml

# Admin bot uses environment variables for bot_user_id
# Set HOTPLEX_SLACK_BOT_USER_ID in ~/.hotplex/.env

# Optional: Override system_prompt for admin-specific identity
# system_prompt: |
#   You are the Admin Bot...
```

- [ ] **Step 2: 创建 admin server.yaml**

```yaml
# configs/admin/server.yaml
# Admin bot server configuration - inherits from base template

inherits: ../base/server.yaml

# Admin bot can have debug logging
server:
  log_level: debug
```

- [ ] **Step 3: 验证目录结构**

Run: `ls -la configs/admin/`
Expected: `slack.yaml` and `server.yaml` exist

- [ ] **Step 4: Commit**

```bash
git add configs/admin/
git commit -m "feat(config): add admin bot config directory with inheritance"
```

---

## Chunk 2: Docker 实例配置结构 (base/ 子目录)

**目标:** 为每个 Docker bot 实例添加 `base/` 子目录，修改继承路径

**⚠️ 重要：目录结构变化**

```
当前结构（共享 base/）:           新结构（内嵌 base/）:
docker/matrix/configs/            docker/matrix/configs/
├── base/                         ├── bot-01/
├── bot-01/                       │   ├── base/          ← 从 configs/base/ 复制
│   └── slack.yaml                │   ├── slack.yaml     ← inherits: ./base/slack.yaml
│       inherits: ../base/        │   └── server.yaml
└── bot-02/                       └── bot-02/
    └── ...                           └── ...
```

每个 bot 实例现在有自己的 `base/` 副本，继承路径从 `../base/` 改为 `./base/`。

### Task 2.1: 更新 bot-01 配置

**Files:**
- Create: `docker/matrix/configs/bot-01/base/` (从 `configs/base/` 复制)
- Modify: `docker/matrix/configs/bot-01/slack.yaml`
- Modify: `docker/matrix/configs/bot-01/server.yaml`

- [ ] **Step 1: 创建 base 子目录并复制模板**

```bash
mkdir -p docker/matrix/configs/bot-01/base
cp configs/base/*.yaml docker/matrix/configs/bot-01/base/
```

Run: `ls docker/matrix/configs/bot-01/base/`
Expected: `server.yaml`, `slack.yaml`, `slack_capabilities.yaml`, `feishu.yaml`

- [ ] **Step 2: 更新 slack.yaml 继承路径**

修改 `docker/matrix/configs/bot-01/slack.yaml`:

```yaml
# docker/matrix/configs/bot-01/slack.yaml
# Bot-01 - Primary Go backend engineer bot
# Inherits from: ./base/slack.yaml

inherits: ./base/slack.yaml

# Optional: Override system_prompt for bot-specific identity
# system_prompt: |
#   You are Bot-01, a senior Go backend engineer...

security:
  permission:
    bot_user_id: U0AHRCL1KCM
```

- [ ] **Step 3: 更新 server.yaml 继承路径**

修改 `docker/matrix/configs/bot-01/server.yaml`:

```yaml
# docker/matrix/configs/bot-01/server.yaml
# Bot-01 - Primary Go backend engineer bot
# Inherits from: ./base/server.yaml

inherits: ./base/server.yaml

server:
  log_level: debug
```

### Task 2.2: 更新 bot-02 配置

**Files:**
- Create: `docker/matrix/configs/bot-02/base/`
- Modify: `docker/matrix/configs/bot-02/slack.yaml`
- Modify: `docker/matrix/configs/bot-02/server.yaml`

- [ ] **Step 1: 创建 base 子目录**

```bash
mkdir -p docker/matrix/configs/bot-02/base
cp configs/base/*.yaml docker/matrix/configs/bot-02/base/
```

- [ ] **Step 2: 更新 slack.yaml**

修改 `docker/matrix/configs/bot-02/slack.yaml`:

```yaml
inherits: ./base/slack.yaml

# Optional: Override system_prompt for bot-specific identity
# system_prompt: |
#   You are Bot-02, a frontend specialist...

security:
  permission:
    bot_user_id: U0AJVRH4YF6
```

- [ ] **Step 3: 更新 server.yaml**

修改 `docker/matrix/configs/bot-02/server.yaml`:

```yaml
inherits: ./base/server.yaml
```

### Task 2.3: 更新 bot-03 配置

**Files:**
- Create: `docker/matrix/configs/bot-03/base/`
- Modify: `docker/matrix/configs/bot-03/slack.yaml`
- Modify: `docker/matrix/configs/bot-03/server.yaml`

- [ ] **Step 1: 创建 base 子目录**

```bash
mkdir -p docker/matrix/configs/bot-03/base
cp configs/base/*.yaml docker/matrix/configs/bot-03/base/
```

- [ ] **Step 2: 更新 slack.yaml**

修改 `docker/matrix/configs/bot-03/slack.yaml`:

```yaml
inherits: ./base/slack.yaml

# Optional: Override system_prompt for bot-specific identity
# system_prompt: |
#   You are Bot-03, a DevOps specialist...

security:
  permission:
    bot_user_id: U0AL7H8UU75
```

- [ ] **Step 3: 更新 server.yaml**

修改 `docker/matrix/configs/bot-03/server.yaml`:

```yaml
inherits: ./base/server.yaml
```

### Task 2.4: 验证并提交

- [ ] **Step 1: 验证目录结构**

Run: `ls -la docker/matrix/configs/bot-*/`
Expected: Each bot directory has `base/`, `slack.yaml`, `server.yaml`

- [ ] **Step 2: Commit**

```bash
git add docker/matrix/configs/
git commit -m "feat(config): add base/ subdirectory to bot configs with inheritance"
```

---

## Chunk 3: Makefile 同步命令

**目标:** 添加 `sync` 和更新 `docker-sync` 目标，添加 `add-bot` 命令

### Task 3.1: 添加 sync 目标

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: 查找当前 docker-sync 目标位置**

Run: `grep -n "docker-sync:" Makefile`
Expected: Line number for insertion point

- [ ] **Step 2: 在 docker-sync 前添加 sync 目标**

在 `docker-sync:` 目标前添加：

```makefile
# --- Config Sync ---

sync: ## @config Sync configs to ~/.hotplex/ (admin bot)
	@$(call SECTION_HEADER,🔄 Syncing Configs)
	@mkdir -p $(HOME_DIR)/.hotplex/seed
	@mkdir -p $(HOME_DIR)/.hotplex/configs
	@printf "  ${CYAN}→${NC} Syncing base templates to seed...\n"
	@cp -r configs/base/* $(HOME_DIR)/.hotplex/seed/
	@printf "  ${CYAN}→${NC} Syncing admin config...\n"
	@cp -r configs/admin/* $(HOME_DIR)/.hotplex/configs/
	@cp -r configs/base $(HOME_DIR)/.hotplex/configs/base/
	@printf "  ${GREEN}✓${NC} Synced to ${BOLD}$(HOME_DIR)/.hotplex/${NC}\n"
```

### Task 3.2: 更新 docker-sync 目标

- [ ] **Step 1: 替换现有 docker-sync 目标**

找到 `docker-sync:` 目标，替换为：

```makefile
docker-sync: ## @docker Sync configs to all Docker instances
	@$(call SECTION_HEADER,🔄 Syncing Docker Instance Configs)
	@for f in docker/matrix/.env-*; do \
		ID=$$(grep "^HOTPLEX_BOT_ID=" $$f | cut -d= -f2 | tr -d ' ' | tr -d '\r'); \
		BOT_NUM=$$(basename $$f | sed 's/.env-//'); \
		if [ -n "$$ID" ]; then \
			INSTANCE_DIR=$(HOME_DIR)/.hotplex/instances/$$ID/configs; \
			mkdir -p "$$INSTANCE_DIR/base"; \
			mkdir -p $(HOME_DIR)/.hotplex/instances/$$ID/claude; \
			mkdir -p $(HOME_DIR)/.hotplex/instances/$$ID/projects; \
			mkdir -p $(HOME_DIR)/.hotplex/instances/$$ID/sessions; \
			mkdir -p $(HOME_DIR)/.hotplex/instances/$$ID/storage; \
			printf "  ${CYAN}→${NC} Syncing ${BOLD}$$ID${NC} (bot-$$BOT_NUM)...\n"; \
			cp -r configs/base/* "$$INSTANCE_DIR/base/"; \
			if [ -d "docker/matrix/configs/bot-$$BOT_NUM" ]; then \
				cp docker/matrix/configs/bot-$$BOT_NUM/*.yaml "$$INSTANCE_DIR/" 2>/dev/null || true; \
			fi; \
			printf "  ${GREEN}✓${NC} Synced ${BOLD}$$ID${NC}\n"; \
		fi; \
	done
	@printf "${GREEN}✅ All Docker instances synced${NC}\n"
```

### Task 3.3: 添加 add-bot 目标

- [ ] **Step 1: 在 docker-sync 后添加 add-bot 目标**

```makefile
add-bot: ## @docker Interactive bot instance creation
	@./docker/matrix/add-bot.sh
```

- [ ] **Step 2: 更新 .PHONY 行**

在 `.PHONY:` 行添加 `sync` 和 `add-bot`

- [ ] **Step 3: Commit**

```bash
git add Makefile
git commit -m "feat(make): add sync target and update docker-sync for unified config"
```

---

## Chunk 4: Docker Compose 配置更新

**目标:** 更新 docker-compose.yml 和 common.yml 以支持统一的挂载结构

### Task 4.1: 更新 common.yml

**Files:**
- Modify: `docker/matrix/common.yml`

- [ ] **Step 1: 更新 volumes 部分**

修改 `docker/matrix/common.yml`，确保 volumes 部分使用实例目录挂载：

```yaml
services:
  hotplex-base:
    <<: *hotplex-common
    volumes:
      # Claude 配置文件 (只读，来自宿主机)
      - ${HOME}/.claude/settings.json:/home/hotplex/.claude/settings.json:ro
      - ${HOME}/.claude/skills:/home/hotplex/.claude/skills:ro
      # Go 模块缓存 (shared named volume)
      - hotplex-matrix-go-mod:/home/hotplex/go/pkg/mod:rw
      # Per-instance build cache 在 docker-compose.yml 各服务中单独挂载
      # Entrypoint mount
      - ../docker-entrypoint.sh:/app/docker-entrypoint.sh:ro
```

> **注意**: `.claude` 状态和 `.cache/go-build` 已改为 per-instance named volume，在 `docker-compose.yml` 的各服务定义中单独挂载。

### Task 4.2: 更新 docker-compose.yml

**Files:**
- Modify: `docker/matrix/docker-compose.yml`

- [ ] **Step 1: 确认 docker-compose.yml 结构**

Run: `cat docker/matrix/docker-compose.yml`
Expected: Current service definitions

- [ ] **Step 2: 验证挂载路径正确**

确保每个服务的 volumes 通过 common.yml 继承，不需要单独指定

- [ ] **Step 3: Commit**

```bash
git add docker/matrix/common.yml docker/matrix/docker-compose.yml
git commit -m "feat(docker): update volume mounts for unified config structure"
```

---

## Chunk 5: add-bot.sh 脚本重构

**目标:** 重构交互流程，支持创建全套配置文件和 .env

### Task 5.1: 重构 add-bot.sh

**Files:**
- Modify: `docker/matrix/add-bot.sh`

- [ ] **Step 1: 添加角色选择功能**

在 Step 3 后添加角色选择：

```bash
# --- 4. Role Selection ---
printf "\n${BOLD}${BLUE}🎭 Step 4: Role Selection${NC}\n"
printf "  ${CYAN}Select bot role:${NC}\n"
printf "    1) ${BOLD}go${NC}      - Go backend engineer (default)\n"
printf "    2) ${BOLD}frontend${NC} - React/Next.js specialist\n"
printf "    3) ${BOLD}devops${NC}   - Docker/K8s specialist\n"
printf "    4) ${BOLD}custom${NC}   - Custom (will open editor)\n"
printf "  ${YELLOW}Choice [1]:${NC} "
read -r ROLE_CHOICE

case "${ROLE_CHOICE:-1}" in
    1) BOT_ROLE="go"; ROLE_NAME="Go backend engineer" ;;
    2) BOT_ROLE="frontend"; ROLE_NAME="Frontend specialist" ;;
    3) BOT_ROLE="devops"; ROLE_NAME="DevOps specialist" ;;
    4) BOT_ROLE="custom"; ROLE_NAME="Custom" ;;
    *) BOT_ROLE="go"; ROLE_NAME="Go backend engineer" ;;
esac
printf "  ${GREEN}✓${NC} Selected role: ${BOLD}$ROLE_NAME${NC}\n"
```

- [ ] **Step 2: 添加配置文件生成函数**

```bash
# --- Helper: Generate slack.yaml ---
generate_slack_yaml() {
    local bot_id=$1
    local role=$2
    local yaml_file=$3

    cat > "$yaml_file" <<EOF
# $(basename $(dirname $yaml_file))/slack.yaml
# Bot configuration - inherits from base template

inherits: ./base/slack.yaml

# Bot identity
security:
  permission:
    bot_user_id: $bot_id

# Optional: Override system_prompt for bot-specific identity
# system_prompt: |
#   You are a $role specialist...
EOF
}

# --- Helper: Generate server.yaml ---
generate_server_yaml() {
    local log_level=$1
    local yaml_file=$2

    cat > "$yaml_file" <<EOF
# $(basename $(dirname $yaml_file))/server.yaml
# Server configuration - inherits from base template

inherits: ./base/server.yaml

server:
  log_level: $log_level
EOF
}
```

- [ ] **Step 3: 添加配置目录创建逻辑**

在生成 .env 后添加：

```bash
# --- 5. Create Config Directory ---
BOT_CONFIG_DIR="configs/$SERVICE_NAME"
mkdir -p "$BOT_CONFIG_DIR/base"

# Copy base templates
cp -r ../../configs/base/* "$BOT_CONFIG_DIR/base/"

# Generate slack.yaml
generate_slack_yaml "$HOTPLEX_BOT_ID" "$BOT_ROLE" "$BOT_CONFIG_DIR/slack.yaml"
printf "  ${GREEN}✓${NC} Created $BOT_CONFIG_DIR/slack.yaml\n"

# Generate server.yaml
generate_server_yaml "info" "$BOT_CONFIG_DIR/server.yaml"
printf "  ${GREEN}✓${NC} Created $BOT_CONFIG_DIR/server.yaml\n"
```

- [ ] **Step 4: 添加实例目录创建逻辑**

```bash
# --- 6. Create Instance Directory ---
INSTANCE_DIR="$HOME/.hotplex/instances/$HOTPLEX_BOT_ID"
mkdir -p "$INSTANCE_DIR"/{configs/base,claude,projects,sessions,storage}

# Copy .env to instance
cp "$ENV_FILE" "$INSTANCE_DIR/.env"
printf "  ${GREEN}✓${NC} Created $INSTANCE_DIR/.env\n"

# Sync configs to instance
cp -r "$BOT_CONFIG_DIR"/* "$INSTANCE_DIR/configs/"
printf "  ${GREEN}✓${NC} Synced configs to instance\n"
```

- [ ] **Step 5: 添加 docker-compose 服务自动添加**

```bash
# --- 7. Add to docker-compose.yml ---
if ! grep -q "hotplex-$BOT_PADDED_INDEX:" docker-compose.yml; then
    # Safety: Ensure file ends with newline to avoid YAML corruption
    if [ -f "docker-compose.yml" ] && [ -s "docker-compose.yml" ]; then
        # Add blank line for separation if not already present
        echo "" >> docker-compose.yml
    fi

    cat >> docker-compose.yml <<EOF
  # ============================================================================
  # Bot $BOT_PADDED_INDEX: $(date +%Y-%m-%d)
  # ============================================================================
  hotplex-$BOT_PADDED_INDEX:
    extends:
      file: common.yml
      service: hotplex-base
    container_name: hotplex-$BOT_PADDED_INDEX
    ports: [ "127.0.0.1:$PORT:8080" ]
    env_file:
      - ~/.hotplex/instances/$HOTPLEX_BOT_ID/.env
    environment:
      HOTPLEX_BOT_ID: $HOTPLEX_BOT_ID
    labels:
      - "hotplex.bot.id=$HOTPLEX_BOT_ID"
EOF
    printf "  ${GREEN}✓${NC} Added service to docker-compose.yml\n"
fi
```

- [ ] **Step 6: Commit**

```bash
git add docker/matrix/add-bot.sh
git commit -m "feat(add-bot): refactor with role selection and full config generation"
```

---

## Chunk 6: 文档和 .gitignore

**目标:** 添加 README 和更新 .gitignore

### Task 6.1: 创建 configs README

**Files:**
- Create: `docker/matrix/configs/README.md`

- [ ] **Step 1: 创建 README**

```markdown
# Docker Matrix Instance Configurations

This directory contains bot-specific configuration files for Docker Matrix deployment.

## Directory Structure

```
configs/
├── bot-01/
│   ├── base/               # Base templates (synced from ./configs/base/)
│   ├── slack.yaml          # Bot-specific Slack config
│   └── server.yaml         # Bot-specific server config
├── bot-02/
└── bot-03/
```

## Configuration Inheritance

Each bot's `slack.yaml` and `server.yaml` inherits from the base template:

```yaml
# slack.yaml
inherits: ./base/slack.yaml

security:
  permission:
    bot_user_id: UXXXXXXXXXX
```

## Creating New Bots

Use the interactive script:

```bash
make add-bot
# or
./docker/matrix/add-bot.sh
```

## ⚠️ Important: Don't Sync to Upstream

This directory contains your bot configurations. **Do not sync to upstream repository.**

Add to your fork's `.gitignore`:
```
docker/matrix/configs/bot-*/
```

## Sensitive Information

Sensitive credentials (tokens, API keys) are stored in `.env` files, not in YAML configs.
The `.env` files are located at `~/.hotplex/instances/$BOT_ID/.env` and are never committed.
```

### Task 6.2: 更新 .gitignore

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: 添加配置目录忽略规则**

在 `.gitignore` 末尾添加：

```gitignore
# User bot configs - don't sync to upstream
docker/matrix/configs/bot-*/

# Keep example files
!docker/matrix/configs/bot-*/*.example
```

- [ ] **Step 2: Commit**

```bash
git add docker/matrix/configs/README.md .gitignore
git commit -m "docs: add configs README and update .gitignore"
```

---

## Chunk 7: 最终验证

**目标:** 验证所有配置正确工作

### Task 7.1: 验证配置继承

- [ ] **Step 1: 测试 admin bot 配置继承**

Run: `cat ~/.hotplex/configs/slack.yaml 2>/dev/null || echo "Run 'make sync' first"`
Expected: slack.yaml with `inherits: ./base/slack.yaml`

- [ ] **Step 2: 运行 make sync**

Run: `make sync`
Expected: Files synced to `~/.hotplex/`

- [ ] **Step 3: 运行 make docker-sync**

Run: `make docker-sync`
Expected: All instance configs updated

### Task 7.2: 验证 Docker 配置

- [ ] **Step 1: 验证 docker-compose 配置**

Run: `cd docker/matrix && docker compose config --services`
Expected: List of hotplex-01, hotplex-02, hotplex-03 services

- [ ] **Step 2: 最终提交**

```bash
git add -A
git commit -m "feat(config): complete unified configuration implementation"
```

---

## Summary

| Chunk | Description | Files |
|-------|-------------|-------|
| 1 | 配置目录结构 | `configs/admin/*` |
| 2 | Docker 实例配置 | `docker/matrix/configs/bot-*/` |
| 3 | Makefile 命令 | `Makefile` |
| 4 | Docker Compose | `docker/matrix/*.yml` |
| 5 | add-bot.sh 重构 | `docker/matrix/add-bot.sh` |
| 6 | 文档 | `docker/matrix/configs/README.md`, `.gitignore` |
| 7 | 验证 | - |
