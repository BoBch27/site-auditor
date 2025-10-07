package main

import "context"

// extractor defines the interface for extracting URLs from different sources
type extractor interface {
	extract(ctx context.Context) ([]string, error)
}
