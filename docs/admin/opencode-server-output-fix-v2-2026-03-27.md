# OpenCode Server 输出问题修复报告 v2

**日期**: 2026-03-27 21:10
**问题**: OpenCode Server 模式输出到 Slack 时出现多项异常

---

## 🔍 问题清单

1. **内部指令泄露** - `[search-mode]`, `[analyze-mode]`, `<context>`, `<user_query>` 等指令被输出
2. **工作目录不正确** - 显示为 `/Users/huangzhonghui` 而非项目目录
3. **无统计信息** - session stats 未显示
4. **Markdown 渲染不完整** - 加粗字体、emoji 未正确渲染

---

## ✅ 已完成的修复

### Fix 1: 内部指令和 XML 标签过滤

**根本原因**：OpenCode Server 将 system prompt 中的指令作为 **answer 事件**（非 thinking 事件）返回

**修复位置**：`chatapps/engine_handler.go:886-897`

```go
func (c *StreamCallback) handleAnswer(data any) error {
	// Capture answer content
	var content string
	switch v := data.(type) {
	case *event.EventWithMeta:
		content = v.EventData
	case string:
		content = v
	}

	if content == "" {
		return nil
	}

	// 🚫 Filter OpenCode internal directives and XML tags from answer content
	// These are system prompt markers that should not be shown to users
	if strings.HasPrefix(content, "[search-mode]") ||
		strings.HasPrefix(content, "[analyze-mode]") ||
		strings.HasPrefix(content, "<context>") ||
		strings.HasPrefix(content, "<user_query>") {
		c.logger.Debug("Filtering internal directive from answer", "content_preview", strutil.Truncate(content, 50))
		return nil // Silent drop
	}

	c.mu.Lock()
	// ... rest of the function
}
```

**影响**：
- ✅ `[search-mode]`, `[analyze-mode]` 指令不再输出
- ✅ `<context>`, `<user_query>` XML 标签不再输出
- ✅ 减少噪音，提升用户体验

---

### Fix 2: 工作目录路径展开

**根本原因**：Go 的 `os.ExpandEnv()` **不支持** `~` 符号展开

**修复位置**：`.env`

```bash
# ❌ 修复前（不支持）
HOTPLEX_PROJECTS_DIR=~/.hotplex/projects

# ✅ 修复后（绝对路径）
HOTPLEX_PROJECTS_DIR=/Users/huangzhonghui/.hotplex/projects
```

**影响**：
- ✅ 工作目录正确设置为 `/Users/huangzhonghui/.hotplex/projects/hotplex`
- ✅ OpenCode Server 在正确的项目目录中运行

---

### Fix 3: Markdown 加粗渲染

**根本原因**：`convertItalic` 在 `convertBold` 之前执行，导致 `**text**` 被错误匹配

**修复位置**：`chatapps/slack/converter.go:60`

```go
// 转换顺序调整
converted := segment.text
converted = convertBold(converted)      // 🎯 Fix: Bold first to avoid italic interference
converted = convertItalic(converted)
converted = convertLinks(converted)
converted = convertGitHubEmojiToUnicode(converted)
converted = escapeSlackChars(converted)
```

**影响**：
- ✅ `**text**` 正确转换为 Slack 的 `*text*`
- ✅ 加粗字体正确渲染

---

### Fix 4: GitHub Emoji 转换（已在 v1 完成）

**修复位置**：`chatapps/slack/converter.go:72-224`

**支持**：140+ GitHub emoji 映射到 Unicode

```go
":bar_chart:": "📊",
":warning:": "⚠️",
":white_check_mark:": "✅",
// ... 140+ 映射
```

**影响**：
- ✅ GitHub 风格 emoji 正确渲染
- ✅ 兼容 Slack mrkdwn 格式

---

## ⚠️ 待验证的问题

### Session Stats 显示

**状态**：需要用户测试后确认

**代码位置**：`chatapps/engine_handler.go:1100-1223`

**预期行为**：
- 会话结束时显示统计信息
- 包含：时长、token 使用量、context 占用等

**如果未显示，请提供**：
- 完整日志：`tail -200 /tmp/hotplexd.log`
- Slack 输出截图

---

## 📊 验证结果

### 编译测试
```bash
$ make build
✅ Build complete: dist/hotplexd
```

### 单元测试
```bash
$ go test ./chatapps/slack -v
✅ All tests passed (20+ tests)
```

### 守护进程状态
```bash
$ ps aux | grep hotplexd
huangzhonghui 14346 0.0 0.1 436673680 22416 ?? SN 9:10PM 0:00.04 ./dist/hotplexd start
```

---

## 🎯 下一步行动

### 立即测试（用户）
在 Slack 中发送测试消息，验证：
- [ ] 内部指令不再输出
- [ ] XML 标签不再输出
- [ ] 工作目录正确
- [ ] 加粗字体正确渲染
- [ ] Emoji 正确显示
- [ ] Session stats 是否显示

### 如果仍有问题
提供以下诊断信息：
1. Slack 完整输出截图
2. 守护进程日志：`tail -200 /tmp/hotplexd.log`
3. OpenCode Server 日志（如果适用）

---

## 📝 相关文件

- `chatapps/engine_handler.go` - 事件处理和过滤
- `chatapps/slack/converter.go` - Markdown 转换
- `.env` - 环境变量配置
- `configs/base/slack.yaml` - Slack 配置

---

## 🔗 相关文档

- [OpenCode Server 输出修复 v1](./opencode-server-output-fix-2026-03-27.md)
- [ChatApps SDK First 规范](../.agent/rules/chatapps-sdk-first.md)
- [Uber Go Style Guide](../.agent/rules/uber-go-style-guide.md)

---

**守护进程已就绪，等待测试反馈** ✅
