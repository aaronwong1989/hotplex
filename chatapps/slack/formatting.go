package slack

import "fmt"

// =============================================================================
// Slack Special Syntax Formatters
// =============================================================================

// FormatChannelMention creates a channel mention: <#C123|channel-name>
func FormatChannelMention(channelID, channelName string) string {
	return fmt.Sprintf("<#%s|%s>", channelID, channelName)
}

// FormatChannelMentionByID creates a channel mention with just ID: <#C123>
func FormatChannelMentionByID(channelID string) string {
	return fmt.Sprintf("<#%s>", channelID)
}

// FormatUserMention creates a user mention: <@U123|username>
func FormatUserMention(userID, userName string) string {
	return fmt.Sprintf("<@%s|%s>", userID, userName)
}

// FormatUserMentionByID creates a user mention with just ID: <@U123>
func FormatUserMentionByID(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}

// FormatSpecialMention creates a special mention: <!here>, <!channel>, <!everyone>
func FormatSpecialMention(mentionType string) string {
	// mentionType: "here", "channel", "everyone"
	return fmt.Sprintf("<!%s>", mentionType)
}

// FormatHereMention creates a @here mention
func FormatHereMention() string {
	return "<!here>"
}

// FormatChannelMention creates a @channel mention
func FormatChannelAllMention() string {
	return "<!channel>"
}

// FormatEveryoneMention creates a @everyone mention
func FormatEveryoneMention() string {
	return "<!everyone>"
}

// FormatDateTime creates a date formatting: <!date^timestamp^format|fallback>
// Reference: https://api.slack.com/reference/surfaces/formatting#date-formatting
func FormatDateTime(timestamp int64, format, fallback string) string {
	return fmt.Sprintf("<!date^%d^%s|%s>", timestamp, format, fallback)
}

// FormatDateTimeWithLink creates a date formatting with link: <!date^timestamp^format^link|fallback>
func FormatDateTimeWithLink(timestamp int64, format, linkURL, fallback string) string {
	return fmt.Sprintf("<!date^%d^%s^%s|%s>", timestamp, format, linkURL, fallback)
}

// FormatDate creates a simple date formatting
func FormatDate(timestamp int64) string {
	return FormatDateTime(timestamp, "{date}", "Unknown date")
}

// FormatDateShort creates a short date formatting (e.g., "Jan 1, 2024")
func FormatDateShort(timestamp int64) string {
	return FormatDateTime(timestamp, "{date_short}", "Unknown date")
}

// FormatDateLong creates a long date formatting (e.g., "Monday, January 1, 2024")
func FormatDateLong(timestamp int64) string {
	return FormatDateTime(timestamp, "{date_long}", "Unknown date")
}

// FormatTime creates a time formatting (e.g., "2:30 PM")
func FormatTime(timestamp int64) string {
	return FormatDateTime(timestamp, "{time}", "Unknown time")
}

// FormatDateTimeCombined creates combined date and time formatting
func FormatDateTimeCombined(timestamp int64) string {
	return FormatDateTime(timestamp, "{date} at {time}", "Unknown datetime")
}

// FormatURL creates a link: <url|text> or <url>
func FormatURL(url, text string) string {
	if text == "" {
		return fmt.Sprintf("<%s>", url)
	}
	return fmt.Sprintf("<%s|%s>", url, text)
}

// FormatEmail creates an email link
func FormatEmail(email string) string {
	return fmt.Sprintf("<mailto:%s|%s>", email, email)
}

// FormatCommand creates a command formatting
func FormatCommand(command string) string {
	return fmt.Sprintf("</%s>", command)
}

// FormatSubteamMention creates a user group mention: <!subteam^S123|@group>
func FormatSubteamMention(subteamID, subteamHandle string) string {
	return fmt.Sprintf("<!subteam^%s|%s>", subteamID, subteamHandle)
}

// FormatObject creates an object mention (for boards, clips, etc.)
func FormatObject(objectType, objectID, objectText string) string {
	return fmt.Sprintf("<%s://%s|%s>", objectType, objectID, objectText)
}
