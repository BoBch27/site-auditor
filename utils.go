package main

import (
	"fmt"
	"net/url"
	"strings"
)

// extractDomain takes in a URL and returns the domain name,
// optionally including the scheme
func extractDomain(fullUrl string, withScheme ...bool) (string, error) {
	u, err := url.Parse(fullUrl)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	if u.Host == "" {
		return "", fmt.Errorf("invalid URL: %s", fullUrl)
	}

	domain := strings.ToLower(u.Host)

	// optionally add scheme
	if len(withScheme) > 0 && withScheme[0] {
		return u.Scheme + "://" + domain + "/", nil
	}

	return domain, nil
}
