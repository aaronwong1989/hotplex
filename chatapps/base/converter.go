package base

// ContentConverter 将 Markdown 转换为平台特定格式
type ContentConverter interface {
	// ConvertMarkdownToPlatform 将 Markdown 文本转换为平台原生格式
	// parseMode 为 None 时返回原文
	ConvertMarkdownToPlatform(content string, parseMode ParseMode) string

	// EscapeSpecialChars 转义平台特殊字符
	EscapeSpecialChars(text string) string
}

// NoOpConverter is a content converter that does no conversion.
type NoOpConverter struct{}

// NewNoOpConverter creates a converter that passes text through unchanged.
func NewNoOpConverter() *NoOpConverter {
	return &NoOpConverter{}
}

// ConvertMarkdownToPlatform returns content unchanged.
func (NoOpConverter) ConvertMarkdownToPlatform(content string, _ ParseMode) string {
	return content
}

// EscapeSpecialChars returns text unchanged.
func (NoOpConverter) EscapeSpecialChars(text string) string {
	return text
}
