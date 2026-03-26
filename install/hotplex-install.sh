#!/bin/bash
# ==============================================================================
# HotPlex 一键安装脚本 v2.1
# ==============================================================================
# 用法:
#   curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install.sh | bash
#   curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install.sh | bash -s -- -v v0.21.0
#
# 参考: https://github.com/hrygo/hotplex/blob/main/docs/superpowers/specs/2026-03-24-one-click-install-design.md
# ==============================================================================

set -euo pipefail

# ==============================================================================
# 全局变量
# ==============================================================================
readonly REPO="hrygo/hotplex"
readonly BINARY_NAME="hotplexd"
readonly SCRIPT_VERSION="3.0.0"
readonly DEFAULT_INSTALL_DIR="/usr/local/bin"
readonly CONFIG_DIR="${HOME}/.hotplex"
readonly GITHUB_API="https://api.github.com/repos"
readonly LOG_FILE="${CONFIG_DIR}/install.log"

setup_secure_logging() {
    mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || sudo mkdir -p "$(dirname "$LOG_FILE")"
    touch "$LOG_FILE" 2>/dev/null || sudo touch "$LOG_FILE"
    chmod 600 "$LOG_FILE" 2>/dev/null || sudo chmod 600 "$LOG_FILE"
}

# 可配置变量
VERSION=""
INSTALL_DIR=""
CONFIG_ONLY=false
UNINSTALL=false
DO_UPGRADE=false
DO_STATUS=false
DO_PURGE=false
DRY_RUN=false
VERBOSE=false
QUIET=false
SKIP_VERIFY=false
SKIP_HEALTH_CHECK=false
SKIP_AUTOSTART=false
SKIP_WIZARD=false
FORCE=false
INTERACTIVE=true
INSTALL_MODE="binary"  # binary | docker
MULTI_BOT_ENABLED=false
BOT_COUNT=1

# Docker 相关
DOCKER_COMPOSE_PATH="${CONFIG_DIR}/docker-compose.yml"
DOCKER_ENV_PATH="${CONFIG_DIR}/.env.docker"

# 日志轮转
LOGROTATE_PATH="/etc/logrotate.d/hotplex"
MAX_LOG_SIZE="10M"
MAX_LOG_FILES=5

# 颜色定义 (非 readonly，允许 init_colors 修改)
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
DIM='\033[2m'
UNDERLINE='\033[4m'
NC='\033[0m'

# 临时文件
TEMP_DIR=""
CLEANUP_PENDING=true

# ==============================================================================
# 工具函数
# ==============================================================================

# 初始化颜色
init_colors() {
    if [[ ! -t 1 ]] || [[ "${NO_COLOR:-}" == "true" ]]; then
        RED="" GREEN="" YELLOW="" BLUE="" CYAN="" MAGENTA="" BOLD="" DIM="" UNDERLINE="" NC=""
    fi
}

# 日志函数
log() {
    local level="$1"; shift
    local msg="$*"
    local timestamp=$(date '+%H:%M:%S')

    case "$level" in
        info)    [[ "$QUIET" == "true" ]] && return; echo -e "${BLUE}▸${NC} $msg" ;;
        success) [[ "$QUIET" == "true" ]] && return; echo -e "${GREEN}✓${NC} $msg" ;;
        warn)    echo -e "${YELLOW}!${NC} $msg" >&2 ;;
        error)   echo -e "${RED}✗${NC} $msg" >&2 ;;
        debug)   [[ "$VERBOSE" == "true" ]] && echo -e "${DIM}[DEBUG]${NC} $msg" ;;
        raw)     [[ "$QUIET" == "true" ]] && return; echo -e "$msg" ;;
        step)    [[ "$QUIET" == "true" ]] && return; echo -e "${CYAN}→${NC} $msg" ;;
    esac

    # 写入日志文件
    if [[ -d "$(dirname "$LOG_FILE")" ]] || mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null; then
        echo "[$timestamp] [$level] $msg" >> "$LOG_FILE" 2>/dev/null || true
    fi
}

info()    { log info "$*"; }
success() { log success "$*"; }
warn()    { log warn "$*"; }
error()   { log error "$*"; exit 1; }
debug()   { log debug "$*"; }
raw()     { log raw "$*"; }
step()    { log step "$*"; }

# 静默执行命令（忽略错误）
silent_run() {
    "$@" 2>/dev/null || true
}

# Docker Compose 封装（自动降级）
docker_compose() {
    if docker compose "$@" 2>/dev/null; then
        return 0
    elif docker-compose "$@" 2>/dev/null; then
        return 0
    fi
    return 1
}

# 进度指示器
show_spinner() {
    local pid=$1
    local msg="$2"
    local spin='⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏'
    local i=0

    while kill -0 $pid 2>/dev/null; do
        i=$(( (i+1) % 10 ))
        printf "\r${CYAN}${spin:$i:1}${NC} ${msg}..."
        sleep 0.1
    done
    printf "\r"
}

# 清理函数
cleanup() {
    if [[ -n "$TEMP_DIR" ]] && [[ -d "$TEMP_DIR" ]] && [[ "$CLEANUP_PENDING" == "true" ]]; then
        rm -rf "$TEMP_DIR"
        debug "已清理临时目录: $TEMP_DIR"
    fi
}

# 错误处理
on_error() {
    local exit_code=$?
    local line_no=$1
    echo ""
    error "安装失败 (第 ${line_no} 行, 退出码: ${exit_code})"
    echo ""
    echo "  故障排除:"
    echo "    1. 检查网络连接"
    echo "    2. 使用 -V 查看详细日志"
    echo "    3. 查看日志: ${LOG_FILE}"
    echo ""
}

# 设置 trap
setup_traps() {
    trap cleanup EXIT
    trap 'on_error $LINENO' ERR
}

# 检查命令是否存在
command_exists() {
    command -v "$1" &>/dev/null
}

# 用户确认
confirm() {
    local prompt="$1"
    local default="${2:-n}"

    if [[ "$INTERACTIVE" != "true" ]] || [[ ! -t 0 ]]; then
        [[ "$default" == "y" ]] && return 0 || return 1
    fi

    local choices
    [[ "$default" == "y" ]] && choices="[Y/n]" || choices="[y/N]"

    echo -ne "${BOLD}?${NC} ${prompt} ${choices}: "
    read -r response
    response=${response:-$default}

    [[ "$response" =~ ^[Yy] ]]
}

# 用户输入
prompt_input() {
    local prompt="$1"
    local default="${2:-}"
    local secret="${3:-false}"

    if [[ "$INTERACTIVE" != "true" ]] || [[ ! -t 0 ]]; then
        echo "$default"
        return
    fi

    echo -ne "${BOLD}?${NC} ${prompt}"
    [[ -n "$default" ]] && echo -ne " [${default}]"
    echo -ne ": "

    if [[ "$secret" == "true" ]]; then
        read -rs response
        echo
    else
        read -r response
    fi

    echo "${response:-$default}"
}

# ==============================================================================
# Phase 2: 敏感信息打码
# ==============================================================================

# 打码敏感信息（Token、密码等）
mask_token() {
    local token="$1"
    local visible="${2:-4}"

    if [[ -z "$token" ]] || [[ ${#token} -le $((visible * 2)) ]]; then
        echo "****"
        return
    fi

    local prefix="${token:0:$visible}"
    local suffix="${token: -$visible}"
    echo "${prefix}...${suffix}"
}

# 带打码的调试日志
debug_token() {
    local label="$1"
    local token="$2"

    if [[ "$VERBOSE" == "true" ]]; then
        debug "${label}: $(mask_token "$token")"
    fi
}

# ==============================================================================
# Phase 2: 端口冲突检测与解决
# ==============================================================================

# 检查端口是否被占用
check_port() {
    local port="${1:-8080}"

    if command_exists lsof; then
        silent_run lsof -ti ":${port}"
    elif command_exists ss; then
        silent_run ss -tlnp | grep ":${port}" | grep -oE 'pid=[0-9]+' | cut -d= -f2
    elif command_exists netstat; then
        silent_run netstat -tlnp | grep ":${port}" | grep -oE '[0-9]+/' | cut -d/ -f1
    fi
}

# 解决端口冲突
resolve_port_conflict() {
    local port="${1:-8080}"
    local pid
    pid=$(check_port "$port")

    if [[ -z "$pid" ]]; then
        return 0
    fi

    warn "端口 ${port} 已被占用 (PID: $pid)"

    local process_name
    if command_exists ps; then
        process_name=$(ps -p "$pid" -o comm= 2>/dev/null || echo "unknown")
    fi

    echo ""
    echo -e "  ${BOLD}请选择解决方案:${NC}"
    echo ""
    echo "    1) 终止现有进程 (PID: $pid - $process_name)"
    echo "    2) 使用备用端口 (${port}+1)"
    echo "    3) 取消安装"
    echo ""

    local choice
    choice=$(prompt_input "选择解决方案" "1")

    case "$choice" in
        1)
            info "正在终止进程 PID $pid..."
            if kill "$pid" 2>/dev/null; then
                sleep 1
                # 验证是否已终止
                if check_port "$port" &>/dev/null; then
                    warn "进程未能终止，尝试强制终止..."
                    kill -9 "$pid" 2>/dev/null || true
                    sleep 1
                fi
                if check_port "$port" &>/dev/null; then
                    error "无法终止进程"
                else
                    success "进程已终止"
                fi
            else
                sudo kill "$pid" 2>/dev/null || error "无法终止进程 (需要 sudo 权限)"
            fi
            HOTPLEX_PORT="$port"
            ;;
        2)
            local new_port=$((port + 1))
            while check_port "$new_port" &>/dev/null; do
                new_port=$((new_port + 1))
            done
            HOTPLEX_PORT="$new_port"
            info "将使用备用端口: ${HOTPLEX_PORT}"
            ;;
        3|*)
            error "安装已取消"
            ;;
    esac

    return 0
}

# 检查依赖
check_dependencies() {
    local missing=()
    local optional_missing=()

    # 必需工具
    if ! command_exists curl && ! command_exists wget; then
        missing+=("curl 或 wget")
    fi

    if ! command_exists tar && ! command_exists unzip; then
        missing+=("tar 或 unzip")
    fi

    # 可选工具（用于向导）
    command_exists jq || optional_missing+=("jq (API 验证)")
    command_exists openssl || optional_missing+=("openssl (生成密钥)")

    if [[ ${#missing[@]} -gt 0 ]]; then
        error "缺少必需依赖: ${missing[*]}\n请安装后重试"
    fi

    if [[ ${#optional_missing[@]} -gt 0 ]] && [[ "$VERBOSE" == "true" ]]; then
        warn "可选依赖未安装: ${optional_missing[*]}"
    fi

    debug "依赖检查通过"
}

# 检测操作系统
detect_os() {
    local os
    case "$(uname -s)" in
        Linux*)  os="linux" ;;
        Darwin*) os="darwin" ;;
        CYGWIN*|MINGW*|MSYS*) os="windows" ;;
        *) error "不支持的操作系统: $(uname -s)" ;;
    esac
    echo "$os"
}

# 检测架构
detect_arch() {
    local arch
    case "$(uname -m)" in
        x86_64|amd64)  arch="amd64" ;;
        arm64|aarch64) arch="arm64" ;;
        *) error "不支持的架构: $(uname -m)" ;;
    esac
    echo "$arch"
}

# HTTP 请求
http_get() {
    local url="$1"
    local output="${2:-}"

    debug "HTTP GET: $url"

    if command_exists curl; then
        local curl_opts=(-fsSL --connect-timeout 30 --max-time 300)
        [[ "$VERBOSE" == "true" ]] && curl_opts+=(-v)

        if [[ -n "$output" ]]; then
            curl "${curl_opts[@]}" -o "$output" "$url"
        else
            curl "${curl_opts[@]}" "$url"
        fi
    elif command_exists wget; then
        local wget_opts=(-q --timeout=30)
        [[ "$VERBOSE" == "true" ]] && wget_opts=()

        if [[ -n "$output" ]]; then
            wget "${wget_opts[@]}" -O "$output" "$url"
        else
            wget "${wget_opts[@]}" -O- "$url"
        fi
    fi
}

# 下载文件（带重试和进度）
download_with_retry() {
    local url="$1"
    local output="$2"
    local max_retries="${3:-3}"
    local retry=0

    while [[ $retry -lt $max_retries ]]; do
        debug "下载尝试 $((retry + 1))/$max_retries: $url"

        if http_get "$url" "$output"; then
            [[ -f "$output" ]] && [[ -s "$output" ]] && return 0
        fi

        retry=$((retry + 1))
        if [[ $retry -lt $max_retries ]]; then
            warn "下载失败，${retry}秒后重试..."
            sleep $retry
        fi
    done

    error "下载失败 (重试 $max_retries 次后): $url"
}

# 获取最新版本
get_latest_version() {
    local version

    # 方法1: GitHub API
    if command_exists curl; then
        version=$(curl -fsSL "${GITHUB_API}/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | cut -d'"' -f4 | sed 's/v//' || true)
        if [[ -n "$version" ]]; then
            echo "$version"
            return 0
        fi
    fi


    # 方法2: 重定向解析（使用 http_get）
    version=$(http_get "https://github.com/${REPO}/releases/latest" | grep -oE 'tag/v?[0-9]+\.[0-9]+\.[0-9]+' | head -1 | sed 's|tag/||' | sed 's|^v||' || true)
    [[ -n "$version" ]] && { echo "$version"; return 0; }

    # 方法3: curl 头信息
    if command_exists curl; then
        version=$(curl -sIo- "https://github.com/${REPO}/releases/latest" 2>/dev/null | grep -i "location:" | sed -E 's/.*\/v?([^\/]+).*/\1/' | tr -d '\r' || true)
        [[ -n "$version" ]] && { echo "$version"; return 0; }
    fi

    return 1
}

# 获取已安装版本
get_installed_version() {
    local binary="${1:-${INSTALL_DIR}}/${BINARY_NAME}"

    if [[ -x "$binary" ]]; then
        "$binary" -version 2>/dev/null | head -1 | sed -E 's/v?([0-9]+\.[0-9]+\.[0-9]+).*/\1/' || echo "unknown"
    fi
}

# 下载校验和文件
download_checksums() {
    local version="$1"
    local output="$2"
    local url="https://github.com/${REPO}/releases/download/v${version}/SHA256SUMS"

    debug "下载校验和: $url"
    http_get "$url" "$output" 2>/dev/null || return 1
    [[ -f "$output" ]] && [[ -s "$output" ]]
}

# 验证校验和
verify_checksum() {
    local archive="$1"
    local checksums_file="$2"
    local archive_name=$(basename "$archive")

    if ! command_exists sha256sum && ! command_exists shasum; then
        warn "无法验证校验和: 缺少 sha256sum 或 shasum"
        return 0
    fi

    debug "验证校验和: $archive_name"

    local expected checksum
    if command_exists sha256sum; then
        expected=$(grep "$archive_name" "$checksums_file" | awk '{print $1}')
        checksum=$(sha256sum "$archive" | awk '{print $1}')
    else
        expected=$(grep "$archive_name" "$checksums_file" | awk '{print $1}')
        checksum=$(shasum -a 256 "$archive" | awk '{print $1}')
    fi

    if [[ "$expected" == "$checksum" ]]; then
        debug "校验和验证通过"
        return 0
    else
        error "校验和验证失败!\n期望: $expected\n实际: $checksum"
    fi
}

# 备份现有安装
backup_existing() {
    local binary="${INSTALL_DIR}/${BINARY_NAME}"

    if [[ -f "$binary" ]]; then
        local backup="${CONFIG_DIR}/backups/${BINARY_NAME}.$(date +%Y%m%d%H%M%S)"
        mkdir -p "${CONFIG_DIR}/backups"
        info "备份现有安装..."
        cp "$binary" "$backup"
        success "备份保存到: $backup"
    fi
}

# 检查已安装版本
check_existing_installation() {
    local current_version
    current_version=$(get_installed_version)

    if [[ -n "$current_version" ]] && [[ "$current_version" != "unknown" ]]; then
        info "检测到已安装版本: ${GREEN}$current_version${NC}"

        if [[ "$FORCE" != "true" ]]; then
            if [[ "$VERSION" == "$current_version" ]] || [[ "$VERSION" == "v${current_version}" ]]; then
                warn "版本 $VERSION 已安装"
                if confirm "是否强制重新安装?" "n"; then
                    FORCE=true
                else
                    echo ""
                    info "使用 ${BINARY_NAME} -version 查看版本"
                    exit 0
                fi
            fi
        fi
    fi
}

# ==============================================================================
# 验证函数
# ==============================================================================

# 验证 Slack Token 格式
# 格式: xoxb-{team_id}-{app_id}-{secret} 或 xoxb-{team_id}-{secret}
# 长度要求: >= 20 字符
validate_slack_token() {
    local token="$1"
    # 放宽验证：只检查前缀和基本结构
    [[ "$token" =~ ^xoxb-[a-zA-Z0-9_-]+$ ]] && [[ ${#token} -ge 20 ]]
}

# 验证 Slack App Token 格式
# 格式: xapp-1-{install_id}-{secret}
# 长度要求: >= 30 字符
validate_slack_app_token() {
    local token="$1"
    # 放宽验证：检查前缀、版本号和基本结构
    [[ "$token" =~ ^xapp-[0-9]+-[a-zA-Z0-9]+$ ]] && [[ ${#token} -ge 30 ]]
}

# 验证 Slack User ID 格式
# 前缀: U (用户), B (Bot), W (Enterprise/Workspace 用户)
# 长度要求: 9-15 字符
validate_slack_user_id() {
    local user_id="$1"
    # 放宽验证：支持 U/B/W 前缀 + 字母数字，长度 9-15
    [[ "$user_id" =~ ^[UBW][A-Z0-9]+$ ]] && [[ ${#user_id} -ge 9 ]] && [[ ${#user_id} -le 15 ]]
}

# 验证 GitHub Token 格式
validate_github_token() {
    local token="$1"
    [[ "$token" =~ ^ghp_[a-zA-Z0-9]{36}$ ]] || [[ "$token" =~ ^github_pat_[a-zA-Z0-9_]+$ ]]
}

# 验证 Slack API 连接
test_slack_connection() {
    local token="$1"

    if ! command_exists curl; then
        warn "无法测试连接: 缺少 curl"
        return 0
    fi

    debug "测试 Slack API 连接..."

    local response
    response=$(curl -fsSL -H "Authorization: Bearer $token" \
        "https://slack.com/api/auth.test" 2>/dev/null || echo '{"ok":false}')

    if echo "$response" | grep -q '"ok":true'; then
        return 0
    else
        local error=$(echo "$response" | grep -oE '"error"[[:space:]]*:[[:space:]]*"[^"]*"' | cut -d'"' -f4 || echo "unknown")
        debug "Slack API 错误: $error"
        return 1
    fi
}

# ==============================================================================
# 核心功能
# ==============================================================================

# 帮助信息
show_help() {
    cat << 'EOF'
HotPlex 一键安装脚本 v2.1

用法:
  curl -sL https://raw.githubusercontent.com/hrygo/hotplex/main/install.sh | bash
  install.sh [命令] [选项]

命令:
  (无命令)             交互式菜单 (默认)
  install              安装 HotPlex (默认，二进制模式)
  docker               安装 HotPlex (Docker 模式)
  upgrade              升级 HotPlex (保留配置)
  status               查看状态
  uninstall            卸载 HotPlex (保留配置)
  purge                完全清理 (删除所有数据)
  config               仅运行配置向导

选项:
  -v, --version VERSION  指定安装版本 (默认: 最新版本)
  -d, --dir DIR          安装目录 (默认: /usr/local/bin)
  -m, --mode MODE        安装模式: binary(默认) 或 docker
  -c, --config           仅运行配置向导
  -u, --uninstall        卸载 HotPlex
      --upgrade          升级 HotPlex
      --status           查看状态
      --purge            完全清理
  -f, --force            强制重新安装
  -n, --dry-run          干运行模式，显示将执行的操作
  -q, --quiet            静默模式
  -V, --verbose          详细输出
  --skip-verify          跳过校验和验证
  --skip-health-check    跳过健康检查
  --skip-autostart       跳过开机自启配置
  --skip-wizard          跳过安装后配置向导
  --non-interactive      非交互模式
  -h, --help             显示帮助信息
  --version              显示脚本版本

示例:
  install.sh                     # 交互式菜单
  install.sh install             # 安装最新版本 + 配置向导
  install.sh -v v0.21.0          # 安装指定版本
  install.sh -m docker           # Docker 模式安装
  install.sh upgrade             # 升级到最新版本
  install.sh status              # 查看状态
  install.sh uninstall           # 卸载
  install.sh purge               # 完全清理
  install.sh config              # 仅运行配置向导

环境变量:
  NO_COLOR=true                  禁用颜色输出

更多信息: https://github.com/hrygo/hotplex/blob/main/INSTALL.md
EOF
    exit 0
}

# 显示版本
show_version() {
    echo "HotPlex 安装脚本 v${SCRIPT_VERSION}"
    exit 0
}

# 解析参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -v|--version)         VERSION="$2"; shift 2 ;;
            -d|--dir)             INSTALL_DIR="$2"; shift 2 ;;
            -m|--mode)            INSTALL_MODE="$2"; shift 2 ;;
            -c|--config)          CONFIG_ONLY=true; shift ;;
            -u|--uninstall)       UNINSTALL=true; shift ;;
            --upgrade)            DO_UPGRADE=true; shift ;;
            --status)             DO_STATUS=true; shift ;;
            --purge)              DO_PURGE=true; shift ;;
            -f|--force)           FORCE=true; shift ;;
            -n|--dry-run)         DRY_RUN=true; shift ;;
            -q|--quiet)           QUIET=true; shift ;;
            -V|--verbose)         VERBOSE=true; shift ;;
            --skip-verify)        SKIP_VERIFY=true; shift ;;
            --skip-health-check)  SKIP_HEALTH_CHECK=true; shift ;;
            --skip-autostart)     SKIP_AUTOSTART=true; shift ;;
            --skip-wizard)        SKIP_WIZARD=true; shift ;;
            --non-interactive)   INTERACTIVE=false; shift ;;
            -h|--help)            show_help ;;
            --version)            show_version ;;
            -*)                   error "未知选项: $1\n使用 -h 查看帮助" ;;
            *)                    break ;;
        esac
    done

    # 设置默认值
    INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

    # 冲突检查
    [[ "$QUIET" == "true" ]] && [[ "$VERBOSE" == "true" ]] && warn "同时设置了 -q 和 -V，忽略 -q" || true

    # 模式验证
    case "$INSTALL_MODE" in
        binary|docker) ;;
        *) error "无效安装模式: $INSTALL_MODE (支持: binary, docker)" ;;
    esac
}

# 卸载
do_uninstall() {
    info "卸载 HotPlex..."
    local binary="${INSTALL_DIR}/${BINARY_NAME}"

    if [[ ! -f "$binary" ]]; then
        warn "HotPlex 未安装在 $binary"
        exit 0
    fi

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] 将删除: $binary"
        return
    fi

    # 检查是否在运行
    if pgrep -x "$BINARY_NAME" &>/dev/null; then
        warn "HotPlex 正在运行"
        if confirm "是否停止并卸载?" "y"; then
            pkill -x "$BINARY_NAME" 2>/dev/null || true
            sleep 1
        else
            exit 1
        fi
    fi

    if [[ -w "$INSTALL_DIR" ]]; then
        rm -f "$binary"
    else
        sudo rm -f "$binary"
    fi

    success "已删除: $binary"

    # 清理备份
    local backups="${CONFIG_DIR}/backups"
    if [[ -d "$backups" ]]; then
        local count=$(find "$backups" -name "${BINARY_NAME}.*" 2>/dev/null | wc -l)
        if [[ $count -gt 0 ]]; then
            info "发现 $count 个备份文件"
            if confirm "是否删除备份?" "n"; then
                rm -rf "$backups"
                success "已删除备份"
            fi
        fi
    fi

    if [[ -d "$CONFIG_DIR" ]]; then
        echo ""
        info "配置目录: $CONFIG_DIR"
        if confirm "是否删除配置目录?" "n"; then
            rm -rf "$CONFIG_DIR"
            success "已删除配置目录"
        fi
    fi

    success "卸载完成"
}

# ==============================================================================
# Phase 3: 升级功能
# ==============================================================================

do_upgrade() {
    info "升级 HotPlex..."

    local os arch current_version
    os=$(detect_os)
    arch=$(detect_arch)
    current_version=$(get_installed_version "${INSTALL_DIR}/${BINARY_NAME}")

    if [[ -z "$current_version" ]] || [[ "$current_version" == "unknown" ]]; then
        error "未检测到已安装的 HotPlex，请先运行安装"
    fi

    info "当前版本: ${GREEN}${current_version}${NC}"

    # 获取目标版本
    if [[ -z "$VERSION" ]]; then
        step "获取最新版本..."
        VERSION=$(get_latest_version) || error "无法获取最新版本，请使用 -v 指定"
        [[ "$VERSION" != v* ]] && VERSION="v${VERSION}"
    fi

    local target_version="${VERSION#v}"
    if [[ "$current_version" == "$target_version" ]]; then
        success "已是最新版本 (${current_version})"
        exit 0
    fi

    info "目标版本: ${GREEN}${target_version}${NC}"

    # 确认升级
    if [[ "$INTERACTIVE" == "true" ]] && [[ -t 0 ]]; then
        if ! confirm "是否升级?" "y"; then
            info "升级已取消"
            exit 0
        fi
    fi

    # 下载新版本
    local archive_name="hotplex_${target_version}_${os}_${arch}"
    [[ "$os" == "windows" ]] && archive_name="${archive_name}.zip" || archive_name="${archive_name}.tar.gz"
    local archive_url="https://github.com/${REPO}/releases/download/v${target_version}/${archive_name}"

    TEMP_DIR=$(mktemp -d)
    local archive_path="${TEMP_DIR}/${archive_name}"

    step "正在下载 v${target_version}..."
    download_with_retry "$archive_url" "$archive_path"

    # 校验
    if [[ "$SKIP_VERIFY" != "true" ]]; then
        local checksums_path="${TEMP_DIR}/checksums.txt"
        if download_checksums "${target_version}" "$checksums_path"; then
            step "验证校验和..."
            verify_checksum "$archive_path" "$checksums_path"
        else
            warn "无法下载校验和文件，跳过验证"
        fi
    fi

    # 停止现有服务
    step "停止现有服务..."
    if pgrep -f "$BINARY_NAME" &>/dev/null; then
        pkill -f "$BINARY_NAME" 2>/dev/null || sudo pkill -f "$BINARY_NAME"
        sleep 1
    fi

    # 解压
    step "正在解压..."
    if [[ "$os" == "windows" ]]; then
        unzip -q "$archive_path" -d "$TEMP_DIR"
    else
        tar -xzf "$archive_path" -C "$TEMP_DIR"
    fi

    # 替换二进制
    step "正在更新二进制..."
    local binary_path="${TEMP_DIR}/${BINARY_NAME}"

    if [[ -w "$INSTALL_DIR" ]]; then
        cp "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        sudo cp "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # 验证
    local new_version
    new_version=$("${INSTALL_DIR}/${BINARY_NAME}" -version 2>/dev/null | head -1 || echo "unknown")
    success "升级完成: ${GREEN}${new_version}${NC}"

    # 重启服务
    step "重启服务..."
    if [[ "${SKIP_AUTOSTART:-false}" != "true" ]]; then
        install_autostart
    fi

    # 健康检查
    if [[ "${SKIP_HEALTH_CHECK:-false}" != "true" ]]; then
        health_check || true
    fi

    CLEANUP_PENDING=false
}

# ==============================================================================
# Phase 3: 状态检查
# ==============================================================================

do_status() {
    info "检查 HotPlex 状态..."
    echo ""

    local binary="${INSTALL_DIR}/${BINARY_NAME}"

    # 1. 检查二进制
    echo -e "  ${BOLD}二进制文件:${NC}"
    if [[ -x "$binary" ]]; then
        local installed_version
        installed_version=$("$binary" -version 2>/dev/null | head -1 || echo "unknown")
        echo -e "    ${GREEN}✓${NC} 已安装: $installed_version"
        echo -e "    ${DIM}   路径: ${INSTALL_DIR}/${BINARY_NAME}${NC}"
    else
        echo -e "    ${RED}✗${NC} 未安装或不可执行"
    fi

    # 2. 检查配置文件
    echo ""
    echo -e "  ${BOLD}配置文件:${NC}"
    local env_file="${CONFIG_DIR}/.env"
    if [[ -f "$env_file" ]]; then
        echo -e "    ${GREEN}✓${NC} 已配置: $env_file"

        # 显示关键配置（打码）
        local bot_token app_token
        bot_token=$(grep "^HOTPLEX_SLACK_BOT_TOKEN=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "")
        app_token=$(grep "^HOTPLEX_SLACK_APP_TOKEN=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "")
        local port
        port=$(grep "^HOTPLEX_PORT=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "8080")

        echo -e "    ${DIM}   Bot Token:  ${GREEN}$(mask_token "$bot_token" 8)${NC}"
        echo -e "    ${DIM}   App Token:  ${GREEN}$(mask_token "$app_token" 8)${NC}"
        echo -e "    ${DIM}   端口:      ${GREEN}${port}${NC}"
    else
        echo -e "    ${YELLOW}○${NC} 未配置 (运行安装)"
    fi

    # 3. 检查进程
    echo ""
    echo -e "  ${BOLD}运行状态:${NC}"
    local pid
    pid=$(pgrep -f "$BINARY_NAME" 2>/dev/null | head -1 || true)
    if [[ -n "$pid" ]]; then
        echo -e "    ${GREEN}✓${NC} 运行中 (PID: $pid)"

        # 内存使用
        if command_exists ps; then
            local mem
            mem=$(ps -p "$pid" -o rss= 2>/dev/null | tr -d ' ' || echo "unknown")
            if [[ "$mem" != "unknown" ]]; then
                echo -e "    ${DIM}   内存: $((mem / 1024)) MB${NC}"
            fi
        fi

        # 健康检查
        local port="${HOTPLEX_PORT:-8080}"
        local endpoint="http://localhost:${port}/health"
        if command_exists curl; then
            if curl -sf --connect-timeout 2 --max-time 5 "$endpoint" 2>/dev/null | grep -q "ok\|OK\|healthy"; then
                echo -e "    ${GREEN}✓${NC} 健康检查: 通过"
            else
                echo -e "    ${YELLOW}○${NC} 健康检查: 未通过 (服务可能仍在启动)"
            fi
        fi
    else
        echo -e "    ${YELLOW}○${NC} 未运行"
    fi

    # 4. 检查开机自启
    echo ""
    echo -e "  ${BOLD}开机自启:${NC}"
    local init_system
    init_system=$(detect_init_system)
    case "$init_system" in
        systemd)
            if systemctl is-enabled "${BINARY_NAME}.service" &>/dev/null; then
                echo -e "    ${GREEN}✓${NC} systemd 服务已启用"
            else
                echo -e "    ${YELLOW}○${NC} systemd 服务未启用"
            fi
            ;;
        launchd)
            if launchctl list | grep -q "com.hotplex.daemon"; then
                echo -e "    ${GREEN}✓${NC} launchd 服务已启用"
            else
                echo -e "    ${YELLOW}○${NC} launchd 服务未启用"
            fi
            ;;
        *)
            echo -e "    ${YELLOW}○${NC} 不支持的开机自启系统"
            ;;
    esac

    # 5. 备份信息
    local backups="${CONFIG_DIR}/backups"
    if [[ -d "$backups" ]]; then
        local backup_count
        backup_count=$(find "$backups" -name "${BINARY_NAME}.*" 2>/dev/null | wc -l | tr -d ' ' || echo "0")
        if [[ "$backup_count" -gt 0 ]]; then
            echo ""
            echo -e "  ${BOLD}备份:${NC}"
            echo -e "    ${GREEN}✓${NC} 备份文件: $backup_count 个 (${backups})"
        fi
    fi

    echo ""
}

# ==============================================================================
# Phase 3: 清理功能 (完全清理)
# ==============================================================================

do_purge() {
    info "完全清理 HotPlex..."

    # 检查是否安装
    local binary="${INSTALL_DIR}/${BINARY_NAME}"
    if [[ ! -f "$binary" ]] && [[ ! -d "$CONFIG_DIR" ]]; then
        warn "HotPlex 未安装，无需清理"
        exit 0
    fi

    # 确认危险操作
    if [[ "$INTERACTIVE" == "true" ]] && [[ -t 0 ]]; then
        echo ""
        raw "${RED}${BOLD}⚠️  警告: 此操作将删除所有数据!${NC}"
        echo ""
        echo "  将删除:"
        echo "    • 二进制文件: ${INSTALL_DIR}/${BINARY_NAME}"
        echo "    • 配置目录: ${CONFIG_DIR}"
        echo "    • 所有备份"
        echo "    • 开机自启服务"
        echo ""
        if ! confirm "确认完全清理?" "n"; then
            info "清理已取消"
            exit 0
        fi
    fi

    # 停止服务
    step "停止服务..."
    if pgrep -f "$BINARY_NAME" &>/dev/null; then
        pkill -f "$BINARY_NAME" 2>/dev/null || sudo pkill -f "$BINARY_NAME"
        sleep 1
    fi

    # 移除 systemd 服务
    if [[ -f "/etc/systemd/system/${BINARY_NAME}.service" ]]; then
        step "移除 systemd 服务..."
        sudo systemctl stop "${BINARY_NAME}.service" 2>/dev/null || true
        sudo systemctl disable "${BINARY_NAME}.service" 2>/dev/null || true
        sudo rm -f "/etc/systemd/system/${BINARY_NAME}.service"
        sudo systemctl daemon-reload
    fi

    # 移除 launchd 服务
    local plist_file="${HOME}/Library/LaunchAgents/com.hotplex.daemon.plist"
    if [[ -f "$plist_file" ]]; then
        step "移除 launchd 服务..."
        launchctl unload "$plist_file" 2>/dev/null || true
        rm -f "$plist_file"
    fi

    # 删除二进制
    step "删除二进制..."
    if [[ -f "$binary" ]]; then
        if [[ -w "$INSTALL_DIR" ]]; then
            rm -f "$binary"
        else
            sudo rm -f "$binary"
        fi
        success "已删除: $binary"
    fi

    # 删除配置目录（递归删除所有数据）
    if [[ -d "$CONFIG_DIR" ]]; then
        step "删除配置目录..."
        rm -rf "$CONFIG_DIR"
        success "已删除: $CONFIG_DIR"
    fi

    success "完全清理完成"
}

# ==============================================================================
# 配置向导
# ==============================================================================

# 向导：配置 Slack Bot 凭据
wizard_slack_credentials() {
    local env_file="${CONFIG_DIR}/.env"

    echo ""
    raw "${BOLD}${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    raw "${BOLD}  Step 1/2: Slack 凭据配置${NC}"
    raw "${BOLD}${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    if [[ ! -f "$env_file" ]]; then
        warn "配置文件不存在，请先运行安装"
        return 1
    fi

    # 读取当前配置
    local current_bot_token=$(grep "^HOTPLEX_SLACK_BOT_TOKEN=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "")
    local current_app_token=$(grep "^HOTPLEX_SLACK_APP_TOKEN=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "")
    local current_user_id=$(grep "^HOTPLEX_SLACK_BOT_USER_ID=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "")
    local current_github=$(grep "^GITHUB_TOKEN=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "")

    # 检查配置状态
    local has_valid_slack=false
    [[ "$current_bot_token" =~ ^xoxb- ]] && has_valid_slack=true

    echo -e "  ${BOLD}当前配置状态:${NC}"
    echo ""
    echo -e "    Slack Bot Token:    $([[ "$current_bot_token" =~ ^xoxb- ]] && echo "${GREEN}✓ 已配置${NC}" || echo "${YELLOW}○ 未配置${NC}")"
    echo -e "    Slack App Token:    $([[ "$current_app_token" =~ ^xapp- ]] && echo "${GREEN}✓ 已配置${NC}" || echo "${YELLOW}○ 未配置${NC}")"
    echo -e "    Slack Bot User ID:  $([[ "$current_user_id" =~ ^[UBW][A-Z0-9]+$ ]] && echo "${GREEN}✓ 已配置${NC}" || echo "${YELLOW}○ 未配置${NC}")"
    echo -e "    GitHub Token:       $([[ "$current_github" =~ ^ghp_ ]] && echo "${GREEN}✓ 已配置${NC}" || echo "${YELLOW}○ 未配置${NC}")"
    echo ""

    # 如果都已配置，询问是否重新配置
    if [[ "$has_valid_slack" == "true" ]]; then
        if ! confirm "是否重新配置 Slack?" "n"; then
            success "Slack 配置保持不变"
            return 0
        fi
    fi

    echo -e "  ${BOLD}${CYAN}如何获取 Slack 凭据:${NC}"
    echo ""
    echo -e "  ${DIM}1. 访问${NC} ${UNDERLINE}https://api.slack.com/apps${NC}"
    echo -e "  ${DIM}2. 创建新 App 或选择现有 App${NC}"
    echo -e "  ${DIM}3. 启用 Socket Mode (推荐)${NC}"
    echo ""

    # 交互式配置
    if [[ "$INTERACTIVE" == "true" ]] && [[ -t 0 ]]; then
        local bot_token app_token user_id github_token
        local updated=false

        # Bot Token
        echo -e "${CYAN}Bot User OAuth Token (xoxb-...)${NC}"
        echo -e "  ${DIM}→ OAuth & Permissions → Bot User OAuth Token${NC}"
        bot_token=$(prompt_input "请输入" "$current_bot_token" "true")

        if [[ -n "$bot_token" ]]; then
            if validate_slack_token "$bot_token"; then
                # 测试连接
                step "验证 Token..."
                if test_slack_connection "$bot_token"; then
                    success "Token 验证成功"
                    sed -i.bak "s|^HOTPLEX_SLACK_BOT_TOKEN=.*|HOTPLEX_SLACK_BOT_TOKEN=${bot_token}|" "$env_file" && rm -f "$env_file.bak"
                    updated=true
                else
                    warn "Token 验证失败，但仍会保存"
                    sed -i.bak "s|^HOTPLEX_SLACK_BOT_TOKEN=.*|HOTPLEX_SLACK_BOT_TOKEN=${bot_token}|" "$env_file" && rm -f "$env_file.bak"
                    updated=true
                fi
            else
                warn "Token 格式无效 (应为 xoxb-...)"
            fi
        fi

        # App Token
        echo ""
        echo -e "${CYAN}App-Level Token (xapp-...)${NC}"
        echo -e "  ${DIM}→ Basic Information → App-Level Tokens${NC}"
        app_token=$(prompt_input "请输入" "$current_app_token" "true")

        if [[ -n "$app_token" ]]; then
            if validate_slack_app_token "$app_token"; then
                sed -i.bak "s|^HOTPLEX_SLACK_APP_TOKEN=.*|HOTPLEX_SLACK_APP_TOKEN=${app_token}|" "$env_file" && rm -f "$env_file.bak"
                updated=true
            else
                warn "Token 格式无效 (应为 xapp-...)"
            fi
        fi

        # Bot User ID
        echo ""
        echo -e "${CYAN}Bot User ID (U... 或 B... 或 W...)${NC}"
        echo -e "  ${DIM}→ 点击机器人头像，查看 Member ID${NC}"
        echo -e "  ${DIM}  U=用户 B=Bot W=企业用户${NC}"
        user_id=$(prompt_input "请输入" "$current_user_id")

        if [[ -n "$user_id" ]]; then
            if validate_slack_user_id "$user_id"; then
                sed -i.bak "s|^HOTPLEX_SLACK_BOT_USER_ID=.*|HOTPLEX_SLACK_BOT_USER_ID=${user_id}|" "$env_file" && rm -f "$env_file.bak"
                updated=true
            else
                warn "User ID 格式无效 (应以 U、B 或 W 开头)"
            fi
        fi

        # GitHub Token
        echo ""
        if confirm "是否配置 GitHub Token?" "$([[ "$current_github" =~ ^ghp_ ]] && echo "n" || echo "y")"; then
            echo -e "${CYAN}GitHub Personal Access Token (ghp_...)${NC}"
            echo -e "  ${DIM}→ https://github.com/settings/tokens${NC}"
            github_token=$(prompt_input "请输入" "$current_github" "true")

            if [[ -n "$github_token" ]]; then
                if validate_github_token "$github_token"; then
                    sed -i.bak "s|^GITHUB_TOKEN=.*|GITHUB_TOKEN=${github_token}|" "$env_file" && rm -f "$env_file.bak"
                    updated=true
                else
                    warn "Token 格式无效 (应为 ghp_...)"
                fi
            fi
        fi

        # 清理备份
        rm -f "${env_file}.bak"

        if [[ "$updated" == "true" ]]; then
            success "配置已更新: $env_file"
        fi
    else
        # 非交互模式，显示指南
        echo -e "  ${BOLD}请手动编辑配置文件:${NC}"
        echo "    ${CONFIG_DIR}/.env"
        echo ""
    fi
}

# 向导：配置 Slack YAML
wizard_slack_yaml() {
    local yaml_file="${CONFIG_DIR}/slack.yaml"
    local yaml_source=""

    # 查找源配置文件
    if [[ -f "${INSTALL_DIR}/../configs/chatapps/slack.yaml" ]]; then
        yaml_source="${INSTALL_DIR}/../configs/chatapps/slack.yaml"
    elif [[ -f "/usr/local/share/hotplex/configs/chatapps/slack.yaml" ]]; then
        yaml_source="/usr/local/share/hotplex/configs/chatapps/slack.yaml"
    fi

    echo ""
    raw "${BOLD}${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    raw "${BOLD}  Step 2/2: ChatApps 行为配置${NC}"
    raw "${BOLD}${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    # 复制默认配置（如果不存在）
    if [[ ! -f "$yaml_file" ]]; then
        if [[ -n "$yaml_source" ]] && [[ -f "$yaml_source" ]]; then
            mkdir -p "$(dirname "$yaml_file")"
            cp "$yaml_source" "$yaml_file"
            success "已创建配置文件: $yaml_file"
        else
            # 生成默认配置
            generate_slack_yaml "$yaml_file"
        fi
    else
        success "配置文件已存在: $yaml_file"
    fi

    echo ""
    echo -e "  ${BOLD}${CYAN}ChatApps 配置选项:${NC}"
    echo ""

    # 读取当前配置
    local current_work_dir=$(grep -E "^  work_dir:" "$yaml_file" 2>/dev/null | awk '{print $2}' || echo "~/projects")
    local current_mode=$(grep "^mode:" "$yaml_file" 2>/dev/null | awk '{print $2}' || echo "socket")
    local current_group_policy=$(grep -E "    group_policy:" "$yaml_file" 2>/dev/null | awk '{print $2}' || echo "multibot")
    local current_model=$(grep -E "  default_model:" "$yaml_file" 2>/dev/null | awk '{print $2}' || echo "sonnet")

    echo -e "  ${DIM}当前设置:${NC}"
    echo -e "    工作目录:     ${GREEN}$current_work_dir${NC}"
    echo -e "    连接模式:     ${GREEN}$current_mode${NC}"
    echo -e "    群组策略:     ${GREEN}$current_group_policy${NC}"
    echo -e "    AI 模型:      ${GREEN}$current_model${NC}"
    echo ""

    if ! confirm "是否修改 ChatApps 配置?" "n"; then
        return 0
    fi

    # 工作目录
    echo ""
    echo -e "  ${DIM}工作目录 (work_dir)${NC}"
    echo -e "  ${DIM}Agent 执行代码的工作空间${NC}"
    local work_dir
    work_dir=$(prompt_input "请输入路径" "$current_work_dir")
    if [[ -n "$work_dir" ]] && [[ "$work_dir" != "$current_work_dir" ]]; then
        sed -i.bak "s|  work_dir:.*|  work_dir: ${work_dir}|" "$yaml_file"
        # 确保目录存在
        mkdir -p "$work_dir" 2>/dev/null || true
    fi

    # 连接模式
    echo ""
    echo -e "${CYAN}连接模式 (mode)${NC}"
    echo -e "  ${DIM}socket${NC} - 本地开发，无需公网 IP (推荐)"
    echo -e "  ${DIM}http${NC}   - 生产环境，使用 Webhook"
    echo ""
    local mode
    if confirm "使用 Socket Mode?" "y"; then
        mode="socket"
    else
        mode="http"
    fi
    if [[ "$mode" != "$current_mode" ]]; then
        sed -i.bak "s/^mode:.*/mode: ${mode}/" "$yaml_file"
    fi

    # 群组策略
    echo ""
    echo -e "${CYAN}群组响应策略 (group_policy)${NC}"
    echo -e "  ${DIM}allow${NC}     - 响应所有消息"
    echo -e "  ${DIM}mention${NC}   - 仅 @提及 时响应"
    echo -e "  ${DIM}multibot${NC}  - 多 Bot 模式，@提及时响应，无 @ 时广播提示"
    echo ""
    echo "  选择群组策略:"
    echo "    1) allow"
    echo "    2) mention"
    echo "    3) multibot (默认)"
    local policy_choice
    policy_choice=$(prompt_input "请选择 [1-3]" "3")
    local group_policy="multibot"
    case "$policy_choice" in
        1) group_policy="allow" ;;
        2) group_policy="mention" ;;
        3) group_policy="multibot" ;;
    esac
    if [[ "$group_policy" != "$current_group_policy" ]]; then
        sed -i.bak "s/    group_policy:.*/    group_policy: ${group_policy}/" "$yaml_file"
    fi

    # AI 模型
    echo ""
    echo -e "${CYAN}AI 模型 (default_model)${NC}"
    echo -e "  ${DIM}sonnet${NC} - 平衡性能与成本 (推荐)"
    echo -e "  ${DIM}haiku${NC}  - 快速响应，低成本"
    echo -e "  ${DIM}opus${NC}   - 最强性能，较高成本"
    echo ""
    echo "  选择 AI 模型:"
    echo "    1) sonnet (默认)"
    echo "    2) haiku"
    echo "    3) opus"
    local model_choice
    model_choice=$(prompt_input "请选择 [1-3]" "1")
    local model="sonnet"
    case "$model_choice" in
        1) model="sonnet" ;;
        2) model="haiku" ;;
        3) model="opus" ;;
    esac
    if [[ "$model" != "$current_model" ]]; then
        sed -i.bak "s/  default_model:.*/  default_model: ${model}/" "$yaml_file"
    fi

    # 清理备份
    rm -f "${yaml_file}.bak"

    echo ""
    success "ChatApps 配置已更新"
    echo ""
    echo -e "  ${DIM}完整配置文件: ${yaml_file}${NC}"
    echo -e "  ${DIM}配置文档: https://github.com/hrygo/hotplex/blob/main/docs/chatapps/chatapps-slack.md${NC}"
}

# 生成默认 Slack YAML 配置
generate_slack_yaml() {
    local yaml_file="$1"
    local work_dir="${HOME}/projects"

    mkdir -p "$(dirname "$yaml_file")"

    cat > "$yaml_file" << 'EOF'
# =============================================================================
# HotPlex Slack Adapter Configuration
# 由安装向导生成
# =============================================================================

platform: slack

# AI Provider
provider:
  type: claude-code
  enabled: true
  default_model: sonnet
  default_permission_mode: bypass-permissions
  dangerously_skip_permissions: true

# Engine
engine:
  work_dir: ~/projects
  timeout: 30m
  idle_timeout: 1h

# Session
session:
  timeout: 1h
  cleanup_interval: 5m

# Connection
mode: socket
server_addr: :8080

# AI Identity
system_prompt: |
  You are HotPlex, an expert software engineer in a Slack conversation.

  ## Environment
  - Running under HotPlex engine (stdin/stdout)
  - Headless mode - cannot prompt for user input

  ## Slack Context
  - Replies go to thread automatically
  - Keep answers concise - user expects quick responses

  ## Output
  - Be concise - short messages preferred
  - Use bullet lists over paragraphs
  - Use code blocks for code snippets
  - Avoid tables - use lists instead

task_instructions: |
  1. Understand before acting
  2. Avoid operations requiring user input
  3. Summarize tool output - don't dump raw data

# Features
features:
  chunking:
    enabled: true
    max_chars: 4000
  threading:
    enabled: true
  rate_limit:
    enabled: true
    max_attempts: 3
    base_delay_ms: 500
    max_delay_ms: 5000
  markdown:
    enabled: true

# Security
security:
  verify_signature: true
  permission:
    dm_policy: allow
    group_policy: multibot
    bot_user_id: ${HOTPLEX_SLACK_BOT_USER_ID}
    broadcast_response: |
      👋 Hello! I'm ready to help.
      Please @mention me if you'd like me to respond specifically to you.
    allowed_users: []
    blocked_users: []
    slash_command_rate_limit: 10.0
EOF

    # 替换工作目录
    sed -i.bak "s|  work_dir: ~/projects|  work_dir: ${work_dir}|" "$yaml_file"
    rm -f "${yaml_file}.bak"

    success "已生成配置文件: $yaml_file"
}

# 运行安装向导
run_setup_wizard() {
    # 检查是否跳过向导
    if [[ "$SKIP_WIZARD" == "true" ]]; then
        debug "跳过配置向导"
        show_quick_start
        return 0
    fi

    # 检查是否是非交互模式
    if [[ ! -t 0 ]] || [[ "$QUIET" == "true" ]] || [[ "$DRY_RUN" == "true" ]]; then
        debug "非交互模式，跳过向导"
        return 0
    fi

    # 显示向导标题
    echo ""
    raw "${BOLD}════════════════════════════════════════════════════════════${NC}"
    raw "${BOLD}                    🧙 配置向导                              ${NC}"
    raw "${BOLD}════════════════════════════════════════════════════════════${NC}"

    wizard_slack_credentials
    wizard_slack_yaml

    # 完成提示
    echo ""
    raw "${BOLD}${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    raw "${BOLD}  ✓ 配置完成${NC}"
    raw "${BOLD}${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    show_quick_start
}

# 显示快速开始指南
show_quick_start() {
    echo ""
    raw "${GREEN}${BOLD}🎉 HotPlex 安装成功!${NC}"
    echo ""
    echo -e "  ${BOLD}快速开始:${NC}"
    echo ""
    echo "    1. 编辑配置 (如需要):"
    echo -e "       ${DIM}${CONFIG_DIR}/.env${NC}"
    echo ""
    echo "    2. 启动服务:"
    echo -e "       ${GREEN}${BINARY_NAME} -env ${CONFIG_DIR}/.env${NC}"
    echo ""
    echo "    3. 查看帮助:"
    echo -e "       ${DIM}${BINARY_NAME} -h${NC}"
    echo ""
    echo -e "  ${DIM}文档: https://github.com/hrygo/hotplex#readme${NC}"
    echo -e "  ${DIM}问题: https://github.com/hrygo/hotplex/issues${NC}"
    echo ""
}

# 生成配置文件
generate_config() {
    info "生成配置文件..."

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] 将创建: ${CONFIG_DIR}/.env"
        return
    fi

    mkdir -p "$CONFIG_DIR"
    mkdir -p "${CONFIG_DIR}/projects"
    mkdir -p "${CONFIG_DIR}/backups"

    local env_file="${CONFIG_DIR}/.env"

    if [[ -f "$env_file" ]] && [[ "$FORCE" != "true" ]]; then
        warn "配置文件已存在: $env_file"
        if ! confirm "是否覆盖?" "n"; then
            return
        fi
    fi

    # 备份现有配置
    if [[ -f "$env_file" ]]; then
        cp "$env_file" "${env_file}.bak.$(date +%Y%m%d%H%M%S)"
    fi

    # 生成随机 API Key
    local api_key
    if command_exists openssl; then
        api_key=$(openssl rand -hex 32)
    else
        api_key="change-me-$(date +%s)-$$"
    fi

    local config_content="# ==============================================================================
# HotPlex 环境配置
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S %z')
# 完整配置参考: https://github.com/hrygo/hotplex/blob/main/.env.example
# ==============================================================================

# 核心服务器
HOTPLEX_PORT=\${HOTPLEX_PORT:-8080}
HOTPLEX_LOG_LEVEL=INFO
HOTPLEX_LOG_FORMAT=text
HOTPLEX_API_KEY=\${api_key}
HOTPLEX_DATA_DIR=\${CONFIG_DIR}

# Provider 配置
HOTPLEX_PROVIDER_TYPE=claude-code
HOTPLEX_PROVIDER_MODEL=sonnet

# Slack Bot 配置 (必填 - P0 安全验证)
HOTPLEX_SLACK_BOT_USER_ID=\${SLACK_BOT_USER_ID:-UXXXXXXXXXX}
HOTPLEX_SLACK_BOT_TOKEN=\${SLACK_BOT_TOKEN:-xoxb-}
HOTPLEX_SLACK_APP_TOKEN=\${SLACK_APP_TOKEN:-xapp-}

# 消息存储
HOTPLEX_MESSAGE_STORE_ENABLED=true
HOTPLEX_MESSAGE_STORE_TYPE=sqlite
HOTPLEX_MESSAGE_STORE_SQLITE_PATH=\${CONFIG_DIR}/chatapp_messages.db

# GitHub Token (用于 Git 操作)
GITHUB_TOKEN=
"

    atomic_install_config "$config_content"
    success "已生成配置文件: $env_file"
}

# ==============================================================================
# 健康检查 (P0)
# ==============================================================================

health_check() {
    local port="${HOTPLEX_PORT:-8080}"
    local timeout="${2:-15}"
    local elapsed=0

    step "执行健康检查 (超时: ${timeout}s)..."

    local binary="${INSTALL_DIR}/${BINARY_NAME}"
    if [[ ! -x "$binary" ]]; then
        error "健康检查失败: 二进制文件不可执行"
        return 1
    fi

    if pgrep -f "$BINARY_NAME" &>/dev/null; then
        debug "进程检查: 已有运行实例"
    fi

    local endpoint="http://localhost:${port}/health"

    while [[ $elapsed -lt $timeout ]]; do
        local start_time=$(date +%s)

        if command_exists curl; then
            local response
            response=$(curl -sf --connect-timeout 2 --max-time 5 "$endpoint" 2>/dev/null || echo "")

            if [[ -n "$response" ]] && echo "$response" | grep -q "ok\|OK\|healthy"; then
                success "健康检查通过"
                return 0
            fi
        fi

        local now=$(date +%s)
        elapsed=$((now - start_time + elapsed))
        [[ $elapsed -lt $timeout ]] && sleep 1
    done

    warn "健康检查超时 (${timeout}s) - 服务可能仍在启动"
    return 1
}

# ==============================================================================
# 开机自启 (P0)
# ==============================================================================

detect_init_system() {
    if command_exists systemctl && systemctl --version &>/dev/null; then
        echo "systemd"
    elif command_exists launchd; then
        echo "launchd"
    else
        echo "unknown"
    fi
}

install_systemd_service() {
    debug "安装 systemd 服务..."
    local service_file="/etc/systemd/system/${BINARY_NAME}.service"
    local systemd_content="[Unit]
Description=HotPlex AI Agent Control Plane
After=network.target

[Service]
Type=simple
User=${USER}
ExecStart=${INSTALL_DIR}/${BINARY_NAME} start
Restart=on-failure
RestartSec=10
Environment=HOTPLEX_DATA_DIR=${CONFIG_DIR}
Environment=HOTPLEX_PORT=${HOTPLEX_PORT:-8080}

[Install]
WantedBy=multi-user.target
"

    if [[ -w "/etc/systemd/system" ]]; then
        echo "$systemd_content" > "$service_file" || return 1
        chmod 644 "$service_file"
        systemctl daemon-reload
        systemctl enable "${BINARY_NAME}.service"
    else
        sudo sh -c "cat > '$service_file' << 'SYSTEMD_EOF'
$systemd_content
SYSTEMD_EOF
chmod 644 '$service_file'
systemctl daemon-reload
systemctl enable '${BINARY_NAME}.service'" || return 1
    fi
    return 0
}

install_launchd_service() {
    debug "安装 launchd 服务..."
    local plist_dir="$HOME/Library/LaunchAgents"
    local plist_file="${plist_dir}/com.hotplex.daemon.plist"
    mkdir -p "$plist_dir"

    cat > "$plist_file" << 'PLIST_EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>com.hotplex.daemon</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BINARY_NAME}</string>
        <string>start</string>
    </array>
    <key>RunAtLoad</key><true/>
    <key>EnvironmentVariables</key>
    <dict>
        <key>HOTPLEX_DATA_DIR</key><string>${CONFIG_DIR}</string>
    </dict>
</dict>
</plist>
PLIST_EOF
    chmod 644 "$plist_file"
    launchctl load "$plist_file" 2>/dev/null || true
    return 0
}

install_autostart() {
    if [[ "${SKIP_AUTOSTART:-false}" == "true" ]]; then
        debug "跳过开机自启配置"
        return 0
    fi

    local init_system=$(detect_init_system)
    step "配置开机自启 ($init_system)..."

    case "$init_system" in
        systemd)
            if install_systemd_service; then
                success "systemd 服务已配置"
                systemctl start "${BINARY_NAME}.service" 2>/dev/null || true
            else
                warn "systemd 服务配置失败"
            fi
            ;;
        launchd)
            if install_launchd_service; then
                success "launchd 服务已配置"
            else
                warn "launchd 服务配置失败"
            fi
            ;;
        *) warn "不支持的初始化系统: $init_system，跳过开机自启" ;;
    esac
}

# ==============================================================================
# 原子安装操作 (P0)
# ==============================================================================

atomic_install_config() {
    local config_content="$1"
    local config_path="${CONFIG_DIR}/.env"
    local temp_config

    temp_config=$(mktemp /tmp/hotplex-env.XXXXXX) || error "无法创建临时配置文件"
    echo "$config_content" > "$temp_config" || {
        rm -f "$temp_config"
        error "无法写入配置"
    }

    [[ ! -d "$CONFIG_DIR" ]] && mkdir -p "$CONFIG_DIR" 2>/dev/null || {
        rm -f "$temp_config"
        error "无法创建配置目录 $CONFIG_DIR"
    }

    mv "$temp_config" "$config_path" || {
        rm -f "$temp_config"
        error "无法写入配置文件"
    }

    chmod 600 "$config_path" 2>/dev/null || sudo chmod 600 "$config_path"
    debug "配置安装完成: $config_path"
}


# 安装
do_install() {
    # Docker 模式
    if [[ "$INSTALL_MODE" == "docker" ]]; then
        install_docker
        return
    fi

    local os arch

    os=$(detect_os)
    arch=$(detect_arch)

    info "系统: $(uname -s) $(uname -m)"
    info "平台: ${os}/${arch}"

    # Phase 2: 端口冲突检测
    local check_port_num="${HOTPLEX_PORT:-8080}"
    if check_port "$check_port_num" &>/dev/null; then
        resolve_port_conflict "$check_port_num"
    fi

    # 获取/验证版本
    if [[ -z "$VERSION" ]]; then
        step "获取最新版本..."
        VERSION=$(get_latest_version) || error "无法获取最新版本，请使用 -v 指定"
        [[ "$VERSION" != v* ]] && VERSION="v${VERSION}"
    fi
    info "目标版本: ${GREEN}$VERSION${NC}"

    # 检查已安装版本
    check_existing_installation

    # 构建下载信息
    local archive_name="hotplex_${VERSION#v}_${os}_${arch}"
    [[ "$os" == "windows" ]] && archive_name="${archive_name}.zip" || archive_name="${archive_name}.tar.gz"
    local archive_url="https://github.com/${REPO}/releases/download/${VERSION}/${archive_name}"

    debug "下载地址: $archive_url"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] 将下载: $archive_url"
        info "[DRY-RUN] 将安装到: ${INSTALL_DIR}/${BINARY_NAME}"
        info "[DRY-RUN] 将生成配置: ${CONFIG_DIR}/.env"
        return
    fi

    # 创建临时目录
    TEMP_DIR=$(mktemp -d)
    debug "临时目录: $TEMP_DIR"

    # 备份现有安装
    backup_existing

    # 下载
    local archive_path="${TEMP_DIR}/${archive_name}"
    step "正在下载..."
    download_with_retry "$archive_url" "$archive_path"

    # 下载并验证校验和
    if [[ "$SKIP_VERIFY" != "true" ]]; then
        local checksums_path="${TEMP_DIR}/checksums.txt"
        if download_checksums "$VERSION" "$checksums_path"; then
            step "验证校验和..."
            verify_checksum "$archive_path" "$checksums_path"
        else
            warn "无法下载校验和文件，跳过验证"
        fi
    fi

    # 解压
    step "正在解压..."
    if [[ "$os" == "windows" ]]; then
        command_exists unzip || error "需要 unzip 来解压 .zip 文件"
        unzip -q "$archive_path" -d "$TEMP_DIR"
    else
        tar -xzf "$archive_path" -C "$TEMP_DIR"
    fi

    # 安装
    step "正在安装到 ${INSTALL_DIR}..."

    if [[ ! -w "$INSTALL_DIR" ]] && [[ ! -d "$INSTALL_DIR" ]]; then
        if [[ -w "$(dirname "$INSTALL_DIR")" ]]; then
            mkdir -p "$INSTALL_DIR"
        else
            sudo mkdir -p "$INSTALL_DIR"
        fi
    fi

    local binary_path="${TEMP_DIR}/${BINARY_NAME}"
    [[ -f "$binary_path" ]] || error "解压后未找到 ${BINARY_NAME}"

    if [[ -w "$INSTALL_DIR" ]]; then
        cp "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    else
        sudo cp "$binary_path" "${INSTALL_DIR}/${BINARY_NAME}"
        sudo chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
    fi

    # 验证安装
    local installed_binary="${INSTALL_DIR}/${BINARY_NAME}"
    if [[ ! -x "$installed_binary" ]]; then
        error "安装验证失败: $installed_binary 不可执行"
    fi

    local installed_version
    installed_version=$("$installed_binary" -version 2>/dev/null | head -1 || echo "unknown")
    success "安装成功: ${GREEN}$installed_version${NC}"

    # 生成配置
    generate_config

    # 运行配置向导
    run_setup_wizard

    # 配置开机自启 (P0)
    install_autostart

    # 健康检查 (P0)
    if [[ "${SKIP_HEALTH_CHECK:-false}" != "true" ]]; then
        health_check || true
    fi

    # 清理备份标记
    CLEANUP_PENDING=false
}

# ==============================================================================
# Phase 4: Docker 模式安装
# ==============================================================================

# 检查 Docker 环境
check_docker() {
    if ! command_exists docker; then
        error "Docker 未安装。请先安装 Docker: https://docs.docker.com/get-docker/"
        return 1
    fi

    if ! command_exists docker-compose && ! docker compose version &>/dev/null; then
        error "Docker Compose 未安装。请先安装 Docker Compose: https://docs.docker.com/compose/install/"
        return 1
    fi

    if ! docker info &>/dev/null; then
        error "Docker 守护进程未运行。请启动 Docker 后重试。"
        return 1
    fi

    return 0
}

# 生成 docker-compose.yml
generate_docker_compose() {
    local compose_file="${CONFIG_DIR}/docker-compose.yml"
    local bot_configs=""

    info "生成 Docker Compose 配置..."

    # 多 Bot 配置
    if [[ "$MULTI_BOT_ENABLED" == "true" ]] && [[ "$BOT_COUNT" -gt 1 ]]; then
        for ((i=1; i<=BOT_COUNT; i++)); do
            local bot_num=$(printf "%02d" $i)
            local bot_name="bot-${bot_num}"
            local env_file="${CONFIG_DIR}/${bot_name}.env"

            if [[ -f "$env_file" ]]; then
                bot_configs="${bot_configs}
  ${bot_name}:
    container_name: hotplex-${bot_num}
    image: ghcr.io/hrygo/hotplex:latest
    restart: unless-stopped
    env_file:
      - ./${bot_name}.env
    volumes:
      - ./data:/app/data
      - ./projects:/app/projects
    ports:
      - \"$((8080 + i - 1)):8080\"
    networks:
      - hotplex-net"
            fi
        done
    fi

    local compose_content="# ==============================================================================
# HotPlex Docker Compose 配置
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S %z')
# ==============================================================================

services:
  hotplex:
    container_name: hotplex
    image: ghcr.io/hrygo/hotplex:latest
    restart: unless-stopped
    env_file:
      - .env
    volumes:
      - ./data:/app/data
      - ./projects:/app/projects
      - ./logs:/app/logs
    ports:
      - \"8080:8080\"
    networks:
      - hotplex-net${bot_configs}
${bot_configs:+$'\n'}

networks:
  hotplex-net:
    driver: bridge
"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] 将创建: $compose_file"
        return
    fi

    mkdir -p "$CONFIG_DIR"
    mkdir -p "${CONFIG_DIR}/data"
    mkdir -p "${CONFIG_DIR}/projects"
    mkdir -p "${CONFIG_DIR}/logs"

    echo "$compose_content" > "$compose_file"
    chmod 600 "$compose_file"
    success "已生成: $compose_file"
}

# Docker 模式安装
install_docker() {
    info "开始 Docker 模式安装..."

    # 检查 Docker 环境
    if ! check_docker; then
        return 1
    fi

    # 多 Bot 配置
    if [[ "$MULTI_BOT_ENABLED" == "true" ]]; then
        wizard_multi_bot
    fi

    # 生成 .env 文件
    generate_config

    # 生成 docker-compose.yml
    generate_docker_compose

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] Docker 模式安装准备完成"
        return
    fi

    # 拉取镜像并启动
    step "拉取最新镜像..."
    if docker_compose pull -q; then
        success "镜像拉取成功"
    else
        warn "镜像拉取失败，将使用本地镜像启动"
    fi

    step "启动容器..."
    if docker_compose up -d; then
        success "容器启动成功"
    else
        error "容器启动失败"
        return 1
    fi

    # 等待服务就绪
    sleep 3

    # 健康检查
    step "检查服务状态..."
    if silent_run docker compose ps | grep -q "Up"; then
        success "服务运行中"
    else
        warn "请手动检查: docker compose logs hotplex"
    fi

    echo ""
    raw "${GREEN}${BOLD}🎉 Docker 模式安装成功!${NC}"
    echo ""
    echo -e "  ${BOLD}管理命令:${NC}"
    echo ""
    echo "    cd ${CONFIG_DIR}"
    echo "    docker compose up -d      # 启动"
    echo "    docker compose logs -f    # 查看日志"
    echo "    docker compose down       # 停止"
    echo "    docker compose restart    # 重启"
    echo ""
    echo -e "  ${DIM}配置文件: ${CONFIG_DIR}/.env${NC}"
    echo -e "  ${DIM}编排文件: ${CONFIG_DIR}/docker-compose.yml${NC}"
    echo ""
}

# ==============================================================================
# Phase 4: 多 Bot 支持
# ==============================================================================

# 多 Bot 配置向导
wizard_multi_bot() {
    echo ""
    raw "${BOLD}════════════════════════════════════════════════════════════${NC}"
    raw "${BOLD}                    🤖 多 Bot 配置向导                      ${NC}"
    raw "${BOLD}════════════════════════════════════════════════════════════${NC}"
    echo ""

    if [[ ! -t 0 ]] || [[ "$INTERACTIVE" != "true" ]]; then
        debug "非交互模式，跳过多 Bot 配置"
        return 0
    fi

    echo -e "  ${BOLD}多 Bot 模式允许在一台服务器上运行多个独立的 HotPlex Bot${NC}"
    echo -e "  ${DIM}每个 Bot 有独立的配置和 Token，适合复杂组织结构${NC}"
    echo ""

    if [[ "$MULTI_BOT_ENABLED" == "true" ]]; then
        echo -e "  ${GREEN}✓ 多 Bot 模式已启用${NC}"
    else
        if ! confirm "是否启用多 Bot 模式?" "n"; then
            info "多 Bot 配置已跳过"
            return 0
        fi
        MULTI_BOT_ENABLED=true
    fi

    # 询问 Bot 数量
    echo ""
    echo -e "  ${BOLD}请选择 Bot 数量:${NC}"
    echo ""
    echo "    1) 单 Bot (默认)"
    echo "    2) 双 Bot"
    echo "    3) 三 Bot"
    echo "    4) 自定义数量"
    echo ""

    local count_choice
    count_choice=$(prompt_input "选择" "1")

    case "$count_choice" in
        1) BOT_COUNT=1 ;;
        2) BOT_COUNT=2 ;;
        3) BOT_COUNT=3 ;;
        4)
            local custom_count
            custom_count=$(prompt_input "请输入 Bot 数量 (1-10)" "1")
            BOT_COUNT=$((custom_count >= 1 && custom_count <= 10 ? custom_count : 1))
            ;;
        *) BOT_COUNT=1 ;;
    esac

    info "将配置 ${BOT_COUNT} 个 Bot"

    # 为每个 Bot 生成配置
    for ((i=1; i<=BOT_COUNT; i++)); do
        local bot_num=$(printf "%02d" $i)
        local bot_name="bot-${bot_num}"
        local env_file="${CONFIG_DIR}/${bot_name}.env"

        echo ""
        raw "${BOLD}════════════════════════════════════════════════════════════${NC}"
        raw "${BOLD}              🤖 配置 Bot ${i}/${BOT_COUNT} (${bot_name})           ${NC}"
        raw "${BOLD}════════════════════════════════════════════════════════════${NC}"
        echo ""

        # 获取现有配置（如有）
        local current_bot_token=""
        local current_user_id=""
        if [[ -f "$env_file" ]]; then
            current_bot_token=$(grep "^HOTPLEX_SLACK_BOT_TOKEN=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "")
            current_user_id=$(grep "^HOTPLEX_SLACK_BOT_USER_ID=" "$env_file" 2>/dev/null | cut -d'=' -f2- || echo "")
        fi

        # Bot Token
        echo -e "  ${BOLD}Bot ${i} 配置${NC}"
        echo ""

        local bot_token
        bot_token=$(prompt_input "输入 Bot Token (xoxb-...)" "$current_bot_token" "true")

        if [[ -z "$bot_token" ]]; then
            warn "Bot Token 不能为空，跳过 Bot ${i}"
            continue
        fi

        # 验证 Token
        if ! validate_slack_token "$bot_token"; then
            warn "Token 格式验证失败，继续使用"
        fi

        # Bot User ID
        local bot_user_id
        bot_user_id=$(prompt_input "输入 Bot User ID (U...)" "$current_user_id")

        # 生成 Bot 配置
        cat > "$env_file" << EOF
# ==============================================================================
# HotPlex Bot ${i} 配置文件
# Bot 名称: ${bot_name}
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S %z')
# ==============================================================================

HOTPLEX_PORT=${HOTPLEX_PORT:-8080}
HOTPLEX_LOG_LEVEL=INFO
HOTPLEX_DATA_DIR=/app/data

# Bot ${i} 特定配置
BOT_NAME=${bot_name}
HOTPLEX_SLACK_BOT_USER_ID=${bot_user_id}
HOTPLEX_SLACK_BOT_TOKEN=${bot_token}
HOTPLEX_SLACK_APP_TOKEN=${HOTPLEX_SLACK_APP_TOKEN:-xapp-}

# Provider 配置
HOTPLEX_PROVIDER_TYPE=claude-code
HOTPLEX_PROVIDER_MODEL=sonnet

# 消息存储
HOTPLEX_MESSAGE_STORE_ENABLED=true
HOTPLEX_MESSAGE_STORE_TYPE=sqlite
HOTPLEX_MESSAGE_STORE_SQLITE_PATH=/app/data/chatapp_messages_${bot_num}.db
EOF

        chmod 600 "$env_file"
        success "已生成: $env_file"
    done

    # 更新主配置
    if [[ "$BOT_COUNT" -gt 1 ]]; then
        MULTI_BOT_ENABLED=true
        wizard_slack_yaml
    fi

    echo ""
    raw "${BOLD}${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    raw "${BOLD}  ✓ 多 Bot 配置完成${NC}"
    raw "${BOLD}${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

# ==============================================================================
# Phase 4: 日志轮转配置
# ==============================================================================

# 配置日志轮转
configure_logrotate() {
    echo ""
    raw "${BOLD}════════════════════════════════════════════════════════════${NC}"
    raw "${BOLD}                    📋 日志管理                         ${NC}"
    raw "${BOLD}════════════════════════════════════════════════════════════${NC}"
    echo ""

    echo -e "  ${BOLD}当前日志状态:${NC}"
    echo ""

    local log_dir="${CONFIG_DIR}/logs"
    local binary_log="${CONFIG_DIR}/${BINARY_NAME}.log"

    if [[ -d "$log_dir" ]]; then
        local log_count=$(find "$log_dir" -name "*.log" 2>/dev/null | wc -l)
        local total_size=$(du -sh "$log_dir" 2>/dev/null | cut -f1 || echo "未知")
        echo -e "  ${GREEN}✓${NC}  日志目录: ${log_dir}"
        echo -e "  ${GREEN}✓${NC}  日志文件数: ${log_count}"
        echo -e "  ${GREEN}✓${NC}  总大小: ${total_size}"
    else
        echo -e "  ${YELLOW}○${NC}  日志目录不存在: ${log_dir}"
    fi

    if [[ -f "$binary_log" ]]; then
        local binary_size=$(du -h "$binary_log" 2>/dev/null | cut -f1 || echo "未知")
        echo -e "  ${GREEN}✓${NC}  二进制日志: ${binary_log} (${binary_size})"
    fi

    echo ""
    echo -e "  ${BOLD}请选择操作:${NC}"
    echo ""
    echo "    1) 配置日志轮转      (自动清理旧日志)"
    echo "    2) 手动清理日志      (删除所有日志文件)"
    echo "    3) 查看日志目录      (ls -la)"
    echo "    4) 返回"
    echo ""

    local log_choice
    log_choice=$(prompt_input "选择" "4")

    case "$log_choice" in
        1)
            install_logrotate
            ;;
        2)
            cleanup_logs
            ;;
        3)
            echo ""
            if [[ -d "$log_dir" ]]; then
                ls -lah "$log_dir"
            else
                echo -e "  ${DIM}日志目录不存在${NC}"
            fi
            echo ""
            if [[ -f "$binary_log" ]]; then
                ls -lah "$binary_log"
            fi
            echo ""
            ;;
        4|*)
            return 0
            ;;
    esac
}

# 安装 logrotate 配置
install_logrotate() {
    step "配置日志轮转..."

    # 检测权限
    local logrotate_conf="/etc/logrotate.d/hotplex"
    local need_sudo=false

    if [[ ! -w "/etc/logrotate.d" ]]; then
        need_sudo=true
    fi

    local logrotate_content="# HotPlex 日志轮转配置
# 生成时间: $(date '+%Y-%m-%d %H:%M:%S')

${CONFIG_DIR}/*.log {
    daily
    rotate ${MAX_LOG_FILES}
    size ${MAX_LOG_SIZE}
    missingok
    notifempty
    compress
    delaycompress
    create 0600 \$(whoami) \$(whoami)
    sharedscripts
    postrotate
        # 通知服务重新打开日志文件（如支持）
        kill -USR1 \$(pgrep -f ${BINARY_NAME}) 2>/dev/null || true
    endscript
}

${CONFIG_DIR}/logs/*.log {
    daily
    rotate ${MAX_LOG_FILES}
    size ${MAX_LOG_SIZE}
    missingok
    notifempty
    compress
    delaycompress
    create 0600 \$(whoami) \$(whoami)
}
"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[DRY-RUN] 将创建: $logrotate_conf"
        info "[DRY-RUN] 日志轮转规则: daily, rotate ${MAX_LOG_FILES}, size ${MAX_LOG_SIZE}"
        return
    fi

    if [[ "$need_sudo" == "true" ]]; then
        echo "$logrotate_content" | sudo tee "$logrotate_conf" > /dev/null
        sudo chmod 644 "$logrotate_conf"
    else
        echo "$logrotate_content" > "$logrotate_conf"
        chmod 644 "$logrotate_conf"
    fi

    success "已配置日志轮转: $logrotate_conf"
    info "规则: 每天轮转，保留 ${MAX_LOG_FILES} 份，大小超过 ${MAX_LOG_SIZE} 时触发"
}

# 清理日志
cleanup_logs() {
    echo ""
    warn "即将清理所有日志文件!"
    echo ""

    if ! confirm "确认清理?" "n"; then
        info "已取消"
        return
    fi

    local cleaned=0
    local log_dir="${CONFIG_DIR}/logs"
    local binary_log="${CONFIG_DIR}/${BINARY_NAME}.log"

    # 清理日志目录
    if [[ -d "$log_dir" ]]; then
        local count
        count=$(find "$log_dir" -name "*.log" 2>/dev/null | wc -l)
        if [[ "$count" -gt 0 ]]; then
            find "$log_dir" -name "*.log" -delete 2>/dev/null
            cleaned=$((cleaned + count))
            info "已清理: $count 个日志文件"
        fi
    fi

    # 清理二进制日志
    if [[ -f "$binary_log" ]] && [[ -s "$binary_log" ]]; then
        : > "$binary_log"
        cleaned=$((cleaned + 1))
        info "已清空: $binary_log"
    fi

    if [[ "$cleaned" -eq 0 ]]; then
        info "没有日志文件需要清理"
    else
        success "共清理 ${cleaned} 个日志文件"
    fi
}

# ==============================================================================
# Phase 2: 交互式主菜单
# ==============================================================================

show_main_menu() {
    echo ""
    raw "${BOLD}${CYAN}╭─ HotPlex 安装程序 ──────────────────────────────╮${NC}"
    raw "${BOLD}${CYAN}╰───────────────────────────────────────────────────╯${NC}"
    echo ""
    echo -e "  ${BOLD}请选择操作:${NC}"
    echo ""
    echo "    1) 安装 HotPlex         (Quick Start - 二进制模式)"
    echo "    2) 安装 HotPlex (Docker) (容器化部署，支持多 Bot)"
    echo "    3) 升级 HotPlex         (保留配置，替换二进制)"
    echo "    4) 卸载 HotPlex         (移除二进制，保留配置)"
    echo "    5) 完全清理            (删除所有数据)"
    echo "    6) 查看状态            (检查安装和运行状态)"
    echo "    7) 仅配置              (运行配置向导)"
    echo "    8) 多 Bot 配置         (添加/管理多个 Bot)"
    echo "    9) 日志管理           (配置日志轮转)"
    echo "   10) 退出"
    echo ""
}

handle_menu_choice() {
    local choice="$1"

    case "$choice" in
        1)
            echo ""
            info "开始安装 (二进制模式)..."
            do_install
            ;;
        2)
            echo ""
            info "开始安装 (Docker 模式)..."
            INSTALL_MODE=docker do_install
            ;;
        3)
            echo ""
            do_upgrade
            ;;
        4)
            echo ""
            UNINSTALL=true
            do_uninstall
            ;;
        5)
            echo ""
            do_purge
            ;;
        6)
            echo ""
            do_status
            ;;
        7)
            echo ""
            if [[ ! -f "${CONFIG_DIR}/.env" ]]; then
                generate_config
            fi
            run_setup_wizard
            ;;
        8)
            echo ""
            wizard_multi_bot
            ;;
        9)
            echo ""
            configure_logrotate
            ;;
        10|q|Q)
            echo ""
            info "再见!"
            exit 0
            ;;
        *)
            error "无效选择: $choice"
            ;;
    esac
}

# ==============================================================================
# 主入口
# ==============================================================================

main() {
    init_colors
    setup_traps
    parse_args "$@"

    # 显示 banner
    if [[ "$QUIET" != "true" ]]; then
        echo ""
        raw "  ${BOLD}╔═══════════════════════════════════════════╗${NC}"
        raw "  ${BOLD}║${NC}         ${CYAN}HotPlex${NC} 安装程序 v${SCRIPT_VERSION}          ${BOLD}║${NC}"
        raw "  ${BOLD}║${NC}       AI Agent Control Plane            ${BOLD}║${NC}"
        raw "  ${BOLD}╚═══════════════════════════════════════════╝${NC}"
        echo ""
    fi

    # 创建日志目录并设置安全权限
    mkdir -p "$CONFIG_DIR" 2>/dev/null || true
    setup_secure_logging

    # 处理子命令 (非交互模式)
    if [[ "$#" -gt 0 ]]; then
        case "$1" in
            upgrade)   do_upgrade; exit 0 ;;
            status)    do_status; exit 0 ;;
            purge)     do_purge; exit 0 ;;
            uninstall) UNINSTALL=true; do_uninstall; exit 0 ;;
            config)    CONFIG_ONLY=true ;;
            docker)    INSTALL_MODE=docker; do_install; exit 0 ;;
            install)   shift; parse_args "$@" ;;
        esac
    fi

    # 处理 flags
    [[ "$DO_UPGRADE" == "true" ]] && { do_upgrade; exit 0; }
    [[ "$DO_STATUS" == "true" ]]  && { do_status; exit 0; }
    [[ "$DO_PURGE" == "true" ]]   && { do_purge; exit 0; }

    # 卸载模式
    if [[ "$UNINSTALL" == "true" ]]; then
        do_uninstall
        exit 0
    fi

    # 仅配置模式
    if [[ "$CONFIG_ONLY" == "true" ]]; then
        if [[ ! -f "${CONFIG_DIR}/.env" ]]; then
            generate_config
        fi
        run_setup_wizard
        exit 0
    fi

    # 检查依赖
    check_dependencies

    # 交互模式：显示主菜单
    if [[ "$INTERACTIVE" == "true" ]] && [[ -t 0 ]] && [[ "$#" -eq 0 ]]; then
        show_main_menu
        echo -ne "${BOLD}请选择 [1-10]:${NC} "
        read -r menu_choice
        echo ""
        handle_menu_choice "$menu_choice"
        exit 0
    fi

    # 安装
    do_install
}

main "$@"
