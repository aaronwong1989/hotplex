# OpenCode Server 宿主机配置修复报告

**日期**: 2026-03-27 21:16
**环境**: 宿主机（非 Docker）
**问题**: 工作目录错误 + 消息重复

---

## ✅ 已修复：工作目录问题

### 根本原因

**宿主机配置文件位置错误**：
- ❌ 我之前修改了项目目录下的 `.env` 文件
- ✅ 宿主机实际使用 `~/.hotplex/.env` 文件

### Go 的 `~` 符号限制

Go 的 `os.ExpandEnv()` **不支持** `~` 展开：
```bash
# ❌ 不支持
HOTPLEX_PROJECTS_DIR=~/.hotplex/projects
# 展开结果：字面量 "~/.hotplex/projects"（不会展开为绝对路径）

# ✅ 正确
HOTPLEX_PROJECTS_DIR=/Users/huangzhonghui/.hotplex/projects
```

### 修复操作

**修改文件**：`/Users/huangzhonghui/.hotplex/.env`

```bash
# 修改前
HOTPLEX_PROJECTS_DIR=~/.hotplex/projects
HOTPLEX_MESSAGE_STORE_SQLITE_PATH=~/.hotplex/chatapp_messages.db

# 修改后
HOTPLEX_PROJECTS_DIR=/Users/huangzhonghui/.hotplex/projects
HOTPLEX_MESSAGE_STORE_SQLITE_PATH=/Users/huangzhonghui/.hotplex/chatapp_messages.db
```

### 验证

```bash
$ ls -la /Users/huangzhonghui/.hotplex/projects/
drwxr-xr-x 46 hotplex hotplex 1472 Mar 27 00:09 hotplex
```

✅ 目录存在且包含 hotplex 项目

---

## ⚠️ 待诊断：消息重复问题

### 问题表现

**Slack 输出**（3 次重复）：
```
[晚上 9:14] 当前工作目录：`/Users/huangzhonghui`
这不是一个 git 仓库。有什么我可以帮助你的吗？

[晚上 9:14] 当前工作目录：`/Users/huangzhonghui`
这不是一个 git 仓库。有什么我可以帮助你的吗？
```

### 可能原因

1. **Native Streaming 重复发送**
   - `StreamWriter.Write()` 被调用多次
   - 可能是 SSE 事件重复

2. **Fallback 机制触发**
   - `handleSessionStats` 中的 fallback 发送
   - 但日志中没有 `Fallback` 相关记录

3. **Slack Adapter 层面重复**
   - `SendThreadReply` 被调用多次
   - 可能是消息队列或重试机制

### 诊断步骤

**1. 检查守护进程日志**：
```bash
tail -500 /tmp/hotplexd.log | grep -E "(Fallback|streaming|Sending|SendThreadReply)"
```

**2. 启用详细日志**：
```bash
# 在 ~/.hotplex/.env 中设置
HOTPLEX_LOG_LEVEL=debug
```

**3. 检查 OpenCode Server 日志**（如果使用）：
```bash
docker logs opencode-server --tail 200
```

### 临时调试建议

在 `chatapps/engine_handler.go` 的关键位置添加日志：

```go
// handleAnswer - 检查是否被调用多次
func (c *StreamCallback) handleAnswer(data any) error {
	// ... 过滤逻辑 ...
	c.logger.Debug("handleAnswer called",
		"content_len", len(content),
		"session_id", c.sessionID,
		"stream_active", c.streamWriterActive)
	// ...
}

// handleSessionStats - 检查 fallback 是否触发
func (c *StreamCallback) handleSessionStats(data any) error {
	// ...
	c.logger.Info("Session stats received",
		"stream_was_active", streamWasActive,
		"stream_used_fallback", streamUsedFallback,
		"accumulated_len", len(accumulatedContent))
	// ...
}
```

---

## 📊 当前状态

### 守护进程
- ✅ 已重启（PID: 16461）
- ✅ 配置已更新（绝对路径）
- ⚠️ 等待测试验证

### 已修复的问题
1. ✅ 内部指令过滤（`[search-mode]`, `<context>` 等）
2. ✅ 工作目录路径（使用绝对路径）
3. ✅ Markdown 加粗渲染（转换顺序调整）
4. ✅ GitHub Emoji 转换（140+ 映射）

### 待解决的问题
1. ⚠️ 消息重复（需要日志诊断）
2. ⚠️ Session Stats 显示（待用户验证）

---

## 🎯 下一步行动

### 用户测试
请在 Slack 中重新测试：
1. ✅ 工作目录应正确（`/Users/huangzhonghui/.hotplex/projects/hotplex`）
2. ✅ 内部指令不应输出
3. ✅ 加粗应正确渲染
4. ⚠️ 检查是否仍有重复

### 如果重复仍然存在
提供以下诊断信息：
```bash
# 1. 完整日志
tail -500 /tmp/hotplexd.log > /tmp/hotplex_debug.log

# 2. 查找重复发送的痕迹
grep -E "(Sending|SendThreadReply|Fallback|stream)" /tmp/hotplex_debug.log

# 3. OpenCode Server 日志（如果使用）
docker logs opencode-server --tail 200 > /tmp/opencode_debug.log
```

---

## 📝 相关文件

- `/Users/huangzhonghui/.hotplex/.env` - 宿主机环境变量
- `/Users/huangzhonghui/.hotplex/configs/base/slack.yaml` - Slack 配置
- `chatapps/engine_handler.go` - 事件处理
- `chatapps/slack/converter.go` - Markdown 转换

---

**状态**：工作目录已修复，消息重复需要进一步诊断 🔍
