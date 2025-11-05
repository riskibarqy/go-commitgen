package util

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var whitespace = regexp.MustCompile(`\s+`)

// CondenseSpaces collapses runs of whitespace to single spaces.
func CondenseSpaces(s string) string {
	return whitespace.ReplaceAllString(s, " ")
}

// TruncateShorten trims a string to at most n characters, adding an ellipsis if trimmed.
func TruncateShorten(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n-1]) + "…"
}

// TrimLines splits and removes empty lines, returning a cleaned slice.
func TrimLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	return cleaned
}

// TrimTo limits a string to max bytes, attempting to cut on line boundaries.
func TrimTo(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	head := s[:max]
	if idx := strings.LastIndex(head, "\n"); idx > 0 {
		head = head[:idx]
	}
	return head + "\n…[diff truncated]"
}
