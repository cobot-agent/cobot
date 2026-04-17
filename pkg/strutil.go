package cobot

import "unicode/utf8"

// Truncate shortens s to maxLen bytes, appending "..." if truncated.
// It respects UTF-8 rune boundaries to avoid breaking multi-byte characters.
func Truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Find last valid rune boundary at or before maxLen to avoid breaking multi-byte UTF-8.
	for i := maxLen; i >= 0; i-- {
		if utf8.RuneStart(s[i]) {
			return s[:i] + "..."
		}
	}
	return "..."
}
