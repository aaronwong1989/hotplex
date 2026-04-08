package feishu

import (
	"regexp"
	"strings"

	"github.com/hrygo/hotplex/chatapps/base"
)

// FeishuConverter converts Markdown content to Feishu-specific format
type FeishuConverter struct{}

// NewFeishuConverter creates a new Feishu content converter
func NewFeishuConverter() *FeishuConverter {
	return &FeishuConverter{}
}

// ConvertMarkdownToPlatform converts Markdown to Feishu interactive card format
func (c *FeishuConverter) ConvertMarkdownToPlatform(content string, parseMode base.ParseMode) string {
	if parseMode == base.ParseModeNone || content == "" {
		return content
	}

	// Apply Markdown processing for Feishu
	text := content

	// Convert code blocks: ```lang\ncode\n``` → ```code```
	text = c.convertCodeBlocks(text)

	// Convert inline code: `code` → `code`
	text = c.convertInlineCode(text)

	// Convert bold: **text** → **text**
	text = c.convertBold(text)

	// Convert italic: *text* or _text_ → *text*
	text = c.convertItalic(text)

	// Convert headers: # Header → **Header**
	text = c.convertHeaders(text)

	// Convert links: [text](url) → text
	text = c.convertLinks(text)

	// Convert lists: - item → • item
	text = c.convertLists(text)

	// Convert blockquotes: > quote → > quote
	text = c.convertBlockquotes(text)

	// Escape special characters for Feishu
	text = c.EscapeSpecialChars(text)

	return text
}

// EscapeSpecialChars escapes Feishu special characters
func (c *FeishuConverter) EscapeSpecialChars(text string) string {
	// Feishu special characters that need escaping in certain contexts
	// These are escaped with backslash in Markdown but need different handling for Feishu
	special := []string{"&", "<", ">"}
	for _, char := range special {
		text = strings.ReplaceAll(text, char, " "+char+" ")
	}
	return strings.TrimSpace(text)
}

func (c *FeishuConverter) convertCodeBlocks(text string) string {
	// Match ```lang\n...\n``` or ```\n...\n```
	re := regexp.MustCompile("```(?:\\w+)?\\n([\\s\\S]*?)```")
	return re.ReplaceAllString(text, "```$1```")
}

func (c *FeishuConverter) convertInlineCode(text string) string {
	re := regexp.MustCompile("`([^`]+)`")
	return re.ReplaceAllString(text, "`$1`")
}

func (c *FeishuConverter) convertBold(text string) string {
	re := regexp.MustCompile("\\*\\*([^*]+)\\*\\*")
	return re.ReplaceAllString(text, "*$1*")
}

func (c *FeishuConverter) convertItalic(text string) string {
	// Handle *text* but not **text** - using simple approach avoiding lookbehind
	re := regexp.MustCompile("\\*([^*]+)\\*")
	return re.ReplaceAllString(text, "_${1}_")
}

func (c *FeishuConverter) convertHeaders(text string) string {
	// Convert # Header to **Header**
	re := regexp.MustCompile("(?m)^#+\\s+(.+)$")
	return re.ReplaceAllString(text, "**$1**")
}

func (c *FeishuConverter) convertLinks(text string) string {
	// Convert [text](url) to text
	re := regexp.MustCompile("\\[([^\\]]+)\\]\\([^)]+\\)")
	return re.ReplaceAllString(text, "$1")
}

func (c *FeishuConverter) convertLists(text string) string {
	// Convert - item or * item to • item
	re := regexp.MustCompile("(?m)^[-*]\\s+(.+)$")
	return re.ReplaceAllString(text, "• $1")
}

func (c *FeishuConverter) convertBlockquotes(text string) string {
	// Convert > quote to quote
	re := regexp.MustCompile("(?m)^>\\s*(.+)$")
	return re.ReplaceAllString(text, "> $1")
}

// Verify FeishuConverter implements base.ContentConverter at compile time
var _ base.ContentConverter = (*FeishuConverter)(nil)
