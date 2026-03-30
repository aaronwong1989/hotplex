# 配置管理 -- 最佳实践调研

> 调研时间：2026-03-30
> 调研范围：12-Factor App、GitOps、Secret 管理、配置热更新

---

## 1. 12-Factor App 配置规范

### 核心原则

[12-Factor App](https://12factor.net/config) 的 **Factor III: Config** 是云原生配置管理的基石，核心要求：

1. **配置与代码严格分离**：代码必须可以开源而不暴露任何凭证
2. **环境变量作为标准存储**：不可在代码中硬编码环境特定的配置
3. **粒度化管理**：每个环境变量独立存在，避免创建"环境"对象（如 `development`、`staging`）
4. **区分配置类型**：
   - **Build-time config**（构建产物绑定）：如数据库凭证、服务凭据
   - **Deploy-time config**（部署时决定）：如 hostname、每环境 URL

### HotPlex 当前实现评估

| 维度 | HotPlex 实现 | 评估 |
|------|-------------|------|
| 环境变量注入 | `os.ExpandEnv(string(data))` 在 YAML 解析前展开 | ✅ 符合 |
| Secret 分离 | `api_key: "${HOTPLEX_API_KEY}"` 从 .env 读取 | ✅ 符合 |
| 环境隔离 | 多 bot 独立 YAML 配置 | ✅ 符合 |
| 配置与代码分离 | `configs/` 目录独立于代码 | ✅ 符合 |
| 配置分层继承 | YAML `inherits` 字段支持 | ⚠️ 基础支持 |

### 配置分层策略

推荐三层配置架构：

```
第1层：YAML 配置文件（默认值，可提交 Git）
第2层：环境变量（覆盖 YAML，高优先级）
第3层：运行时 API（最高优先级，用于热更新）
```

---

## 2. 配置分层与继承

### 分层模式

| 层级 | 存储位置 | 变更方式 | 版本化 |
|------|---------|---------|--------|
| **应用默认** | YAML (`configs/base/`) | 代码审查 | Git |
| **环境覆盖** | `.env` / 环境变量 | 环境更新 | - |
| **运行时** | Admin API / ConfigMap | 热更新 | API |

### YAML 继承（Inherits）

当前 HotPlex 使用自定义 `inherits` 字段实现 YAML 继承，这是可行的，但需要注意：

```yaml
# configs/admin/server.yaml
inherits: ../base/server.yaml
server:
  log_level: debug  # Override parent
```

**局限性**：
- 非标准 YAML 特性，需要自定义解析
- 循环继承无保护（当前 CLAUDE.md 有警告，但代码中未实现检测）

### 推荐：Kustomize / Helm

对于更复杂的场景，推荐：

```yaml
# Kustomize overlay 示例
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
bases:
  - ../../base
patches:
  - path: patch.yaml
```

---

## 3. 配置验证方案

### 验证策略对比

| 方案 | 适用语言 | 特点 | 推荐度 |
|------|---------|------|--------|
| **Go struct tag + json schema** | Go | 内置，轻量 | ⭐⭐⭐⭐ 适合 HotPlex |
| **CUE Language** | 跨语言 | 统一 schema/配置语法 | ⭐⭐⭐ |
| **Pydantic** | Python | 成熟，类型丰富 | ⭐⭐⭐ |
| **JSON Schema** | 跨语言 | 标准化，工具链完善 | ⭐⭐⭐ |
| **Protocol Buffers** | 跨语言 | 编译时验证，性能高 | ⭐⭐⭐ |

### CUE Language

[CUE](https://cuelang.org/) 是一个专为配置设计的语言，具有独特优势：

- **Schema 与数据同语法**：schema 就是合法的 CUE 代码
- **约束一次定义，重复验证**：
  ```cue
  #Timeout: >0 & <=24h
  timeout: #Timeout
  ```
- **与 JSON/YAML/TOML 互操作**：适合渐进式迁移
- **工具链**：`cue vet`、`cue export`、`cue import`

### HotPlex 当前验证

当前 `validate()` 函数仅做枚举和范围检查，能力有限：

```go
// 当前仅支持：
// - permission_mode 枚举校验
// - log_level 枚举校验
// - timeout 数值范围校验
```

### 推荐方案

**短期**：增强现有 `validate()` 函数
- 添加字段必填性检查
- 添加格式校验（URL、端口范围、路径存在性）
- 添加业务规则校验（如 `idle_timeout > timeout`）

**长期**：引入 CUE Schema
- 定义 `server_schema.cue` 作为配置契约
- 启动时用 `cue vet` 验证 YAML 配置
- 渐进迁移，从关键配置开始

---

## 4. Secret 管理

### Secret 管理方案对比

| 方案 | 特点 | 适用场景 | HotPlex 适配度 |
|------|------|---------|---------------|
| **环境变量** | 简单，内置 | 开发/小规模 | ⚠️ 生产不推荐 |
| **HashiCorp Vault** | 动态 Secret、审计、密钥轮换 | 中大型生产 | ⭐⭐⭐⭐ |
| **AWS Secrets Manager** | 与 AWS 深度集成 | AWS 环境 | ⭐⭐⭐ |
| **SOPS/Sealed Secrets** | GitOps 友好，加密存储 | GitOps 工作流 | ⭐⭐⭐⭐ |
| **Kubernetes Secrets + External Secrets Operator** | 云原生 | K8s 环境 | ⭐⭐⭐ |

### HashiCorp Vault 最佳实践

根据 [HashiCorp 官方文档](https://developer.hashicorp.com/vault/docs/configuration)：

1. **Storage Backend**：推荐 Raft（内置存储）或 Consul（高可用）
2. **mlock 启用**：防止内存交换泄露 Secret（除非使用加密 swap）
3. **最小权限原则**：每个 Secret engine 独立 namespace
4. **审计日志**：启用 `audit` device，记录所有访问

### 动态 Secret

Vault 的核心能力——按需生成临时凭证：

```go
// 数据库动态凭证示例
creds, err := client.NewDatabaseCredential("role-name")
// 使用后自动撤销，无需手动轮换
```

**收益**：
- 无长期凭证泄露风险
- 凭证生命周期与请求绑定
- 审计粒度细化到每一次访问

### HotPlex 当前状态

| 组件 | 状态 | 说明 |
|------|------|------|
| `EnvProvider` | ✅ 已实现 | 基于 `os.Getenv` |
| `FileProvider` | ❌ TODO | 加密文件存储未实现 |
| `VaultProvider` | ❌ TODO | Vault 客户端未集成 |

### 零信任 Secret 管理

核心原则（Zero Trust Architecture）：
1. **永不信任**：每次访问都验证，不依赖网络位置
2. **最小权限**：JWT/Token 级别细粒度控制
3. **持续验证**：短期 Token + 频繁刷新
4. **审计全覆盖**：所有 Secret 访问记录可追溯

---

## 5. 配置热更新

### 热更新策略对比

| 策略 | 实现方式 | 一致性 | 复杂度 | 适用场景 |
|------|---------|--------|--------|---------|
| **fsnotify 文件监听** | 内核事件通知 | 高 | 低 | 单机配置 |
| **ConfigMap 卷挂载** | K8s Secret/ConfigMap | 中 | 中 | K8s 部署 |
| **轮询 + etag** | HTTP HEAD + Last-Modified | 中 | 低 | 远程配置服务 |
| **WebSocket 推送** | 服务端主动推送 | 高 | 高 | 实时性要求 |
| **gRPC Stream** | 流式配置更新 | 高 | 高 | 大规模微服务 |

### 当前 HotPlex 实现

`YAMLHotReloader` 已实现：

```go
// 关键特性：
// ✅ fsnotify 文件监听（跨平台）
// ✅ 100ms 防抖（debounce）
// ✅ 回调机制（onReload callback）
// ✅ 幂等 Start（防止重复 watcher）
// ⚠️ 仅支持单文件，不支持多配置目录
// ⚠️ 无变更审计日志
// ⚠️ 无版本回滚能力
```

### 配置变更追踪（Audit Trail）

生产环境必须具备的能力：

1. **变更记录**：谁、何时、什么配置变了
2. **变更前快照**：每次变更前保存上一版本
3. **回滚能力**：支持回退到任意历史版本
4. **通知机制**：变更时通知相关组件

### 渐进式配置推送（Progressive Rollout）

适合 Feature Flags 场景：

```
Phase 1: 10% 用户 → 监控错误率
Phase 2: 50% 用户 → 扩大范围
Phase 3: 100% 用户 → 全量
```

推荐工具链：
- **LaunchDarkly**：企业级，功能完整
- **Flagsmith**：开源，可自托管
- **Unleash**：开源，K8s 原生

---

## 6. 推荐方案

### HotPlex 配置管理架构

```
                    ┌─────────────────────────────────────────────┐
                    │              Configuration Flow               │
                    └─────────────────────────────────────────────┘

Git Repo (configs/)
      │
      │ YAML files (base + inherits)
      ▼
┌──────────────────────────────────────────────────────────────────┐
│  Config Loader (server_config.go)                                 │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ 1. Read YAML → os.ExpandEnv() 展开环境变量                   │  │
│  │ 2. Env override (HOTPLEX_*) 最高优先级                      │  │
│  │ 3. Schema validation (增强 validate())                       │  │
│  │ 4. Deep copy → 返回不可变配置                               │  │
│  └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
      │
      ▼
┌──────────────────────────────────────────────────────────────────┐
│  Secret Manager (secrets/manager.go)                             │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐   │
│  │ EnvProvider  │  │ FileProvider │  │ VaultProvider         │   │
│  │ (已实现)      │  │ (TODO)       │  │ (TODO: Vault 集成)    │   │
│  └──────────────┘  └──────────────┘  └───────────────────────┘   │
│                                                                   │
│  Cache (5min TTL) → 减少 Provider 调用                           │
└──────────────────────────────────────────────────────────────────┘
      │
      ▼
┌──────────────────────────────────────────────────────────────────┐
│  Hot Reload (hotreload_yaml.go)                                  │
│  ┌─────────────────────────────────────────────────────────────┐  │
│  │ fsnotify → debounce(100ms) → reload → callback → 组件更新   │  │
│  │ Audit log: 每次变更记录 (who/when/what/before/after)        │  │
│  └─────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
```

### 分阶段实施方案

#### Phase 1: 强化当前配置（1-2 周）

| 改进项 | 描述 | 优先级 |
|--------|------|--------|
| 增强 `validate()` | 添加字段必填性、格式、业务规则校验 | P0 |
| 配置变更审计日志 | 记录每次 YAML 重载的时间、内容差异 | P0 |
| 循环继承检测 | 防止 `inherits` 循环引用 | P1 |
| 多文件热重载 | 监听整个 `configs/` 目录 | P1 |

#### Phase 2: Secret 管理增强（2-4 周）

| 改进项 | 描述 | 优先级 |
|--------|------|--------|
| `FileProvider` 实现 | 加密文件存储（age 或 go-jose） | P0 |
| Vault 集成 | 接入 HashiCorp Vault KV v2 | P1 |
| Secret 轮换通知 | 凭证变更时自动重载 | P1 |

#### Phase 3: 配置治理（长期）

| 改进项 | 描述 | 优先级 |
|--------|------|--------|
| CUE Schema | 引入 CUE 进行配置契约验证 | P2 |
| Feature Flags | 集成 Flagsmith 或自托管 Unleash | P2 |
| GitOps 工作流 | 配置变更走 PR review + 自动部署 | P2 |

---

## 7. 关键决策点

### Q1: 是否需要支持多环境配置管理？

**当前状态**：通过 YAML `inherits` 实现环境隔离，但没有环境概念（如 `env: production`）

**推荐**：
- 短期：保持当前 `inherits` 模式，适合 3-5 个 bot 场景
- 中期：引入 `ENV` 环境变量，`configs/{env}/` 目录结构

### Q2: Secret 管理选择 EnvProvider 还是 Vault？

**当前状态**：纯 EnvProvider，VaultProvider 是空壳

**推荐分场景**：
- 开发/单 bot：EnvProvider 足够，`.env` 文件管理
- 生产/多 bot：**必须引入 Vault**，否则凭证管理将成为瓶颈
- 过渡方案：`FileProvider`（加密文件）+ Vault 长期目标

### Q3: 热重载的范围和粒度？

**当前状态**：单文件 fsnotify + debounce

**推荐**：
- 保留当前 `fsnotify` 方案（成熟可靠）
- 扩展到目录监听（支持多配置文件）
- 增加**变更类型区分**：哪些字段支持热更新，哪些需要重启
  - `log_level`：✅ 可热更新
  - `permission_mode`：✅ 可热更新
  - `engine.timeout`：`⚠️ 需要 session 重建后再生效`

### Q4: 是否引入 Feature Flags？

**推荐**：不需要，HotPlex 是基础设施层

Feature Flags 是应用层需求（灰度发布、A/B 测试），HotPlex 作为控制平面，其配置属于"部署配置"而非"运行时行为配置"，不适合引入 Feature Flags。

如果需要灰度能力，应该在**调用方**（如 Slack adapter）实现。

### Q5: 配置 Schema 验证工具选型？

**推荐**：Go 原生 + 可选 CUE

理由：
- HotPlex 是 Go 项目，原生集成成本最低
- CUE 是优秀的设计，但引入新语言增加认知负担
- 当前 `validate()` 增强即可满足 80% 需求

### Q6: 配置版本化和回滚？

**推荐**：轻量方案，无需 K8s 级别的配置 operator

```
# configs/admin/server.yaml.v0, .v1, .v2 保留最近 3 个版本
# Admin API 提供：
#   GET  /admin/config/history
#   POST /admin/config/rollback?to=v1
```

---

## 附录：关键参考

| 主题 | 参考来源 |
|------|---------|
| 12-Factor App | https://12factor.net/config |
| HashiCorp Vault 配置 | https://developer.hashicorp.com/vault/docs/configuration |
| GitOps 最佳实践 | https://www.gitops.tech/ |
| AWS Secrets Manager | https://docs.aws.amazon.com/secretsmanager/latest/userguide/best-practices.html |
| CUE Language | https://cuelang.org/ |
| Kubernetes ConfigMap | https://kubernetes.io/docs/concepts/configuration/ |
