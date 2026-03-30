#!/usr/bin/env bash
#
# Claude Configuration Seed Processor (Incremental + Directory mtime)
# Transforms host ~/.claude/ configs for container compatibility
#
# Usage: ./scripts/claude-seed-processor.sh [--verify] [--force] [--stats]
#

set -euo pipefail

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
BLUE='\033[0;34m'; CYAN='\033[0;36m'; NC='\033[0m'

SOURCE_DIR="${HOME}/.claude"
OUTPUT_DIR="${HOME}/.hotplex/claude-seed"
MANIFEST_FILE="${HOME}/.hotplex/claude-seed.manifest"
HOST_USER=$(whoami)
CONTAINER_USER="hotplex"
LOG_FILE="${HOME}/.hotplex/claude-seed.log"

SED_I_FLAG="-i"
sed --version 2>&1 | grep -q "GNU" || SED_I_FLAG='-i ""'

# Args
VERIFY_ONLY=false; FORCE_REPROCESS=false; SHOW_STATS=false
for arg in "${1:-}"; do
    case "$arg" in
        --verify) VERIFY_ONLY=true ;;
        --force)  FORCE_REPROCESS=true ;;
        --stats)  SHOW_STATS=true ;;
    esac
done

S_CHANGED=0; S_SKIPPED=0; S_ERRORS=0

log_info()   { echo -e "${BLUE}[INFO]${NC}   $1"; echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] $1" >> "$LOG_FILE"; }
log_success(){ echo -e "${GREEN}[SUCCESS]${NC} $1"; echo "[$(date '+%Y-%m-%d %H:%M:%S')] [SUCCESS] $1" >> "$LOG_FILE"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1"; echo "[$(date '+%Y-%m-%d %H:%M:%S')] [WARN] $1" >> "$LOG_FILE"; }
log_error() { echo -e "${RED}[ERROR]${NC}  $1" >&2; echo "[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] $1" >> "$LOG_FILE"; }
log_chg()  { echo -e "${GREEN}[+]${NC} $1"; }

# Batch log collector (accumulates lines, flushes once)
_batched_chg=()
flush_chg() {
    if [[ ${#_batched_chg[@]} -gt 0 ]]; then
        for l in "${_batched_chg[@]}"; do echo -e "${GREEN}[+]${NC} $l"; done
        _batched_chg=()
    fi
}

replace_paths() {
    sed $SED_I_FLAG \
        -e "s|/Users/${HOST_USER}|/home/${CONTAINER_USER}|g" \
        -e "s|/home/${HOST_USER}|/home/${CONTAINER_USER}|g" \
        "$1" 2>/dev/null || true
}

fp() {
    [[ ! -f "$1" ]] && { echo "0"; return; }
    if [[ "$(uname)" == "Darwin" ]]; then
        stat -f "%m %z" "$1" 2>/dev/null || echo "0"
    else
        stat -c "%Y %s" "$1" 2>/dev/null || echo "0"
    fi
}

dir_mtime() {
    if [[ "$(uname)" == "Darwin" ]]; then
        stat -f "%m" "$1" 2>/dev/null || echo "0"
    else
        stat -c "%Y" "$1" 2>/dev/null || echo "0"
    fi
}

# File count
fcount() { find "$1" -type f 2>/dev/null | wc -l | tr -d ' '; }

# Manifest lookup: single grep on manifest file
# Returns fingerprint or empty string
mlookup() {
    grep "^${1}|" "$MANIFEST_FILE" 2>/dev/null | tail -1 | cut -d'|' -f2 || echo ""
}

# Dir mtime from manifest
dmtime() {
    grep "^D:${1}|" "$MANIFEST_FILE" 2>/dev/null | tail -1 | cut -d'|' -f2 || echo ""
}

# Update manifest
mupd() { printf '%s|%s\n' "$1" "$2" >> "$MANIFEST_FILE".new; }

# ------------------------------------------------------------------------------
# Batch process directory: find -> single awk batch check
# Args: src dst prefix [--force-skip-mtime-check]
process_dir() {
    local src="$1" dst="$2" pfx="$3"
    local force_skip_mtime="${4:-false}"

    # Dir mtime precheck (skip in force mode)
    local dm prev
    if ! $force_skip_mtime; then
        dm=$(dir_mtime "$src")
        prev=$(dmtime "$pfx")
        if [[ -n "$prev" ]] && [[ "$dm" == "$prev" ]]; then
            local cnt; cnt=$(fcount "$src")
            log_info "  $pfx: unchanged (mtime), skip $cnt files"
            S_SKIPPED=$((S_SKIPPED + cnt)); return
        fi
        mupd "D:$pfx" "$dm"
    fi

    # Collect files: exclude cache, node_modules, .git dirs and marketplace subdirs entirely
    # find -not -path is more reliable than grep pipelines for path filtering
    local flist; flist=$(mktemp "${MANIFEST_FILE}.fl.XXXXXX") || flist="/tmp/claude-fl.$$"
    find "$src" -type f \
        -not -path '*/cache/*' \
        -not -path '*/node_modules/*' \
        -not -path '*/.git/*' \
        -not -path '*/marketplaces/*' \
        -not -path '*/local/*' \
        -not -path '*/repos/*' \
        -print0 2>/dev/null | tr '\0' '\n' | sed "s|${src}/||" > "$flist"

    # Single awk: loads manifest, checks all files, outputs changed/skipped
    # Reads: MANIFEST_FILE (manifest), then flist (file paths)
    local results; results=$(awk -F'|' '
    FILENAME == "-" { next }  # skip stdin placeholder
    FNR == NR { arr[$1]=$2; next }  # load manifest
    {
        key = $1
        fp = arr[key]
        print key "|" fp
    }
    ' "$MANIFEST_FILE" "$flist" 2>/dev/null)

    local changed=0 skipped=0
    while IFS='|' read -r rel mfp; do
        [[ -z "$rel" ]] && continue
        [[ "$rel" == "D:"* ]] && continue
        local sfp; sfp=$(fp "$src/$rel")
        if [[ -z "$mfp" ]] || [[ "$sfp" != "$mfp" ]]; then
            mupd "$pfx/$rel" "$sfp"
            mkdir -p "$(dirname "$dst/$rel")"
            if cp "$src/$rel" "$dst/$rel" 2>/dev/null; then
                replace_paths "$dst/$rel"
                _batched_chg+=("  $pfx/$rel")
                ((changed++))
            fi
        else
            ((skipped++))
        fi
    done <<< "$results"

    rm -f "$flist"
    S_CHANGED=$((S_CHANGED + changed)); S_SKIPPED=$((S_SKIPPED + skipped))
    flush_chg
    if [[ $changed -gt 0 ]]; then
        log_success "  $pfx: $changed changed, $skipped unchanged"
    else
        log_info "  $pfx: unchanged ($skipped files)"
    fi
}

# ------------------------------------------------------------------------------
# Skills (extension filter)
# ------------------------------------------------------------------------------
process_skills() {
    local force_skip="${1:-false}"
    log_info "Processing skills directory..."
    [[ ! -d "$SOURCE_DIR/skills" ]] && { log_warn "  skills/ not found"; return; }

    local dm prev; dm=$(dir_mtime "$SOURCE_DIR/skills")
    prev=$(dmtime "skills")
    if ! $force_skip && [[ -n "$prev" ]] && [[ "$dm" == "$prev" ]]; then
        local cnt; cnt=$(find "$SOURCE_DIR/skills" -type f \( -name "*.md" -o -name "*.txt" -o -name "*.json" \) 2>/dev/null | wc -l | tr -d ' ')
        log_info "  skills: unchanged (mtime), skip $cnt files"
        S_SKIPPED=$((S_SKIPPED + cnt)); return
    fi
    mupd "D:skills" "$dm"

    local flist; flist=$(mktemp "${MANIFEST_FILE}.fl.XXXXXX") || flist="/tmp/claude-fl.$$"
    find "$SOURCE_DIR/skills" -type f \( -name "*.md" -o -name "*.txt" -o -name "*.json" \) 2>/dev/null | \
        grep -v -E '/(\.backup|node_modules|\.git|benchmarks)/' | \
        sed "s|$SOURCE_DIR/skills/||" > "$flist"

    local results; results=$(awk -F'|' '
    FNR == NR { arr[$1]=$2; next }
    { print $1 "|" arr[$1] }
    ' "$MANIFEST_FILE" "$flist" 2>/dev/null)

    local changed=0 skipped=0
    while IFS='|' read -r rel mfp; do
        [[ -z "$rel" ]] && continue
        local sfp; sfp=$(fp "$SOURCE_DIR/skills/$rel")
        if [[ -z "$mfp" ]] || [[ "$sfp" != "$mfp" ]]; then
            mupd "skills/$rel" "$sfp"
            mkdir -p "$(dirname "$OUTPUT_DIR/skills/$rel")"
            if cp "$SOURCE_DIR/skills/$rel" "$OUTPUT_DIR/skills/$rel" 2>/dev/null; then
                replace_paths "$OUTPUT_DIR/skills/$rel"
                _batched_chg+=("  skills/$rel")
                ((changed++))
            fi
        else
            ((skipped++))
        fi
    done <<< "$results"

    rm -f "$flist"
    S_CHANGED=$((S_CHANGED + changed)); S_SKIPPED=$((S_SKIPPED + skipped))
    flush_chg
    if [[ $changed -gt 0 ]]; then
        log_success "  skills: $changed changed, $skipped unchanged"
    else
        log_info "  skills: unchanged ($skipped files)"
    fi
}

# ------------------------------------------------------------------------------
# Scripts + hooks
# ------------------------------------------------------------------------------
process_scripts_hooks() {
    log_info "Processing scripts, hooks..."

    if [[ -d "$SOURCE_DIR/scripts" ]]; then
        mkdir -p "$OUTPUT_DIR/scripts"
        process_dir "$SOURCE_DIR/scripts" "$OUTPUT_DIR/scripts" "scripts" "$FORCE_MTIME_SKIP"
    else
        log_info "  scripts/ not found"
    fi

    if [[ -d "$SOURCE_DIR/hooks" ]]; then
        mkdir -p "$OUTPUT_DIR/hooks"
        process_dir "$SOURCE_DIR/hooks" "$OUTPUT_DIR/hooks" "hooks" "$FORCE_MTIME_SKIP"
    else
        log_info "  hooks/ not found"
    fi
}

# ------------------------------------------------------------------------------
# Plugins (marketplaces tracked individually)
# ------------------------------------------------------------------------------
process_plugins() {
    log_info "Processing plugins directory..."
    [[ ! -d "$SOURCE_DIR/plugins" ]] && { log_info "  plugins/ not found"; return; }
    mkdir -p "$OUTPUT_DIR/plugins"

    process_dir "$SOURCE_DIR/plugins" "$OUTPUT_DIR/plugins" "plugins" "$FORCE_MTIME_SKIP"
    # Note: D:plugins already written by process_dir's mtime check; do NOT overwrite it

    for subdir in marketplaces local repos; do
        local sp="$SOURCE_DIR/plugins/$subdir"
        [[ ! -d "$sp" ]] && continue
        mkdir -p "$OUTPUT_DIR/plugins/$subdir"

        local dm prev; dm=$(dir_mtime "$sp")
        prev=$(dmtime "plugins/$subdir")
        if [[ -n "$prev" ]] && [[ "$dm" == "$prev" ]]; then
            local cnt; cnt=$(fcount "$sp")
            log_info "  plugins/$subdir: unchanged (mtime), skip $cnt files"
            S_SKIPPED=$((S_SKIPPED + cnt)); continue
        fi
        mupd "D:plugins/$subdir" "$dm"

        if [[ "$subdir" == "marketplaces" ]]; then
            local changed=0
            find "$sp" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | \
            while IFS= read -r mdir; do
                local mname; mname=$(basename "$mdir")
                local mm dm2; mm=$(dmtime "plugins/$subdir/$mname"); dm2=$(dir_mtime "$mdir")
                if [[ -n "$mm" ]] && [[ "$dm2" == "$mm" ]]; then
                    log_info "  plugins/$subdir/$mname (unchanged)"
                    S_SKIPPED=$((S_SKIPPED + $(find "$mdir" -type f 2>/dev/null | wc -l | tr -d ' ')))
                else
                    mupd "D:plugins/$subdir/$mname" "$dm2"
                    rm -rf "$OUTPUT_DIR/plugins/$subdir/$mname" 2>/dev/null || true
                    mkdir -p "$OUTPUT_DIR/plugins/$subdir/$mname"
                    # rsync: preserve mtimes, exclude node_modules (massive, unused in container)
                    if rsync -a --exclude='node_modules' --exclude='.git' \
                        "$mdir/" "$OUTPUT_DIR/plugins/$subdir/$mname/" 2>/dev/null; then
                        # Fix execute permissions for hooks (git may lose +x bit on macOS)
                        find "$OUTPUT_DIR/plugins/$subdir/$mname/hooks" -name "*.sh" -exec chmod +x {} \; 2>/dev/null || true
                        _batched_chg+=("  plugins/$subdir/$mname")
                        ((changed++))
                    fi
                fi
            done
            S_CHANGED=$((S_CHANGED + changed)); flush_chg
        else
            process_dir "$sp" "$OUTPUT_DIR/plugins/$subdir" "plugins/$subdir"
        fi
    done
}

# ------------------------------------------------------------------------------
# JSON configs + .claude.json
# ------------------------------------------------------------------------------
process_json_configs() {
    log_info "Processing JSON configuration files..."
    for file in settings.json settings.local.json; do
        local src="$SOURCE_DIR/$file"
        if [[ -f "$src" ]]; then
            local mfp sfp; mfp=$(mlookup "$file"); sfp=$(fp "$src")
            if [[ -z "$mfp" ]] || [[ "$sfp" != "$mfp" ]]; then
                mupd "$file" "$sfp"
                cp "$src" "$OUTPUT_DIR/$file"
                replace_paths "$OUTPUT_DIR/$file"
                log_chg "  $file"; S_CHANGED=$((S_CHANGED + 1))
            else
                log_info "  $file (unchanged)"; S_SKIPPED=$((S_SKIPPED + 1))
            fi
        else
            log_warn "  $file not found"
        fi
    done
}

process_claude_json() {
    local src="${HOME}/.claude.json"
    log_info "Processing .claude.json..."
    local mfp sfp; mfp=$(mlookup ".claude.json"); sfp=$(fp "$src")
    if [[ -z "$mfp" ]] || [[ "$sfp" != "$mfp" ]]; then
        mupd ".claude.json" "$sfp"
        if [[ -f "$src" ]] && command -v jq >/dev/null 2>&1; then
            local tmp="$OUTPUT_DIR/.claude.json.tmp"
            if jq '{ hasCompletedOnboarding, mcpServers }' "$src" > "$tmp" 2>/dev/null; then
                replace_paths "$tmp"; mv "$tmp" "$OUTPUT_DIR/.claude.json"
                log_chg "  .claude.json (minimal)"; S_CHANGED=$((S_CHANGED + 1))
            else
                echo '{"hasCompletedOnboarding":true}' > "$OUTPUT_DIR/.claude.json"
                log_warn "  jq failed"; S_CHANGED=$((S_CHANGED + 1))
            fi
        else
            echo '{"hasCompletedOnboarding":true}' > "$OUTPUT_DIR/.claude.json"
            log_chg "  .claude.json (minimal)"; S_CHANGED=$((S_CHANGED + 1))
        fi
    else
        log_info "  .claude.json (unchanged)"; S_SKIPPED=$((S_SKIPPED + 1))
    fi
}

verify_seed() {
    log_info "Verifying seed at $OUTPUT_DIR..."
    [[ ! -d "$OUTPUT_DIR" ]] && { log_error "Seed dir missing"; return 1; }
    local leaks=0
    for pat in "$HOST_USER" "/Users/"; do
        # Exclude only top-level marketplace dirs from path leak check (they are mount targets)
        # Do NOT blanket-filter "marketplaces" keyword — that hides leaks inside marketplace dirs
        if grep -rI --exclude="*.log" --exclude="*.md" \
             "$pat" "$OUTPUT_DIR" 2>/dev/null | \
             grep -v "^${OUTPUT_DIR}/plugins/marketplaces/" | grep -q . 2>/dev/null; then
            log_error "Leaked path: $pat"; leaks=1; break
        fi
    done
    [[ $leaks -eq 1 ]] && return 1
    log_success "Verification passed: no hardcoded paths"
    return 0
}

# ------------------------------------------------------------------------------
# Cleanup
# ------------------------------------------------------------------------------
cleanup() { rm -f "${MANIFEST_FILE}.new" "${MANIFEST_FILE}.fl."* 2>/dev/null || true; }
trap cleanup EXIT

# ------------------------------------------------------------------------------
# Main
# ------------------------------------------------------------------------------
main() {
    if [[ "$VERIFY_ONLY" == "true" ]]; then
        verify_seed; exit $?
    fi

    log_info "Starting Claude seed processing..."
    log_info "Host: $HOST_USER -> $CONTAINER_USER"
    log_info "Source: $SOURCE_DIR"
    log_info "Output: $OUTPUT_DIR"

    [[ ! -d "$SOURCE_DIR" ]] && { log_error "Source not found"; exit 1; }
    mkdir -p "$OUTPUT_DIR" "$(dirname "$LOG_FILE")"
    echo "=== Claude Seed Log ===" > "$LOG_FILE"
    echo "Started: $(date)" >> "$LOG_FILE"

    local start; start=$(date +%s)

    # Init new manifest update file
    : > "${MANIFEST_FILE}.new"

    if [[ "$FORCE_REPROCESS" == "true" ]]; then
        log_warn "Force mode: full reprocessing"
        FORCE_MTIME_SKIP=true
    else
        FORCE_MTIME_SKIP=false
        local cnt; cnt=$(wc -l < "$MANIFEST_FILE" 2>/dev/null | tr -d ' ' || echo "0")
        log_info "Incremental: $cnt manifest entries"
    fi

    process_json_configs
    process_skills "$FORCE_MTIME_SKIP"
    process_scripts_hooks
    process_plugins
    process_claude_json

    # Merge: existing manifest + new updates
    {
        cat "$MANIFEST_FILE" 2>/dev/null || true
        cat "${MANIFEST_FILE}.new"
    } | awk -F'|' '{if (!seen[$1]++) print}' > "${MANIFEST_FILE}.merged" && \
      mv "${MANIFEST_FILE}.merged" "$MANIFEST_FILE"

    local elapsed=$(( $(date +%s) - start ))

    log_info "Running final verification..."
    if verify_seed; then
        log_success "=========================================="
        log_success "Seed processing completed!"
        log_success "Output: $OUTPUT_DIR"
        log_success "=========================================="
        log_info "Stats: $S_CHANGED changed, $S_SKIPPED skipped, $S_ERRORS errors (${elapsed}s)"
        exit 0
    else
        log_error "Verification failed"; exit 1
    fi
}

main "$@"
