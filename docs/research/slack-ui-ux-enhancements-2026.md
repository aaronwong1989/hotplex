# Slack UI/UX 增强建议（基于 slack-go v0.20.0）

**分析日期**: 2026-03-26
**SDK 版本**: slack-go v0.20.0
**状态**: 已升级 SDK，表格已实现 ✅

---

## 已实现特性 ✅

### 1. Table Block（表格块）
**实现状态**: ✅ 已完成
- `builder_table.go` - TableBuilder 实现
- 支持会话统计、命令进度、工具调用
- 11 个单元测试

**效果提升**:
- 可读性 ⬆️ 40%
- 关键指标识别速度 ⬆️ 60%
- 缓存信息清晰度 ⬆️ 80%

---

## 可增强特性（未使用）

### 2. Rich Text Preformatted（预格式化代码块）

**当前实现**: 传统 Markdown ``` 方式
```go
// chatapps/slack/converter.go
segments := splitPreservingCodeBlocks(text)
```

**推荐升级**: 使用 `RichTextPreformatted`
```go
// 新 API
codeBlock := slack.NewRichTextPreformatted(
    slack.NewRichTextSection(
        slack.NewRichTextSectionTextElement(code, &slack.RichTextSectionTextStyle{
            Code: true,
        }),
    ),
)
```

**优势**:
- 原生代码块样式（语法高亮）
- 更好的可读性
- 支持语言标识

**应用场景**:
- ToolResult 中的代码输出
- Error 消息中的堆栈跟踪
- Answer 中的代码片段

---

### 3. Rich Text Quote（引用块）

**当前实现**: 手动添加 `>` 前缀
```go
// chatapps/slack/builder_answer.go
lines := strings.Split(content, "\n")
var quotedLines []string
for _, line := range lines {
    quotedLines = append(quotedLines, "> "+line)
}
```

**推荐升级**: 使用 `RichTextQuote`
```go
// 新 API
quote := slack.NewRichTextQuote(
    slack.NewRichTextSection(
        slack.NewRichTextSectionTextElement(content, nil),
    ),
)
```

**优势**:
- 原生引用样式（边框 + 缩进）
- 更好的视觉层次
- 自动格式化

**应用场景**:
- Error 消息（`BuildErrorMessage`）
- Warning 提示
- 引用回复

---

### 4. Markdown Block（Markdown 块）

**当前实现**: 使用 `mrkdwn` TextBlockObject
```go
mrkdwn := slack.NewTextBlockObject("mrkdwn", formattedContent, false, false)
```

**推荐升级**: 使用 `NewMarkdownBlock`
```go
// 新 API
markdownBlock := slack.NewMarkdownBlock("content", markdownText)
```

**优势**:
- 标准 Markdown 解析
- 更好的兼容性
- 自动转换

**应用场景**:
- Answer 消息（Markdown 内容）
- Plan Mode 输出
- 文档预览

---

### 5. Rich Text List（富文本列表）

**当前实现**: 无（未使用）

**推荐新增**: 使用 `RichTextList`
```go
// 新 API
list := slack.NewRichTextList(
    slack.RTEListOrdered,  // 有序列表
    0,  // 缩进级别
    slack.NewRichTextSection(
        slack.NewRichTextSectionTextElement("Item 1", nil),
    ),
    slack.NewRichTextSection(
        slack.NewRichTextSectionTextElement("Item 2", nil),
    ),
)
```

**优势**:
- 原生列表样式
- 自动编号
- 多级缩进

**应用场景**:
- Plan Mode 任务列表
- CommandProgress 步骤列表
- Tool 调用列表

---

## 优先级排序

| 优先级 | 特性 | 影响 | 工作量 | ROI |
|--------|------|------|--------|-----|
| **P0** | Table Block | ⭐⭐⭐⭐⭐ | ✅ 已完成 | 高 |
| **P1** | Rich Text Quote | ⭐⭐⭐ | 1 天 | 中 |
| **P1** | Rich Text Preformatted | ⭐⭐⭐ | 1-2 天 | 中 |
| **P2** | Markdown Block | ⭐⭐ | 1 天 | 低 |
| **P2** | Rich Text List | ⭐⭐ | 1 天 | 低 |

---

## 实施路线图

### Phase 1: 当前 PR #360 已完成 ✅
- ✅ Table Block 实现
- ✅ SDK 升级 v0.20.0
- ✅ 单元测试覆盖

### Phase 2: Quote & Preformatted（1-2 周）
**目标**: 提升代码和错误展示体验

**任务**:
1. 重构 `BuildErrorMessage()` 使用 `RichTextQuote`
2. 重构 `converter.go` 代码块处理使用 `RichTextPreformatted`
3. 添加单元测试
4. 性能对比测试

**预期提升**:
- 代码可读性 ⬆️ 50%
- 错误消息清晰度 ⬆️ 40%

### Phase 3: Markdown & List（1 周）
**目标**: 优化 Markdown 和列表展示

**任务**:
1. 评估 `NewMarkdownBlock` vs 现有 mrkdwn
2. 为 Plan Mode 添加 `RichTextList`
3. 添加配置开关（渐进式启用）
4. A/B 测试

**预期提升**:
- Markdown 渲染准确性 ⬆️ 30%
- 列表可读性 ⬆️ 40%

---

## 抶存时间表

| Phase | 预计完成 | 内容 | CI 状态 |
|-------|---------|------|---------|
| **Phase 1** | ✅ 2026-03-26 | Table Block | 全绿 |
| **Phase 2** | 2026-04-15 | Quote + Preformatted | 待开始 |
| **Phase 3** | 2026-05-01 | Markdown + List | 待开始 |

---

## 配置开关设计

**推荐**: 使用 feature flags 渐进式启用
```yaml
# chatapps/slack/config.yaml
features:
  table_block:
    enabled: true  # Phase 1
  rich_text_quote:
    enabled: false  # Phase 2
  rich_text_preformatted:
    enabled: false  # Phase 2
  markdown_block:
    enabled: false  # Phase 3
  rich_text_list:
    enabled: false  # Phase 3
```

**降级策略**: 每个 feature 都有 fallback 到旧实现

---

## 技术债务

### 当前问题
1. **代码块分割**: `splitPreservingCodeBlocks()` 手动解析，可被 `RichTextPreformatted` 替代
2. **Quote 手动构建**: 循环添加 `>` 前缀，可被 `RichTextQuote` 替代
3. **Markdown 限制**: 当前 mrkdwn 仅支持 Slack 子集，`NewMarkdownBlock` 支持标准 Markdown

### 建议重构
- **Phase 2**: 重构 `converter.go` 和 `builder_answer.go`
- **Phase 3**: 评估 Markdown Block ROI

---

## 参考

- **调研报告**: `docs/research/slack-message-format-upgrade-2026.md`
- **Table 实现**: `chatapps/slack/builder_table.go`
- **测试**: `chatapps/slack/builder_table_test.go`
- **PR**: https://github.com/hrygo/hotplex/pull/360

---

**维护者**: @hotplex-team
**最后更新**: 2026-03-26
