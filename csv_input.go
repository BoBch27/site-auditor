package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// csvSource extracts URLs by reading them from a CSV file
// - it satisfies the extractor interface
type csvSource struct {
	inputFile string
}

// newCSVSource creates a new csvSource instance
func newCSVSource(inputFile string) (*csvSource, error) {
	if inputFile == "" {
		return nil, nil // not using CSV source
	}

	newSource := csvSource{inputFile}
	err := newSource.validateInputFile()
	if err != nil {
		return nil, fmt.Errorf("failed to initialise csv source: %w", err)
	}

	return &newSource, nil
}

// validateInputFile checks if the input CSV file exists and is readable
func (s *csvSource) validateInputFile() error {
	_, err := os.Stat(s.inputFile)
	if os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", s.inputFile)
	} else if err != nil {
		return fmt.Errorf("cannot access input file: %w", err)
	}

	return nil
}

// extract reads the given CSV file and returns a slice of URLs
// assumes the first column contains URLs and skips the header
func (s *csvSource) extract(_ context.Context) ([]string, error) {
	if s == nil || s.inputFile == "" {
		return nil, nil
	}

	file, err := os.Open(s.inputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 1 {
		return nil, fmt.Errorf("CSV file is empty or missing header")
	}

	var urls []string
	for _, row := range records[1:] { // skip header
		if len(row) > 0 {
			url := strings.TrimSpace(row[0])

			if url != "" {
				urls = append(urls, url)
			}
		}
	}

	return urls, nil
}
