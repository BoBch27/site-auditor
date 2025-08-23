package main

import (
	"context"
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

// extractWebsites collects websites based on input method
func extractWebsites(ctx context.Context, searchPrompt, scrapePrompt, inputFile string) ([]*website, error) {
	var urls []string

	// search for URLs from Google Places
	if searchPrompt != "" {
		placesURLs, err := searchURLsFromGooglePlaces(ctx, searchPrompt)
		if err != nil {
			return nil, err
		}

		urls = append(urls, placesURLs...)
	}

	// scrape URLs from Google Search
	if scrapePrompt != "" {
		scrapedURLs, err := scrapeURLsFromGoogleSearch(scrapePrompt)
		if err != nil {
			return nil, err
		}

		urls = append(urls, scrapedURLs...)
	}

	// extract URLs from CSV
	if inputFile != "" {
		readURLs, err := readURLsFromCSV(inputFile)
		if err != nil {
			return nil, err
		}

		urls = append(urls, readURLs...)
	}

	return filterWebsites(urls), nil
}

// filterWebsites converts raw URLs to websites and
// filters out duplicates/ignored domains
func filterWebsites(rawURLs []string) []*website {
	websites := []*website{}
	seen := map[string]bool{}

	for _, url := range rawURLs {
		if url == "" {
			continue
		}

		website, err := newWebsite(url)
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
