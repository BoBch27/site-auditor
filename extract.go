package main

import "context"

// extractor defines the interface for extracting URLs from different sources
type extractor interface {
	extract(ctx context.Context) ([]string, error)
}

// extractWebsites collects websites based on input method
func extractWebsites(ctx context.Context, placesPrompt, searchPrompt, inputFile string) ([]*website, error) {
	extractors := []extractor{
		newGooglePlacesSource(placesPrompt),
		newGoogleSearchSource(searchPrompt),
		newCSVSource(inputFile),
	}

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
