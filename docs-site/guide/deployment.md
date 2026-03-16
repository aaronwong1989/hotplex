# Production Deployment

## Scaling HotPlex to Enterprise Grade

Moving from local development to a production-grade deployment requires a focus on **Reliability, Scalability, and Security.** This guide outlines the best practices for deploying HotPlex in a professional environment.

---

### Command Line Options

HotPlex supports the following command line flags:

| Flag | Description |
| :--- | :---------- |
| `--config` | Path to server config YAML file |
| `--config-dir` | ChatApps config directory |
| `--env-file` | Path to .env file |

```bash
# Use custom config file
./hotplexd --config /etc/hotplex/config.yaml

# Use custom env file
./hotplexd --env-file /etc/hotplex/.env

# Use custom ChatApps config directory
./hotplexd --config-dir /etc/hotplex/chatapps
```

---

### Configuration File Discovery

HotPlex automatically discovers configuration files in the following priority order:

#### .env File Discovery

1. **CLI Flag**: `--env-file` parameter
2. **Environment Variable**: `ENV_FILE`
3. **Current Directory**: `.env` in working directory
4. **XDG Config**: `~/.config/hotplex/.env` (fallback)

```bash
# Priority 1: Explicit flag
./hotplexd --env-file /path/to/.env

# Priority 2: ENV_FILE environment variable
export ENV_FILE=/path/to/.env
./hotplexd

# Priority 3: .env in current directory
./hotplexd  # looks for .env in CWD

# Priority 4: XDG fallback
./hotplexd  # looks for ~/.config/hotplex/.env
```

---

### XDG Base Directory Support

HotPlex follows the [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec.html) for configuration and data files.

| Environment Variable | Default Value | Description |
| :------------------- | :------------ | :---------- |
| `XDG_CONFIG_HOME` | `~/.config` | Configuration files directory |
| `XDG_DATA_HOME` | `~/.local/share` | Data files directory |
| `XDG_STATE_HOME` | `~/.local/state` | State files directory |

#### HotPlex XDG Paths

| Path | Environment Variable Override | Default Location |
| :--- | :---------------------------- | :--------------- |
| Config | `HOTPLEX_CONFIG_DIR` | `~/.config/hotplex` |
| Data | `HOTPLEX_DATA_ROOT` | `~/.local/share/hotplex` |
| Logs | - | `~/.local/share/hotplex/logs` |
| Sessions | - | `~/.config/hotplex/sessions` |

```bash
# Example: Custom XDG paths
export XDG_CONFIG_HOME=/opt/hotplex/config
export XDG_DATA_HOME=/opt/hotplex/data
./hotplexd

# Or use environment variables directly
export HOTPLEX_DATA_ROOT=/mnt/hotplex/data
export HOTPLEX_CONFIG_DIR=/mnt/hotplex/config
```

---

### SSOT Configuration System (v0.30.0+)

HotPlex v0.30.0 introduces a **Single Source of Truth (SSOT)** configuration architecture with inheritance support.

#### Configuration Directory Structure

```
configs/
├── base/                    # SSOT: Base configuration templates
│   ├── server.yaml          # Core server configuration
│   ├── slack.yaml           # Slack adapter configuration
│   └── feishu.yaml          # Feishu adapter configuration
├── admin/                   # Admin bot configuration (inherits base/)
│   ├── slack.yaml           # inherits: ../base/slack.yaml
│   └── server.yaml          # inherits: ../base/server.yaml
└── templates/               # Role templates for new instances
    └── roles/              # Role-specific system prompts
        ├── go.yaml         # Go Backend Engineer
        ├── frontend.yaml   # React/Next.js Frontend Engineer
        └── devops.yaml     # Docker/K8s DevOps Engineer
```

#### Configuration Inheritance

Use the `inherits` field to extend base configurations:

```yaml
# configs/admin/slack.yaml
inherits: ../base/slack.yaml

# Override specific values
assistant:
  bot_user_id: "${HOTPLEX_ADMIN_BOT_USER_ID}"
  dm_policy: allow
```

#### Environment Variable Expansion

Use `${VAR}` syntax in YAML configurations:

```yaml
# configs/base/slack.yaml
provider:
  type: ${HOTPLEX_PROVIDER_TYPE}
  api_key: ${HOTPLEX_API_KEY}

engine:
  work_dir: ~/hotplex/projects
```

> **Note**: Shell-style defaults (`${VAR:-default}`) are NOT supported. Use `${VAR}` only.

---

### Deployment Strategies

#### 1. 🐳 Containerization (Recommended)
The official `hotplex` Docker image is the preferred way to deploy. It is optimized for size and security.

```bash
docker pull hrygo/hotplex:latest
docker run -p 8080:8080 -v ./config:/etc/hotplex -e HOTPLEX_STATE_DB=postgres://...
```

#### 2. ☸️ Kubernetes
For large-scale deployments, use our official **Helm Chart**. This provides built-in:
- **High Availability**: Multi-replica deployments with leader election.
- **Auto-scaling**: Scale workers based on message throughput.
- **Ingress Management**: Automated SSL termination and routing.

---

### Hardening Your Instance

In production, security is paramount:

- **External State Stores**: Move away from in-memory or SQLite. Use **PostgreSQL or Redis** for cross-node state persistence.
- **TLS Everywhere**: Always run `hotplexd` behind a reverse proxy (Nginx, Traefik) providing TLS 1.3 encryption.
- **Authentication**: Enable the `AuthHook` to integrate with your existing OIDC or LDAP provider.

---

### Resource Planning

| Load Level | CPU    | RAM   | Max Concurrent Sessions |
| :--------- | :----- | :---- | :---------------------- |
| **Small**  | 2 vCPU | 4 GB  | ~50                     |
| **Medium** | 4 vCPU | 8 GB  | ~200                    |
| **Large**  | 8 vCPU | 16 GB | ~1000+                  |

---

### Monitoring Health

Always configure a liveness probe directed at our health endpoint:

```http
GET /health
```

[View the Docker Deployment Guide](https://github.com/hrygo/hotplex/blob/main/docs/production-guide.md)

## Alternative: Local Docker Build

```bash
# Build locally
docker build -t hotplex:local .

# Run with persistent storage
docker run -p 8080:8080 \
  -v hotplex-data:/data \
  -e HOTPLEX_STATE_DB="sqlite:///data/hotplex.db" \
  hotplex:local
```

---

### Docker 1+n Architecture

HotPlex uses a **1+n architecture** (1 Base + n Stacks) for optimal build caching and environment specialization.

#### Available Images

| Image | Description | Includes |
| :---- | :---------- | :------- |
| `hotplex:base` | Shared base image | Go 1.25, Python 3.14, Node.js 24 |
| `hotplex:node` | Node.js stack | Node.js 24, bun, npm tools |
| `hotplex:python` | Python stack | Python 3.14, uv, pydantic-ai |
| `hotplex:java` | Java stack | Java 21, Maven/Gradle |
| `hotplex:rust` | Rust stack | Rust 1.94, cargo tools |
| `hotplex:full` | All-in-one | All stacks combined |

#### Build Commands

```bash
# Build base image only
make docker-build-base

# Build specific stack
make docker-build-stack STACK=python

# Build all stacks
make stack-all
```

---

### PIP_TOOLS Support (v0.28.0+)

HotPlex Docker images support automatic Python package installation via the `PIP_TOOLS` environment variable.

#### Usage

```yaml
# docker-compose.yml
services:
  hotplex:
    image: hrygo/hotplex:python
    environment:
      - PIP_TOOLS=notebooklm-py:notebooklm,pandas,numpy
```

#### Format

- Package names only: `pkg1,pkg2,pkg3`
- With binary alias: `pkg:bin,pkg2` (installs `pkg` but creates `bin` command)

#### Examples

| PIP_TOOLS | Effect |
| :-------- | :------ |
| `pandas` | Install pandas package |
| `notebooklm-py:notebooklm` | Install notebooklm-py, create `notebooklm` command |
| `pandas,numpy,requests` | Install multiple packages |
