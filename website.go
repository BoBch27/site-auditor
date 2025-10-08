package main

import (
	"fmt"
	"net/url"
	"strings"
)

// Website represents a website being audited
type Website struct {
	originalURL string
	scheme      string
	domain      string
}

// NewWebsite creates a new Website instance
func NewWebsite(rawURL string) (*Website, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %s: %w", rawURL, err)
	}

	if parsed.Host == "" {
		return nil, fmt.Errorf("URL missing host: %s", rawURL)
	}

	return &Website{
		domain:      strings.ToLower(parsed.Host),
		scheme:      parsed.Scheme,
		originalURL: rawURL,
	}, nil
}

// isIgnored reports whether the given website domain
// matches any of the ignored patterns to help avoid duplicates
func (w *Website) isIgnored(ignoredPatterns []string) bool {
	return isIgnoredResource(w.domain, ignoredPatterns)
}

// filterWebsites converts raw URLs to websites and
// filters out duplicates/ignored domains
func FilterWebsites(rawURLs []string) []*Website {
	websites := []*Website{}
	seen := map[string]bool{}

	for _, url := range rawURLs {
		if url == "" {
			continue
		}

		website, err := NewWebsite(url)
		if err != nil {
			fmt.Printf("⚠️ %v\n", err)
			continue
		}

		if seen[website.domain] || website.isIgnored(ignoredBusinessPatterns) {
			continue
		}

		seen[website.domain] = true
		websites = append(websites, website)
	}

	return websites
}

// patterns to ignore when filtering business websites
var ignoredBusinessPatterns = []string{
	"facebook.com", "instagram.com", "twitter.com", "linkedin.com",
	"booksy.com", "treatwell.co.uk", "fresha.com",
	"yelp.com", "yelp.co.uk", "yell.com", "tripadvisor.com",
	"boots.com", "superdrug.com", "directory",
	"google.com", "maps.google.com", "bizmapgo",
}
