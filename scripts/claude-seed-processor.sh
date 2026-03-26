#!/usr/bin/env bash
#
# Claude Configuration Seed Processor (Optimized)
# Transforms host-machine ~/.claude/ configs for container compatibility
#
# Usage:
#   ./scripts/claude-seed-processor.sh [--verify]
#
# Options:
#   --verify    Only verify existing seed, skip processing
#
# Exit codes:
#   0 - Success
#   1 - Processing error or verification failed
#

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SOURCE_DIR="${HOME}/.claude"
OUTPUT_DIR="${HOME}/.hotplex/claude-seed"
HOST_USER=$(whoami)
CONTAINER_USER="hotplex"
LOG_FILE="${HOME}/.hotplex/claude-seed.log"

# Parse arguments
VERIFY_ONLY=false
if [[ "${1:-}" == "--verify" ]]; then
    VERIFY_ONLY=true
fi

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] $1" >> "$LOG_FILE"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [SUCCESS] $1" >> "$LOG_FILE"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [WARN] $1" >> "$LOG_FILE"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] $1" >> "$LOG_FILE"
}

# Verify seed directory
verify_seed() {
    log_info "Verifying seed at $OUTPUT_DIR..."

    if [[ ! -d "$OUTPUT_DIR" ]]; then
        log_error "Seed directory not found: $OUTPUT_DIR"
        return 1
    fi

    # Check for hardcoded paths
    local found_paths=0

    # Check for host username (skip binary files and logs)
    if grep -rI "$HOST_USER" "$OUTPUT_DIR" --exclude-dir="plugins" --exclude="*.log" 2>/dev/null | grep -v "Binary file"; then
        log_error "Found hardcoded username: $HOST_USER"
        found_paths=1
    fi

    # Check for specific macOS path patterns (skip binary files and logs)
    if grep -rI "/Users/" "$OUTPUT_DIR" --exclude-dir="plugins" --exclude="*.log" 2>/dev/null | grep -v "Binary file"; then
        log_error "Found hardcoded macOS paths: /Users/"
        found_paths=1
    fi

    # Check for Linux home paths (if running on Linux host)
    if [[ "$(uname)" == "Linux" ]]; then
        if grep -rI "/home/${HOST_USER}" "$OUTPUT_DIR" --exclude-dir="plugins" --exclude="*.log" 2>/dev/null | grep -v "Binary file"; then
            log_error "Found hardcoded Linux paths: /home/${HOST_USER}"
            found_paths=1
        fi
    fi

    if [[ $found_paths -eq 1 ]]; then
        log_error "Verification failed: hardcoded paths detected"
        return 1
    fi

    log_success "Verification passed: no hardcoded paths found"
    return 0
}

# Main processing function
process_seed() {
    log_info "Starting Claude seed processing..."
    log_info "Host user: $HOST_USER"
    log_info "Container user: $CONTAINER_USER"
    log_info "Source: $SOURCE_DIR"
    log_info "Output: $OUTPUT_DIR"

    # Check source directory
    if [[ ! -d "$SOURCE_DIR" ]]; then
        log_error "Source directory not found: $SOURCE_DIR"
        exit 1
    fi

    # Prepare output directory
    log_info "Preparing output directory..."
    rm -rf "$OUTPUT_DIR"
    mkdir -p "$OUTPUT_DIR"

    # Initialize log file
    mkdir -p "$(dirname "$LOG_FILE")"
    echo "=== Claude Seed Processing Log ===" > "$LOG_FILE"
    echo "Started at: $(date)" >> "$LOG_FILE"
    echo "Host user: $HOST_USER" >> "$LOG_FILE"
    echo "Source: $SOURCE_DIR" >> "$LOG_FILE"
    echo "Output: $OUTPUT_DIR" >> "$LOG_FILE"
    echo "" >> "$LOG_FILE"

    # 1. Process JSON configuration files (settings.json, settings.local.json)
    log_info "Processing JSON configuration files..."
    for file in settings.json settings.local.json; do
        if [[ -f "$SOURCE_DIR/$file" ]]; then
            log_info "  Processing $file"

            # Replace hardcoded paths
            # macOS pattern: /Users/username -> /home/hotplex
            sed "s|/Users/${HOST_USER}|/home/${CONTAINER_USER}|g" \
                "$SOURCE_DIR/$file" > "$OUTPUT_DIR/$file"

            # Additional replacements for safety
            # Linux pattern (if running on Linux host)
            if [[ "$(uname)" == "Linux" ]]; then
                sed -i "s|/home/${HOST_USER}|/home/${CONTAINER_USER}|g" "$OUTPUT_DIR/$file"
            fi

            log_success "  ✓ $file processed"
        else
            log_warn "  $file not found, skipping"
        fi
    done

    # 2. Process skills directory (skip symlinks, plugins, projects, backups, benchmarks, and cache directories)
    log_info "Processing skills directory..."
    if [[ -d "$SOURCE_DIR/skills" ]]; then
        # Copy only regular files and directories, skip symlinks, heavy directories, caches, backups, and benchmarks
        find "$SOURCE_DIR/skills" -mindepth 1 -type f \( -name "*.md" -o -name "*.txt" -o -name "*.json" \) ! -type l -print0 | \
        while IFS= read -r -d '' file; do
            # Skip if in excluded directories
            rel_path="${file#$SOURCE_DIR/skills/}"

            # Skip backup, benchmark, cache directories
            if [[ "$rel_path" =~ (^|/)(\.backup|\.mypy_cache|__pycache__|\.pytest_cache|node_modules|\.git)(/|$) ]] || \
               [[ "$rel_path" =~ backup ]] || \
               [[ "$rel_path" =~ (^|/)benchmarks(/|$) ]]; then
                continue
            fi

            target_path="$OUTPUT_DIR/skills/$rel_path"

            mkdir -p "$(dirname "$target_path")"
            cp "$file" "$target_path"

            # Process paths in skill files
            if file "$target_path" | grep -q "text"; then
                sed -i '' "s|/Users/${HOST_USER}|/home/${CONTAINER_USER}|g" "$target_path" 2>/dev/null || \
                sed -i "s|/Users/${HOST_USER}|/home/${CONTAINER_USER}|g" "$target_path" 2>/dev/null || true

                if [[ "$(uname)" == "Linux" ]]; then
                    sed -i "s|/home/${HOST_USER}|/home/${CONTAINER_USER}|g" "$target_path" 2>/dev/null || true
                fi
            fi
        done

        log_success "  ✓ skills/ processed (symlinks, caches, backups, and benchmarks skipped)"
    else
        log_warn "  skills/ directory not found"
    fi

    # 3. Process scripts, hooks, and statusline directories (resolve symlinks)
    log_info "Processing scripts, hooks, and statusline directories..."

    # Process scripts
    if [[ -d "$SOURCE_DIR/scripts" ]]; then
        mkdir -p "$OUTPUT_DIR/scripts"
        find "$SOURCE_DIR/scripts" -type f ! -type l -print0 | while IFS= read -r -d '' file; do
            rel_path="${file#$SOURCE_DIR/scripts/}"
            target_path="$OUTPUT_DIR/scripts/$rel_path"
            mkdir -p "$(dirname "$target_path")"
            cp "$file" "$target_path"
        done
        log_success "  ✓ scripts/ processed"
    else
        log_warn "  scripts/ directory not found"
    fi

    # Process hooks
    if [[ -d "$SOURCE_DIR/hooks" ]]; then
        mkdir -p "$OUTPUT_DIR/hooks"
        find "$SOURCE_DIR/hooks" -type f ! -type l -print0 | while IFS= read -r -d '' file; do
            rel_path="${file#$SOURCE_DIR/hooks/}"
            target_path="$OUTPUT_DIR/hooks/$rel_path"
            mkdir -p "$(dirname "$target_path")"
            cp "$file" "$target_path"
        done
        log_success "  ✓ hooks/ processed"
    else
        log_info "  hooks/ directory not found (optional)"
    fi

    # Process statusline.sh
    if [[ -f "$SOURCE_DIR/statusline.sh" ]]; then
        mkdir -p "$OUTPUT_DIR"
        # Resolve if symlink
        if [[ -L "$SOURCE_DIR/statusline.sh" ]]; then
            cp "$SOURCE_DIR/statusline.sh" "$OUTPUT_DIR/statusline.sh"
        else
            cp "$SOURCE_DIR/statusline.sh" "$OUTPUT_DIR/statusline.sh"
        fi

        # Process paths in statusline.sh
        if file "$OUTPUT_DIR/statusline.sh" | grep -q "text"; then
            sed -i '' "s|/Users/${HOST_USER}|/home/${CONTAINER_USER}|g" "$OUTPUT_DIR/statusline.sh" 2>/dev/null || \
            sed -i "s|/Users/${HOST_USER}|/home/${CONTAINER_USER}|g" "$OUTPUT_DIR/statusline.sh" 2>/dev/null || true

            if [[ "$(uname)" == "Linux" ]]; then
                sed -i "s|/home/${HOST_USER}|/home/${CONTAINER_USER}|g" "$OUTPUT_DIR/statusline.sh" 2>/dev/null || true
            fi
        fi
        chmod +x "$OUTPUT_DIR/statusline.sh"
        log_success "  ✓ statusline.sh processed"
    else
        log_info "  statusline.sh not found (optional)"
    fi

    # 4. Copy additional config files (exclude large directories)
    log_info "Copying additional config files..."
    for file in ".claude.json" "statusline.sh"; do
        if [[ -f "$SOURCE_DIR/$file" ]]; then
            cp "$SOURCE_DIR/$file" "$OUTPUT_DIR/"
            log_success "  ✓ $file copied"
        fi
    done

    # 5. Final verification
    log_info "Running final verification..."
    if verify_seed; then
        log_success "══════════════════════════════════════════"
        log_success "Claude seed processing completed!"
        log_success "Output: $OUTPUT_DIR"
        log_success "Log: $LOG_FILE"
        log_success "══════════════════════════════════════════"
        return 0
    else
        log_error "Verification failed after processing"
        log_error "Check log file: $LOG_FILE"
        return 1
    fi
}

# Main execution
main() {
    if [[ "$VERIFY_ONLY" == "true" ]]; then
        verify_seed
        exit $?
    else
        process_seed
        exit $?
    fi
}

# Run main
main "$@"
