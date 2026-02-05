package util

import (
	"regexp"
	"strings"
)

var invalidCharsRe = regexp.MustCompile(`[\\/:*?"<>|]`)

// SanitizeFileName replaces invalid filename characters with dashes.
// Returns "Untitled" if the result is empty after trimming.
func SanitizeFileName(name string) string {
	result := invalidCharsRe.ReplaceAllString(name, "-")
	result = strings.TrimSpace(result)
	if result == "" {
		return "Untitled"
	}
	return result
}

// JoinPath joins path segments with forward slashes.
// Core uses forward slashes internally; OS-specific resolution happens at the adapter level.
func JoinPath(parts ...string) string {
	return strings.Join(parts, "/")
}
