package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// csvSink handles writing audit results to a CSV file
type csvSink struct {
	outputFile string
}

// newCSVSink creates a new csvSink instance
func newCSVSink(outputFile string) (*csvSink, error) {
	newSink := csvSink{outputFile}
	err := newSink.validateOutputFile()
	if err != nil {
		return nil, fmt.Errorf("failed to initialise csv sink: %w", err)
	}

	return &newSink, nil
}

// validateOutputFile ensures the output directory exists and is writable
func (s *csvSink) validateOutputFile() error {
	if s.outputFile == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	// check if we can create the output file by attempting to create it
	// this validates both directory existence and write permissions
	file, err := os.Create(s.outputFile)
	if err != nil {
		return fmt.Errorf("cannot create output file %s: %w", s.outputFile, err)
	}
	file.Close()

	// remove the test file
	err = os.Remove(s.outputFile)
	if err != nil {
		return fmt.Errorf("cannot remove output test file %s: %w", s.outputFile, err)
	}

	return nil
}

// writeResults writes the results to the output CSV
func (s *csvSink) writeResults(results []auditResult) error {
	outFile, err := os.Create(s.outputFile)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
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
func (s *csvSink) getEnabledChecks(checks auditChecks) (headers []string, values []string) {
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
func (s *csvSink) boolToEmoji(ok bool) string {
	if !ok {
		return "❌"
	}

	return "✅"
}
