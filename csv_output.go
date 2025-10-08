package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// CSVSink handles writing audit results to a CSV file
type CSVSink struct {
	outputFile string
}

// NewCSVSink creates a new CSVSink instance
func NewCSVSink(outputFile string) (*CSVSink, error) {
	newSink := CSVSink{outputFile}
	err := newSink.validateAndCreateOutputFile()
	if err != nil {
		return nil, fmt.Errorf("failed csv output file validation/creation: %w", err)
	}

	return &newSink, nil
}

// validateAndCreateOutputFile ensures the output directory exists and is writable
func (s *CSVSink) validateAndCreateOutputFile() error {
	if s.outputFile == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	// create the output file
	// this validates both directory existence and write permissions
	file, err := os.Create(s.outputFile)
	if err != nil {
		return fmt.Errorf("cannot create output file %s: %w", s.outputFile, err)
	}
	file.Close()

	return nil
}

// WriteResults writes the results to the output CSV
func (s *CSVSink) WriteResults(results []auditResult) error {
	if s == nil || s.outputFile == "" {
		return fmt.Errorf("nil csv sink")
	}

	outFile, err := os.Open(s.outputFile)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	headers := []string{"Website"}
	enabledHeaders, _ := s.getEnabledChecks(results[0].checks)
	headers = append(headers, enabledHeaders...)
	headers = append(headers, "Audit Errors")

	err = writer.Write(headers)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	for _, res := range results {
		row := []string{res.website}
		_, enabledValues := s.getEnabledChecks(res.checks)
		row = append(row, enabledValues...)
		row = append(row, strings.Join(res.auditErrs, ";\n"))

		err := writer.Write(row)
		if err != nil {
			return fmt.Errorf("failed to write to file: %w", err)
		}
	}

	writer.Flush()
	return nil
}

// getEnabledChecks returns headers and values for enabled checks
func (s *CSVSink) getEnabledChecks(checks auditChecks) (headers []string, values []string) {
	if checks.secure.enabled {
		headers = append(headers, "Secure")
		values = append(values, s.boolToEmoji(checks.secure.result))
	}
	if checks.lcp.enabled {
		headers = append(headers, "LCP (ms)")
		values = append(values, fmt.Sprint(checks.lcp.result))
	}
	if checks.consoleErrs.enabled {
		headers = append(headers, "Console Errors")
		values = append(values, strings.Join(checks.consoleErrs.result, ";\n"))
	}
	if checks.requestErrs.enabled {
		headers = append(headers, "Request Errors")
		values = append(values, strings.Join(checks.requestErrs.result, ";\n"))
	}
	if checks.missingHeaders.enabled {
		headers = append(headers, "Missing Headers")
		values = append(values, strings.Join(checks.missingHeaders.result, ";\n"))
	}
	if checks.responsiveIssues.enabled {
		headers = append(headers, "Responsive Issues")
		values = append(values, strings.Join(checks.responsiveIssues.result, ";\n"))
	}
	if checks.formIssues.enabled {
		headers = append(headers, "Form Issues")
		values = append(values, strings.Join(checks.formIssues.result, ";\n"))
	}
	if checks.techStack.enabled {
		headers = append(headers, "Detected Tech")
		values = append(values, strings.Join(checks.techStack.result, ";\n"))
	}
	if checks.screenshot.enabled {
		headers = append(headers, "Screenshot")
		values = append(values, s.boolToEmoji(checks.screenshot.result))
	}

	return headers, values
}

// boolToEmoji takes in a boolean and returns corresponding
// emoji to visual inspection
func (s *CSVSink) boolToEmoji(ok bool) string {
	if !ok {
		return "❌"
	}

	return "✅"
}
