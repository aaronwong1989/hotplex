# Slack TableBlock API Research Report

**Date**: 2026-03-29
**Project**: HotPlex
**SDK Version**: slack-go/slack v0.20.0

---

## Executive Summary

Slack's TableBlock feature is **generally available** (not beta), but has **critical implementation issues** in the slack-go SDK that cause "invalid_blocks" errors. HotPlex has implemented a workaround using custom JSON marshaling to bypass SDK bugs.

---

## 1. TableBlock API Status

### Official Status: Generally Available

Based on Slack's official documentation at https://docs.slack.dev/reference/block-kit/blocks/table-block/:

- **Not labeled as beta, preview, or experimental**
- Fully documented with official API reference
- Part of standard Block Kit framework
- No special feature flags or workspace enrollment mentioned

**Conclusion**: TableBlock is a production-ready feature, not a beta feature.

---

## 2. JSON Requirements for TableBlock Cells

### Correct Schema (Slack API)

```json
{
  "type": "table",
  "block_id": "table_123",
  "rows": [
    [
      {
        "type": "rich_text",
        "elements": [
          {
            "type": "rich_text_section",
            "elements": [
              {
                "type": "text",
                "text": "Cell content"
              }
            ]
          }
        ]
      }
    ]
  ]
}
```

### Key Requirements

1. **Cells MUST NOT have `block_id` field**
   - Slack API rejects cells with `block_id` inside `rows`
   - Only the top-level table block should have `block_id`

2. **Cell structure**:
   ```json
   {
     "type": "rich_text",
     "elements": [...]
   }
   ```

3. **Limits**:
   - Maximum 100 rows
   - Maximum 20 cells per row
   - Cell types: `raw_text` or `rich_text`

---

## 3. slack-go SDK Implementation Issues

### SDK Version: v0.20.0 (Latest at time of research)

#### Issue #1: RichTextBlock MarshalJSON Bug

**Problem**: The SDK's `RichTextBlock.MarshalJSON()` method incorrectly includes `block_id` in the JSON output.

**Expected** (Slack API compliant):
```json
{
  "type": "rich_text",
  "elements": [...]
}
```

**Actual** (SDK output):
```json
{
  "type": "rich_text",
  "block_id": "",  // ← Slack API rejects this in table cells
  "elements": [...]
}
```

**Impact**: Causes "invalid_blocks" error when sending TableBlock to Slack API.

**Evidence**: HotPlex code comments in `chatapps/slack/builder_table.go:6-7`:
```
// NO block_id field in cells (slack-go SDK's RichTextBlock.MarshalJSON incorrectly emits it,
// causing Slack API "invalid_blocks" error).
```

#### Issue #2: Recent Addition (November 2025)

- **PR #1490**: Added TableBlock support on 2025-11-13
- **PR #1511**: Fixed unmarshaling on 2025-12-16
- **First stable release**: After v0.12.0 (current is v0.20.0)

**Issue #1498 Comment** (2025-11-21):
> "I tried this quickly a few days back and was unable to render the table block successfully --
> it was an `invalid_blocks` error message"

**Conclusion**: TableBlock support is recent and has known bugs in the SDK.

---

## 4. HotPlex Workaround Implementation

### Strategy: Custom MarshalJSON

HotPlex implements a custom `tableBlock` wrapper that:

1. **Wraps** `slack.TableBlock` for type safety
2. **Overrides** `MarshalJSON()` to produce Slack-compliant JSON
3. **Removes** `block_id` from cell serialization

**File**: `/Users/huangzhonghui/hotplex/chatapps/slack/builder_table.go`

```go
type tableBlock struct {
    *slack.TableBlock
}

func (t tableBlock) MarshalJSON() ([]byte, error) {
    // Custom logic that excludes block_id from cells
    type alias struct {
        Type    string                `json:"type"`
        BlockID string                `json:"block_id,omitempty"`
        Rows    [][]tableCell         `json:"rows"`
        ColSets []slack.ColumnSetting `json:"column_settings,omitempty"`
    }
    // ... custom serialization logic
}
```

### Fallback Mechanism

**File**: `/Users/huangzhonghui/hotplex/chatapps/slack/messages.go:109-112`

```go
// If Slack rejects TableBlock with invalid_blocks, retry with classic format.
if isInvalidBlocksError(err) && msg.Type == base.MessageTypeSessionStats {
    a.Logger().Warn("TableBlock rejected by Slack API, retrying with classic format")
    // ... fallback to SectionBlock with mrkdwn
}
```

**Reasoning**: Handles edge cases where:
- Workspace has TableBlock disabled
- SDK bug still produces invalid JSON despite workaround
- Slack API changes validation rules

---

## 5. Known GitHub Issues

### slack-go/slack Repository

#### Issue #1498: "Support the table block"
- **Created**: 2025-11-13
- **Closed**: 2025-11-13 (same day as PR #1490 merged)
- **Status**: Merged into master
- **Commenter reported**: "invalid_blocks" error when testing

#### No dedicated issues for "invalid_blocks" with TableBlock
- Likely underreported due to recent feature addition
- Workaround discussions in community channels (not tracked in GitHub issues)

---

## 6. Recommendations

### For HotPlex

✅ **Current approach is correct**:
1. Keep custom `tableBlock` wrapper with `MarshalJSON` override
2. Maintain fallback to `SectionBlock` for compatibility
3. Monitor SDK releases for bug fixes

### For SDK Users

⚠️ **Do not use `slack.TableBlock` directly** until SDK bug is fixed:
- Use custom marshaling (like HotPlex)
- Or wait for upstream fix in slack-go/slack

### For SDK Contributors

🔧 **Fix needed in slack-go/slack**:
1. Modify `RichTextBlock.MarshalJSON()` to accept a "omit block_id" flag
2. Update `TableBlock.MarshalJSON()` to suppress block_id in cells
3. Add integration tests with actual Slack API validation

---

## 7. Code Examples

### ❌ Incorrect (Will cause "invalid_blocks")

```go
table := slack.NewTableBlock("my_table")
table.AddRow(
    slack.NewRichTextBlock("cell1", section),  // block_id "cell1" causes error
)
json.Marshal(table)  // SDK includes block_id in cell JSON
```

### ✅ Correct (HotPlex approach)

```go
table := slack.NewTableBlock("my_table")
table.AddRow(
    slack.NewRichTextBlock("", section),  // empty block_id
)
tableBlock{TableBlock: table}.MarshalJSON()  // custom marshaler removes block_id
```

---

## 8. References

### Official Documentation
- https://docs.slack.dev/reference/block-kit/blocks/table-block/

### SDK Source Code
- https://github.com/slack-go/slack/blob/master/block_table.go
- https://github.com/slack-go/slack/blob/master/block_rich_text.go

### HotPlex Implementation
- `/Users/huangzhonghui/hotplex/chatapps/slack/builder_table.go`
- `/Users/huangzhonghui/hotplex/chatapps/slack/messages.go`

### GitHub Issues/PRs
- https://github.com/slack-go/slack/pull/1490 (TableBlock support)
- https://github.com/slack-go/slack/issues/1498 (Feature request)
- https://github.com/slack-go/slack/pull/1511 (Unmarshal fix)

---

## 9. Timeline

| Date | Event |
|------|-------|
| 2025-11-06 | PR #1490 opened (TableBlock support) |
| 2025-11-13 | PR #1490 merged into master |
| 2025-11-21 | User reports "invalid_blocks" error in issue comments |
| 2025-12-16 | PR #1511 merged (unmarshal fix) |
| 2026-03-29 | HotPlex using v0.20.0 with custom workaround |

---

## Conclusion

Slack's TableBlock API is **production-ready**, but the **slack-go SDK has a critical bug** that causes cells to include `block_id`, triggering "invalid_blocks" errors. HotPlex has correctly implemented a workaround using custom JSON marshaling. Monitor the slack-go/slack repository for fixes in future releases.
