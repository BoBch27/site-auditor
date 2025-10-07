package main

import "context"

// extractor defines the interface for extracting URLs from different sources
type extractor interface {
	extract(ctx context.Context) ([]string, error)
}

// extractWebsites collects websites based on input method
func extractWebsites(ctx context.Context, placesPrompt, searchPrompt, inputFile string) ([]*website, error) {
	var urls []string

	// search for URLs from Google Places
	placesSource := newGooglePlacesSource(placesPrompt)
	placesURLs, err := placesSource.extract(ctx)
	if err != nil {
		return nil, err
	}
	urls = append(urls, placesURLs...)

	// scrape URLs from Google Search
	searchSource := newGoogleSearchSource(searchPrompt)
	scrapedURLs, err := searchSource.extract(ctx)
	if err != nil {
		return nil, err
	}
	urls = append(urls, scrapedURLs...)

	// extract URLs from CSV
	csvSource := newCSVSource(inputFile)
	readURLs, err := csvSource.extract(ctx)
	if err != nil {
		return nil, err
	}
	urls = append(urls, readURLs...)

	return filterWebsites(urls), nil
}
