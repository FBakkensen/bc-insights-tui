package util

import "strings"

// FirstNonEmpty returns v if it contains non-whitespace content; otherwise fallback.
func FirstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}
