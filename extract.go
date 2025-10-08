package main

import (
	"context"
	"fmt"
)

// Extractor defines the interface for extracting URLs from different sources
type Extractor interface {
	GetName() string // makes debugging easier
	Extract(ctx context.Context) ([]string, error)
}

// NewExtractors is a factory function to initialise different URL sources
func NewExtractors(placesPrompt, searchPrompt, inputFile string) ([]Extractor, error) {
	var extractors []Extractor

	googlePlacesSource, err := NewGooglePlacesSource(placesPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise google places source: %w", err)
	}
	if googlePlacesSource != nil {
		extractors = append(extractors, googlePlacesSource)
	}

	googleSearchSource := NewGoogleSearchSource(searchPrompt)
	if googleSearchSource != nil {
		extractors = append(extractors, googleSearchSource)
	}

	csvSource, err := NewCSVSource(inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to initialise csv source: %w", err)
	}
	if csvSource != nil {
		extractors = append(extractors, csvSource)
	}

	return extractors, nil
}

// ExtractWebsites collects websites from different sources
func ExtractWebsites(ctx context.Context, extractors []Extractor) ([]*Website, error) {
	type result struct {
		urls []string
		err  error
	}

	resultCh := make(chan result, len(extractors))

	// launch all extractors concurrently
	for _, ext := range extractors {
		go func(e Extractor) {
			urls, err := e.Extract(ctx)
			resultCh <- result{urls, err}
		}(ext)
	}

	// collect results
	var allURLs []string
	for _, ext := range extractors {
		r := <-resultCh
		if r.err != nil {
			return nil, fmt.Errorf("failed to extract from %s: %w", ext.GetName(), r.err) // fail on first error
		}

		allURLs = append(allURLs, r.urls...)
	}

	return FilterWebsites(allURLs), nil
}
