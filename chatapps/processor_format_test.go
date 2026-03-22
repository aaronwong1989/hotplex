package chatapps

import (
	"context"
	"testing"

	"github.com/hrygo/hotplex/chatapps/base"
)

func TestFormatConversionProcessor_NoOp(t *testing.T) {
	p := NewFormatConversionProcessor(nil, FormatProcessorOptions{})

	msg := &base.ChatMessage{Content: "**bold** text"}
	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// NoOp converter should not change content
	if result.Content != "**bold** text" {
		t.Errorf("content should be unchanged with NoOp, got %q", result.Content)
	}
}

func TestFormatConversionProcessor_EmptyContent(t *testing.T) {
	p := NewFormatConversionProcessor(nil, FormatProcessorOptions{})

	msg := &base.ChatMessage{Content: ""}
	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "" {
		t.Errorf("expected empty content, got %q", result.Content)
	}
}

func TestFormatConversionProcessor_CustomConverter(t *testing.T) {
	converter := &mockConverter{result: "CONVERTED"}
	p := NewFormatConversionProcessor(nil, FormatProcessorOptions{Converter: converter})

	msg := &base.ChatMessage{
		Content:     "hello",
		RichContent: &base.RichContent{ParseMode: base.ParseModeMarkdown},
	}

	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "CONVERTED" {
		t.Errorf("expected 'CONVERTED', got %q", result.Content)
	}
}

func TestFormatConversionProcessor_MetadataParseMode(t *testing.T) {
	converter := &mockConverter{result: "HTML"}
	p := NewFormatConversionProcessor(nil, FormatProcessorOptions{Converter: converter})

	msg := &base.ChatMessage{
		Content:  "<b>hello</b>",
		Metadata: map[string]any{"parse_mode": "HTML"},
	}

	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "HTML" {
		t.Errorf("expected 'HTML', got %q", result.Content)
	}
}

func TestFormatConversionProcessor_MetadataParseModeCaseInsensitive(t *testing.T) {
	converter := &mockConverter{result: "MD"}
	p := NewFormatConversionProcessor(nil, FormatProcessorOptions{Converter: converter})

	msg := &base.ChatMessage{
		Content:  "hello",
		Metadata: map[string]any{"parse_mode": "MarkDown"},
	}

	result, err := p.Process(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Content != "MD" {
		t.Errorf("expected 'MD', got %q", result.Content)
	}
}

func TestFormatConversionProcessor_Name(t *testing.T) {
	p := NewFormatConversionProcessor(nil, FormatProcessorOptions{})
	if p.Name() != "FormatConversionProcessor" {
		t.Errorf("expected 'FormatConversionProcessor', got %q", p.Name())
	}
}

func TestFormatConversionProcessor_Order(t *testing.T) {
	p := NewFormatConversionProcessor(nil, FormatProcessorOptions{})
	if p.Order() <= 0 {
		t.Errorf("expected positive order, got %d", p.Order())
	}
}

// mockConverter implements base.ContentConverter for testing
type mockConverter struct {
	result string
}

func (m *mockConverter) ConvertMarkdownToPlatform(_ string, _ base.ParseMode) string {
	return m.result
}

func (m *mockConverter) EscapeSpecialChars(text string) string {
	return text
}
