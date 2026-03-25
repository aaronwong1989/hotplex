# Token 统计异常分析报告

**生成时间**: 2026-03-25 20:37:01
**更新时间**: 2026-03-25 21:00:00 (官方文档验证版)

## 问题描述

用户报告显示异常： `⚡ 67.9K/367 (cache: 136K/0)`
- cache 统计 (136K) 超过了 input 统计 (67.9K)
- 用户疑惑：为什么 cache 会比总量还大?

## 根本原因

**这是正常行为，不是 bug！**

### ✅ 官方文档验证（2026-03-25）

经过联网调研，查阅 **Claude API 官方文档**：
- 文档地址：https://platform.claude.com/docs/en/build-with-claude/prompt-caching
- 关键发现：

> **The `input_tokens` field represents only the tokens that come AFTER the last cache breakpoint in your request - not all the input tokens you sent.**
> （input_tokens 字段仅表示请求中**最后一个缓存断点之后**的 token，而不是您发送的所有输入 token。）

### API 返回的数据结构

从 `~/hotplex/.logs/daemon.log` 提取的原始数据：
```json
{
  "input_tokens": 67878,
  "cache_creation_input_tokens": 0,
  "cache_read_input_tokens": 136000,
  "output_tokens": 367
}
```

### 字段含义（官方定义）

| 字段 | 官方含义 | 数值 |
|:-----|:---------|:-----|
| **input_tokens** | **最后一个缓存断点之后**的新 token | 67,878 (67.9K) |
| **cache_read_input_tokens** | 从 API 缓存读取的 token | 136,000 (136K) |
| **cache_creation_input_tokens** | 写入 API 缓存的 token | 0 |
| **output_tokens** | 模型生成的 token | 367 |

### 官方公式

```
total_input_tokens = cache_read_input_tokens + cache_creation_input_tokens + input_tokens
```

### 为什么 cache > input?

根据官方文档说明：

- **input_tokens (67.9K)** = 最后一个缓存断点之后的新内容
- **cache_read_input_tokens (136K)** = 缓存命中的内容（之前已发送）

**总输入 = 67.9K + 0 + 136K = 203.9K** ✅ 符合 200K 上下文限制

### 显示逻辑（参考 chatapps/slack/builder_stats.go:42-46）

```go
if cacheRead > 0 || cacheWrite > 0 {
    stats = append(stats, fmt.Sprintf("⚡ %s/%s (cache: %s/%s)",
        formatTokenCount(tokensIn), formatTokenCount(tokensOut),
        formatTokenCount(cacheRead), formatTokenCount(cacheWrite)))
}
```

显示格式: `⚡ {新token}/{输出} (cache: {缓存读}/{缓存写})`

## 计算验证

- 新 token（断点后）: 67,878
- 缓存 token（断点前）: 136,000
- 总输入: 67,878 + 0 + 136,000 = 203,878 (约 200K)
- 输出 token: 367
- 成本: $0.249939

## 结论

**✅ 显示是正确的，符合官方 API 规范！**

### 关键理解

1. **input_tokens 不是总输入**，而是最后一个缓存断点之后的部分
2. **cache_read_input_tokens 表示缓存命中**，这部分内容不需要重新计费
3. **总输入 = 三者之和**，不要误以为 input_tokens 是总量

### 显示改进建议（可选）

当前显示可能引起误解，建议考虑：

- **当前**: `⚡ 67.9K/367 (cache: 136K/0)`
- **方案 A**: `⚡ 67.9K+136K/367` (直观显示总量)
- **方案 B**: `⚡ 203.9K/367 (67.9K new, 136K cached)`
- **方案 C**: 添加 tooltip 说明 "input_tokens 为缓存断点后的部分"

### 参考资料

**官方文档**:
- https://platform.claude.com/docs/en/build-with-claude/prompt-caching

**代码实现**:
- 显示逻辑: `chatapps/slack/builder_stats.go:36-46`
- 类型定义: `types/types.go:67-76`
- API 响应: `~/hotplex/.logs/daemon.log`

**官方示例**（来自文档）:
```
If you have 100,000 cached tokens and 50 new tokens:
- cache_read_input_tokens: 100,000
- input_tokens: 50
- Total: 100,050
```
