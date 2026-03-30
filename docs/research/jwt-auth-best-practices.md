# JWT 认证授权 — 最佳实践调研

> 调研日期：2026-03-30
> 调研范围：JWT 签发验证、WebSocket 认证、行业参考方案
> 信息来源：RFC 7519、RFC 7515、RFC 6455、RFC 9449 (DPoP)、Auth0 Docs、Firebase Docs、Discord Gateway Docs、AWS API Gateway Docs

---

## 1. JWT 签发与验证

### 1.1 算法选择：RS256 vs HS256 vs ES256

**结论：推荐 ES256，次选 RS256，禁止使用 HS256 做跨服务认证。**

| 维度 | HS256 (HMAC) | RS256 (RSA) | ES256 (ECDSA) |
|------|-------------|-------------|---------------|
| **密钥类型** | 对称（共享密钥） | 非对称（公私钥） | 非对称（椭圆曲线） |
| **签发方** | 持有共享密钥 | 持有私钥 | 持有私钥 |
| **验证方** | 持有相同共享密钥 | 持有公钥 | 持有公钥 |
| **密钥长度** | 256 bit | 2048+ bit | 256 bit（曲线 P-256） |
| **性能** | 快 | 慢 | 快 |
| **跨服务适用性** | ❌ 服务越多密钥泄露风险越大 | ✅ 公钥可分发 | ✅ 公钥可分发，性能更优 |
| **RFC 地位** | MUST implement | RECOMMENDED | RECOMMENDED |

**RFC 7519 §8 明确定义**：HMAC SHA-256 (HS256) 和 `none` 是必须实现的算法；RS256 和 ES256 是推荐算法。

**为什么 HS256 是危险选项？**

当系统有 N 个服务时，HS256 要求每个服务持有相同的共享密钥。任意一个服务被攻破即全量泄露。ES256 的公钥可公开发布，私钥仅签发方持有，验证方无泄露风险。

**为什么选 ES256 而非 RS256？**

- 签名更短（ES256 约 64 字节，RS256 约 256 字节），对 WebSocket 消息头和 HTTP Header 传输更友好
- 验证速度显著快于 RSA（尤其在嵌入式/移动场景）
- 相同安全强度下密钥体积小 6 倍

**HotPlex 推荐**：使用 ES256（P-256 曲线），由 Admin API 服务签发 JWT，公钥可被 Gateway 服务和各 Adapter 持有验证。

### 1.2 Claims 结构设计

#### 标准注册声明（Reserved Claims）

| Claim | 全称 | 用途 | HotPlex 建议 |
|-------|------|------|-------------|
| `iss` | Issuer | 签发者标识 | `hotplex` 或 `hotplex-admin` |
| `sub` | Subject | 用户/主体标识 | `user_id` 或 `session_id` |
| `aud` | Audience | 接收方验证 | 必须验证：`hotplex-gateway`、`hotplex-engine` |
| `exp` | Expiration Time | 过期时间 | 短令牌建议 5-15 分钟 |
| `iat` | Issued At | 签发时间 | 必须，包含用于时间窗口验证 |
| `nbf` | Not Before | 生效时间 | 可选，防时钟漂移 |

#### 公共声明（Public Claims）

| Claim | 用途 | 示例 |
|-------|------|------|
| `jti` | JWT ID，防重放 | UUID v4，32 字符 |
| `nonce` | 防 CSRF/重放 | 随机 16+ 字节，base64url 编码 |
| `scope` | 权限范围 | `"engine:run engine:read"` |
| `role` | 角色 | `"admin"` `"operator"` `"viewer"` |
| `bot_id` | 机器人标识 | 对应 `bot_user_id`，防跨 bot 会话混淆 |
| `session_id` | 会话标识 | 用于 WebSocket session 绑定 |
| `channel_id` | 频道标识 | 多租户隔离 |

**关键原则**：
- **禁止在 JWT payload 中存储敏感信息**（JWT 只做身份断言，不做数据载体）
- JWT payload 是 base64url 编码的明文，可被任何人解码
- 敏感数据存服务端，用 session_id 关联

#### 示例 Claims 结构

```json
{
  "iss": "hotplex-admin",
  "sub": "u12345",
  "aud": "hotplex-gateway",
  "exp": 1743400000,
  "iat": 1743399900,
  "jti": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "role": "operator",
  "scope": "engine:run engine:read",
  "bot_id": "B0123456789",
  "session_id": "slack:U12345:B0123456789:C001:null",
  "nonce": "xK9mN2pQ7rT4sV6w"
}
```

### 1.3 jti — 防重放的核心机制

**RFC 7519 §4.1.7 明确定义**：`jti` claim 提供 JWT 的唯一标识符，用于防止 JWT 被重放。

**实现方式**：
1. 签发时生成 UUID v4 作为 `jti`
2. 验证时在 Redis/内存中检查 `jti` 是否已被使用
3. 已使用的 `jti` 加入短时黑名单（TTL = token 剩余有效期）
4. **check-then-set 必须是原子操作**（使用 Redis SETNX）

**典型黑名单存储**（Redis）：

```
SETNX hotplex:jwt:revoked:<jti> 1
EXPIRE hotplex:jwt:revoked:<jti> <remaining_ttl_seconds>
```

**重要**：黑名单机制是配合短令牌使用的——若令牌本身 5 分钟过期，黑名单最多存在 5 分钟，存储成本可控。

### 1.4 nonce — 防 CSRF 和预计算攻击

**nonce 的核心价值**：
- **防 CSRF**：绑定 JWT 到特定请求上下文
- **防预计算**：配合 `nbf` 限制令牌在特定时间窗口后才能使用
- **DPoP 场景**（RFC 9449）：nonce 由服务端提供，客户端必须回传，防止 DPoP 证明被预生成和重放

**WebSocket 场景的 nonce 策略**：
- 连接握手阶段：服务端生成 `connect_nonce`，客户端在 JWT 中包含此 nonce
- 每次发消息时：包含消息级 nonce，防止消息重放

### 1.5 aud — 接收方验证

**RFC 7519 §4.1.3 明确要求**：如果 JWT 中存在 `aud` claim，验证方必须确认自己的标识在 `aud` 中，否则必须拒绝该令牌。

**HotPlex 的 aud 设计**：

| 服务 | 期望的 aud 值 |
|------|-------------|
| Gateway (WebSocket) | `hotplex-gateway` |
| Engine Runner | `hotplex-engine` |
| Admin API | `hotplex-admin` |

验证示例：

```go
func validateAudience(claims jwt.RegisteredClaims, expectedAud string) error {
    found := false
    for _, aud := range claims.Audience {
        if aud == expectedAud {
            found = true
            break
        }
    }
    if !found {
        return fmt.Errorf("invalid audience: expected %s", expectedAud)
    }
    return nil
}
```

---

## 2. Token 生命周期管理

### 2.1 短期令牌 + 刷新令牌的经典模式

**推荐配置**：

| 令牌类型 | TTL | 存储 | 用途 |
|---------|-----|------|------|
| Access Token | 5-15 分钟 | 内存/内存变量 | API 鉴权 |
| Refresh Token | 7-30 天 | HttpOnly Cookie 或加密存储 | 获取新 Access Token |
| WebSocket Session Token | 1-24 小时 | 仅服务端关联 | WebSocket 连接保活 |

**OAuth 2.0 Security Best Practices (IETF draft) 明确要求**：
> Refresh tokens for public clients MUST use sender-constrained tokens or refresh token rotation.

**Refresh Token Rotation**：
- 每次使用 refresh token 签发新的 access token + 新的 refresh token
- 旧的 refresh token 立即失效
- 若收到已被撤销的 refresh token，表明存在 token 泄露，触发安全告警

### 2.2 HotPlex 场景的特殊性

HotPlex 是 **Cli-as-a-Service**，其认证需求与普通 Web 应用有本质差异：

```
传统 Web 应用：用户登录 → 持有 Access Token → 短期 → 刷新
HotPlex Gateway：Admin 预配 bot credentials → Gateway 持有长效 token → 启动 session
```

**两种场景**：

**场景 A：外部用户接入 HotPlex Gateway**
- 用户通过 OAuth2/SSO 登录，获取短期 JWT（5-15 分钟）
- JWT 中包含 `scope`（engine:run, engine:read 等）
- Gateway 验证 JWT 后启动对应 engine session
- 需要 refresh token rotation 机制

**场景 B：Bot-to-Gateway 内部认证**（Discord、Slack 适配器）
- Bot 使用平台提供的 Bot Token 直接连接 WebSocket
- 无需 JWT，由平台 Gateway 直接做身份验证
- 内部 HotPlex JWT 用于 engine session 管理

**推荐 HotPlex 策略**：
- **对外 API**（Admin API）：短期 JWT (5 分钟) + Refresh Token Rotation
- **WebSocket Gateway**：使用更长的 session-bound token (1 小时)，通过心跳保活检测 token 有效性
- **Engine Runner 间通信**：ES256 签名 JWT，aud 限制为 `hotplex-engine`

### 2.3 Token 吊销策略

| 策略 | 适用场景 | 实现成本 |
|------|---------|---------|
| **jti 黑名单**（Redis SETNX） | 短期令牌即时吊销 | 低 |
| **令牌版本号**（family 机制） | Refresh Token Rotation | 中 |
| **定期轮转签名密钥**（密钥轮换） | 长期密钥泄露应对 | 高 |

**HotPlex 推荐**：jti 黑名单 + 短期令牌组合，Redis 存储，TTL 同步令牌有效期。

---

## 3. WebSocket 认证方案

### 3.1 RFC 6455 认证机制

RFC 6455 §10.5 明确定义了 WebSocket 握手阶段的认证方式：

**方式一：HTTP 认证头（WWW-Authenticate）**
- 服务端在握手响应返回 401 + `WWW-Authenticate` 头
- 客户端在重新发起握手时携带 `Authorization` 头
- 局限：仅在建立连接时一次认证，后续消息无认证

**方式二：Cookie（RFC 6265）**
- 握手请求携带 Cookie（通过 HTTP 请求头）
- 服务端验证 Cookie 中的 session/token
- 优势：利用现有 HttpOnly Cookie 防 XSS，可与服务端 session 机制联动

**方式三：Subprotocol 认证**
- 通过 `Sec-WebSocket-Protocol` 头携带认证信息
- 应用层协议定义认证语义
- 灵活性最高，HotPlex 可自定义子协议格式

**RFC 6455 §10.6 强制要求**：生产环境必须使用 WSS（TLS 加密），纯 WS 明文传输是严重安全漏洞。

### 3.2 Token 传递方式对比

| 方式 | 握手阶段 | 安全风险 | HotPlex 适用性 |
|------|---------|---------|---------------|
| **Cookie（HttpOnly）** | HTTP 握手 Header | 防 XSS 最佳，CSRF 风险需额外防御 | ✅ Admin API WebSocket 场景 |
| **Query Parameter** | WSS URL 参数 | ❌ Token 暴露在 URL（日志、Referer 头、浏览器历史） | ⚠️ 仅开发调试 |
| **Authorization Header** | HTTP 握手 Header | 优于 Query Param，但握手阶段明文传输 | ⚠️ 仅 WSS 下安全 |
| **子协议载荷** | 连接建立后首条消息 | 最灵活，可携带完整 JWT | ✅ 自定义协议首选 |
| **自定义 Header** | HTTP 握手 Header | 与 Authorization Header 类似 | ⚠️ 部分 CDN/代理可能剥离 |

**Query Parameter 的致命问题**：
- URL 会写入 Web 服务器访问日志
- URL 会出现在浏览器历史记录
- URL 会通过 Referer 头泄漏给第三方
- WSS URL 中的 token 会在浏览器 WebSocket 对象中可见

**Discord Gateway 的选择**：Token 在 WebSocket 握手后通过首条消息（Identify payload）以 JSON 载荷传递，握手阶段仅建立 TLS 连接。

### 3.3 推荐方案：握手期 Cookie 验证 + 子协议认证双保险

```
Step 1: 客户端发起 WSS 握手（Cookie 自动携带）
Step 2: 服务端验证 Cookie 中的 session
Step 3: 若 Cookie 无效/缺失 → 返回 401
Step 4: 若 Cookie 有效 → 升级为 WebSocket
Step 5: 连接建立后，客户端发送首条认证消息（含 JWT）
Step 6: 服务端验证 JWT（jti、aud、exp、nonce）
Step 7: 进入消息循环
```

**为何双保险？**
- Cookie 验证在握手阶段快速过滤未认证连接
- JWT 验证提供细粒度权限控制（scope、bot_id、session_id）
- JWT 的 `jti` 检查提供重放保护
- 即使 Cookie 被窃取，JWT 的短 TTL 和 aud 限制攻击面

### 3.4 Session 级别权限验证

WebSocket 连接建立后，每次消息仍需权限验证：

```go
// 消息权限检查示例
func (h *Handler) handleMessage(tokenClaims *Claims, msg *Message) error {
    // 1. 检查 session 归属
    if tokenClaims.SessionID != msg.SessionID {
        return ErrSessionMismatch
    }
    // 2. 检查 scope 权限
    if !hasScope(tokenClaims.Scope, msg.RequiredScope) {
        return ErrInsufficientScope
    }
    // 3. 检查 bot_id 一致性（防跨 bot 操作）
    if tokenClaims.BotID != msg.BotID {
        return ErrBotIDMismatch
    }
    return nil
}
```

---

## 4. Session 权限验证

### 4.1 多租户隔离与权限模型

HotPlex 的 session 由 `platform:userID:botUserID:channelID:threadID` 构成，JWT 中应包含完整 session 上下文用于验证。

**权限分层**：

```
Role: admin
  ├── scope: "*" (所有操作)
  └── 全部 session 可访问

Role: operator
  ├── scope: "engine:run engine:read"
  └── 仅可操作 own sessions (sub == userID)

Role: viewer
  ├── scope: "engine:read"
  └── 仅读操作，无写权限
```

**JWT 中的权限表达**：

```go
type Claims struct {
    jwt.RegisteredClaims
    JTI       string   `json:"jti"`       // 重放保护
    Role      string   `json:"role"`      // 角色
    Scope     string   `json:"scope"`     // 权限范围（空格分隔）
    BotID     string   `json:"bot_id"`    // Bot 标识
    SessionID string   `json:"session_id"` // Session 绑定
    Nonce     string   `json:"nonce"`     // CSRF 防护
}
```

### 4.2 Bot 级别隔离（Multi-Bot 场景）

MEMORY.md 记录的关键教训：每个 bot 必须有唯一 `bot_user_id`，否则 session ID 会冲突。

JWT 中的 `bot_id` claim 用于：
1. 验证连接者有权操作该 bot 的 session
2. 防止 Bot A 的操作渗透到 Bot B 的 session
3. 审计追踪：每个 JWT 操作可追溯到具体 bot

---

## 5. 推荐方案

### 5.1 HotPlex 整体认证架构

```
[外部用户] ──→ OAuth2/SSO ──→ Admin API (短期 JWT 签发)
                │
                ▼
        [短期 JWT (5min)]
        + Refresh Token Rotation
                │
                ▼
        [WebSocket Gateway]
                │
                ├── Cookie 验证（握手阶段）
                ├── JWT 验证（首条消息）
                │     └── ES256 签名
                │     └── aud: "hotplex-gateway"
                │     └── jti 黑名单（Redis，TTL=5min）
                │     └── nonce 检查
                └── [Engine Runner]（Engine-bound JWT）
                      └── aud: "hotplex-engine"
```

### 5.2 算法与密钥

| 项目 | 推荐值 | 理由 |
|------|--------|------|
| **签名算法** | ES256 (P-256) | 公钥可分发，签名短（64B），性能优于 RSA |
| **访问令牌 TTL** | 5 分钟 | 平衡安全与用户体验 |
| **WebSocket Session Token TTL** | 1 小时 | 配合心跳保活，超时重新认证 |
| **刷新令牌 TTL** | 7 天 | Rotation 机制降低泄露窗口 |
| **签名密钥轮换** | 90 天 | 支持新旧密钥共存平滑过渡 |
| **jti 黑名单 TTL** | 等于令牌 TTL | 令牌过期后黑名单自动失效 |

### 5.3 JWT Claims 标准模板

```go
// StandardClaims 定义 HotPlex 全局标准 claims
type StandardClaims struct {
    jwt.RegisteredClaims
    JTI       string   `json:"jti"`       // UUID v4，防重放
    Role      string   `json:"role"`      // admin | operator | viewer
    Scope     string   `json:"scope"`     // "engine:run engine:read"
    BotID     string   `json:"bot_id"`    // 唯一 bot 标识
    SessionID string   `json:"session_id"`// session 绑定
    Nonce     string   `json:"nonce,omitempty"` // 可选，CSRF 防护
}
```

### 5.4 WebSocket 认证握手流程

```
客户端                                    服务端
  │                                        │
  │  WSS connect (Cookie 携带)            │
  │ ───────────────────────────────────→  │
  │                                        │ 验证 Cookie
  │                                        │  → 失败: 401 Unauthorized
  │                                        │  → 成功: 101 Switching Protocols
  │  ←───────────────────────────────────  │
  │                                        │
  │  { "type": "auth", "token": "<JWT>" } │
  │ ───────────────────────────────────→  │
  │                                        │ 1. JWT 签名验证 (ES256 公钥)
  │                                        │ 2. aud 验证 ("hotplex-gateway")
  │                                        │ 3. exp/iat/nbf 验证
  │                                        │ 4. jti 黑名单检查 (Redis)
  │                                        │ 5. nonce 验证（若存在）
  │                                        │ 6. scope 验证
  │  { "type": "auth_ok", "session": "..."}
  │ ←───────────────────────────────────  │
  │                                        │
  │  [消息循环: 每条消息验证 session_id 一致性]
  │                                        │
```

### 5.5 Redis jti 黑名单实现

```go
func (s *RevocationStore) IsRevoked(jti string) (bool, error) {
    exists, err := s.redis.Exists(ctx, "hotplex:jwt:revoked:"+jti).Result()
    return exists > 0, err
}

func (s *RevocationStore) Revoke(jti string, ttl time.Duration) error {
    return s.redis.SetNX(ctx, "hotplex:jwt:revoked:"+jti, "1", ttl).Err()
}
```

---

## 6. 行业参考

### 6.1 Discord Gateway 认证方案

Discord 是 WebSocket 实时通信的标杆实现，其方案对 HotPlex 有直接参考价值。

**核心机制**：
1. 通过 HTTPS 获取 WSS Gateway URL（带版本、编码参数）
2. 建立 WSS 连接，收到 Hello 事件（心跳间隔）
3. 发送 Identify 消息（opcode 2），payload 包含 bot token
4. 服务器返回 Ready 事件，包含 `session_id` 和 `resume_gateway_url`
5. 断连后使用 RESUME 消息（opcode 6）恢复 session

**关键设计决策**：
- Token 在 WebSocket 握手后通过应用层消息传递（不是 URL 参数）
- Session 可恢复（RESUME），避免每次重连完整认证
- 心跳保活机制：每 `heartbeat_interval * jitter` 毫秒发送一次
- 心跳 ACK 超时 → 立即重连

**对 HotPlex 的启示**：
- 采用类似的 IDENTIFY/RESUME 模式管理 session 生命周期
- 心跳机制是保持 session 活跃的必要条件
- Session invalidation 需要考虑 DISCONNECT + RECONNECT 场景

### 6.2 Firebase Auth 实践

**自定义声明（Custom Claims）**：
```json
{
  "sub": "user123",
  "role": "admin",
  "bot_permissions": ["B001", "B002"]
}
```

**设计原则**：
- ID Token（JWT）：短期（1 小时），用于客户端身份断言
- Session Cookie：服务端长期认证，适合 SSR 场景
- Refresh Token：自动轮换，客户端 SDK 管理
- 自定义声明：存储最小权限信息，验证时从 JWT 读取

**对 HotPlex 的启示**：
- `role` 和 `scope` claims 可直接对应 Firebase Custom Claims 模式
- Firebase 推荐 ID Token 验证在服务端完成，客户端不应缓存验证结果

### 6.3 Auth0 最佳实践

- 强制使用 `RS256` 或 `ES256`，禁止 `HS256`（除非有充分理由）
- `aud` claim 必须验证，指定 JWT 预期的接收方
- Token 轮换：Refresh Token 使用 Rotation + 单次使用保证
- 签名密钥支持自动轮换，应用无需重启

### 6.4 Clerk 方案

- JWT 包含完整的用户 session 信息（无服务端 session 依赖）
- 通过中间件自动验证 JWT，注入到请求上下文
- 支持自定义 JWT 模板（template-based JWT generation）
- 提供基于 JWT 的 Webhook/Real-time 认证钩子

### 6.5 AWS API Gateway WebSocket 认证

- 支持 IAM 授权、Lambda 授权、Cognito 授权
- `connect` 路由可配置独立的授权器
- IAM 授权：签名版本 4，token 在 query string（但配合签名保证安全）
- JWT 授权：通过 query string 参数传递，Lambda 授权器验证

**注意**：AWS API Gateway 的 JWT via Query String 模式仅在配合签名机制时安全，不适合裸 JWT 传递。

---

## 7. 关键决策点（需要人工确认）

### 决策 1：短期 JWT vs 长效 Session Token

**方案 A**：所有场景使用短期 JWT（5 分钟）+ 刷新令牌
- 优点：安全性高，token 泄露窗口小
- 缺点：Engine session 可能频繁需要重新认证

**方案 B**：WebSocket Gateway 使用 1 小时 session token，Admin API 使用 5 分钟 JWT
- 优点：Gateway 连接稳定，无需频繁重认证
- 缺点：token 泄露窗口更大（1 小时 vs 5 分钟）
- 缓解：配合 jti 黑名单 + 连接级别验证

**推荐方案 B**（需确认）：Engine session 的稳定性优先于极致安全，短窗口重认证会影响用户体验。

### 决策 2：Refresh Token 存储位置

| 方案 | 优点 | 缺点 |
|------|------|------|
| **HttpOnly Cookie** | 防 XSS，浏览器自动管理 | 跨域配置复杂，不适合非浏览器客户端 |
| **加密客户端存储** | 支持所有客户端 | 需要客户端 SDK 管理加密密钥 |
| **服务端 session + 随机 token** | 灵活，支持服务端撤销 | 增加服务端存储 |

**推荐**：HttpOnly Cookie（Admin API）+ 服务端 session（内部服务间）

### 决策 3：多 Bot 场景的认证层级

**问题**：HotPlex 支持多个 Slack/Discord Bot，每个 Bot 有独立的 bot credentials。如何设计认证层级？

**方案 A**：每个 Bot 持有独立的 JWT 签发密钥
- 优点：Bot 间完全隔离，单个 Bot 泄露不影响其他
- 缺点：密钥管理复杂度 O(N)

**方案 B**：共享 ES256 私钥，通过 `bot_id` claim 区分
- 优点：统一密钥管理
- 缺点：需要严格验证 `bot_id` 与 token 持有者的对应关系
- 风险：签发服务被攻破 → 所有 Bot session 可被伪造

**推荐方案 A**（需确认）：每个 Bot 有独立签发密钥，Bot 级别隔离。但需确认管理成本是否可接受。

### 决策 4：Session Resumption 机制

Discord Gateway 的 RESUME 机制对于长期 WebSocket 连接至关重要。HotPlex 是否需要实现类似机制？

- **需要**：如果 HotPlex 支持长时间会话（>数小时），断线重连应恢复 session
- **不需要**：如果 HotPlex 的 session 设计为短生命周期（<30 分钟），每次重连重新认证即可

**推荐**：待确认 HotPlex 的 session 设计生命周期。若支持长时间连接，参考 Discord RESUME 模式。

---

## 参考资料

- [RFC 7519 - JSON Web Token (JWT)](https://www.rfc-editor.org/rfc/rfc7519)
- [RFC 7515 - JSON Web Signature (JWS)](https://www.rfc-editor.org/rfc/rfc7515)
- [RFC 6455 - The WebSocket Protocol](https://www.rfc-editor.org/rfc/rfc6455)
- [RFC 9449 - OAuth 2.0 Demonstrating Proof of Possession (DPoP)](https://datatracker.ietf.org/doc/html/rfc9449)
- [IETF OAuth 2.0 Security Best Practices](https://datatracker.ietf.org/doc/html/draft-ietf-oauth-security-topics)
- [Discord Gateway Documentation](https://docs.discord.com/developers/docs/topics/gateway)
- [Firebase Authentication - Custom Claims](https://firebase.google.com/docs/auth/admin/custom-claims)
- [Auth0 - JSON Web Token Best Practices](https://auth0.com/docs/secure/tokens/json-web-tokens)
- [AWS API Gateway - WebSocket API Authorizer](https://docs.aws.amazon.com/apigateway/latest/developerguide/apigateway-websocket-api-authorizer.html)
- [Passport.js - passport-jwt](https://www.passportjs.org/packages/passport-jwt/)
