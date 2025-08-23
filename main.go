package main

import (
	"context"
	"flag"
	"fmt"
	"log"
)

type config struct {
	search    string
	scrape    string
	input     string
	output    string
	checks    string
	important bool
}

func main() {
	ctx := context.Background()
	spinner := newSpinner()

	// parse flags
	spinner.start("Parsing input...")
	config := parseFlags()
	spinner.stop()

	// validate flags
	spinner.start("Validating input...")
	checksToRun, err := config.validateAndExtract()
	if err != nil {
		log.Fatalf("❌ %v\n", err)
	}
	spinner.stop()

	// collect websites based on specified input methods
	spinner.start("Extracting websites...")
	websites, err := extractWebsites(ctx, config.search, config.scrape, config.input)
	if err != nil {
		log.Fatalf("❌ %v\n", err)
	}
	spinner.stop()

	// perform audits in a headless browser
	spinner.start("Auditing websites...")
	audits, err := auditWebsites(ctx, websites, checksToRun, config.important)
	if err != nil {
		log.Fatalf("❌ %v\n", err)
	}
	spinner.stop()

	// write audit results to csv
	spinner.start("Writing results...")
	err = writeResultsToCSV(config.output, audits)
	if err != nil {
		log.Fatalf("❌ %v\n", err)
	}
	spinner.stop()

	fmt.Println("✅ Done")
}

// parseFlags parses command line flags and returns a config
func parseFlags() config {
	var config config

	// define flags
	flag.StringVar(&config.search, "search", "", "Search prompt for which to find URLs from Google Places")
	flag.StringVar(&config.scrape, "scrape", "", "Google input prompt to scrape URLs for")
	flag.StringVar(&config.input, "input", "", "Path to input CSV file with URLs")
	flag.StringVar(&config.output, "output", "report.csv", "Path to output CSV report")
	flag.StringVar(&config.checks, "checks", "", "Comma-separated checks to run (security,lcp,console,request,headers,mobile,form,tech,screenshot). Empty = all checks")
	flag.BoolVar(&config.important, "important", false, "Run only critical/important checks (faster)")

	flag.Parse()
	return config
}

// validateAndExtract ensures the configuration is valid and
// extracts specified audit checks to perform
func (c *config) validateAndExtract() (auditChecks, error) {
	if c.search == "" && c.scrape == "" && c.input == "" {
		return auditChecks{}, fmt.Errorf("neither search prompt, nor scrape prompt, nor input file are specified")
	}

	err := validateInputFile(c.input)
	if err != nil {
		return auditChecks{}, err
	}

	err = validateOutputFile(c.output)
	if err != nil {
		return auditChecks{}, err
	}

	checksToRun, err := validateAndExtractChecks(c.checks, c.important)
	if err != nil {
		return auditChecks{}, err
	}

	err = validatePlacesSearchPrompt(c.search)
	if err != nil {
		return auditChecks{}, err
	}

	return checksToRun, nil
}
