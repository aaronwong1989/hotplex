package chatapps

import (
	"context"
	"log/slog"
	"strings"

	"github.com/hrygo/hotplex/chatapps/base"
)

// FormatConversionProcessor converts message content to platform-specific formats.
// It delegates platform-specific conversion to an injected ContentConverter.
type FormatConversionProcessor struct {
	logger    *slog.Logger
	converter base.ContentConverter
}

// FormatProcessorOptions configures the FormatConversionProcessor.
type FormatProcessorOptions struct {
	Converter base.ContentConverter
}

// NewFormatConversionProcessor creates a new FormatConversionProcessor.
// If no converter is provided, a no-op converter is used.
func NewFormatConversionProcessor(logger *slog.Logger, opts FormatProcessorOptions) *FormatConversionProcessor {
	if logger == nil {
		logger = slog.Default()
	}
	converter := opts.Converter
	if converter == nil {
		converter = base.NewNoOpConverter()
	}
	return &FormatConversionProcessor{
		logger:    logger,
		converter: converter,
	}
}

// Name returns the processor name
func (p *FormatConversionProcessor) Name() string {
	return "FormatConversionProcessor"
}

// Order returns the processor order
func (p *FormatConversionProcessor) Order() int {
	return int(OrderFormatConversion)
}

// Process converts message content based on parse mode using the injected converter.
func (p *FormatConversionProcessor) Process(ctx context.Context, msg *base.ChatMessage) (*base.ChatMessage, error) {
	if msg.Content == "" {
		return msg, nil
	}

	// Determine parse mode
	parseMode := base.ParseModeNone
	if msg.RichContent != nil {
		parseMode = msg.RichContent.ParseMode
	}

	// If no parse mode specified, check metadata for hints
	if parseMode == base.ParseModeNone {
		if mode, ok := msg.Metadata["parse_mode"].(string); ok {
			switch strings.ToLower(mode) {
			case "markdown":
				parseMode = base.ParseModeMarkdown
			case "html":
				parseMode = base.ParseModeHTML
			}
		}
	}

	// Delegate platform-specific conversion to the injected converter
	if parseMode != base.ParseModeNone {
		originalLen := len(msg.Content)
		msg.Content = p.converter.ConvertMarkdownToPlatform(msg.Content, parseMode)
		if len(msg.Content) != originalLen {
			p.logger.Debug("Converted content",
				"session_id", msg.SessionID,
				"parse_mode", parseMode,
				"original_len", originalLen,
				"converted_len", len(msg.Content))
		}
	}

	return msg, nil
}

// Verify FormatConversionProcessor implements MessageProcessor at compile time
var _ MessageProcessor = (*FormatConversionProcessor)(nil)
