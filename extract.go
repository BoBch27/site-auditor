package main

import "context"

// extractor defines the interface for extracting URLs from different sources
type extractor interface {
	extract(ctx context.Context) ([]string, error)
}

// newExtractors is a factory function to initialise different URL sources
func newExtractors(placesPrompt, searchPrompt, inputFile string) []extractor {
	googlePlacesSource := newGooglePlacesSource(placesPrompt)
	googleSearchSource := newGoogleSearchSource(searchPrompt)
	csvSource := newCSVSource(inputFile)

	return []extractor{googlePlacesSource, googleSearchSource, csvSource}
}

// extractWebsites collects websites from different sources
func extractWebsites(ctx context.Context, extractors []extractor) ([]*website, error) {
	type result struct {
		urls []string
		err  error
	}

	resultCh := make(chan result, len(extractors))

	// launch all extractors concurrently
	for _, ext := range extractors {
		go func(e extractor) {
			urls, err := e.extract(ctx)
			resultCh <- result{urls: urls, err: err}
		}(ext)
	}

	// collect results
	var allURLs []string
	for range len(extractors) {
		r := <-resultCh
		if r.err != nil {
			return nil, r.err // fail on first error
		}

		allURLs = append(allURLs, r.urls...)
	}

	return filterWebsites(allURLs), nil
}
