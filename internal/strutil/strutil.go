package strutil

import "unicode/utf8"

// MapKeys extracts the keys from a map for logging without leaking content.
func MapKeys[T comparable](m map[T]any) []T {
	keys := make([]T, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Truncate strings at rune level to avoid invalid UTF-8.
// If maxLen < 4, returns a raw byte slice without appending "..." to avoid panic.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}

	if utf8.ValidString(s) {
		runes := []rune(s)
		if len(runes) > maxLen {
			return string(runes[:maxLen-3]) + "..."
		}
		return s
	}

	// Fallback to byte truncation if not valid UTF-8
	return s[:maxLen-3] + "..."
}
