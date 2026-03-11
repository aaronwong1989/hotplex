# HotPlex Makefile
# A premium CLI experience for building and managing HotPlex

# Colors for UI
CYAN          := \033[0;36m
GREEN         := \033[0;32m
YELLOW        := \033[1;33m
RED           := \033[0;31m
PURPLE        := \033[0;35m
BLUE          := \033[0;34m
BOLD          := \033[1m
DIM           := \033[2m
NC            := \033[0m

# Metadata
BINARY_NAME   := hotplexd
CMD_PATH      := ./cmd/hotplexd
DIST_DIR      := dist
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT        ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS       := -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(BUILD_TIME)'

LOG_DIR       := .logs
LOG_FILE      := $(LOG_DIR)/daemon.log

.PHONY: all help build build-all fmt vet test test-unit test-ci test-race test-integration test-all lint tidy clean install-hooks run stop restart docs svg2png service-install service-uninstall service-start service-stop service-restart service-status service-logs service-enable service-disable

# Default target
all: help

# Service management script
SERVICE_SCRIPT := ./scripts/service.sh

# =============================================================================
# 🎯 Helper: Styled Section Header
# =============================================================================
define SECTION_HEADER
printf "\n${BOLD}${BLUE}╭─ %s ────────────────────────────────────${NC}\n" "$1"
endef

define SECTION_FOOTER
printf "${DIM}${BLUE}╰─────────────────────────────────────────────${NC}\n"
endef

# =============================================================================
# 📋 HELP
# =============================================================================
help: ## Show this help message
	@printf "\n${BOLD}${CYAN}🔥 HotPlex Build System${NC} ${DIM}${VERSION}${NC}\n"
	@printf "${DIM}Usage: make ${YELLOW}<target>${NC} ${DIM}[args]${NC}\n"
	@printf "\n"
	@$(call SECTION_HEADER,🔨 Build)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@build/ {gsub(/@build /, "", $$2); printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🧪 Test)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@test/ {gsub(/@test /, "", $$2); printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🔧 Development)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@dev/ {gsub(/@dev /, "", $$2); printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🚀 Runtime)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@runtime/ {gsub(/@runtime /, "", $$2); printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,📦 Service)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@service/ {gsub(/@service /, "", $$2); printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🛠️ Utils)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@util/ {gsub(/@util /, "", $$2); printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@$(call SECTION_HEADER,🐳 Docker)
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## .*$$/ && /@docker/ {gsub(/@docker /, "", $$2); printf "  ${GREEN}%-20s${NC} %s\n", $$1, $$2}' $(MAKEFILE_LIST) | sort
	@$(call SECTION_FOOTER)
	@printf "\n${DIM}💡 Tip: Use 'make <target> V=1' for verbose output${NC}\n\n"

# =============================================================================
# 🔨 BUILD
# =============================================================================
build: fmt vet tidy ## @build Compile the hotplexd daemon
	@printf "${GREEN}🚀 Building HotPlex Daemon (${VERSION})...${NC}\n"
	@mkdir -p $(DIST_DIR)
	@go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME) $(CMD_PATH)
	@printf "${GREEN}✅ Build complete: ${DIST_DIR}/$(BINARY_NAME)${NC}\n"

# =============================================================================
# 🔧 INSTALL
# =============================================================================
install: build config-info ## @runtime Install and run with config info
	@printf "${PURPLE}🔥 Starting HotPlex Daemon...${NC}\n"
	@./$(DIST_DIR)/$(BINARY_NAME)

config-info: ## @util Display current configuration status
	@printf "\n${BOLD}${CYAN}╭─ 🔧 Configuration Files ─────────────────────────────${NC}\n"
	@printf "  ${BOLD}📋 Configuration Priority (effective):${NC}\n"
	@printf "\n"
	@printf "  ${GREEN}1. Main Config (.env)${NC}\n"
	@if [ -f .env ]; then \
		printf "     ${GREEN}✓${NC} Active\n"; \
		printf "     ${CYAN}Path:${NC} $$(pwd)/.env\n"; \
	else \
		printf "     ${YELLOW}⚠${NC} Not found\n"; \
		printf "     ${CYAN}Template:${NC} $$(pwd)/.env.example\n"; \
	fi
	@printf "\n"
	@printf "  ${GREEN}2. ChatApps Configs (priority order)${NC}\n"
	@printf "     ${CYAN}a) --config flag / HOTPLEX_CHATAPPS_CONFIG_DIR:${NC}\n"
	@if [ -n "$$HOTPLEX_CHATAPPS_CONFIG_DIR" ]; then \
		printf "         ${GREEN}✓${NC} Using: $$HOTPLEX_CHATAPPS_CONFIG_DIR\n"; \
	else \
		printf "         ${YELLOW}Not set${NC}\n"; \
	fi
	@printf "     ${CYAN}b) User config (~/.hotplex/configs):${NC}\n"
	@if [ -d "$HOME/.hotplex/configs" ]; then \
		printf "         ${GREEN}✓${NC} Active\n"; \
		printf "         ${CYAN}Path:${NC} $$HOME/.hotplex/configs/\n"; \
	else \
		printf "         ${YELLOW}⚠${NC} Not found${NC}\n"; \
	fi
	@printf "     ${CYAN}c) Default (./configs/chatapps):${NC}\n"
	@if [ -d "configs/chatapps" ]; then \
		printf "         ${GREEN}✓${NC} Active\n"; \
		printf "         ${CYAN}Path:${NC} $$(pwd)/configs/chatapps/\n"; \
		for f in configs/chatapps/*.yaml; do \
			if [ -f "$$f" ]; then \
				printf "            - $$(basename $$f)\n"; \
			fi; \
		done; \
	else \
		printf "         ${YELLOW}⚠${NC} Not found${NC}\n"; \
	fi
	@printf "${BOLD}${CYAN}╰─────────────────────────────────────────────────────${NC}\n\n"

build-all: fmt vet tidy ## @build Compile for all platforms (Linux/macOS/Windows)
	@printf "${GREEN}🚀 Building HotPlex Daemon for all platforms (${VERSION})...${NC}\n"
	@mkdir -p $(DIST_DIR)
	@GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)
	@GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)
	@GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)
	@GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)
	@GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)
	@printf "${GREEN}✅ Cross-compilation complete in ${DIST_DIR}/${NC}\n"

# =============================================================================
# 🧪 TEST
# =============================================================================
test: test-unit ## @test Run fast unit tests (default)

test-unit: ## @test Run unit tests (fast, no race detection)
	@printf "${CYAN}🧪 Running fast unit tests...${NC}\n"
	@go test -v -short ./...
	@printf "${GREEN}✅ Unit tests passed!${NC}\n"

test-ci: ## @test Run tests optimized for CI (parallel, timeout, short mode)
	@printf "${CYAN}🧪 Running CI-optimized tests...${NC}\n"
	@go test -v -short -timeout=5m -parallel=4 ./...
	@printf "${GREEN}✅ CI tests passed!${NC}\n"

test-race: ## @test Run unit tests with race detection
	@printf "${CYAN}🧪 Running unit tests with race detection...${NC}\n"
	@go test -v -race ./...
	@printf "${GREEN}✅ Race detection passed!${NC}\n"

test-integration: ## @test Run heavy integration tests
	@printf "${YELLOW}🏗️  Running heavy integration tests...${NC}\n"
	@go test -v -tags=integration ./...
	@printf "${GREEN}✅ Integration tests passed!${NC}\n"

test-all: test-race test-integration ## @test Run all tests

coverage: ## @test Generate coverage report
	@printf "${CYAN}📊 Generating coverage report...${NC}\n"
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@printf "${GREEN}✅ Coverage report generated: coverage.out${NC}\n"

coverage-html: coverage ## @test Generate coverage HTML report
	@go tool cover -html=coverage.out -o coverage.html
	@printf "${GREEN}✅ Coverage HTML report: coverage.html${NC}\n"

# =============================================================================
# 🔧 DEVELOPMENT
# =============================================================================
fmt: ## @dev Format Go code
	@printf "${CYAN}🔧 Formatting code...${NC}\n"
	@go fmt ./...

vet: ## @dev Check for suspicious constructs
	@printf "${CYAN}🔍 Vetting code...${NC}\n"
	@go vet ./...

lint: ## @dev Run golangci-lint
	@printf "${PURPLE}🔍 Linting code...${NC}\n"
	@if command -v golangci-lint &> /dev/null; then \
		golangci-lint run ./...; \
		printf "${GREEN}✅ Linting passed!${NC}\n"; \
	else \
		printf "${RED}❌ golangci-lint not found. Install it first.${NC}\n"; \
		exit 1; \
	fi

tidy: ## @dev Clean up go.mod dependencies
	@printf "${YELLOW}📦 Tidying up Go modules...${NC}\n"
	@go mod tidy
	@printf "${GREEN}✅ Modules synchronized.${NC}\n"

clean: ## @dev Remove build artifacts
	@printf "${RED}🧹 Cleaning up build artifacts...${NC}\n"
	@rm -rf $(DIST_DIR)
	@go clean
	@printf "${GREEN}✅ Cleanup done.${NC}\n"

install-hooks: ## @dev Install Git hooks
	@printf "${CYAN}🔗 Installing HotPlex Git Hooks...${NC}\n"
	@chmod +x scripts/*.sh 2>/dev/null || true
	@if [ -d scripts ] && [ -f scripts/setup_hooks.sh ]; then ./scripts/setup_hooks.sh; fi
	@printf "${GREEN}✅ Hooks are active.${NC}\n"

# =============================================================================
# 🚀 RUNTIME
# =============================================================================
run: build config-info ## @runtime Build and start daemon in foreground
	@printf "${PURPLE}🔥 Starting HotPlex Daemon...${NC}\n"
	@./$(DIST_DIR)/$(BINARY_NAME)


stop: ## @runtime Stop the running daemon and all its child processes
	@printf "${YELLOW}🛑 Stopping HotPlex Daemon...${NC}\n"
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
		printf "${GREEN}✅ Daemon stopped${NC}\n"; \
	else \
		printf "${YELLOW}⚠️  No running daemon found${NC}\n"; \
	fi

restart: build config-info ## @runtime Restart daemon with latest source code
	@mkdir -p $(LOG_DIR)
	@./scripts/restart_helper.sh "$$(pwd)/$(DIST_DIR)/$(BINARY_NAME)" "$(LOG_FILE)"

# =============================================================================
# 📦 SERVICE (System Service)
# =============================================================================
service-install: build ## @service Install as system service
	@printf "${CYAN}📦 Installing HotPlex as system service...${NC}\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) install

service-uninstall: ## @service Remove the system service
	@printf "${YELLOW}🗑️  Removing HotPlex system service...${NC}\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) uninstall

service-start: ## @service Start the system service
	@printf "${GREEN}▶️  Starting HotPlex service...${NC}\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) start

service-stop: ## @service Stop the system service
	@printf "${YELLOW}⏹️  Stopping HotPlex service...${NC}\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) stop

service-restart: ## @service Restart the system service
	@printf "${PURPLE}🔄 Restarting HotPlex service...${NC}\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) restart

service-status: ## @service Check service status
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) status

service-logs: ## @service Tail service logs (Ctrl+C to stop)
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) logs

service-enable: ## @service Enable auto-start on boot
	@printf "${GREEN}🔔 Enabling auto-start...${NC}\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) enable

service-disable: ## @service Disable auto-start on boot
	@printf "${YELLOW}🔕 Disabling auto-start...${NC}\n"
	@chmod +x $(SERVICE_SCRIPT)
	@$(SERVICE_SCRIPT) disable

# =============================================================================
# 🛠️ UTILS
# =============================================================================
svg2png: ## @util Convert SVG to 4K PNG
	@printf "${CYAN}🖼️  Converting SVG to PNG...${NC}\n"
	@chmod +x scripts/svg2png.sh 2>/dev/null || true
	@./scripts/svg2png.sh
	@printf "${GREEN}✅ PNG assets generated in docs/images/png/${NC}\n"

# =============================================================================
# 🐳 DOCKER & STACKS (Consolidated)
# =============================================================================

DOCKER_IMAGE    ?= hotplex
DOCKER_TAG      ?= latest
DOCKER_REGISTRY ?= ghcr.io/hrygo
HOST_UID        ?= $(shell id -u)
STACK_TAG       ?= latest

# 代理与源配置 (Optimized for mainland China)
HTTP_PROXY       ?= http://host.docker.internal:7897
HTTPS_PROXY      ?= http://host.docker.internal:7897
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

# DRY 构建函数
define do_stack_build
	@printf "${CYAN}🔨 Building HotPlex + $(1) stack...${NC}\n"
	@if [ "$(1)" = "go" ]; then \
		docker build -f docker/Dockerfile \
			$(DOCKER_BUILD_COMMON_ARGS) \
			$(DOCKER_BUILD_PROXY_ARGS) \
			-t hotplex:$(STACK_TAG) -t hotplex:go -t hotplex:go-$(STACK_VERSION_go) .; \
	else \
		docker build -f docker/Dockerfile.$(1) \
			$(DOCKER_BUILD_COMMON_ARGS) \
			$(DOCKER_BUILD_PROXY_ARGS) \
			-t hotplex:$(1) $(if $(2),-t hotplex:$(1)-$(2),) .; \
	fi
	@printf "${GREEN}✅ Built hotplex:$(1)${NC}\n"
endef

db: ## @docker Container build (T=<type> [S=<stack>]). T: app, base, stack, all, cache
	@case "$(T)" in \
		base) \
			printf "${CYAN}🏗️  Building hotplex:base फाउंडेशन...${NC}\n"; \
			docker build -t hotplex:base $(DOCKER_BUILD_COMMON_ARGS) --build-arg HTTP_PROXY=$(HTTP_PROXY) --build-arg HTTPS_PROXY=$(HTTPS_PROXY) -f docker/Dockerfile.base . ;; \
		stack) \
			if [ -z "$(S)" ]; then printf "${RED}❌ Error: Use S=<name> (Options: $(VALID_STACKS))${NC}\n"; exit 1; fi; \
			case "$(S)" in \
				go|node|python|java|rust|full) $(call do_stack_build,$(S),$(STACK_VERSION_$(S))) ;; \
				*) printf "${RED}❌ Invalid stack '$(S)'${NC}\n"; exit 1 ;; \
			esac ;; \
		all) \
			$(MAKE) db T=base; \
			for s in $(VALID_STACKS); do $(MAKE) db T=stack S=$$s; done; \
			printf "${GREEN}🎉 All stacks built!${NC}\n" ;; \
		cache) \
			printf "${CYAN}🐳 Building with cache...${NC}\n"; \
			docker build --cache-from $(DOCKER_IMAGE):latest $(DOCKER_BUILD_COMMON_ARGS) -t $(DOCKER_IMAGE):$(DOCKER_TAG) . ;; \
		app|*) \
			printf "${CYAN}🐳 Building app image...${NC}\n"; \
			$(MAKE) db T=base; \
			COMPOSE_FILE=docker-compose.yml:docker-compose.build.yml HOST_UID=$(HOST_UID) VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) docker compose build hotplex ;; \
	esac

d: ## @docker Lifecycle management (C=<cmd>). C: up, down, restart, logs, health, check-net, sync, upgrade, clean
	@case "$(C)" in \
		sync) \
			printf "${CYAN}🔄 Syncing configs...${NC}\n"; \
			mkdir -p $(HOME)/.hotplex/configs; \
			cp configs/chatapps/*.yaml $(HOME)/.hotplex/configs/ ;; \
		up) \
			$(MAKE) d C=sync; \
			HOST_UID=$(HOST_UID) VERSION=$(VERSION) COMMIT=$(COMMIT) BUILD_TIME=$(BUILD_TIME) docker compose up -d ;; \
		down) \
			docker compose down --timeout 30 ;; \
		restart) \
			$(MAKE) d C=down; sleep 2; $(MAKE) d C=up ;; \
		logs) \
			docker compose logs -f ;; \
		health) \
			for svc in $$(docker compose ps --services 2>/dev/null); do \
				status=$$(docker inspect --format='{{.State.Health.Status}}' $$svc 2>/dev/null || echo "not_found"); \
				printf "  $$svc: $$status\n"; \
			done ;; \
		check-net) \
			for svc in $$(docker compose ps --services 2>/dev/null); do \
				printf "  $$svc: "; \
				docker exec $$svc nc -zv host.docker.internal 15721 2>&1 | grep -q succeeded && printf "LLM Proxy OK, " || printf "LLM Proxy FAIL, "; \
				docker exec $$svc nc -zv host.docker.internal 7897 2>&1 | grep -q succeeded && printf "General Proxy OK\n" || printf "General Proxy FAIL\n"; \
			done ;; \
		upgrade) \
			docker compose pull; $(MAKE) d C=restart ;; \
		clean) \
			for s in $(VALID_STACKS); do docker rmi -f hotplex:$$s 2>/dev/null || true; done; \
			docker rmi -f hotplex:$(STACK_TAG) 2>/dev/null || true ;; \
		*) \
			printf "${RED}❌ Error: Please specify command C=<up|down|restart|logs|health|sync|upgrade|clean>${NC}\n"; exit 1 ;; \
	esac

# Convenience Aliases
docker-up: ## @docker Start services
	@$(MAKE) d C=up
docker-down: ## @docker Stop services
	@$(MAKE) d C=down
docker-restart: ## @docker Sync & Restart
	@$(MAKE) d C=restart
docker-logs: ## @docker View logs
	@$(MAKE) d C=logs
docker-sync: ## @docker Sync configs
	@$(MAKE) d C=sync
docker-health: ## @docker Check health
	@$(MAKE) d C=health
docker-check-net: ## @docker Check network
	@$(MAKE) d C=check-net

stack: ## @docker Build specific stack (S=node)
	@$(MAKE) db T=stack S=$(S)
stack-all: ## @docker Build all stacks
	@$(MAKE) db T=all
stack-clean: ## @docker Remove stacks
	@$(MAKE) d C=clean

.PHONY: all help build build-all fmt vet test test-unit test-race test-integration test-all lint tidy clean install-hooks run stop restart docs svg2png service-install service-uninstall service-start service-stop service-restart service-status service-logs service-enable service-disable config-info d db docker-up docker-down docker-restart docker-logs docker-sync docker-health docker-check-net stack stack-all stack-clean
