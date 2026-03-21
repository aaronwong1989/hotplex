package base

// Platform 元数据 Key 定义（统一命名）
// Adapters should use these constants when setting/getting metadata to ensure
// platform-agnostic code in engine_handler.go and processors.
//
// Adapter-specific keys are still stored directly (e.g., Slack uses "thread_ts"
// internally, but should be accessible via KeyThreadID when crossing adapter boundary).
const (
	// KeyRoomID is the unified key for the room/channel/conversation identifier.
	// Slack: channel_id | Feishu: chat_id | Telegram: chat_id | Discord: channel_id
	KeyRoomID = "room_id"
	// KeyThreadID is the unified key for thread/reply identifier.
	// Slack: thread_ts | Feishu: message_id | Telegram: message_thread_id | Discord: thread_id
	KeyThreadID = "thread_id"
	// KeyUserID is the unified key for the user who sent the message.
	// Universal across all platforms.
	KeyUserID = "user_id"
	// KeyBotUserID is the unified key for the bot's own user identifier.
	KeyBotUserID = "bot_user_id"
	// KeyPlatform is the platform identifier (slack, feishu, telegram, etc.)
	KeyPlatform = "platform"
)

// GetMetadata extracts platform-agnostic values from metadata map.
// Returns zero values if keys are not present.
func GetMetadata(m map[string]any) (roomID, threadID, userID string) {
	if m == nil {
		return
	}
	roomID, _ = m[KeyRoomID].(string)
	threadID, _ = m[KeyThreadID].(string)
	userID, _ = m[KeyUserID].(string)
	return
}

// SetMetadata sets platform-agnostic values in the metadata map.
// Initializes the map if nil.
func SetMetadata(m map[string]any, roomID, threadID, userID string) map[string]any {
	if m == nil {
		m = make(map[string]any)
	}
	if roomID != "" {
		m[KeyRoomID] = roomID
	}
	if threadID != "" {
		m[KeyThreadID] = threadID
	}
	if userID != "" {
		m[KeyUserID] = userID
	}
	return m
}

// GetMetadataString extracts a string value from metadata safely.
func GetMetadataString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// SlackMetadata provides backward-compatible access to Slack-specific metadata keys.
// Use GetMetadata for platform-agnostic access.
func SlackMetadata(m map[string]any) (channelID, threadTS, userID string) {
	// Unified keys (preferred)
	channelID = GetMetadataString(m, KeyRoomID)
	threadTS = GetMetadataString(m, KeyThreadID)
	userID = GetMetadataString(m, KeyUserID)
	// Fallback: Slack-specific keys (for existing adapters that haven't migrated)
	if channelID == "" {
		channelID = GetMetadataString(m, "channel_id")
	}
	if threadTS == "" {
		threadTS = GetMetadataString(m, "thread_ts")
	}
	if userID == "" {
		userID = GetMetadataString(m, "user_id")
	}
	return
}

// MergeMetadata merges the unified metadata keys from src into dst.
// Unifies channel_id → KeyRoomID, thread_ts → KeyThreadID.
// Intentionally skips message_ts as it refers to the user's message, not the bot's.
func MergeMetadata(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any)
	}
	if src == nil {
		return dst
	}

	// Set unified keys (preferred)
	if v := GetMetadataString(src, KeyRoomID); v != "" {
		dst[KeyRoomID] = v
	}
	if v := GetMetadataString(src, KeyThreadID); v != "" {
		dst[KeyThreadID] = v
	}
	if v := GetMetadataString(src, KeyUserID); v != "" {
		dst[KeyUserID] = v
	}

	// Also migrate Slack-specific keys for backward compatibility
	if v := GetMetadataString(src, "channel_id"); v != "" && dst[KeyRoomID] == nil {
		dst["channel_id"] = v // Keep for adapters that still read it
	}
	if v := GetMetadataString(src, "thread_ts"); v != "" && dst[KeyThreadID] == nil {
		dst["thread_ts"] = v // Keep for adapters that still read it
	}

	// Skip "message_ts" intentionally — it refers to the user's message,
	// not the bot's. Copying it causes platform API errors (cant_update_message).

	return dst
}
