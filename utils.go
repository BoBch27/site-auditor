package main

import (
	"fmt"
	"net/url"
	"strings"
)

// extractUrlParts takes in a URL and returns the scheme (http/https),
// and domain name (host)
func extractUrlParts(fullUrl string) (scheme string, domain string, err error) {
	u, err := url.Parse(fullUrl)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse URL: %w", err)
	}

	if u.Host == "" {
		return "", "", fmt.Errorf("invalid URL: %s", fullUrl)
	}

	return u.Scheme, strings.ToLower(u.Host), nil
}

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

// boolToEmoji takes in a boolean and returns corresponding
// emoji to visual inspection
func boolToEmoji(ok bool) string {
	if !ok {
		return "❌"
	}

	return "✅"
}
