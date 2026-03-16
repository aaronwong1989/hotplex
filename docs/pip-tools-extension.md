# PIP_TOOLS Dynamic Extension Mechanism

> Technical documentation for the runtime Python package extension system

## Overview

HotPlex Docker images support dynamic installation of Python packages at container startup via the `PIP_TOOLS` environment variable. This mechanism enables zero-image-rebuild extension of CLI capabilities.

## Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                     Container Startup                             │
├──────────────────────────────────────────────────────────────────┤
│  docker-entrypoint.sh                                            │
│       │                                                          │
│       ▼                                                          │
│  ┌─────────────────────────────────────────┐                    │
│  │  Check PIP_TOOLS environment variable   │                    │
│  │  Format: "pkg1:bin1 pkg2:bin2 ..."      │                    │
│  └─────────────────────────────────────────┘                    │
│       │                                                          │
│       ▼                                                          │
│  ┌─────────────────────────────────────────┐                    │
│  │  For each entry:                         │                    │
│  │  1. Extract package name & binary name   │                    │
│  │  2. Validate package name (security)     │                    │
│  │  3. Check if binary already exists       │                    │
│  │  4. Install via uv (fast) or pip3        │                    │
│  └─────────────────────────────────────────┘                    │
│       │                                                          │
│       ▼                                                          │
│  Execute CMD (hotplexd, bash, etc.)                             │
└──────────────────────────────────────────────────────────────────┘
```

## Usage

### Environment Variable Format

```bash
PIP_TOOLS="package-name:binary-name [package-name:binary-name ...]"
```

| Component | Description | Example |
|-----------|-------------|---------|
| `package-name` | PyPI package name (supports extras) | `notebooklm-py[browser]` |
| `binary-name` | Executable to check for existence | `notebooklm` |

### Examples

```yaml
# docker-compose.yml or .env
environment:
  # Single package
  PIP_TOOLS: "pandas:pandas"

  # Multiple packages
  PIP_TOOLS: "pandas:pandas requests:requests numpy:numpy"

  # Package with extras
  PIP_TOOLS: "notebooklm-py[browser]:notebooklm"
```

### Docker Compose Example

```yaml
services:
  hotplex-custom:
    image: ghcr.io/hotplex/hotplex:latest-go
    environment:
      - PIP_TOOLS=notebooklm-py:notebooklm pandas:pandas
```

## Implementation Details

### Entry Point Logic

Location: `docker/docker-entrypoint.sh` (lines 182-226)

```bash
if [[ -n "${PIP_TOOLS:-}" ]]; then
    echo "--> Checking pip tools: ${PIP_TOOLS}"

    for tool in ${PIP_TOOLS}; do
        # Extract package name (before :) and binary name (after :)
        pkg_name="${tool%%:*}"
        bin_name="${tool#*:}"

        # Security: Validate package name to prevent command injection
        if ! validate_pkg_name "${pkg_name}"; then
            echo "--> ERROR: Skipping invalid package name: ${pkg_name}"
            continue
        fi

        # Check if binary exists
        if ! command -v "${bin_name}" >/dev/null 2>&1; then
            echo "--> Installing ${pkg_name} (binary: ${bin_name})..."
            # Installation logic...
        fi
    done
fi
```

### Validation Function

```bash
validate_pkg_name() {
    local name="$1"
    # Allow: letters, numbers, hyphens, underscores, dots (for version specs)
    if [[ ! "$name" =~ ^[a-zA-Z0-9._-]+$ ]]; then
        echo "ERROR: Invalid package name: $name" >&2
        return 1
    fi
    return 0
}
```

### Installation Order

1. **uv** (preferred): Fast Rust-based installer
   ```bash
   uv pip install --system --break-system-packages --no-cache "${pkg_name}"
   ```

2. **pip3** (fallback): Standard Python installer
   ```bash
   pip3 install --break-system-packages --no-cache-dir "${pkg_name}"
   ```

## Security Considerations

| Aspect | Mitigation |
|--------|------------|
| **Command Injection** | Package name validation via regex `^[a-zA-Z0-9._-]+$` |
| **Privilege Escalation** | Installation runs as `hotplex` user (not root) |
| **Supply Chain** | Packages sourced from PyPI (trusted by design) |
| **Cache Bloat** | `--no-cache` flag prevents accumulation |

## Performance Characteristics

| Scenario | Time (approx.) |
|----------|---------------|
| Binary exists (skip) | < 10ms |
| Small package (uv) | 1-3s |
| Large package (uv) | 5-15s |
| Small package (pip3) | 3-8s |
| Large package (pip3) | 10-30s |

## Built-in vs Dynamic Installation

### Pre-installed Packages (Dockerfile.base)

Packages required by core features are pre-installed for zero startup delay:

```dockerfile
# Pre-install notebooklm-py CLI (for NotebookLM skill)
# Browser support requires local GUI - container uses CLI mode only
RUN pip3 install --break-system-packages "notebooklm-py"
```

### When to Use Each Method

| Scenario | Recommendation |
|----------|---------------|
| Core feature dependency | Pre-install in Dockerfile.base |
| Optional/skill-specific tool | Use PIP_TOOLS |
| Large package (>100MB) | Pre-install to avoid startup delay |
| Packages needing system deps | Pre-install with `playwright install-deps` |

## Troubleshooting

### Binary Not Found After Installation

```bash
# Check if package was installed
docker exec hotplex-01 pip3 list | grep <package>

# Verify binary location
docker exec hotplex-01 which <binary>

# Check entrypoint logs
docker logs hotplex-01 2>&1 | grep "pip tools"
```

### Package Validation Failed

```
--> ERROR: Skipping invalid package name: foo;rm -rf /
```

This indicates a potential command injection attempt. The validation regex blocked execution.

### Installation Timeout

For large packages, consider pre-installing in the Docker image:

```dockerfile
# In Dockerfile.base
RUN pip3 install --break-system-packages "large-package"
```

## Related Files

| File | Purpose |
|------|---------|
| `docker/Dockerfile.base` | Base image with pre-installed tools |
| `docker/docker-entrypoint.sh` | PIP_TOOLS runtime logic |
| `docker/matrix/.env-XX` | Per-bot environment configuration |

## History

- **v0.26.x**: Initial PIP_TOOLS implementation
- **v0.27.0**: Pre-installed `notebooklm-py` CLI for NotebookLM skill
