package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// validateOutputFile ensures the output directory exists and is writable
func validateOutputFile(filepath string) error {
	if filepath == "" {
		return fmt.Errorf("output path cannot be empty")
	}

	// check if we can create the output file by attempting to create it
	// this validates both directory existence and write permissions
	file, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("cannot create output file %s: %w", filepath, err)
	}
	file.Close()

	// remove the test file
	err = os.Remove(filepath)
	if err != nil {
		return fmt.Errorf("cannot remove output test file %s: %w", filepath, err)
	}

	return nil
}

// writeResultsToCSV writes the results to the output CSV
func writeResultsToCSV(filepath string, results []auditResult) error {
	outFile, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	headers := []string{"Website"}
	enabledHeaders, _ := getEnabledChecks(results[0].checks)
	headers = append(headers, enabledHeaders...)
	headers = append(headers, "Audit Errors")

	err = writer.Write(headers)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	for _, res := range results {
		row := []string{res.website}
		_, enabledValues := getEnabledChecks(res.checks)
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
func getEnabledChecks(checks auditChecks) (headers []string, values []string) {
	if checks.secure.enabled {
		headers = append(headers, "Secure")
		values = append(values, boolToEmoji(checks.secure.result))
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
		values = append(values, boolToEmoji(checks.screenshot.result))
	}

	return headers, values
}

// boolToEmoji takes in a boolean and returns corresponding
// emoji to visual inspection
func boolToEmoji(ok bool) string {
	if !ok {
		return "❌"
	}

	return "✅"
}
