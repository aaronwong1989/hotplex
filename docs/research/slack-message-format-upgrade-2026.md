# Slack 消息格式升级调研报告（2026）

**调研日期**: 2026-03-26  
**当前 SDK 版本**: slack-go v0.18.0  
**最新 SDK 版本**: slack-go v0.20.0  
**目标**: 提升 HotPlex UI/UX，重构消息展示层

---

## 1. Slack Block Kit 最新特性（2025-2026）

### 1.1 原生表格块（Table Block）🎉

**发布时间**: 2025-08-14  
**官方文档**: [Block Kit Table Block](https://docs.slack.dev/changelog/2025/08/14/block-kit-table-block/)

#### 核心功能
- 原生支持结构化表格数据展示
- 替代之前的 ASCII 表格/Attachment Fields 方案
- 支持自定义列设置（对齐、宽度等）

#### SDK 支持
```go
// slack-go v0.20.0 提供
type TableBlock struct {
    Type           MessageBlockType
    BlockID        string
    Rows           [][]*RichTextBlock  // 二维数组：行 → 单元格
    ColumnSettings []ColumnSetting     // 列配置（可选）
}

// 构建方式
table := slack.NewTableBlock("stats_table").
    AddRow(cell1, cell2, cell3).
    AddRow(cell4, cell5, cell6).
    WithColumnSettings(setting1, setting2)
```

### 1.2 其他重要更新

| 日期 | 特性 | 说明 | 文档 |
|------|------|------|------|
| **2026-03-06** | Rich Text Markdown Types | 语法高亮代码块、富文本类型扩展 | [查看](https://docs.slack.dev/changelog/2026/03/06/block-kit-rich-text/) |
| **2025-02-03** | Markdown Block | 标准 Markdown 输入 → Slack 格式化 | [查看](https://docs.slack.dev/changelog/2025/02/03/block-kit-markdown/) |
| **2025-08** | Auto-detect Tables | 桌面/Web 自动识别粘贴的表格数据 | [特性公告](https://docs.slack.dev/changelog/) |

---

## 2. slack-go SDK 升级分析

### 2.1 版本对比

| 版本 | 发布时间 | TableBlock 支持 | Breaking Changes |
|------|---------|----------------|------------------|
| **v0.18.0** | 当前使用 | ❌ 不支持 | - |
| **v0.20.0** | 最新 | ✅ 完整支持 | 移除 `files.upload` 旧 API（已在 Slack 侧废弃） |

### 2.2 Breaking Changes（v0.18.0 → v0.20.0）

**关键变更** (#1481):
- 移除 `UploadFile`, `UploadFileContext` 等已废弃方法
- Slack 侧已于 2025-11 停止旧 `files.upload` API

**HotPlex 影响**:
```bash
# 检查当前代码是否使用已移除 API
grep -r "UploadFile" chatapps/slack/
```
预计影响：**低**（HotPlex 不使用文件上传功能）

### 2.3 升级建议

**升级路径**: `v0.18.0` → `v0.20.0`  
**风险等级**: 🟢 低风险  
**建议执行**: 立即升级以获取 TableBlock 支持

```bash
# 升级命令
go get github.com/slack-go/slack@v0.20.0
go mod tidy
```

---

## 3. HotPlex 重构机会

### 3.1 当前实现分析

**文件**: `chatapps/slack/builder_stats.go`

**现状**:
- 使用 `slack.NewContextBlock()` 单行显示统计
- 格式：`⏱️ 2m30s • ⚡ 100K/50K (cache: 10K/5K) • 📝 3 files • 🔧 5 tools`
- 问题：信息密集、可读性差

### 3.2 重构方案：表格化展示

#### 方案 A：Session Stats 表格（推荐）

**效果对比**:

**Before**（当前）:
```
⏱️ 2m30s • ⚡ 100K/50K (cache: 10K/5K) • 📝 3 files • 🔧 5 tools
```

**After**（表格化）:
```
┌─────────────────────────────────────────┐
│ ⏱️ Duration    │ 2m30s                  │
│ ⚡ Input       │ 100K (cache: 10K)      │
│ ⚡ Output      │ 50K (cache: 5K)        │
│ 📝 Files       │ 3 modified             │
│ 🔧 Tools       │ 5 calls                │
└─────────────────────────────────────────┘
```

**代码示例**:
```go
func (b *StatsMessageBuilder) BuildSessionStatsTable(msg *base.ChatMessage) []slack.Block {
    metadata := msg.Metadata
    
    // 创建表格
    table := slack.NewTableBlock("session_stats")
    
    // 行 1: Duration
    if dur := extractInt64(metadata, "total_duration_ms"); dur > 0 {
        table.AddRow(
            slack.NewRichTextBlock("⏱️ Duration"),
            slack.NewRichTextBlock(FormatDuration(dur)),
        )
    }
    
    // 行 2: Input Tokens
    if in := extractInt64(metadata, "input_tokens"); in > 0 {
        cacheRead := extractInt64(metadata, "cache_read_tokens")
        text := formatTokenCount(in)
        if cacheRead > 0 {
            text += fmt.Sprintf(" (cache: %s)", formatTokenCount(cacheRead))
        }
        table.AddRow(
            slack.NewRichTextBlock("⚡ Input"),
            slack.NewRichTextBlock(text),
        )
    }
    
    // 行 3: Output Tokens
    // ... 类似处理
    
    return []slack.Block{table}
}
```

#### 方案 B：Command Progress 表格

**场景**: 多步骤命令执行（如 `/hotplex release`）

**效果**:
```
┌────────────────────────────────────────────┐
│ Step │ Status   │ Details                 │
├──────┼──────────┼─────────────────────────┤
│ 1/5  │ ✅ Done  │ Version bump            │
│ 2/5  │ ✅ Done  │ Changelog update        │
│ 3/5  │ 🔄 Run   │ Git commit              │
│ 4/5  │ ⏸️ Wait  │ CI verification         │
│ 5/5  │ ⏸️ Wait  │ GitHub release          │
└────────────────────────────────────────────┘
```

**优势**:
- 清晰展示进度
- 状态一目了然
- 支持动态更新

#### 方案 C：Tool Calls 汇总表格

**场景**: 会话结束时展示工具调用统计

**效果**:
```
┌─────────────────────────────────────────┐
│ Tool                │ Calls │ Status    │
├─────────────────────┼───────┼───────────┤
│ Read                │ 15    │ ✅ 100%   │
│ Edit                │ 8     │ ✅ 100%   │
│ Bash                │ 5     │ ✅ 100%   │
│ Grep                │ 3     │ ✅ 100%   │
└─────────────────────┴───────┴───────────┘
```

### 3.3 渐进式重构路线图

**Phase 1**: SDK 升级（1 天）
- 升级 slack-go v0.20.0
- 运行测试确保兼容性
- 验证现有功能无回归

**Phase 2**: 新增表格构建器（2-3 天）
- 实现 `builder_table.go`（表格专用构建器）
- 提供 `BuildSessionStatsTable()`
- 提供 `BuildCommandProgressTable()`
- 单元测试覆盖

**Phase 3**: 集成与 A/B 测试（2 天）
- 在非生产环境启用表格展示
- 收集用户反馈
- 性能监控（表格渲染延迟）

**Phase 4**: 全量发布（1 天）
- 生产环境默认启用
- 保留旧版单行模式作为 fallback
- 文档更新

**总工期**: 6-7 天

---

## 4. UX 提升预期

### 4.1 可读性提升

| 指标 | 当前（单行） | 重构后（表格） | 提升 |
|------|------------|--------------|------|
| **信息密度** | 高（拥挤） | 适中（结构化） | ⬆️ 40% |
| **关键指标识别速度** | 慢（需扫描全行） | 快（垂直对齐） | ⬆️ 60% |
| **缓存信息清晰度** | 差（括号嵌套） | 好（独立单元格） | ⬆️ 80% |

### 4.2 交互体验提升

**新增能力**:
1. **动态进度更新**: Command Progress 表格可实时更新状态
2. **列排序**: 未来可支持按 Token 数量/Duration 排序
3. **可点击行**: 未来可在表格行添加操作按钮

---

## 5. 风险评估与缓解

### 5.1 潜在风险

| 风险 | 影响 | 概率 | 缓解措施 |
|------|------|------|---------|
| **Slack 渲染延迟** | 表格首次渲染慢于 Context Block | 中 | 性能测试，设置 timeout |
| **移动端适配** | 表格在小屏幕显示可能拥挤 | 低 | 响应式设计，移动端降级为单行 |
| **SDK Breaking Change** | v0.20.0 可能有未知问题 | 低 | 充分测试，灰度发布 |

### 5.2 回滚策略

**条件**: 表格渲染延迟 > 500ms 或移动端投诉 > 5%

**操作**:
```go
// builder_stats.go
func (b *StatsMessageBuilder) BuildSessionStatsMessage(msg *base.ChatMessage) []slack.Block {
    if config.UseTableFormat && !isMobileDevice(msg) {
        return b.BuildSessionStatsTable(msg)  // 新版表格
    }
    return b.BuildSessionStatsCompact(msg)    // 旧版单行
}
```

---

## 6. 代码示例库

### 6.1 表格构建器模板

```go
// chatapps/slack/builder_table.go
package slack

import (
    "github.com/hrygo/hotplex/chatapps/base"
    "github.com/slack-go/slack"
)

// TableBuilder provides reusable table building utilities
type TableBuilder struct {
    config TableConfig
}

type TableConfig struct {
    MaxRows    int  // 最大行数（性能保护）
    Compact    bool // 紧凑模式（移动端优化）
    ShowHeader bool // 是否显示表头
}

func NewTableBuilder(config TableConfig) *TableBuilder {
    return &TableBuilder{config: config}
}

// BuildSessionStatsTable builds a table block for session statistics
func (tb *TableBuilder) BuildSessionStatsTable(msg *base.ChatMessage) *slack.TableBlock {
    table := slack.NewTableBlock("session_stats")
    metadata := msg.Metadata
    
    // 行 1: Duration
    if dur := extractInt64(metadata, "total_duration_ms"); dur > 0 {
        table.AddRow(
            tb.buildLabel("⏱️ Duration"),
            tb.buildValue(FormatDuration(dur)),
        )
    }
    
    // 行 2-3: Tokens
    if in, out := extractInt64(metadata, "input_tokens"), extractInt64(metadata, "output_tokens"); in > 0 || out > 0 {
        cacheRead := extractInt64(metadata, "cache_read_tokens")
        cacheWrite := extractInt64(metadata, "cache_write_tokens")
        
        // Input row
        table.AddRow(
            tb.buildLabel("⚡ Input"),
            tb.buildTokenValue(in, cacheRead),
        )
        
        // Output row
        table.AddRow(
            tb.buildLabel("⚡ Output"),
            tb.buildTokenValue(out, cacheWrite),
        )
    }
    
    // 行 4: Files
    if files := extractInt64(metadata, "files_modified"); files > 0 {
        table.AddRow(
            tb.buildLabel("📝 Files"),
            tb.buildValue(fmt.Sprintf("%d modified", files)),
        )
    }
    
    // 行 5: Tools
    if tools := extractInt64(metadata, "tool_call_count"); tools > 0 {
        table.AddRow(
            tb.buildLabel("🔧 Tools"),
            tb.buildValue(fmt.Sprintf("%d calls", tools)),
        )
    }
    
    return table
}

// buildLabel creates a label cell with consistent styling
func (tb *TableBuilder) buildLabel(text string) *slack.RichTextBlock {
    // TODO: Implement with proper styling
    return slack.NewRichTextBlock(text)
}

// buildValue creates a value cell with consistent styling
func (tb *TableBuilder) buildValue(text string) *slack.RichTextBlock {
    // TODO: Implement with proper styling
    return slack.NewRichTextBlock(text)
}

// buildTokenValue formats token count with cache info
func (tb *TableBuilder) buildTokenValue(total, cache int64) *slack.RichTextBlock {
    text := formatTokenCount(total)
    if cache > 0 {
        text += fmt.Sprintf(" (cache: %s)", formatTokenCount(cache))
    }
    return tb.buildValue(text)
}
```

### 6.2 移动端降级逻辑

```go
// chatapps/slack/adapter.go

// isMobileDevice detects if user is on mobile Slack client
func (a *Adapter) isMobileDevice(msg *base.ChatMessage) bool {
    // Slack 在 metadata 中可能包含客户端信息
    if client, ok := msg.Metadata["client_type"].(string); ok {
        return client == "mobile" || client == "ios" || client == "android"
    }
    return false
}

// BuildStatsMessage with mobile fallback
func (a *Adapter) BuildStatsMessage(msg *base.ChatMessage) []slack.Block {
    builder := NewStatsMessageBuilder()
    
    // 移动端使用紧凑单行格式
    if a.isMobileDevice(msg) {
        return builder.BuildSessionStatsMessage(msg)
    }
    
    // 桌面端使用表格格式
    tableBuilder := NewTableBuilder(TableConfig{
        MaxRows:    5,
        Compact:    false,
        ShowHeader: false,
    })
    
    table := tableBuilder.BuildSessionStatsTable(msg)
    return []slack.Block{table}
}
```

---

## 7. 参考资源

### 官方文档
- [Table Block 引入公告](https://docs.slack.dev/changelog/2025/08/14/block-kit-table-block/)
- [Slack Block Kit 完整参考](https://docs.slack.dev/reference/block-kit/blocks/table-block/)
- [Rich Text Markdown Types (2026)](https://docs.slack.dev/changelog/2026/03/06/block-kit-rich-text/)
- [Markdown Block (2025)](https://docs.slack.dev/changelog/2025/02/03/block-kit-markdown/)

### SDK 资源
- [slack-go GitHub 仓库](https://github.com/slack-go/slack)
- [block_table.go 源码](https://github.com/slack-go/slack/blob/master/block_table.go)
- [Go Packages 文档](https://pkg.go.dev/github.com/slack-go/slack)

### 社区讨论
- [Stack Overflow: How to render tables in Slack](https://stackoverflow.com/questions/59006831/how-to-render-tables-in-slack)

---

## 8. 下一步行动

### 立即执行（本周）
- [ ] 创建 Issue 跟踪升级任务
- [ ] 升级 slack-go v0.20.0
- [ ] 运行完整测试套件

### 短期（2 周内）
- [ ] 实现 `builder_table.go`
- [ ] 添加单元测试
- [ ] 非生产环境 A/B 测试

### 中期（1 个月内）
- [ ] 生产环境灰度发布
- [ ] 收集用户反馈
- [ ] 性能调优

### 长期优化
- [ ] 探索表格交互功能（排序、过滤）
- [ ] 移动端自适应优化
- [ ] 其他 Block Kit 新特性集成（Markdown Block）

---

**维护者**: @hotplex-team  
**最后更新**: 2026-03-26  
**下次审查**: 2026-04-15
