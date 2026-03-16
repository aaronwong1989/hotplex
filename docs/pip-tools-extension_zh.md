# PIP_TOOLS 动态扩展机制

> Python 包运行时扩展系统技术文档

## 概述

HotPlex Docker 镜像支持通过 `PIP_TOOLS` 环境变量在容器启动时动态安装 Python 包。此机制无需重建镜像即可扩展 CLI 能力。

## 架构

```
┌──────────────────────────────────────────────────────────────────┐
│                     容器启动流程                                  │
├──────────────────────────────────────────────────────────────────┤
│  docker-entrypoint.sh                                            │
│       │                                                          │
│       ▼                                                          │
│  ┌─────────────────────────────────────────┐                    │
│  │  检查 PIP_TOOLS 环境变量                 │                    │
│  │  格式: "pkg1:bin1 pkg2:bin2 ..."         │                    │
│  └─────────────────────────────────────────┘                    │
│       │                                                          │
│       ▼                                                          │
│  ┌─────────────────────────────────────────┐                    │
│  │  对每个条目:                             │                    │
│  │  1. 提取包名和二进制名                   │                    │
│  │  2. 验证包名（安全检查）                 │                    │
│  │  3. 检查二进制是否已存在                 │                    │
│  │  4. 通过 uv（快）或 pip3 安装            │                    │
│  └─────────────────────────────────────────┘                    │
│       │                                                          │
│       ▼                                                          │
│  执行 CMD (hotplexd, bash 等)                                    │
└──────────────────────────────────────────────────────────────────┘
```

## 使用方法

### 环境变量格式

```bash
PIP_TOOLS="包名:二进制名 [包名:二进制名 ...]"
```

| 组件 | 描述 | 示例 |
|------|------|------|
| `包名` | PyPI 包名（支持 extras） | `notebooklm-py[browser]` |
| `二进制名` | 用于检查存在性的可执行文件 | `notebooklm` |

### 示例

```yaml
# docker-compose.yml 或 .env
environment:
  # 单个包
  PIP_TOOLS: "pandas:pandas"

  # 多个包
  PIP_TOOLS: "pandas:pandas requests:requests numpy:numpy"

  # 带 extras 的包
  PIP_TOOLS: "notebooklm-py[browser]:notebooklm"
```

### Docker Compose 完整示例

```yaml
services:
  hotplex-custom:
    image: ghcr.io/hotplex/hotplex:latest-go
    environment:
      - PIP_TOOLS=notebooklm-py:notebooklm pandas:pandas
```

## 实现细节

### 入口点逻辑

位置：`docker/docker-entrypoint.sh`（第 182-226 行）

```bash
if [[ -n "${PIP_TOOLS:-}" ]]; then
    echo "--> Checking pip tools: ${PIP_TOOLS}"

    for tool in ${PIP_TOOLS}; do
        # 提取包名（冒号前）和二进制名（冒号后）
        pkg_name="${tool%%:*}"
        bin_name="${tool#*:}"

        # 安全：验证包名以防止命令注入
        if ! validate_pkg_name "${pkg_name}"; then
            echo "--> ERROR: Skipping invalid package name: ${pkg_name}"
            continue
        fi

        # 检查二进制是否存在
        if ! command -v "${bin_name}" >/dev/null 2>&1; then
            echo "--> Installing ${pkg_name} (binary: ${bin_name})..."
            # 安装逻辑...
        fi
    done
fi
```

### 验证函数

```bash
validate_pkg_name() {
    local name="$1"
    # 允许：字母、数字、连字符、下划线、点（用于版本规格）
    if [[ ! "$name" =~ ^[a-zA-Z0-9._-]+$ ]]; then
        echo "ERROR: Invalid package name: $name" >&2
        return 1
    fi
    return 0
}
```

### 安装顺序

1. **uv**（首选）：快速的 Rust 安装器
   ```bash
   uv pip install --system --break-system-packages --no-cache "${pkg_name}"
   ```

2. **pip3**（后备）：标准 Python 安装器
   ```bash
   pip3 install --break-system-packages --no-cache-dir "${pkg_name}"
   ```

## 安全考虑

| 方面 | 缓解措施 |
|------|----------|
| **命令注入** | 通过正则 `^[a-zA-Z0-9._-]+$` 验证包名 |
| **权限提升** | 以 `hotplex` 用户（非 root）运行安装 |
| **供应链** | 包来自 PyPI（设计上可信） |
| **缓存膨胀** | `--no-cache` 标志防止累积 |

## 性能特征

| 场景 | 时间（约） |
|------|-----------|
| 二进制已存在（跳过） | < 10ms |
| 小包（uv） | 1-3s |
| 大包（uv） | 5-15s |
| 小包（pip3） | 3-8s |
| 大包（pip3） | 10-30s |

## 内置 vs 动态安装

### 预安装包（Dockerfile.base）

核心功能所需的包预安装以实现零启动延迟：

```dockerfile
# 预安装 notebooklm-py CLI（用于 NotebookLM skill）
# 浏览器支持需要本地 GUI - 容器仅使用 CLI 模式
RUN pip3 install --break-system-packages "notebooklm-py"
```

### 何时使用每种方法

| 场景 | 建议 |
|------|------|
| 核心功能依赖 | 在 Dockerfile.base 中预安装 |
| 可选/skill 专用工具 | 使用 PIP_TOOLS |
| 大包（>100MB） | 预安装以避免启动延迟 |
| 需要系统依赖的包 | 使用 `playwright install-deps` 预安装 |

## 故障排除

### 安装后找不到二进制

```bash
# 检查包是否已安装
docker exec hotplex-01 pip3 list | grep <package>

# 验证二进制位置
docker exec hotplex-01 which <binary>

# 检查入口点日志
docker logs hotplex-01 2>&1 | grep "pip tools"
```

### 包验证失败

```
--> ERROR: Skipping invalid package name: foo;rm -rf /
```

这表示潜在的命令注入尝试。验证正则已阻止执行。

### 安装超时

对于大包，考虑在 Docker 镜像中预安装：

```dockerfile
# 在 Dockerfile.base 中
RUN pip3 install --break-system-packages "large-package"
```

## 相关文件

| 文件 | 用途 |
|------|------|
| `docker/Dockerfile.base` | 包含预安装工具的基础镜像 |
| `docker/docker-entrypoint.sh` | PIP_TOOLS 运行时逻辑 |
| `docker/matrix/.env-XX` | 每 bot 环境配置 |

## 历史

- **v0.26.x**: PIP_TOOLS 初始实现
- **v0.27.0**: 预安装 `notebooklm-py` CLI 用于 NotebookLM skill
