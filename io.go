package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"strings"
)

// readURLsFromCSV reads the given CSV file and returns a slice of URLs
// assumes the first column contains URLs and skips the header
func readURLsFromCSV(filename string) ([]string, error) {
	file, err := os.Open(filename)
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
		return nil, errors.New("CSV file is empty or missing header")
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

// writeResultsToCSV writes the results to the output CSV
func writeResultsToCSV(filename string, results []auditResult) error {
	outFile, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	err = writer.Write([]string{"URL", "LCP (ms)", "Console Errors", "Missing Headers"})
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	for _, res := range results {
		err := writer.Write([]string{
			res.url,
			fmt.Sprint(res.lcp),
			strings.Join(res.consoleErrs, ";\n"),
			strings.Join(res.missingHeaders, ";\n"),
		})
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	writer.Flush()
	return nil
}
