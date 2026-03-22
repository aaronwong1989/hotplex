package base

import (
	"testing"
)

func TestExtractStringFromMetadata_Nil(t *testing.T) {
	if ExtractStringFromMetadata(nil, "key") != "" {
		t.Error("expected empty for nil message")
	}
}

func TestExtractStringFromMetadata_NilMetadata(t *testing.T) {
	if ExtractStringFromMetadata(&ChatMessage{}, "key") != "" {
		t.Error("expected empty for nil metadata")
	}
}

func TestExtractStringFromMetadata_String(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"chat_id": "C123"}}
	if ExtractStringFromMetadata(msg, "chat_id") != "C123" {
		t.Error("expected C123")
	}
}

func TestExtractStringFromMetadata_NonString(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"chat_id": 123}}
	if ExtractStringFromMetadata(msg, "chat_id") != "" {
		t.Error("expected empty for non-string value")
	}
}

func TestExtractStringFromMetadata_MissingKey(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"other": "value"}}
	if ExtractStringFromMetadata(msg, "chat_id") != "" {
		t.Error("expected empty for missing key")
	}
}

func TestExtractInt64FromMetadata_Nil(t *testing.T) {
	if ExtractInt64FromMetadata(nil, "key") != 0 {
		t.Error("expected 0 for nil message")
	}
}

func TestExtractInt64FromMetadata_Int64(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"count": int64(42)}}
	if ExtractInt64FromMetadata(msg, "count") != 42 {
		t.Error("expected 42")
	}
}

func TestExtractInt64FromMetadata_Float64(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"count": float64(42.5)}}
	if ExtractInt64FromMetadata(msg, "count") != 42 {
		t.Error("expected 42 from float64")
	}
}

func TestExtractInt64FromMetadata_Int(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"count": 42}}
	if ExtractInt64FromMetadata(msg, "count") != 42 {
		t.Error("expected 42 from int")
	}
}

func TestExtractInt64FromMetadata_NonNumeric(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"count": "not a number"}}
	if ExtractInt64FromMetadata(msg, "count") != 0 {
		t.Error("expected 0 for non-numeric value")
	}
}

func TestExtractInt64FromMetadata_MissingKey(t *testing.T) {
	msg := &ChatMessage{Metadata: map[string]any{"other": 42}}
	if ExtractInt64FromMetadata(msg, "count") != 0 {
		t.Error("expected 0 for missing key")
	}
}
