package main

import (
	"fmt"
	"net/url"
	"strings"
)

type website struct {
	originalURL string
	scheme      string
	domain      string
}

// newWebsite takes in a raw URL, parses it and returns a website
// instance
func newWebsite(rawURL string) (*website, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %s: %w", rawURL, err)
	}

	if parsed.Host == "" {
		return nil, fmt.Errorf("URL missing host: %s", rawURL)
	}

	return &website{
		domain:      strings.ToLower(parsed.Host),
		scheme:      parsed.Scheme,
		originalURL: rawURL,
	}, nil
}

// isIgnored reports whether the given website domain
// matches any of the ignored patterns to help avoid duplicates
func (w *website) isIgnored(ignoredPatterns []string) bool {
	return isIgnoredResource(w.domain, ignoredPatterns)
}
