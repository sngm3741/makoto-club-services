package common

import (
	"strconv"
	"strings"
)

// ParsePositiveInt parses positive integers with fallback.
func ParsePositiveInt(value string, fallback int) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, false
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback, false
	}
	return parsed, true
}

// IntPtr returns pointer helper for ints.
func IntPtr(v int) *int {
	return &v
}

// IntPtrValue returns value or zero.
func IntPtrValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
