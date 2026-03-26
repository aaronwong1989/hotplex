#!/usr/bin/env bash
#
# Claude Configuration Seed Processor (Incremental + Directory-level mtime)
# Transforms host-machine ~/.claude/ configs for container compatibility
#
# Usage:
#   ./scripts/claude-seed-processor.sh [--verify] [--force] [--stats]
#
# Options:
#   --verify    Only verify existing seed, skip processing
#   --force     Force full reprocessing (ignore manifest)
#   --stats     Show change statistics after processing
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
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
SOURCE_DIR="${HOME}/.claude"
OUTPUT_DIR="${HOME}/.hotplex/claude-seed"
MANIFEST_FILE="${HOME}/.hotplex/claude-seed.manifest"
HOST_USER=$(whoami)
CONTAINER_USER="hotplex"
LOG_FILE="${HOME}/.hotplex/claude-seed.log"

# Detect sed compatibility (BSD vs GNU)
if sed --version 2>&1 | grep -q "GNU"; then
    SED_CMD=("sed" "-i")
else
    SED_CMD=("sed" "-i" "")
fi

# Parse arguments
VERIFY_ONLY=false
FORCE_REPROCESS=false
SHOW_STATS=false
for arg in "${1:-}"; do
    case "$arg" in
        --verify) VERIFY_ONLY=true ;;
        --force)  FORCE_REPROCESS=true ;;
        --stats)  SHOW_STATS=true ;;
    esac
done

# Stats (global)
STAT_CHANGED=0
STAT_SKIPPED=0
STAT_ERRORS=0

# Logging
log_info()   { echo -e "${BLUE}[INFO]${NC}   $1";  echo "[$(date '+%Y-%m-%d %H:%M:%S')] [INFO] $1" >> "$LOG_FILE"; }
log_success(){ echo -e "${GREEN}[SUCCESS]${NC} $1"; echo "[$(date '+%Y-%m-%d %H:%M:%S')] [SUCCESS] $1" >> "$LOG_FILE"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1";  echo "[$(date '+%Y-%m-%d %H:%M:%S')] [WARN] $1" >> "$LOG_FILE"; }
log_error() { echo -e "${RED}[ERROR]${NC}  $1" >&2; echo "[$(date '+%Y-%m-%d %H:%M:%S')] [ERROR] $1" >> "$LOG_FILE"; }
log_changed(){ echo -e "${GREEN}[CHANGED]${NC} $1"; }
log_skip()  { echo -e "${CYAN}[SKIP]${NC}   $1"; }

# ------------------------------------------------------------------------------
# Path replacement (cross-platform)
# ------------------------------------------------------------------------------
replace_paths() {
    "${SED_CMD[@]}" \
        -e "s|/Users/${HOST_USER}|/home/${CONTAINER_USER}|g" \
        -e "s|/home/${HOST_USER}|/home/${CONTAINER_USER}|g" \
        "$1" 2>/dev/null || true
}

# ------------------------------------------------------------------------------
# Fast fingerprint: mtime + size (portable)
# ------------------------------------------------------------------------------
file_fingerprint() {
    if [[ ! -f "$1" ]]; then echo "MISSING"; return; fi
    if [[ "$(uname)" == "Darwin" ]]; then
        stat -f "%m %z" "$1" 2>/dev/null || echo "0 0"
    else
        stat -c "%Y %s" "$1" 2>/dev/null || echo "0 0"
    fi
}

dir_mtime() {
    if [[ "$(uname)" == "Darwin" ]]; then
        stat -f "%m" "$1" 2>/dev/null || echo "0"
    else
        stat -c "%Y" "$1" 2>/dev/null || echo "0"
    fi
}

# ------------------------------------------------------------------------------
# Manifest system (Bash 3 compatible)
# Format: "D:<dirpath>|<mtime>" for dirs, "<filepath>|<mtime_size>" for files
# ------------------------------------------------------------------------------
UPDATES_FILE=""
MANIFEST_AWK=""

open_updates() {
    UPDATES_FILE=$(mktemp "${MANIFEST_FILE}.tmp.XXXXXX") || \
        UPDATES_FILE="/tmp/claude-seed-updates.$$"
}

# Build awk associative array from manifest (single awk process for all lookups)
build_awk_lookup() {
    MANIFEST_AWK=$(mktemp "${MANIFEST_FILE}.awk.XXXXXX") || \
        MANIFEST_AWK="/tmp/claude-seed-awk.$$"
    awk -F'|' '{
        gsub(/[\\]/, "\\\\"); gsub(/"/, "\\\"")
        arr[$1]=$2
    } END {
        for (k in arr) printf "A[%s]=%q\n", k, arr[k]
    }' "$MANIFEST_FILE" 2>/dev/null > "$MANIFEST_AWK" || true
}

manifest_update() {
    printf '%s|%s\n' "$1" "$2" >> "$UPDATES_FILE" 2>/dev/null || true
}

# Single awk call to check MANIFEST_AWK for a list of files
# Returns fingerprint from manifest or empty
awk_lookup() {
    local key="F:$1"
    awk -F'|' -v k="$key" '
        BEGIN { n=ARGC-1; for(i=1;i<=n;i++){f=ARGV[i];arr[f]=1} ARGC=0 }
        FNR==NR && ($1==":" || $1~/^[DF]:/){next}
        { split($0,f,"|"); if(arr["F:"f[1]]) print f[1],f[2] }
    ' "$(cat "$MANIFEST_AWK")" /dev/null 2>/dev/null || echo ""
}

# Check multiple files at once using awk
# Reads list of files from stdin (one per line, relative path)
# Prints: "<relpath> <manifest_fp>" for each file (manifest_fp empty if not found)
awk_batch_lookup() {
    awk -F'|' -v d="${1:-}" '
    BEGIN {
        # Read manifest from ENV var via the awk data file
        while ((getline line < ENVIRON["AWKMANIFEST"]) > 0) {
            split(line, p, "|"); m[p[1]] = p[2]
        }
        close(ENVIRON["AWKMANIFEST"])
    }
    FNR==1 { next }
    {
        key = (d == "" ? $0 : d "/" $0)
        fp = (("F:" key) in m ? m["F:" key] : ("D:" key) in m ? m["D:" key] : "")
        print $0 "|" fp
    }' /dev/null 2>/dev/null
}

# Quick dir mtime lookup via grep (only 4 dirs to check)
manifest_dir_mtime() {
    grep "^D:${1}|" "$MANIFEST_FILE" 2>/dev/null | tail -1 | cut -d'|' -f2 || echo ""
}

save_manifest() {
    # Merge existing + new updates, keep last entry per key
    {
        grep -v "^$" "$MANIFEST_FILE" 2>/dev/null || true
        cat "$UPDATES_FILE"
    } | sort -t'|' -k1,1 -u | \
      awk -F'|' '{if (!seen[$1]++) print}' > "${MANIFEST_FILE}.new" && \
      mv "${MANIFEST_FILE}.new" "$MANIFEST_FILE"
    # Cleanup
    rm -f "$UPDATES_FILE" "${MANIFEST_AWK}" 2>/dev/null || true
    UPDATES_FILE=""
    MANIFEST_AWK=""
}

# Returns 0 if file changed, 1 if unchanged
needs_processing() {
    local fp prev
    fp=$(file_fingerprint "$1")
    # Use grep on manifest (fast for ~3752 entries)
    prev=$(grep "^${2}|" "$MANIFEST_FILE" 2>/dev/null | tail -1 | cut -d'|' -f2 || echo "")
    if [[ -z "$prev" ]] || [[ "$fp" != "$prev" ]]; then
        manifest_update "$2" "$fp"
        return 0
    fi
    return 1
}

# Returns 0 if directory has changed, 1 if unchanged
dir_needs_scan() {
    local dm="$1" dp="$2" cur prev
    cur=$(dir_mtime "$dm")
    prev=$(manifest_dir_mtime "$dp")
    if [[ -z "$prev" ]] || [[ "$cur" != "$prev" ]]; then
        manifest_update "D:$dp" "$cur"
        return 0
    fi
    return 1
}

# Count files in dir (portable)
count_files() {
    find "$1" -type f 2>/dev/null | wc -l | tr -d ' '
}

# ------------------------------------------------------------------------------
# Verify seed (quick grep)
# ------------------------------------------------------------------------------
verify_seed() {
    log_info "Verifying seed at $OUTPUT_DIR..."
    if [[ ! -d "$OUTPUT_DIR" ]]; then
        log_error "Seed directory not found: $OUTPUT_DIR"
        return 1
    fi
    local leaks=0
    for pat in "$HOST_USER" "/Users/"; do
        if grep -rI --exclude-dir="plugins" --exclude="*.log" \
             --exclude="*.md" "$pat" "$OUTPUT_DIR" 2>/dev/null | \
             grep -v "marketplaces" | grep -q . 2>/dev/null; then
            log_error "Found hardcoded path: $pat"
            leaks=1
            break
        fi
    done
    [[ $leaks -eq 1 ]] && return 1
    log_success "Verification passed: no hardcoded paths found"
    return 0
}

# Helper: process a single dir with dir-level mtime precheck
# Args: src_dir dst_dir rel_prefix file_filter_pattern
# Sets C_CHANGED and C_SKIPPED
process_dir() {
    local src="$1" dst="$2" rel_pfx="$3" filter="$4"
    local dm cur prev changed=0 skipped=0

    # Directory mtime check (always, even in force mode)
    cur=$(dir_mtime "$src")
    prev=$(manifest_dir_mtime "$rel_pfx")
    if [[ -n "$prev" ]] && [[ "$cur" == "$prev" ]]; then
        local cnt; cnt=$(count_files "$src")
        log_info "  $rel_pfx: unchanged (mtime), skip $cnt files"
        STAT_SKIPPED=$((STAT_SKIPPED + cnt))
        return
    fi
    manifest_update "D:$rel_pfx" "$cur"

    # Scan files
    while IFS= read -r -d '' file; do
        local rel="${file#$src/}"
        local full_rel="${rel_pfx}/${rel}"

        if needs_processing "$file" "$full_rel"; then
            local target="$dst/$rel"
            mkdir -p "$(dirname "$target")"
            if cp "$file" "$target" 2>/dev/null; then
                replace_paths "$target"
                log_changed "  $full_rel"
                ((changed++))
            fi
        else
            ((skipped++))
        fi
    done < <(find "$src" -type f -print0 2>/dev/null)

    STAT_CHANGED=$((STAT_CHANGED + changed))
    STAT_SKIPPED=$((STAT_SKIPPED + skipped))
    if [[ $changed -gt 0 ]]; then
        log_success "  $rel_pfx: $changed changed, $skipped unchanged"
    else
        log_info "  $rel_pfx: unchanged ($skipped files)"
    fi
}

# ------------------------------------------------------------------------------
# Process skills
# ------------------------------------------------------------------------------
process_skills() {
    log_info "Processing skills directory..."
    if [[ ! -d "$SOURCE_DIR/skills" ]]; then
        log_warn "  skills/ not found"
        return
    fi
    # skills/ root precheck
    local cur prev
    cur=$(dir_mtime "$SOURCE_DIR/skills")
    prev=$(manifest_dir_mtime "skills")
    if [[ -n "$prev" ]] && [[ "$cur" == "$prev" ]]; then
        local cnt; cnt=$(count_files "$SOURCE_DIR/skills")
        log_info "  skills: unchanged (mtime), skip $cnt files"
        STAT_SKIPPED=$((STAT_SKIPPED + cnt))
        return
    fi
    manifest_update "D:skills" "$cur"

    local changed=0 skipped=0
    while IFS= read -r -d '' file; do
        local rel="${file#$SOURCE_DIR/skills/}"
        # Skip heavy/cache dirs
        case "$rel" in
            *".backup"*|*"node_modules"*|*"/.git/"*|*"/benchmarks/"*) continue ;;
        esac
        # Filter to text files only
        case "$rel" in *.md|*.txt|*.json) ;; *) continue ;; esac

        if needs_processing "$file" "skills/$rel"; then
            local target="$OUTPUT_DIR/skills/$rel"
            mkdir -p "$(dirname "$target")"
            if cp "$file" "$target" 2>/dev/null; then
                replace_paths "$target"
                log_changed "  skills/$rel"
                ((changed++))
            fi
        else
            ((skipped++))
        fi
    done < <(find "$SOURCE_DIR/skills" -mindepth 1 -type f -print0 2>/dev/null)

    STAT_CHANGED=$((STAT_CHANGED + changed))
    STAT_SKIPPED=$((STAT_SKIPPED + skipped))
    if [[ $changed -gt 0 ]]; then
        log_success "  skills: $changed changed, $skipped unchanged"
    else
        log_info "  skills: unchanged ($skipped files)"
    fi
}

# ------------------------------------------------------------------------------
# Process scripts, hooks, statusline
# ------------------------------------------------------------------------------
process_scripts_hooks() {
    log_info "Processing scripts, hooks, and statusline..."

    # Scripts
    if [[ -d "$SOURCE_DIR/scripts" ]]; then
        process_dir "$SOURCE_DIR/scripts" "$OUTPUT_DIR/scripts" "scripts" ""
    else
        log_info "  scripts/ not found"
    fi

    # Hooks
    if [[ -d "$SOURCE_DIR/hooks" ]]; then
        process_dir "$SOURCE_DIR/hooks" "$OUTPUT_DIR/hooks" "hooks" ""
    else
        log_info "  hooks/ not found"
    fi

    # Statusline
    if [[ -f "$SOURCE_DIR/statusline.sh" ]]; then
        if needs_processing "$SOURCE_DIR/statusline.sh" "statusline.sh"; then
            cp "$SOURCE_DIR/statusline.sh" "$OUTPUT_DIR/statusline.sh"
            replace_paths "$OUTPUT_DIR/statusline.sh"
            chmod +x "$OUTPUT_DIR/statusline.sh"
            log_changed "  statusline.sh"
            STAT_CHANGED=$((STAT_CHANGED + 1))
            log_success "  statusline.sh updated"
        else
            log_info "  statusline.sh unchanged"
            STAT_SKIPPED=$((STAT_SKIPPED + 1))
        fi
    fi
}

# ------------------------------------------------------------------------------
# Process plugins
# ------------------------------------------------------------------------------
process_plugins() {
    log_info "Processing plugins directory..."
    if [[ ! -d "$SOURCE_DIR/plugins" ]]; then
        log_info "  plugins/ not found"
        return
    fi

    mkdir -p "$OUTPUT_DIR/plugins"
    local changed=0 skipped=0

    # Regular files (skip cache/)
    while IFS= read -r -d '' file; do
        local rel="${file#$SOURCE_DIR/plugins/}"
        [[ "$rel" == cache/* ]] && continue

        if needs_processing "$file" "plugins/$rel"; then
            local target="$OUTPUT_DIR/plugins/$rel"
            mkdir -p "$(dirname "$target")"
            if cp "$file" "$target" 2>/dev/null; then
                replace_paths "$target"
                log_changed "  plugins/$rel"
                ((changed++))
            fi
        else
            ((skipped++))
        fi
    done < <(find "$SOURCE_DIR/plugins" -type f -print0 2>/dev/null)

    # Subdirs: marketplaces, local, repos (each with mtime precheck)
    for subdir in marketplaces local repos; do
        local src_sub="$SOURCE_DIR/plugins/$subdir"
        [[ ! -d "$src_sub" ]] && continue
        mkdir -p "$OUTPUT_DIR/plugins/$subdir"

        local cur prev
        cur=$(dir_mtime "$src_sub")
        prev=$(manifest_dir_mtime "plugins/$subdir")
        if [[ -n "$prev" ]] && [[ "$cur" == "$prev" ]]; then
            local cnt; cnt=$(count_files "$src_sub")
            log_info "  plugins/$subdir: unchanged (mtime), skip $cnt"
            STAT_SKIPPED=$((STAT_SKIPPED + cnt))
            continue
        fi
        manifest_update "D:plugins/$subdir" "$cur"

        if [[ "$subdir" == "marketplaces" ]]; then
            while IFS= read -r -d '' mdir; do
                local mname=$(basename "$mdir")
                if needs_processing "$mdir" "plugins/$subdir/$mname"; then
                    rm -rf "$OUTPUT_DIR/plugins/$subdir/$mname" 2>/dev/null || true
                    mkdir -p "$OUTPUT_DIR/plugins/$subdir/$mname"
                    if cp -r "$mdir"/* "$OUTPUT_DIR/plugins/$subdir/$mname/" 2>/dev/null; then
                        log_changed "  plugins/$subdir/$mname"
                        ((changed++))
                    fi
                else
                    log_info "  plugins/$subdir/$mname (unchanged)"
                fi
            done < <(find "$src_sub" -mindepth 1 -maxdepth 1 -type d -print0 2>/dev/null)
        else
            while IFS= read -r -d '' f; do
                local fname=$(basename "$f")
                if needs_processing "$f" "plugins/$subdir/$fname"; then
                    if cp "$f" "$OUTPUT_DIR/plugins/$subdir/$fname" 2>/dev/null; then
                        log_changed "  plugins/$subdir/$fname"
                        ((changed++))
                    fi
                else
                    ((skipped++))
                fi
            done < <(find "$src_sub" -maxdepth 1 -type f -print0 2>/dev/null)
        fi
    done

    STAT_CHANGED=$((STAT_CHANGED + changed))
    STAT_SKIPPED=$((STAT_SKIPPED + skipped))
    if [[ $changed -gt 0 ]]; then
        log_success "  plugins: $changed changed, $skipped unchanged"
    else
        log_info "  plugins: unchanged ($skipped files)"
    fi
}

# ------------------------------------------------------------------------------
# Process JSON configs
# ------------------------------------------------------------------------------
process_json_configs() {
    log_info "Processing JSON configuration files..."
    local changed=0
    for file in settings.json settings.local.json; do
        local src="$SOURCE_DIR/$file"
        if [[ -f "$src" ]]; then
            if needs_processing "$src" "$file"; then
                cp "$src" "$OUTPUT_DIR/$file"
                replace_paths "$OUTPUT_DIR/$file"
                log_changed "  $file"
                ((changed++))
                STAT_CHANGED=$((STAT_CHANGED + 1))
            else
                log_info "  $file (unchanged)"
                STAT_SKIPPED=$((STAT_SKIPPED + 1))
            fi
        else
            log_warn "  $file not found"
        fi
    done
    [[ $changed -gt 0 ]] && log_success "  $changed JSON file(s) updated"
}

# ------------------------------------------------------------------------------
# Process .claude.json
# ------------------------------------------------------------------------------
process_claude_json() {
    local src="${HOME}/.claude.json"
    log_info "Processing .claude.json..."
    if needs_processing "$src" ".claude.json"; then
        if [[ -f "$src" ]] && command -v jq >/dev/null 2>&1; then
            local tmp="$OUTPUT_DIR/.claude.json.tmp"
            if jq '{ hasCompletedOnboarding, mcpServers }' "$src" > "$tmp" 2>/dev/null; then
                replace_paths "$tmp"
                mv "$tmp" "$OUTPUT_DIR/.claude.json"
                log_changed "  .claude.json (minimal)"
                STAT_CHANGED=$((STAT_CHANGED + 1))
            else
                echo '{"hasCompletedOnboarding":true}' > "$OUTPUT_DIR/.claude.json"
                log_warn "  jq failed, created minimal config"
                STAT_CHANGED=$((STAT_CHANGED + 1))
            fi
        else
            echo '{"hasCompletedOnboarding":true}' > "$OUTPUT_DIR/.claude.json"
            log_changed "  .claude.json (minimal)"
            STAT_CHANGED=$((STAT_CHANGED + 1))
        fi
    else
        log_info "  .claude.json (unchanged)"
        STAT_SKIPPED=$((STAT_SKIPPED + 1))
    fi
}

# ------------------------------------------------------------------------------
# Cleanup trap
# ------------------------------------------------------------------------------
cleanup() {
    rm -f "$UPDATES_FILE" "${MANIFEST_LOOKUP}" 2>/dev/null || true
}
trap cleanup EXIT

# ------------------------------------------------------------------------------
# Main
# ------------------------------------------------------------------------------
process_seed() {
    log_info "Starting Claude seed processing..."
    log_info "Host user: $HOST_USER"
    log_info "Container user: $CONTAINER_USER"
    log_info "Source: $SOURCE_DIR"
    log_info "Output: $OUTPUT_DIR"

    if [[ ! -d "$SOURCE_DIR" ]]; then
        log_error "Source directory not found: $SOURCE_DIR"
        exit 1
    fi

    mkdir -p "$OUTPUT_DIR"
    mkdir -p "$(dirname "$LOG_FILE")"
    echo "=== Claude Seed Processing Log ===" > "$LOG_FILE"
    echo "Started at: $(date)" >> "$LOG_FILE"

    local start_sec; start_sec=$(date +%s)

    open_updates

    if [[ "$FORCE_REPROCESS" == "true" ]]; then
        log_warn "Force mode: full reprocessing"
    else
        load_manifest
        local cnt; cnt=$(grep -c . "$MANIFEST_FILE" 2>/dev/null || echo "0")
        log_info "Incremental mode: $cnt manifest entries"
    fi

    process_json_configs
    process_skills
    process_scripts_hooks
    process_plugins
    process_claude_json

    save_manifest

    local elapsed=$(( $(date +%s) - start_sec ))

    log_info "Running final verification..."
    if verify_seed; then
        log_success "══════════════════════════════════════════"
        log_success "Claude seed processing completed!"
        log_success "Output: $OUTPUT_DIR"
        log_success "Log: $LOG_FILE"
        log_success "══════════════════════════════════════════"
        log_info "Stats: $STAT_CHANGED changed, $STAT_SKIPPED skipped, $STAT_ERRORS errors (${elapsed}s)"
        return 0
    else
        log_error "Verification failed"
        return 1
    fi
}

main() {
    if [[ "$VERIFY_ONLY" == "true" ]]; then
        verify_seed
    else
        process_seed
    fi
    exit $?
}

main "$@"
