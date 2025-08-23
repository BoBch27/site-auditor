package main

import (
	"strings"
)

// isIgnoredResource reports whether the given resource (URL or domain)
// matches any of the ignored patterns to help avoid duplicates, or waiting on
// analytics, tracking, or other non-critical requests
func isIgnoredResource(resource string, ignoredPatterns []string) bool {
	// (web workers, service workers, generated content, etc.)
	if strings.HasPrefix(resource, "blob:") {
		return true
	}

	// (inline content)
	if strings.HasPrefix(resource, "data:") {
		return true
	}

	for _, pattern := range ignoredPatterns {
		if strings.Contains(resource, pattern) {
			return true
		}
	}

	return false
}
