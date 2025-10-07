package main

import "context"

// extractor defines the interface for extracting URLs from different sources
type extractor interface {
	extract(ctx context.Context) ([]string, error)
}

// extractWebsites collects websites based on input method
func extractWebsites(ctx context.Context, searchPrompt, scrapePrompt, inputFile string) ([]*website, error) {
	var urls []string

	// search for URLs from Google Places
	placesURLs, err := searchURLsFromGooglePlaces(ctx, searchPrompt)
	if err != nil {
		return nil, err
	}
	urls = append(urls, placesURLs...)

	// scrape URLs from Google Search
	scrapedURLs, err := scrapeURLsFromGoogleSearch(scrapePrompt)
	if err != nil {
		return nil, err
	}
	urls = append(urls, scrapedURLs...)

	// extract URLs from CSV
	readURLs, err := readURLsFromCSV(inputFile)
	if err != nil {
		return nil, err
	}
	urls = append(urls, readURLs...)

	return filterWebsites(urls), nil
}
