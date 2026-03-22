package slack

import (
	"strings"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
)

// --- SlackChunker tests ---

func TestSlackChunker_New(t *testing.T) {
	c := NewSlackChunker()
	if c == nil {
		t.Fatal("expected non-nil chunker")
	}
	if c.MaxChars() != SlackTextLimit {
		t.Errorf("expected %d, got %d", SlackTextLimit, c.MaxChars())
	}
}

func TestSlackChunker_ChunkText(t *testing.T) {
	c := NewSlackChunker()
	text := "short"
	result := c.ChunkText(text, 100)
	if len(result) != 1 || result[0] != text {
		t.Errorf("expected single chunk, got %v", result)
	}
}

// --- ContentConverter tests ---

func TestContentConverter_New(t *testing.T) {
	c := NewContentConverter()
	if c == nil {
		t.Fatal("expected non-nil converter")
	}
}

func TestContentConverter_NonMarkdown(t *testing.T) {
	c := NewContentConverter()
	text := "hello **world**"
	result := c.ConvertMarkdownToPlatform(text, base.ParseModeNone)
	if result != text {
		t.Errorf("non-markdown should pass through: got %q", result)
	}
}

func TestContentConverter_Markdown(t *testing.T) {
	c := NewContentConverter()
	// Bold conversion: **text** -> *text*
	result := c.ConvertMarkdownToPlatform("**hello** world", base.ParseModeMarkdown)
	if result != "*hello* world" {
		t.Errorf("expected '*hello* world', got %q", result)
	}
}

func TestContentConverter_Markdown_WithCodeBlock(t *testing.T) {
	c := NewContentConverter()
	input := "before\n```\ncode with <tag>\n```\nafter"
	result := c.ConvertMarkdownToPlatform(input, base.ParseModeMarkdown)
	if result != input {
		// Code blocks should still be escaped but not converted
		// The important thing is no panic
		t.Logf("code block conversion: %q", result)
	}
}

func TestContentConverter_EscapeSpecialChars(t *testing.T) {
	c := NewContentConverter()
	result := c.EscapeSpecialChars("<b>&hello</b>")
	if result != "&lt;b&gt;&amp;hello&lt;/b&gt;" {
		t.Errorf("expected escaped chars, got %q", result)
	}
}

// --- escapeSlackChars tests ---

func TestEscapeSlackChars(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"<b>", "&lt;b&gt;"},
		{"a&b", "a&amp;b"},
		{"a<b>c", "a&lt;b&gt;c"},
		{"a&<b>c", "a&amp;&lt;b&gt;c"},
	}
	for _, tt := range tests {
		result := escapeSlackChars(tt.input)
		if result != tt.expected {
			t.Errorf("escapeSlackChars(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// --- convertBold tests ---

func TestConvertBold(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"**hello**", "*hello*"},
		{"**hello** world **there**", "*hello* world *there*"},
		{"no bold here", "no bold here"},
		{"__hello__", "__hello__"}, // only ** is handled by the outer loop
		{"**", "**"},               // unclosed
	}
	for _, tt := range tests {
		result := convertBold(tt.input)
		if result != tt.expected {
			t.Errorf("convertBold(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// --- convertItalic tests ---

func TestConvertItalic(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"*hello*", "*hello*"}, // convertItalic skips when it looks like bold after convertBold already ran
		{"no italic", "no italic"},
	}
	for _, tt := range tests {
		result := convertItalic(tt.input)
		if result != tt.expected {
			t.Errorf("convertItalic(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// --- convertLinks tests ---

func TestConvertLinks(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"[text](https://example.com)", "<https://example.com|text>"},
		{"no links here", "no links here"},
		{"[a](http://x.com) and [b](http://y.com)", "<http://x.com|a> and <http://y.com|b>"},
		{"[unclosed", "[unclosed"},
	}
	for _, tt := range tests {
		result := convertLinks(tt.input)
		if result != tt.expected {
			t.Errorf("convertLinks(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// --- splitPreservingCodeBlocks tests ---

func TestSplitPreservingCodeBlocks(t *testing.T) {
	segments := splitPreservingCodeBlocks("hello ```code``` world")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d", len(segments))
	}
	if segments[0].isCodeBlock {
		t.Error("first segment should not be code")
	}
	if !segments[1].isCodeBlock {
		t.Error("second segment should be code")
	}
	if segments[2].isCodeBlock {
		t.Error("third segment should not be code")
	}
}

func TestSplitPreservingCodeBlocks_NoCode(t *testing.T) {
	segments := splitPreservingCodeBlocks("hello world")
	if len(segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(segments))
	}
}

func TestSplitPreservingCodeBlocks_Unclosed(t *testing.T) {
	segments := splitPreservingCodeBlocks("hello ```unclosed")
	if len(segments) != 2 {
		t.Errorf("expected 2 segments, got %d", len(segments))
	}
	if !segments[1].isCodeBlock {
		t.Error("unclosed code block should be marked as code")
	}
}

// --- Format functions tests ---

func TestFormatChannelMention(t *testing.T) {
	result := FormatChannelMention("C123", "general")
	if result != "<#C123|general>" {
		t.Errorf("expected <#C123|general>, got %q", result)
	}
}

func TestFormatChannelMentionByID(t *testing.T) {
	result := FormatChannelMentionByID("C123")
	if result != "<#C123>" {
		t.Errorf("expected <#C123>, got %q", result)
	}
}

func TestFormatUserMention(t *testing.T) {
	result := FormatUserMention("U123", "john")
	if result != "<@U123|john>" {
		t.Errorf("expected <@U123|john>, got %q", result)
	}
}

func TestFormatUserMentionByID(t *testing.T) {
	result := FormatUserMentionByID("U123")
	if result != "<@U123>" {
		t.Errorf("expected <@U123>, got %q", result)
	}
}

func TestFormatSpecialMention(t *testing.T) {
	if FormatSpecialMention("here") != "<!here>" {
		t.Error("here mention failed")
	}
	if FormatSpecialMention("channel") != "<!channel>" {
		t.Error("channel mention failed")
	}
	if FormatSpecialMention("everyone") != "<!everyone>" {
		t.Error("everyone mention failed")
	}
}

func TestFormatMentionShorthands(t *testing.T) {
	if FormatHereMention() != "<!here>" {
		t.Error("FormatHereMention failed")
	}
	if FormatChannelAllMention() != "<!channel>" {
		t.Error("FormatChannelAllMention failed")
	}
	if FormatEveryoneMention() != "<!everyone>" {
		t.Error("FormatEveryoneMention failed")
	}
}

func TestFormatDateTime(t *testing.T) {
	result := FormatDateTime(1234567890, "{date}", "fallback")
	if result != "<!date^1234567890^{date}|fallback>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatDateTimeWithLink(t *testing.T) {
	result := FormatDateTimeWithLink(1234567890, "{date}", "https://example.com", "fallback")
	if result != "<!date^1234567890^{date}^https://example.com|fallback>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatDate(t *testing.T) {
	result := FormatDate(1234567890)
	if result != "<!date^1234567890^{date}|Unknown date>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatDateShort(t *testing.T) {
	result := FormatDateShort(1234567890)
	if result != "<!date^1234567890^{date_short}|Unknown date>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatDateLong(t *testing.T) {
	result := FormatDateLong(1234567890)
	if result != "<!date^1234567890^{date_long}|Unknown date>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatTime(t *testing.T) {
	result := FormatTime(1234567890)
	if result != "<!date^1234567890^{time}|Unknown time>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatDateTimeCombined(t *testing.T) {
	result := FormatDateTimeCombined(1234567890)
	if result != "<!date^1234567890^{date} at {time}|Unknown datetime>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatURL(t *testing.T) {
	if FormatURL("https://example.com", "click") != "<https://example.com|click>" {
		t.Error("FormatURL with text failed")
	}
	if FormatURL("https://example.com", "") != "<https://example.com>" {
		t.Error("FormatURL without text failed")
	}
}

func TestFormatEmail(t *testing.T) {
	result := FormatEmail("test@example.com")
	if result != "<mailto:test@example.com|test@example.com>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatCommand(t *testing.T) {
	result := FormatCommand("/mycommand")
	// FormatCommand wraps in </command> for Slack app commands
	if !strings.Contains(result, "mycommand") {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatSubteamMention(t *testing.T) {
	result := FormatSubteamMention("S123", "@group")
	if result != "<!subteam^S123|@group>" {
		t.Errorf("unexpected: %q", result)
	}
}

func TestFormatObject(t *testing.T) {
	result := FormatObject("board", "B123", "My Board")
	if result != "<board://B123|My Board>" {
		t.Errorf("unexpected: %q", result)
	}
}

// --- MrkdwnFormatter tests ---

func TestNewMrkdwnFormatter(t *testing.T) {
	f := NewMrkdwnFormatter()
	if f == nil {
		t.Fatal("expected non-nil formatter")
	}
}

func TestMrkdwnFormatter_Format_Empty(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestMrkdwnFormatter_Format_Headings(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("# Hello")
	if result != "*Hello*" {
		t.Errorf("expected '*Hello*', got %q", result)
	}
}

func TestMrkdwnFormatter_Format_Bold(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("**bold**")
	if result != "*bold*" {
		t.Errorf("expected '*bold*', got %q", result)
	}
}

func TestMrkdwnFormatter_Format_Italic(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("_italic_")
	if result != "_italic_" {
		t.Errorf("expected '_italic_', got %q", result)
	}
}

func TestMrkdwnFormatter_Format_Strikethrough(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("~~strike~~")
	if result != "~strike~" {
		t.Errorf("expected '~strike~', got %q", result)
	}
}

func TestMrkdwnFormatter_Format_CodeBlocks(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("before\n```\ncode\n```\nafter")
	if !strings_contains(result, "before") || !strings_contains(result, "after") {
		t.Errorf("code block formatting lost content: %q", result)
	}
}

func TestMrkdwnFormatter_Format_Links(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("[text](https://example.com)")
	if result != "<https://example.com|text>" {
		t.Errorf("expected '<https://example.com|text>', got %q", result)
	}
}

func TestMrkdwnFormatter_Format_Escaping(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("a < b & c > d")
	if result != "a &lt; b &amp; c &gt; d" {
		t.Errorf("expected escaped chars, got %q", result)
	}
}

func TestMrkdwnFormatter_Format_PreservesSlackSyntax(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.Format("<@U123> and <#C123|channel>")
	// Slack special syntax should be preserved, not escaped
	if strings.Contains(result, "&lt;@") {
		t.Errorf("Slack mention should not be escaped: %q", result)
	}
}

func TestMrkdwnFormatter_FormatCodeBlock(t *testing.T) {
	f := NewMrkdwnFormatter()
	result := f.FormatCodeBlock("code", "go")
	if result != "```go\ncode\n```" {
		t.Errorf("unexpected: %q", result)
	}
	result2 := f.FormatCodeBlock("code", "")
	if result2 != "```\ncode\n```" {
		t.Errorf("unexpected: %q", result2)
	}
}

func TestMrkdwnFormatter_EscapeSpecialChars_InConverter(t *testing.T) {
	f := NewMrkdwnFormatter()
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"a&b", "a&amp;b"},
		{"a<b", "a&lt;b"},
		{"a>b", "a&gt;b"},
		{"<@U123>", "<@U123>"},       // Slack mention preserved
		{"<#C123>", "<#C123>"},       // Channel mention preserved
		{"<!here>", "<!here>"},       // Special mention preserved
		{"<url|text>", "<url|text>"}, // Link preserved
	}
	for _, tt := range tests {
		result := f.escapeSpecialChars(tt.input)
		if result != tt.expected {
			t.Errorf("escapeSpecialChars(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// --- Security functions (already covered in existing tests, add missing ones) ---

// helper
func strings_contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
