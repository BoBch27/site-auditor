package main

import (
	"context"
	"fmt"
)

// extractor defines the interface for extracting URLs from different sources
type extractor interface {
	extract(ctx context.Context) ([]string, error)
}

// newExtractors is a factory function to initialise different URL sources
func newExtractors(placesPrompt, searchPrompt, inputFile string) ([]extractor, error) {
	var extractors []extractor

	googlePlacesSource := newGooglePlacesSource(placesPrompt)
	extractors = append(extractors, googlePlacesSource)

	googleSearchSource := newGoogleSearchSource(searchPrompt)
	extractors = append(extractors, googleSearchSource)

	csvSource, err := newCSVSource(inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise extractors: %w", err)
	}
	if csvSource != nil {
		extractors = append(extractors, csvSource)
	}

	return extractors, nil
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
