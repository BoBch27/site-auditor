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

// boolToEmoji takes in a boolean and returns corresponding
// emoji to visual inspection
func boolToEmoji(ok bool) string {
	if !ok {
		return "❌"
	}

	return "✅"
}
