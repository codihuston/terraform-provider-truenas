package services

import "strings"

// isNotFoundError reports whether an API error indicates a missing resource.
// TrueNAS returns "[ENOENT]" errors whose message ends in "does not exist".
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "[ENOENT]")
}
