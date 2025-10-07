package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// validateInputFile checks if the input CSV file exists and is readable
func validateInputFile(filename string) error {
	if filename == "" {
		return nil // not using input file
	}

	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", filename)
	} else if err != nil {
		return fmt.Errorf("cannot access input file: %w", err)
	}

	return nil
}

// csvExtractor is responsible for reading a CSV file and extracting URLs
// from it - it satisfies the extractor interface
type csvExtractor struct {
	inputFile string
}

// newCSVExtractor creates a new csvExtractor instance
func newCSVExtractor(inputFile string) *csvExtractor {
	return &csvExtractor{inputFile}
}

// extract reads the given CSV file and returns a slice of URLs
// assumes the first column contains URLs and skips the header
func (c *csvExtractor) extract(_ context.Context) ([]string, error) {
	if c.inputFile == "" {
		return nil, nil
	}

	file, err := os.Open(c.inputFile)
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
