# HotPlex Makefile
# A premium CLI experience for building and managing HotPlex

# =============================================================================
# 🌍 Cross-Platform Compatibility
# =============================================================================

# Detect OS
ifeq ($(OS),Windows_NT)
    PLATFORM := Windows
else
    PLATFORM := Unix
endif

# Determine Home Directory
ifeq ($(PLATFORM),Windows)
    HOME_DIR := $(USERPROFILE)
else
    HOME_DIR := $(HOME)
endif

# Host configs directory (default to ~/.hotplex/configs)
HOST_CONFIGS_DIR := $(if $(HOTPLEX_HOST_CONFIGS_DIR),$(HOTPLEX_HOST_CONFIGS_DIR),$(HOME_DIR)/.hotplex/configs)

# Export HOME for subprocess visibility on Windows
export HOME := $(HOME_DIR)

# Check shell environment on Windows - require POSIX shell
# Valid POSIX shells on Windows: Git Bash, MSYS2, MinGW, WSL, Cygwin
ifeq ($(OS),Windows_NT)
    # Check if running in cmd.exe or PowerShell (no POSIX environment)
    ifndef MSYSTEM
        ifndef BASH_VERSION
            $(error [ERROR] Windows CMD/PowerShell detected - not supported. \
HotPlex Makefile requires a POSIX-compatible shell: \
  - Option 1: Git Bash (recommended) - https://git-scm.com/download/win \
  - Option 2: WSL (Windows Subsystem for Linux) - run 'wsl' in terminal \
  - Option 3: MSYS2 - https://www.msys2.org/ \
  - Option 4: Cygwin - https://www.cygwin.com/ \
)
        endif
    endif
endif

# Common Commands (POSIX-Standard)
MKDIR := mkdir -p
RM    := rm -rf

# Colors for UI (use printf for cross-platform compatibility)
CYAN          := $(shell printf '\033[0;36m')
GREEN         := $(shell printf '\033[0;32m')
YELLOW        := $(shell printf '\033[1;33m')
RED           := $(shell printf '\033[0;31m')
PURPLE        := $(shell printf '\033[0;35m')
BLUE          := $(shell printf '\033[0;34m')
BOLD          := $(shell printf '\033[1m')
DIM           := $(shell printf '\033[2m')
NC            := $(shell printf '\033[0m')

# Metadata
BINARY_NAME   := hotplexd
CMD_PATH      := ./cmd/hotplexd
DIST_DIR      := dist
VERSION       ?= 0.36.0
COMMIT        ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS       := -X 'main.version=v$(VERSION)' -X 'github.com/hrygo/hotplex.Version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(BUILD_TIME)'

LOG_DIR       := .logs
LOG_FILE      := $(LOG_DIR)/daemon.log

.PHONY: all help build build-all fmt vet test test-unit test-ci test-race test-integration test-all lint tidy clean install-hooks run stop restart docs svg2png service-install service-uninstall service-start service-stop service-restart service-status service-logs service-enable service-disable

# Default target
all: help

# Service management script
SERVICE_SCRIPT := ./scripts/ops/service.sh

# =============================================================================
# 🎯 Helper: Styled Section Header
# =============================================================================
define SECTION_HEADER
printf "\n${BOLD}${BLUE}╭─ %s ────────────────────────────────────$(NC)\n" "$1"
endef

define SECTION_FOOTER
printf "${DIM}${BLUE}╰─────────────────────────────────────────────$(NC)\n"
endef

# =============================================================================
# 📋 HELP
# =============================================================================
help: ## Show this help message
	@printf "\n${BOLD}${CYAN}🔥 HotPlex Build System$(NC) ${DIM}${VERSION}$(NC)\n"
	@printf "${DIM}Usage: make ${YELLOW}<target>$(NC) ${DIM}[args]$(NC)\n"
	@printf "\n"
	@$(call SECTION_HEADER,🔨 Build)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@build/ {gsub(/@build /, "", $$2); printf "  ${GREEN}%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🧪 Test)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@test/ {gsub(/@test /, "", $$2); printf "  ${GREEN}%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🔧 Development)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@dev/ {gsub(/@dev /, "", $$2); printf "  ${GREEN}%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🚀 Runtime)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@runtime/ {gsub(/@runtime /, "", $$2); printf "  ${GREEN}%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,📦 Service)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@service/ {gsub(/@service /, "", $$2); printf "  ${GREEN}%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🛠️ Utils)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@util/ {gsub(/@util /, "", $$2); printf "  ${GREEN}%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🐳 Docker)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@docker/ {gsub(/@docker /, "", $$2); printf "  ${GREEN}%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🎯 OpenCode Server)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@opencode/ {gsub(/@opencode /, "", $$2); printf "  ${GREEN}%-20s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@printf "\n${DIM}💡 Tip: Use 'make <target> V=1' for verbose output$(NC)\n\n"

# =============================================================================
# 🔨 BUILD
# =============================================================================
build: fmt vet tidy ## @build Compile the hotplexd daemon
	@printf "${GREEN}🚀 Building HotPlex Daemon (${VERSION})...$(NC)\n"
	@mkdir -p $(DIST_DIR)
	@go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@printf "${GREEN}✅ Build complete: ${DIST_DIR}/$(BINARY_NAME)$(NC)\n"

# =============================================================================
# 🔧 INSTALL
# =============================================================================
install: config-info ## @runtime Build and install hotplexd to /usr/local/bin
	@printf "${PURPLE}📦 Building HotPlex Daemon...$(NC)\n"
	go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@printf "${PURPLE}📦 Installing to /usr/local/bin...$(NC)\n"
	cp $(DIST_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@printf "${GREEN}✅ Installed: /usr/local/bin/hotplexd$(NC)\n"

config-info: ## @util Display current configuration status
	@printf "\n${BOLD}${CYAN}╭─ 🔧 Configuration Files ─────────────────────────────${NC}\n"
	@printf "\n"
	@printf "  ${BOLD}📋 Environment Variables Source (.env):${NC}\n"
	@if [ -f .env ]; then \
		printf "     ${GREEN}✓${NC} Active\n"; \
		printf "     ${CYAN}Path:${NC} $$(pwd)/.env\n"; \
	else \
		printf "     ${YELLOW}⚠${NC} Not found\n"; \
		printf "     ${CYAN}Template:${NC} $$(pwd)/.env.example\n"; \
	fi
	@printf "\n"
	@printf "  ${BOLD}📋 ChatApps Config Directory Priority:${NC}\n"
	@printf "     ${GREEN}1. --config flag (highest)${NC}\n"
	@printf "        ${DIM}Usage: hotplexd --config /path/to/configs${NC}\n"
	@printf "\n"
	@printf "     ${GREEN}2. HOTPLEX_CHATAPPS_CONFIG_DIR env${NC}\n"
	@if [ -n "$$HOTPLEX_CHATAPPS_CONFIG_DIR" ]; then \
		printf "        ${GREEN}✓${NC} Set: $$HOTPLEX_CHATAPPS_CONFIG_DIR\n"; \
	else \
		printf "        ${YELLOW}⚠${NC} Not set${NC}\n"; \
	fi
	@printf "\n"
	@printf "     ${GREEN}3. User config (~/.hotplex/configs)${NC}\n"
	@if [ -d "$$HOME/.hotplex/configs" ]; then \
		printf "        ${GREEN}✓${NC} Active\n"; \
		printf "        ${CYAN}Path:${NC} $$HOME/.hotplex/configs/\n"; \
	else \
		printf "        ${YELLOW}⚠${NC} Not found${NC}\n"; \
	fi
	@printf "\n"
	@printf "     ${GREEN}4. Default (./configs/admin)${NC}\n"
	@if [ -d "configs/admin" ]; then \
		printf "        ${GREEN}✓${NC} Active (Admin Bot)\n"; \
		printf "        ${CYAN}Path:${NC} $$(pwd)/configs/admin/\n"; \
		for f in configs/admin/*.yaml; do \
			if [ -f "$$f" ]; then \
				printf "            - $$(basename $$f)\n"; \
			fi; \
		done; \
		printf "        ${DIM}Inherits: configs/base/${NC}\n"; \
	else \
		printf "        ${YELLOW}⚠${NC} Not found${NC}\n"; \
	fi
	@printf "${BOLD}${CYAN}╰─────────────────────────────────────────────────────${NC}\n\n"

build-all: fmt vet tidy ## @build Compile for all platforms (Linux/macOS/Windows)
	@printf "${GREEN}🚀 Building HotPlex Daemon for all platforms (${VERSION})...$(NC)\n"
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	@GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)
	@GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	@GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)
	@GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)
	@printf "${GREEN}✅ Cross-compilation complete in ${DIST_DIR}/$(NC)\n"

# =============================================================================
# 🧪 TEST
# =============================================================================
test: test-unit ## @test Run fast unit tests (default)

test-unit: ## @test Run unit tests (fast, no race detection)
	@printf "${CYAN}🧪 Running fast unit tests...$(NC)\n"
	@go test -v -short ./...
	@printf "${GREEN}✅ Unit tests passed!$(NC)\n"

test-ci: ## @test Run tests optimized for CI (parallel, timeout, short mode)
	@printf "${CYAN}🧪 Running CI-optimized tests...$(NC)\n"
	@go test -v -short -timeout=5m -parallel=4 ./...
	@printf "${GREEN}✅ CI tests passed!$(NC)\n"

test-race: ## @test Run unit tests with race detection
	@printf "${CYAN}🧪 Running unit tests with race detection...$(NC)\n"
	@go test -v -race ./...
	@printf "${GREEN}✅ Race detection passed!$(NC)\n"

test-integration: ## @test Run heavy integration tests
	@printf "${YELLOW}🏗️  Running heavy integration tests...$(NC)\n"
	@go test -short -v -tags=integration ./...
	@printf "${GREEN}✅ Integration tests passed!$(NC)\n"

test-all: test-race test-integration ## @test Run all tests

coverage: ## @test Generate coverage report
	@printf "${CYAN}📊 Generating coverage report...$(NC)\n"
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@printf "${GREEN}✅ Coverage report generated: coverage.out$(NC)\n"

coverage-html: coverage ## @test Generate coverage HTML report
	@go tool cover -html=coverage.out -o coverage.html
	@printf "${GREEN}✅ Coverage HTML report: coverage.html$(NC)\n"

# =============================================================================
# 🔧 DEVELOPMENT
# =============================================================================
fmt: ## @dev Format Go code
	@printf "${CYAN}🔧 Formatting code...$(NC)\n"
	@go fmt ./...

vet: ## @dev Check for suspicious constructs
	@printf "${CYAN}🔍 Vetting code...$(NC)\n"
	@go vet ./...

lint: ## @dev Run golangci-lint
	@printf "${PURPLE}🔍 Linting code...$(NC)\n"
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run ./...; \
		printf "${GREEN}✅ Linting passed!$(NC)\n"; \
	else \
		printf "${RED}❌ golangci-lint not found. Install it first.$(NC)\n"; \
		exit 1; \
	fi

tidy: ## @dev Clean up go.mod dependencies
	@printf "${YELLOW}📦 Tidying up Go modules...$(NC)\n"
	@go mod tidy
	@printf "${GREEN}✅ Modules synchronized.$(NC)\n"

clean: ## @dev Remove build artifacts
	@printf "${RED}🧹 Cleaning up build artifacts...$(NC)\n"
	@rm -rf $(DIST_DIR)
	@go clean
	@printf "${GREEN}✅ Cleanup done.$(NC)\n"

install-hooks: ## @dev Install Git hooks
	@printf "${CYAN}🔗 Installing HotPlex Git Hooks...$(NC)\n"
	@chmod +x scripts/git-hooks/*.sh 2>/dev/null || true
	@if [ -d scripts/git-hooks ] && [ -f scripts/git-hooks/setup_hooks.sh ]; then ./scripts/git-hooks/setup_hooks.sh; fi
	@printf "${GREEN}✅ Hooks are active.$(NC)\n"

# =============================================================================
# 🚀 RUNTIME
# =============================================================================
run: sync build config-info ## @runtime Build and start daemon in foreground
	@printf "${PURPLE}🔥 Starting HotPlex Daemon...$(NC)\n"
	@./$(DIST_DIR)/$(BINARY_NAME)


stop: ## @runtime Stop the running daemon and all its child processes
	@printf "${YELLOW}🛑 Stopping HotPlex Daemon...$(NC)\n"
	@PID=$$(pgrep -f $(BINARY_NAME) | head -1); \
	if [ -n "$$PID" ]; then \
		PGID=$$(ps -o pgid= -p $$PID | tr -d ' '); \
		if [ -n "$$PGID" ] && [ "$$PGID" != "1" ]; then \
			kill -- -$$PGID 2>/dev/null; \
			sleep 1; \
			if ps -p $$PID > /dev/null 2>&1; then \
				kill -9 -- -$$PGID 2>/dev/null; \
			fi; \
		fi; \
		printf "${GREEN}✅ Daemon stopped$(NC)\n"; \
	else \
		printf "${YELLOW}⚠️  No running daemon found$(NC)\n"; \
	fi

restart: sync build config-info ## @runtime Restart daemon with latest source code
	@mkdir -p $(LOG_DIR)
	@./scripts/ops/restart_helper.sh "$$(pwd)/$(DIST_DIR)/$(BINARY_NAME)" "$(LOG_FILE)"

# =============================================================================
# 📦 SERVICE (System Service)
# =============================================================================
service-install: build ## @service Install as system service
	@printf "${CYAN}📦 Installing HotPlex as system service...$(NC)\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) install

service-uninstall: ## @service Remove the system service
	@printf "${YELLOW}🗑️  Removing HotPlex system service...$(NC)\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) uninstall

service-start: sync ## @service Start the system service
	@printf "${GREEN}▶️  Starting HotPlex service...$(NC)\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) start

service-stop: ## @service Stop the system service
	@printf "${YELLOW}⏹️  Stopping HotPlex service...$(NC)\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) stop

service-restart: sync ## @service Restart the system service
	@printf "${PURPLE}🔄 Restarting HotPlex service...$(NC)\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) restart

service-status: ## @service Check service status
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) status

service-logs: ## @service Tail service logs (Ctrl+C to stop)
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) logs

service-enable: ## @service Enable auto-start on boot
	@printf "${GREEN}🔔 Enabling auto-start...$(NC)\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) enable

service-disable: ## @service Disable auto-start on boot
	@printf "${YELLOW}🔕 Disabling auto-start...$(NC)\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) disable

# =============================================================================
# 🔄 CONFIG SYNC (Local Development)
# =============================================================================

sync: ## @runtime Sync local configs (.env, configs/base, top-level configs) to ~/.hotplex/configs
	@$(call SECTION_HEADER,🔄 Syncing Local Configs to Host)
	@mkdir -p $(HOST_CONFIGS_DIR)
	@printf "  ${CYAN}→ Target:$(NC) $(HOST_CONFIGS_DIR)\n"

	# Sync .env (if exists in project root)
	@if [ -f ".env" ]; then \
		printf "  ${CYAN}→$(NC) Syncing ${BOLD}.env${NC}...\n"; \
		cp .env $(HOST_CONFIGS_DIR)/.env; \
		printf "  ${GREEN}✓$(NC) .env synced\n"; \
	fi

	# Sync base configs (templates)
	@if [ -d "configs/base" ]; then \
		printf "  ${CYAN}→$(NC) Syncing ${BOLD}configs/base/${NC}...\n"; \
		mkdir -p $(HOST_CONFIGS_DIR)/base; \
		cp -r configs/base/* $(HOST_CONFIGS_DIR)/base/; \
		printf "  ${GREEN}✓$(NC) configs/base synced\n"; \
	fi

	# Sync top-level platform configs (not in subdirectories)
	@for f in configs/*.yaml; do \
		if [ -f "$$f" ]; then \
			printf "  ${CYAN}→$(NC) Syncing ${BOLD}$$f${NC}...\n"; \
			cp $$f $(HOST_CONFIGS_DIR)/; \
			printf "  ${GREEN}✓$(NC) $$f synced\n"; \
		fi; \
	done

	@printf "${GREEN}✅ Config sync complete → $(HOST_CONFIGS_DIR)/${NC}\n"

# =============================================================================
# 🛠️ UTILS
# =============================================================================
svg2png: ## @util Convert SVG to 4K PNG
	@printf "${CYAN}🖼️  Converting SVG to PNG...$(NC)\n"
	@chmod +x scripts/tools/svg2png.sh 2>/dev/null || true
	@./scripts/tools/svg2png.sh
	@printf "${GREEN}✅ PNG assets generated in docs/images/png/$(NC)\n"

# =============================================================================
# 🐳 DOCKER & STACKS (Consolidated)
# =============================================================================

DOCKER_IMAGE    ?= hotplex
DOCKER_TAG      ?= latest
DOCKER_REGISTRY ?= ghcr.io/hrygo
HOST_UID        ?= $(shell id -u)
STACK_TAG       ?= latest

# 代理与源配置 (Optimized for mainland China)
# HTTP_PROXY/HTTPS_PROXY: 留空表示不使用代理，需要时手动设置
HTTP_PROXY       ?=
HTTPS_PROXY      ?=
ALPINE_MIRROR    ?= mirrors.aliyun.com
NPM_MIRROR       ?= https://registry.npmmirror.com
PYTHON_MIRROR    ?= https://pypi.tuna.tsinghua.edu.cn/simple
GOPROXY          ?= https://goproxy.cn,direct
RUSTUP_DIST_SERVER ?= https://rsproxy.cn
GITHUB_PROXY     ?= https://mirror.ghproxy.com/

VALID_STACKS := go node python java rust full
STACK_VERSION_go     := 1.26
STACK_VERSION_node   := 24
STACK_VERSION_python := 3.14
STACK_VERSION_java   := 21
STACK_VERSION_rust   := 1.94
STACK_VERSION_full   := $(STACK_TAG)

# 统一构建参数
DOCKER_BUILD_COMMON_ARGS := --build-arg HOST_UID=$(HOST_UID) \
                            --build-arg VERSION=$(VERSION) \
                            --build-arg COMMIT=$(COMMIT) \
                            --build-arg BUILD_TIME=$(BUILD_TIME)

DOCKER_BUILD_PROXY_ARGS := --build-arg HTTP_PROXY=$(HTTP_PROXY) \
                           --build-arg HTTPS_PROXY=$(HTTPS_PROXY) \
                           --build-arg ALPINE_MIRROR=$(ALPINE_MIRROR) \
                           --build-arg NPM_MIRROR=$(NPM_MIRROR) \
                           --build-arg PYTHON_MIRROR=$(PYTHON_MIRROR) \
                           --build-arg GOPROXY=$(GOPROXY) \
                           --build-arg RUSTUP_DIST_SERVER=$(RUSTUP_DIST_SERVER) \
                           --build-arg GITHUB_PROXY=$(GITHUB_PROXY)

# =============================================================================
# 🐳 DOCKER — BUILD TARGETS
# =============================================================================

DOCKER_IMAGE    ?= hotplex
DOCKER_TAG      ?= latest
DOCKER_REGISTRY ?= ghcr.io/hrygo
HOST_UID        ?= $(shell id -u)
STACK_TAG       ?= latest

VALID_STACKS := go node python java rust full
STACK_VERSION_go     := 1.26
STACK_VERSION_node   := 24
STACK_VERSION_python := 3.14
STACK_VERSION_java   := 21
STACK_VERSION_rust   := 1.94

# Proxy & mirror config (optimized for mainland China)
HTTP_PROXY         ?= http://host.docker.internal:7897
HTTPS_PROXY        ?= http://host.docker.internal:7897
DEBIAN_MIRROR      ?= mirrors.aliyun.com
ALPINE_MIRROR      ?= mirrors.aliyun.com
NPM_MIRROR         ?= https://registry.npmmirror.com
PYTHON_MIRROR      ?= https://pypi.tuna.tsinghua.edu.cn/simple
GOPROXY            ?= https://goproxy.cn,direct
RUSTUP_DIST_SERVER ?= https://rsproxy.cn
GITHUB_PROXY       ?= https://mirror.ghproxy.com/

# Reusable build arg blocks
DOCKER_BUILD_COMMON_ARGS := \
	--build-arg HOST_UID=$(HOST_UID) \
	--build-arg VERSION=$(VERSION) \
	--build-arg COMMIT=$(COMMIT) \
	--build-arg BUILD_TIME=$(BUILD_TIME)

DOCKER_BUILD_PROXY_ARGS := \
	--build-arg HTTP_PROXY=$(HTTP_PROXY) \
	--build-arg HTTPS_PROXY=$(HTTPS_PROXY) \
	--build-arg ALPINE_MIRROR=$(ALPINE_MIRROR) \
	--build-arg NPM_MIRROR=$(NPM_MIRROR) \
	--build-arg PYTHON_MIRROR=$(PYTHON_MIRROR) \
	--build-arg GOPROXY=$(GOPROXY) \
	--build-arg RUSTUP_DIST_SERVER=$(RUSTUP_DIST_SERVER) \
	--build-arg DEBIAN_MIRROR=$(DEBIAN_MIRROR) \
	--build-arg GITHUB_PROXY=$(GITHUB_PROXY)

# --- Build ---

docker-build-artifacts: ## @docker Build the HotPlex binary provider
	@printf "${CYAN}🏗️  Building hotplex:artifacts...$(NC)\n"
	@docker build -f docker/Dockerfile.artifacts \
		$(DOCKER_BUILD_COMMON_ARGS) \
		--build-arg GOPROXY=$(GOPROXY) \
		-t hotplex:artifacts .
	@printf "${GREEN}✅ Built hotplex:artifacts$(NC)\n"

docker-build-foundation: ## @docker Build the shared foundation image (hotplex:base)
	@printf "${CYAN}🏗️  Building hotplex:base (Foundation)...$(NC)\n"
	@docker build -f docker/Dockerfile.base \
		$(DOCKER_BUILD_COMMON_ARGS) \
		$(DOCKER_BUILD_PROXY_ARGS) \
		-t hotplex:base .
	@printf "${GREEN}✅ Built hotplex:base$(NC)\n"

docker-build-base: docker-build-foundation ## @docker Alias for foundation build

docker-build-go: docker-build-foundation docker-build-artifacts ## @docker Build the Go stack
	@printf "${CYAN}🏗️  Building hotplex:go...$(NC)\n"
	@docker build -f docker/Dockerfile.golang \
		$(DOCKER_BUILD_COMMON_ARGS) \
		-t hotplex:go .
	@printf "${GREEN}✅ Built hotplex:go$(NC)\n"

docker-build-stack: docker-build-foundation docker-build-artifacts ## @docker Build a tech-stack image. Usage: make docker-build-stack S=node
	@if [ -z "$(S)" ]; then \
		printf "${RED}❌ Error: S=<stack> is required. Options: $(VALID_STACKS)$(NC)\n"; \
		exit 1; \
	fi
	@printf "${CYAN}🔨 Building hotplex:$(S)...$(NC)\n"
	@if [ "$(S)" = "go" ]; then \
		docker build -f docker/Dockerfile.golang \
			$(DOCKER_BUILD_COMMON_ARGS) \
			$(DOCKER_BUILD_PROXY_ARGS) \
			-t hotplex:$(S) .; \
	else \
		docker build -f docker/Dockerfile.$(S) \
			$(DOCKER_BUILD_COMMON_ARGS) \
			$(DOCKER_BUILD_PROXY_ARGS) \
			-t hotplex:$(S) .; \
	fi
	@printf "${GREEN}✅ Built hotplex:$(S)$(NC)\n"

docker-build-all: docker-build-artifacts ## @docker Build all tech-stack images sequentially
	@printf "${CYAN}🔨 Building all stacks...$(NC)\n"
	@for s in $(VALID_STACKS); do \
		printf "${CYAN}  → Building hotplex:$$s...$(NC)\n"; \
		$(MAKE) docker-build-stack S=$$s || exit 1; \
	done
	@printf "${GREEN}🎉 All stacks built!$(NC)\n"

# --- Runtime ---
docker-prepare: ## @docker Prepare host directories for all bot instances
	@mkdir -p $(HOST_CONFIGS_DIR)
	@mkdir -p $(HOME_DIR)/.claude
	@printf "${CYAN}📂 Preparing bot instances...$(NC)\n"
	@for f in docker/matrix/.env-*; do \
		ID=$$(grep "^HOTPLEX_BOT_ID=" $$f | cut -d= -f2 | tr -d ' ' | tr -d '\r'); \
		if [ -n "$$ID" ]; then \
			printf "  - Instance: ${BOLD}$$ID$(NC)\n"; \
			mkdir -p $(HOME_DIR)/.hotplex/instances/$$ID/storage; \
			mkdir -p $(HOME_DIR)/.hotplex/instances/$$ID/claude; \
			mkdir -p $(HOME_DIR)/.hotplex/instances/$$ID/projects; \
		fi; \
	done
	@printf "${GREEN}✅ Host environment ready$(NC)\n"

docker-up: claude-seed docker-prepare docker-sync ## @docker Start Matrix services using REMOTE image
	@IMG=$$(cd docker/matrix && docker compose config --images 2>/dev/null | head -n 1); \
	[ -z "$$IMG" ] && IMG="ghcr.io/hrygo/hotplex:latest-go (default)"; \
	printf "${YELLOW}🚀 Environment: MATRIX (REMOTE)$(NC)\n"; \
	printf "${PURPLE}🐳 Image: ${BOLD}$$IMG$(NC)\n"; \
	cd docker/matrix && \
		HOST_UID=$(HOST_UID) \
		VERSION=$(VERSION) \
		COMMIT=$(COMMIT) \
		BUILD_TIME=$(BUILD_TIME) \
		HOTPLEX_HOST_CONFIGS_DIR=$(HOST_CONFIGS_DIR) \
		docker compose up -d

docker-dev: docker-prepare docker-sync ## @docker Start Matrix services using LOCAL image (hotplex:go)
	@printf "${YELLOW}🚀 Environment: LOCAL DEVELOPMENT$(NC)\n"; \
	printf "${PURPLE}🐳 Image: ${BOLD}hotplex:go$(NC)\n"; \
	cd docker/matrix && \
		HOTPLEX_IMAGE=hotplex:go \
		HOST_UID=$(HOST_UID) \
		VERSION=$(VERSION) \
		COMMIT=$(COMMIT) \
		BUILD_TIME=$(BUILD_TIME) \
		HOTPLEX_HOST_CONFIGS_DIR=$(HOST_CONFIGS_DIR) \
		docker compose up -d

docker-dev-all: docker-build-go docker-dev ## @docker Rebuild local Go image and start dev services

docker-down: ## @docker Stop and remove services
	cd docker/matrix && docker compose down --timeout 30

docker-restart: ## @docker Restart services (down → sync → up)
	@$(MAKE) docker-down
	@sleep 2
	@$(MAKE) docker-up

docker-logs: ## @docker Follow container logs (Ctrl+C to stop)
	cd docker/matrix && docker compose logs -f

# --- OpenCode Server Management ---
.PHONY: opencode-start opencode-stop opencode-restart opencode-status opencode-logs \
        opencode-config opencode-verify opencode-password opencode-docker-integrate

OPENCODE_PORT ?= 4096
OPENCODE_BINARY ?= opencode
OPENCODE_LOG_DIR ?= $(HOME_DIR)/.hotplex/logs
OPENCODE_LOG ?= $(OPENCODE_LOG_DIR)/opencode-server.log
OPENCODE_PID_FILE ?= $(HOME_DIR)/.hotplex/.opencode-server.pid
OPENCODE_PASSWORD_FILE ?= $(HOME_DIR)/.hotplex/.opencode-password
OPENCODE_DEBUG ?= false

opencode-config: ## @opencode Display OpenCode server configuration
	@$(call SECTION_HEADER,📋 OpenCode Server Configuration)
	@printf "  ${CYAN}Binary:$(NC) $(OPENCODE_BINARY)\n"
	@printf "  ${CYAN}Port:$(NC) $(OPENCODE_PORT)\n"
	@printf "  ${CYAN}Log File:$(NC) $(OPENCODE_LOG)\n"
	@printf "  ${CYAN}PID File:$(NC) $(OPENCODE_PID_FILE)\n"
	@printf "  ${CYAN}Password File:$(NC) $(OPENCODE_PASSWORD_FILE)\n"
	@if [ -f "$(OPENCODE_PASSWORD_FILE)" ]; then \
		printf "  ${GREEN}✓$(NC) Password configured\n"; \
	else \
		printf "  ${YELLOW}⚠$(NC) No password set (run: make opencode-password)\n"; \
	fi
	@printf "${CYAN}Environment Variables:$(NC)\n"
	@printf "  HOTPLEX_OPEN_CODE_SERVER_URL: $${HOTPLEX_OPEN_CODE_SERVER_URL:-not set}\n"
	@printf "  HOTPLEX_OPEN_CODE_PASSWORD: $${HOTPLEX_OPEN_CODE_PASSWORD:-not set}\n"
	@printf "  HOTPLEX_OPEN_CODE_PORT: $${HOTPLEX_OPEN_CODE_PORT:-$(OPENCODE_PORT)}\n"

opencode-verify: ## @opencode Verify OpenCode binary and dependencies
	@$(call SECTION_HEADER,🔍 Verifying OpenCode Dependencies)
	@printf "${CYAN}→ Checking binary...$(NC)\n"
	@if ! command -v $(OPENCODE_BINARY) >/dev/null 2>&1; then \
		printf "${RED}✗ OpenCode binary not found: $(OPENCODE_BINARY)$(NC)\n"; \
		printf "${YELLOW}  Install: https://github.com/opencode-ai/opencode$(NC)\n"; \
		exit 1; \
	else \
		printf "${GREEN}✓ Binary found: $$(command -v $(OPENCODE_BINARY))$(NC)\n"; \
	fi
	@printf "${CYAN}→ Checking port availability...$(NC)\n"
	@if lsof -i :$(OPENCODE_PORT) >/dev/null 2>&1; then \
		printf "${YELLOW}⚠ Port $(OPENCODE_PORT) is already in use$(NC)\n"; \
		lsof -i :$(OPENCODE_PORT); \
	else \
		printf "${GREEN}✓ Port $(OPENCODE_PORT) is available$(NC)\n"; \
	fi
	@printf "${GREEN}✅ Verification complete$(NC)\n"

opencode-password: ## @opencode Generate or update OpenCode server password
	@$(call SECTION_HEADER,🔐 Managing OpenCode Password)
	@mkdir -p $(HOME_DIR)/.hotplex
	@if [ -f "$(OPENCODE_PASSWORD_FILE)" ]; then \
		printf "${YELLOW}⚠ Password file exists: $(OPENCODE_PASSWORD_FILE)$(NC)\n"; \
		read -p "Overwrite? (y/N): " CONFIRM; \
		if [ "$$CONFIRM" != "y" ] && [ "$$CONFIRM" != "Y" ]; then \
			printf "${CYAN}→ Cancelled$(NC)\n"; \
			exit 0; \
		fi; \
	fi
	@printf "${CYAN}→ Generating secure password...$(NC)\n"
	@openssl rand -base64 32 > $(OPENCODE_PASSWORD_FILE)
	@chmod 600 $(OPENCODE_PASSWORD_FILE)
	@printf "${GREEN}✓ Password saved to: $(OPENCODE_PASSWORD_FILE)$(NC)\n"
	@printf "${CYAN}→ Password (copy this):$(NC)\n"
	@cat $(OPENCODE_PASSWORD_FILE)
	@printf "\n${YELLOW}⚠ Add to .env:$(NC)\n"
	@printf "HOTPLEX_OPEN_CODE_PASSWORD=$$(cat $(OPENCODE_PASSWORD_FILE))\n"

opencode-start: opencode-verify ## @opencode Start OpenCode server in daemon mode
	@$(call SECTION_HEADER,🚀 Starting OpenCode Server)
	@mkdir -p $(OPENCODE_LOG_DIR)
	@if lsof -i :$(OPENCODE_PORT) >/dev/null 2>&1; then \
		EXISTING_PID=$$(lsof -t -i :$(OPENCODE_PORT)); \
		printf "${YELLOW}⚠ Port $(OPENCODE_PORT) already in use by PID $$EXISTING_PID$(NC)\n"; \
		printf "${CYAN}→ Run 'make opencode-stop' first or choose a different port$(NC)\n"; \
		exit 1; \
	fi
	@printf "${CYAN}→ Port: $(OPENCODE_PORT)$(NC)\n"
	@printf "${CYAN}→ Log: $(OPENCODE_LOG)$(NC)\n"
	@if [ -f "$(OPENCODE_PASSWORD_FILE)" ]; then \
		export OPENCODE_SERVER_PASSWORD=$$(cat $(OPENCODE_PASSWORD_FILE)); \
		printf "${GREEN}✓ Using password from file$(NC)\n"; \
	elif [ -n "$$HOTPLEX_OPEN_CODE_PASSWORD" ]; then \
		export OPENCODE_SERVER_PASSWORD=$$HOTPLEX_OPEN_CODE_PASSWORD; \
		printf "${GREEN}✓ Using password from env$(NC)\n"; \
	else \
		printf "${YELLOW}⚠ No password configured (unprotected mode)$(NC)\n"; \
	fi; \
	DEBUG_FLAG=""; \
	if [ "$(OPENCODE_DEBUG)" = "true" ]; then \
		DEBUG_FLAG="--debug"; \
		printf "${PURPLE}→ Debug mode enabled$(NC)\n"; \
	fi; \
	{ cd $(HOME_DIR) && exec $(OPENCODE_BINARY) serve --port $(OPENCODE_PORT) $$DEBUG_FLAG; } >> $(OPENCODE_LOG) 2>&1 & \
		sleep 1; PID=$$(lsof -t -i :$(OPENCODE_PORT) 2>/dev/null); \
		[ -n "$$PID" ] && echo $$PID > $(OPENCODE_PID_FILE); \
		printf "${GREEN}✓ OpenCode server started (PID: $$(cat $(OPENCODE_PID_FILE) 2>/dev/null))${NC}\n"
	@printf "${CYAN}→ Waiting for server to be ready...$(NC)\n"
	@sleep 2
	@$(MAKE) opencode-status

opencode-stop: ## @opencode Stop OpenCode server
	@$(call SECTION_HEADER,🛑 Stopping OpenCode Server)
	@if lsof -t -i :$(OPENCODE_PORT) >/dev/null 2>&1; then \
		PIDS=$$(lsof -t -i :$(OPENCODE_PORT)); \
		for PID in $$PIDS; do \
			PGID=$$(ps -o pgid= -p $$PID 2>/dev/null | tr -d " "); \
			[ -n "$$PGID" ] && kill -$$PGID 2>/dev/null; \
		done; \
		sleep 1; \
		REMAINING=$$(lsof -t -i :$(OPENCODE_PORT) 2>/dev/null); \
		[ -n "$$REMAINING" ] && kill -9 $$REMAINING 2>/dev/null; \
		rm -f $(OPENCODE_PID_FILE); \
		printf "${GREEN}✓ OpenCode server stopped${NC}\n"; \
	elif [ -f "$(OPENCODE_PID_FILE)" ]; then \
		rm -f $(OPENCODE_PID_FILE); \
		printf "${YELLOW}⚠ Server not running on port $(OPENCODE_PORT), cleaned up stale PID file${NC}\n"; \
	else \
		printf "${YELLOW}⚠ Server not running${NC}\n"; \
	fi

opencode-restart: ## @opencode Restart OpenCode server
	@$(MAKE) opencode-stop
	@sleep 2
	@$(MAKE) opencode-start

opencode-status: ## @opencode Check OpenCode server status and health
	@$(call SECTION_HEADER,🔍 OpenCode Server Status)
	@if lsof -t -i :$(OPENCODE_PORT) >/dev/null 2>&1; then \
		PID=$$(lsof -t -i :$(OPENCODE_PORT)); \
		printf "${GREEN}✓ Server running (PID: $$PID)${NC}\n"; \
		printf "${CYAN}→ Health check:${NC}\n"; \
		HTTP_RESP=$$(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:$(OPENCODE_PORT)/ 2>/dev/null); \
		if [ "$$HTTP_RESP" = "200" ]; then \
			printf "  ${GREEN}✓ HTTP endpoint healthy (200 OK)${NC}\n"; \
		elif [ "$$HTTP_RESP" = "401" ] || [ "$$HTTP_RESP" = "403" ]; then \
			printf "  ${GREEN}✓ Server online (auth required, HTTP $$HTTP_RESP)${NC}\n"; \
		else \
			printf "  ${RED}✗ HTTP endpoint error (HTTP $$HTTP_RESP)${NC}\n"; \
		fi; \
		printf "${CYAN}→ Process info:${NC}\n"; \
		ps -p $$PID -o pid,ppid,%cpu,%mem,etime,command 2>/dev/null || true; \
	else \
		printf "${YELLOW}⚠ Server not running on port $(OPENCODE_PORT)${NC}\n"; \
	fi

opencode-logs: ## @opencode View OpenCode server logs (Ctrl+C to stop)
	@if [ -f "$(OPENCODE_LOG)" ]; then \
		tail -f $(OPENCODE_LOG); \
	else \
		printf "${YELLOW}⚠ No log file found at $(OPENCODE_LOG)$(NC)\n"; \
	fi

opencode-docker-integrate: ## @opencode Integrate OpenCode server with Docker Compose
	@$(call SECTION_HEADER,🐳 Docker Integration Setup)
	@printf "${CYAN}→ Checking Docker Compose configuration...$(NC)\n"
	@if [ ! -f "docker/matrix/docker-compose.yml" ]; then \
		printf "${RED}✗ docker-compose.yml not found$(NC)\n"; \
		exit 1; \
	fi
	@printf "${CYAN}→ Adding OpenCode sidecar service...$(NC)\n"
	@printf "${YELLOW}⚠ Manual step required:$(NC)\n"
	@printf "Add this to docker/matrix/docker-compose.yml:\n\n"
	@printf "  opencode-server:\n"
	@printf "    image: opencode/opencode:latest\n"
	@printf "    command: serve --port 4096\n"
	@printf "    ports:\n"
	@printf "      - \"4096:4096\"\n"
	@printf "    networks:\n"
	@printf "      - hotplex-network\n"
	@printf "    environment:\n"
	@printf "      - OPEN_CODE_PASSWORD=$${HOTPLEX_OPEN_CODE_PASSWORD}\n\n"
	@printf "${CYAN}→ Update HotPlex service to depend on OpenCode:$(NC)\n"
	@printf "In your HotPlex service, add:\n"
	@printf "    depends_on:\n"
	@printf "      - opencode-server\n"
	@printf "    environment:\n"
	@printf "      - HOTPLEX_OPEN_CODE_SERVER_URL=http://opencode-server:4096\n"
	@printf "${GREEN}✓ Docker integration guide complete$(NC)\n"

opencode-test: ## @opencode Run OpenCode server verification tests
	@$(call SECTION_HEADER,🧪 Testing OpenCode Server)
	@if [ ! -f "scripts/test_opencode_server.py" ]; then \
		printf "${YELLOW}⚠ Test script not found: scripts/test_opencode_server.py$(NC)\n"; \
		printf "${CYAN}→ Running basic health check instead...$(NC)\n"; \
		$(MAKE) opencode-status; \
		exit 0; \
	fi
	@printf "${CYAN}→ Running Python verification script...$(NC)\n"
	@if [ -f "$(OPENCODE_PASSWORD_FILE)" ]; then \
		python3 scripts/test_opencode_server.py --password $$(cat $(OPENCODE_PASSWORD_FILE)); \
	elif [ -n "$$HOTPLEX_OPEN_CODE_PASSWORD" ]; then \
		python3 scripts/test_opencode_server.py --password $$HOTPLEX_OPEN_CODE_PASSWORD; \
	else \
		python3 scripts/test_opencode_server.py; \
	fi

opencode-logs-truncate: ## @opencode Truncate OpenCode server logs (keep last 1000 lines)
	@$(call SECTION_HEADER,✂️ Truncating OpenCode Logs)
	@if [ -f "$(OPENCODE_LOG)" ]; then \
		tail -n 1000 "$(OPENCODE_LOG)" > "$(OPENCODE_LOG).tmp" && \
		mv "$(OPENCODE_LOG).tmp" "$(OPENCODE_LOG)" && \
		printf "${GREEN}✓ Log truncated to last 1000 lines$(NC)\n"; \
		printf "${CYAN}→ Size: $$(du -h $(OPENCODE_LOG) | cut -f1)$(NC)\n"; \
	else \
		printf "${YELLOW}⚠ No log file found$(NC)\n"; \
	fi

opencode-with-hotplex: opencode-start ## @opencode Start OpenCode server then start HotPlex
	@$(call SECTION_HEADER,🔥 Starting HotPlex with OpenCode)
	@printf "${CYAN}→ OpenCode server is ready$(NC)\n"
	@printf "${CYAN}→ Starting HotPlex daemon...$(NC)\n"
	@sleep 2
	@$(MAKE) run

# --- Interactive Shell ---

# Enter a container as the hotplex user.
# Usage:
#   make docker-shell              # Interactive (select from running services)
#   make docker-shell SVC=hotplex-01  # Direct jump to specific service
#   make docker-shell SVC=hotplex-01 SHELL=zsh
docker-shell: ## @docker Interactive shell: auto-detect → select service → enter as hotplex user
	@cd docker/matrix && { \
		RUNNING=$$(docker compose ps --services --status running 2>/dev/null); \
		if [ -z "$$RUNNING" ]; then \
			printf "%s❌ No running services in docker/matrix/%s\n" "$(RED)" "$(NC)"; \
			exit 1; \
		fi; \
		set -- $$RUNNING; \
		if [ -n "$(SVC)" ]; then \
			SVC_NAME="$(SVC)"; \
		elif [ $$# -eq 1 ]; then \
			SVC_NAME="$$1"; \
			printf "%s→ Auto-entering: %s%s%s\n" "$(CYAN)" "$(BOLD)" "$$SVC_NAME" "$(NC)"; \
		else \
			printf "\n%s╭─ Select Service ───────────────────────────────%s\n" "$(BOLD)$(CYAN)" "$(NC)"; \
			i=1; \
			for svc in $$RUNNING; do \
				printf "  [%d] %s\n" "$$i" "$$svc"; \
				i=$$((i+1)); \
			done; \
			printf "%s  [Enter] select first service (default)%s\n" "$(DIM)" "$(NC)"; \
			printf "%s╰─────────────────────────────────────────────────%s\n" "$(BOLD)$(CYAN)" "$(NC)"; \
			read -r CHOICE; \
			if [ -z "$$CHOICE" ]; then \
				SVC_NAME="$$1"; \
			else \
				i=1; \
				for svc in $$RUNNING; do \
					if [ $$i -eq $$CHOICE ]; then SVC_NAME="$$svc"; fi; \
					i=$$((i+1)); \
				done; \
			fi; \
		fi; \
		CONTAINER=$$(docker compose ps -q "$$SVC_NAME" 2>/dev/null); \
		if [ -z "$$CONTAINER" ]; then \
			printf "%s❌ Container '%s' not found or not running.%s\n" "$(RED)" "$$SVC_NAME" "$(NC)"; \
			exit 1; \
		fi; \
		printf "%s→ Entering %s%s%s%s as %shotplex%s...%s\n" "$(CYAN)" "$(BOLD)" "$$SVC_NAME" "$(NC)" "$(CYAN)" "$(BOLD)" "$(NC)" "$(CYAN)"; \
		docker exec -u hotplex -it "$$CONTAINER" $(or $(SHELL),/bin/bash); \
	}


# --- Config Sync ---


# --- Claude Configuration Seed --

.PHONY: claude-seed claude-seed-verify
claude-seed: ## @docker Process ~/.claude/ for container compatibility
	@echo "$(CYAN)🔄 Processing ~/.claude/ for container compatibility...$(NC)"
	@./scripts/claude-seed-processor.sh
	@echo "${GREEN}✓$(NC) Claude seed ready at ~/.hotplex/claude-seed/"

claude-seed-verify: ## @docker Verify no hardcoded paths in seed
	@echo "$(CYAN)🔍 Verifying claude seed...$(NC)"
	@if grep -r "$(whoami)" ~/.hotplex/claude-seed/ 2>/dev/null; then \
		echo "$(RED}❌ ERROR: Found hardcoded paths$(NC)"; \
		exit 1; \
	else \
		echo "$(GREEN}✓ No hardcoded paths found$(NC)"; \
	fi

add-bot: ## @docker Interactive bot instance creation
	@./docker/matrix/add-bot.sh

docker-sync: docker-prepare ## @docker Sync configs to all Docker instances
	@$(call SECTION_HEADER,🔄 Syncing Docker Instance Configs)

	# Sync Claude statusline.sh to seed directory (if updated on host)
	@if [ -f "$(HOME)/.claude/statusline.sh" ]; then \
		printf "  ${CYAN}→$(NC) Syncing statusline.sh to seed...\n"; \
		mkdir -p $(HOME)/.hotplex/claude-seed; \
		cp $(HOME)/.claude/statusline.sh $(HOME)/.hotplex/claude-seed/statusline.sh; \
		printf "  ${GREEN}✓$(NC) statusline.sh synced to seed\n"; \
	fi

	# Sync instance-specific configs
	@for f in docker/matrix/.env-*; do \
		ID=$$(grep "^HOTPLEX_BOT_ID=" $$f | cut -d= -f2 | tr -d ' ' | tr -d '\r'); \
		BOT_NUM=$$(basename $$f | sed 's/.env-//'); \
		if [ -n "$$ID" ]; then \
			INSTANCE_DIR=$(HOME)/.hotplex/instances/$$ID/configs; \
			mkdir -p "$$INSTANCE_DIR/base"; \
			mkdir -p $(HOME)/.hotplex/instances/$$ID/claude; \
			mkdir -p $(HOME)/.hotplex/instances/$$ID/projects; \
			mkdir -p $(HOME)/.hotplex/instances/$$ID/sessions; \
			mkdir -p $(HOME)/.hotplex/instances/$$ID/storage; \
			printf "  ${CYAN}→$(NC) Syncing ${BOLD}$$ID$(NC) (bot-$$BOT_NUM)...\n"; \
			cp "$$f" "$(HOME)/.hotplex/instances/$$ID/.env"; \
			cp -r configs/base/* "$$INSTANCE_DIR/base/"; \
			if [ -d "docker/matrix/configs/bot-$$BOT_NUM" ]; then \
				cp docker/matrix/configs/bot-$$BOT_NUM/*.yaml "$$INSTANCE_DIR/" 2>/dev/null || true; \
			fi; \
			printf "${GREEN}✓$(NC) Synced ${BOLD}$$ID$(NC)\n"; \
		fi; \
	done
	@printf "${GREEN}✅ All Docker instances synced$(NC)\n"

docker-health: ## @docker Show health status of all services
	cd docker/matrix && for svc in $$(docker compose ps --services 2>/dev/null); do \
		status=$$(docker inspect --format='{{.State.Health.Status}}' $$svc 2>/dev/null || echo "not_found"); \
		printf "  $$svc: $$status\n"; \
	done

docker-check-net: ## @docker Test proxy connectivity from inside containers
	cd docker/matrix && for svc in $$(docker compose ps --services 2>/dev/null); do \
		printf "  $$svc: "; \
		docker exec $$svc nc -zv host.docker.internal 15721 2>&1 | grep -q succeeded && printf "LLM Proxy OK, " || printf "LLM Proxy FAIL, "; \
		docker exec $$svc nc -zv host.docker.internal 7897 2>&1 | grep -q succeeded && printf "General Proxy OK\n" || printf "General Proxy FAIL\n"; \
	done

docker-upgrade: ## @docker Pull latest images and restart services
	@printf "${CYAN}🚀 Pulling latest images...$(NC)\n"
	cd docker/matrix && docker compose pull
	@$(MAKE) docker-restart

docker-clean: ## @docker Remove all local hotplex stack images
	@for s in $(VALID_STACKS); do docker rmi -f hotplex:$$s 2>/dev/null || true; done
	@docker rmi -f hotplex:$(STACK_TAG) hotplex:base 2>/dev/null || true
	@printf "${GREEN}✅ Local images removed$(NC)\n"

# Short aliases
stack: docker-build-stack
stack-all: docker-build-all
stack-clean: docker-clean

.PHONY: all help build build-all fmt vet test test-unit test-ci test-race test-integration test-all lint tidy clean \
        install-hooks run stop restart docs svg2png config-info sync add-bot \
        service-install service-uninstall service-start service-stop service-restart \
        service-status service-logs service-enable service-disable \
        docker-build-base docker-build-app docker-build-stack docker-build-all \
        docker-up docker-down docker-restart docker-logs docker-sync docker-shell \
        docker-health docker-check-net docker-upgrade docker-clean \
        stack stack-all stack-clean \
        opencode-config opencode-verify opencode-password opencode-start opencode-stop \
        opencode-restart opencode-status opencode-logs opencode-docker-integrate \
        opencode-test opencode-logs-truncate opencode-with-hotplex

