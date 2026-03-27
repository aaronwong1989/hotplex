// Package slack provides the Slack adapter implementation for the hotplex engine.
package slack

import (
	"strings"

	"github.com/hrygo/hotplex/chatapps/base"
)

// SlackChunker implements base.Chunker using Slack's Markdown-aware chunking.
type SlackChunker struct{}

// NewSlackChunker creates a Slack-specific text chunker.
func NewSlackChunker() *SlackChunker {
	return &SlackChunker{}
}

// ChunkText uses Slack's code-block-aware Markdown chunking.
func (c *SlackChunker) ChunkText(text string, limit int) []string {
	return ChunkMessageMarkdown(text, limit)
}

// MaxChars returns Slack's character limit.
func (c *SlackChunker) MaxChars() int { return SlackTextLimit }

// ContentConverter converts message content to Slack mrkdwn format.
type ContentConverter struct{}

// NewContentConverter creates a new Slack content converter.
func NewContentConverter() *ContentConverter {
	return &ContentConverter{}
}

// ConvertMarkdownToPlatform converts Markdown text to Slack mrkdwn format.
func (c *ContentConverter) ConvertMarkdownToPlatform(content string, parseMode base.ParseMode) string {
	if parseMode != base.ParseModeMarkdown {
		return content
	}
	return convertMarkdownToSlackMrkdwn(content)
}

// EscapeSpecialChars escapes Slack-specific special characters.
func (c *ContentConverter) EscapeSpecialChars(text string) string {
	return escapeSlackChars(text)
}

// convertMarkdownToSlackMrkdwn converts Markdown to Slack's mrkdwn format.
// It preserves code blocks (```...```) and inline code (`...`) without conversion.
func convertMarkdownToSlackMrkdwn(text string) string {
	segments := splitPreservingCodeBlocks(text)

	var result strings.Builder
	result.Grow(len(text) * 2)

	for _, segment := range segments {
		if segment.isCodeBlock {
			result.WriteString(escapeSlackChars(segment.text))
		} else {
			converted := segment.text
			// Convert bold (**text**) before italic (*text*) to prevent the italic converter
			// from matching the first asterisk of **text** as an italic marker.
			// Order matters: bold creates *inner* with markers, then italic skips markers.
			converted = convertBold(converted)
			converted = convertItalic(converted)
			// Remove bold markers after italic conversion
			converted = strings.ReplaceAll(converted, "\x00", "")
			converted = convertLinks(converted)
			converted = convertGitHubEmojiToUnicode(converted)
			converted = escapeSlackChars(converted)
			result.WriteString(converted)
		}
	}

	return result.String()
}

// convertGitHubEmojiToUnicode converts GitHub-style :emoji: to Unicode characters
// Slack uses Unicode emoji, not :emoji: syntax
func convertGitHubEmojiToUnicode(text string) string {
	// Common GitHub emoji mappings
	emojiMap := map[string]string{
		":bar_chart:":                  "📊",
		":warning:":                    "⚠️",
		":white_check_mark:":           "✅",
		":x:":                          "❌",
		":arrow_forward:":              "▶️",
		":arrow_backward:":             "◀️",
		":arrow_up:":                   "⬆️",
		":arrow_down:":                 "⬇️",
		":checkered_flag:":             "🏁",
		":construction:":               "🚧",
		":rocket:":                     "🚀",
		":bug:":                        "🐛",
		":sparkles:":                   "✨",
		":memo:":                       "📝",
		":book:":                       "📖",
		":bulb:":                       "💡",
		":fire:":                       "🔥",
		":heart:":                      "❤️",
		":thumbsup:":                   "👍",
		":thumbsdown:":                 "👎",
		":laughing:":                   "😆",
		":tada:":                       "🎉",
		":heavy_check_mark:":           "✔️",
		":heavy_multiplication_x:":     "✖️",
		":information_source:":         "ℹ️",
		":lock:":                       "🔒",
		":unlock:":                     "🔓",
		":star:":                       "⭐",
		":zap:":                        "⚡",
		":boom:":                       "💥",
		":wrench:":                     "🔧",
		":hammer:":                     "🔨",
		":gear:":                       "⚙️",
		":mag:":                        "🔍",
		":chart_with_upwards_trend:":   "📈",
		":chart_with_downwards_trend:": "📉",
		":stop_sign:":                  "🛑",
		":traffic_light:":              "🚦",
		":hourglass:":                  "⏳",
		":clock1:":                     "🕐",
		":calendar:":                   "📅",
		":package:":                    "📦",
		":computer:":                   "💻",
		":file_folder:":                "📁",
		":page_facing_up:":             "📄",
		":link:":                       "🔗",
		":email:":                      "📧",
		":phone:":                      "📞",
		":speech_balloon:":             "💬",
		":thought_balloon:":            "💭",
		":bell:":                       "🔔",
		":pushpin:":                    "📌",
		":round_pushpin:":              "📍",
		":paperclip:":                  "📎",
		":pencil2:":                    "✏️",
		":crayon:":                     "🖍️",
		":art:":                        "🎨",
		":musical_score:":              "🎼",
		":guitar:":                     "🎸",
		":camera:":                     "📷",
		":video_camera:":               "📹",
		":movie_camera:":               "🎥",
		":microphone:":                 "🎤",
		":headphones:":                 "🎧",
		":radio:":                      "📻",
		":globe_with_meridians:":       "🌐",
		":map:":                        "🗺️",
		":compass:":                    "🧭",
		":mountain:":                   "⛰️",
		":beach:":                      "🏖️",
		":desert:":                     "🏜️",
		":island:":                     "🏝️",
		":cityscape:":                  "🏙️",
		":night_with_stars:":           "🌃",
		":sunrise:":                    "🌅",
		":sunset:":                     "🌇",
		":bridge_at_night:":            "🌉",
		":ferris_wheel:":               "🎡",
		":roller_coaster:":             "🎢",
		":fountain:":                   "⛲",
		":circus_tent:":                "🎪",
		":steam_locomotive:":           "🚂",
		":train:":                      "🚋",
		":bullettrain_side:":           "🚄",
		":bullettrain_front:":          "🚅",
		":train2:":                     "🚆",
		":metro:":                      "🚇",
		":light_rail:":                 "🚈",
		":station:":                    "🚉",
		":tram:":                       "🚊",
		":monorail:":                   "🚝",
		":mountain_railway:":           "🚞",
		":bus:":                        "🚌",
		":oncoming_bus:":               "🚍",
		":trolleybus:":                 "🚎",
		":minibus:":                    "🚐",
		":ambulance:":                  "🚑",
		":fire_engine:":                "🚒",
		":police_car:":                 "🚓",
		":oncoming_police_car:":        "🚔",
		":taxi:":                       "🚕",
		":oncoming_taxi:":              "🚖",
		":car:":                        "🚗",
		":oncoming_automobile:":        "🚘",
		":blue_car:":                   "🚙",
		":truck:":                      "🚚",
		":articulated_lorry:":          "🚛",
		":tractor:":                    "🚜",
		":racing_car:":                 "🏎️",
		":motorcycle:":                 "🏍️",
		":motor_scooter:":              "🛵",
		":bike:":                       "🚲",
		":kick_scooter:":               "🛴",
		":busstop:":                    "🚏",
		":motorway:":                   "🛣️",
		":railway_track:":              "🛤️",
		":oil_drum:":                   "🛢️",
		":fuelpump:":                   "⛽",
		":rotating_light:":             "🚨",
		":vertical_traffic_light:":     "🚦",
		":octagonal_sign:":             "🛑",
		":anchor:":                     "⚓",
		":boat:":                       "⛵",
		":canoe:":                      "🛶",
		":speedboat:":                  "🚤",
		":passenger_ship:":             "🛳️",
		":ferry:":                      "⛴️",
		":motor_boat:":                 "🛥️",
		":ship:":                       "🚢",
		":airplane:":                   "✈️",
		":small_airplane:":             "🛩️",
		":flight_departure:":           "🛫",
		":flight_arrival:":             "🛬",
		":seat:":                       "💺",
		":helicopter:":                 "🚁",
		":suspension_railway:":         "🚟",
		":mountain_cableway:":          "🚠",
		":aerial_tramway:":             "🚡",
		":artificial_satellite:":       "🛰️",
		":flying_saucer:":              "🛸",
	}

	result := text
	for github, unicode := range emojiMap {
		result = strings.ReplaceAll(result, github, unicode)
	}
	return result
}

// textSegment represents a portion of text with code block status
type textSegment struct {
	text        string
	isCodeBlock bool
}

// splitPreservingCodeBlocks splits text into code blocks and non-code segments
func splitPreservingCodeBlocks(text string) []textSegment {
	var segments []textSegment
	remaining := text

	for {
		codeStart := strings.Index(remaining, "```")
		if codeStart == -1 {
			if len(remaining) > 0 {
				segments = append(segments, textSegment{text: remaining, isCodeBlock: false})
			}
			break
		}

		if codeStart > 0 {
			segments = append(segments, textSegment{text: remaining[:codeStart], isCodeBlock: false})
		}

		afterStart := remaining[codeStart+3:]
		codeEnd := strings.Index(afterStart, "```")
		if codeEnd == -1 {
			segments = append(segments, textSegment{text: remaining[codeStart:], isCodeBlock: true})
			break
		}

		codeBlock := remaining[codeStart : codeStart+3+codeEnd+3]
		segments = append(segments, textSegment{text: codeBlock, isCodeBlock: true})
		remaining = remaining[codeStart+3+codeEnd+3:]
	}

	return segments
}

// escapeSlackChars escapes special characters for Slack
func escapeSlackChars(text string) string {
	result := strings.Builder{}
	result.Grow(len(text))

	for _, r := range text {
		switch r {
		case '&':
			result.WriteString("&amp;")
		case '<':
			result.WriteString("&lt;")
		case '>':
			result.WriteString("&gt;")
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// convertBold converts **text** to *text*
func convertBold(text string) string {
	result := text
	for strings.Contains(result, "**") {
		start := strings.Index(result, "**")
		if start == -1 {
			break
		}
		end := strings.Index(result[start+2:], "**")
		if end == -1 {
			break
		}
		end += start + 2
		inner := result[start+2 : end]
		// Use \x01BOLD\x01 marker to prevent italic conversion, will be replaced with * later
		result = result[:start] + "\x01BOLD\x01" + inner + "\x01BOLD\x01" + result[end+2:]
	}
	return result
}

// convertItalic converts *text* to _text_ (but not ** or *** or marked bold)
func convertItalic(text string) string {
	result := text
	for {
		start := -1
		for i := 0; i < len(result)-1; i++ {
			// Skip if this is part of \x01BOLD\x01 marker
			if result[i] == '\x01' {
				// Skip to end of marker
				for j := i + 1; j < len(result); j++ {
					if result[j] == '\x01' {
						i = j
						break
					}
				}
				continue
			}
			if result[i] == '*' && result[i+1] != '*' {
				if i > 0 && result[i-1] == '*' {
					continue
				}
				start = i
				break
			}
		}
		if start == -1 {
			break
		}

		end := -1
		for i := start + 1; i < len(result); i++ {
			// Skip if this is part of \x01BOLD\x01 marker
			if result[i] == '\x01' {
				// Skip to end of marker
				for j := i + 1; j < len(result); j++ {
					if result[j] == '\x01' {
						i = j
						break
					}
				}
				continue
			}
			if i < len(result)-1 && result[i] == '*' && result[i+1] != '*' {
				end = i
				break
			}
			if i == len(result)-1 && result[i] == '*' {
				end = i
				break
			}
		}
		if end == -1 {
			break
		}

		inner := result[start+1 : end]
		result = result[:start] + "_" + inner + "_" + result[end+1:]
	}

	// Replace bold markers with * after italic conversion
	result = strings.ReplaceAll(result, "\x01BOLD\x01", "*")
	return result
}

// convertLinks converts [text](url) to <url|text>
func convertLinks(text string) string {
	result := text
	for strings.Contains(result, "[") {
		textStart := strings.Index(result, "[")
		if textStart == -1 {
			break
		}
		textEnd := strings.Index(result[textStart:], "]")
		if textEnd == -1 {
			break
		}
		textEnd += textStart

		urlStart := strings.Index(result[textEnd:], "(")
		if urlStart == -1 {
			break
		}
		urlStart += textEnd

		urlEnd := strings.Index(result[urlStart:], ")")
		if urlEnd == -1 {
			break
		}
		urlEnd += urlStart

		linkText := result[textStart+1 : textEnd]
		linkURL := result[urlStart+1 : urlEnd]

		replacement := "<" + linkURL + "|" + linkText + ">"
		result = result[:textStart] + replacement + result[urlEnd+1:]
	}
	return result
}
