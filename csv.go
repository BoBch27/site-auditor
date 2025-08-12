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
func writeResultsToCSV(cfg config, results []auditResult) error {
	outFile, err := os.Create(cfg.output)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	headers := []string{"URL"}
	headers = append(headers, getEnabledHeaders(results[0].checks)...)
	headers = append(headers, "Audit Errors")

	err = writer.Write(headers)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	for _, res := range results {
		row := []string{res.url}
		row = append(row, getEnabledValues(res.checks)...)
		row = append(row, strings.Join(res.auditErrs, ";\n"))

		err := writer.Write(row)
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	writer.Flush()
	return nil
}

// getEnabledHeaders returns headers for enabled checks
func getEnabledHeaders(checks auditChecks) []string {
	var headers []string
	if checks.secure.enabled {
		headers = append(headers, "Secure")
	}
	if checks.lcp.enabled {
		headers = append(headers, "LCP (ms)")
	}
	if checks.consoleErrs.enabled {
		headers = append(headers, "Console Errors")
	}
	if checks.requestErrs.enabled {
		headers = append(headers, "Request Errors")
	}
	if checks.missingHeaders.enabled {
		headers = append(headers, "Missing Headers")
	}
	if checks.responsiveIssues.enabled {
		headers = append(headers, "Responsive Issues")
	}
	if checks.formIssues.enabled {
		headers = append(headers, "Form Issues")
	}
	if checks.techStack.enabled {
		headers = append(headers, "Detected Tech")
	}
	if checks.screenshot.enabled {
		headers = append(headers, "Screenshot")
	}
	return headers
}

// getEnabledValues returns formatted values for enabled checks
func getEnabledValues(checks auditChecks) []string {
	var values []string
	if checks.secure.enabled {
		values = append(values, boolToEmoji(checks.secure.result))
	}
	if checks.lcp.enabled {
		values = append(values, fmt.Sprint(checks.lcp.result))
	}
	if checks.consoleErrs.enabled {
		values = append(values, strings.Join(checks.consoleErrs.result, ";\n"))
	}
	if checks.requestErrs.enabled {
		values = append(values, strings.Join(checks.requestErrs.result, ";\n"))
	}
	if checks.missingHeaders.enabled {
		values = append(values, strings.Join(checks.missingHeaders.result, ";\n"))
	}
	if checks.responsiveIssues.enabled {
		values = append(values, strings.Join(checks.responsiveIssues.result, ";\n"))
	}
	if checks.formIssues.enabled {
		values = append(values, strings.Join(checks.formIssues.result, ";\n"))
	}
	if checks.techStack.enabled {
		values = append(values, strings.Join(checks.techStack.result, ";\n"))
	}
	if checks.screenshot.enabled {
		values = append(values, boolToEmoji(checks.screenshot.result))
	}
	return values
}
