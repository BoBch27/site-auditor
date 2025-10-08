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
	config, err := parseFlags()
	if err != nil {
		log.Fatalf("\n❌ failed flag parsing: %v\n", err)
	}
	spinner.stop()

	// validate flags
	spinner.start("Validating input...")
	err = config.validateAndExtract()
	if err != nil {
		log.Fatalf("\n❌ failed input validation: %v\n", err)
	}
	spinner.stop()

	// initiate different url sources
	spinner.start("Initialising resources...")
	extractors, err := newExtractors(config.search, config.scrape, config.input)
	if err != nil {
		log.Fatalf("\n❌ failed extractors initialisation: %v\n", err)
	}

	audit, err := newAudit(config.checks, config.important)
	if err != nil {
		log.Fatalf("\n❌ failed audit service initialisation: %v\n", err)
	}

	// initiate result sink
	csvSink, err := newCSVSink(config.output)
	if err != nil {
		log.Fatalf("\n❌ failed csv output initialisation: %v\n", err)
	}
	spinner.stop()

	// collect websites from different sources
	spinner.start("Extracting websites...")
	websites, err := extractWebsites(ctx, extractors)
	if err != nil {
		log.Fatalf("\n❌ failed website extracting: %v\n", err)
	}
	spinner.stop()

	// perform audits in a headless browser
	spinner.start("Auditing websites...")
	audits, err := audit.run(ctx, websites)
	if err != nil {
		log.Fatalf("\n❌ failed website auditing: %v\n", err)
	}
	spinner.stop()

	// write audit results to csv
	spinner.start("Writing results...")
	err = csvSink.writeResults(audits)
	if err != nil {
		log.Fatalf("\n❌ failed results writing: %v\n", err)
	}
	spinner.stop()

	fmt.Println("✅ Done")
}

// parseFlags parses command line flags and returns a config
func parseFlags() (*config, error) {
	var config config

	// define flags
	flag.StringVar(&config.search, "search", "", "Search prompt for which to find URLs from Google Places")
	flag.StringVar(&config.scrape, "scrape", "", "Google input prompt to scrape URLs for")
	flag.StringVar(&config.input, "input", "", "Path to input CSV file with URLs")
	flag.StringVar(&config.output, "output", "report.csv", "Path to output CSV report")
	flag.StringVar(&config.checks, "checks", "", "Comma-separated checks to run (security,lcp,console,request,headers,mobile,form,tech,screenshot). Empty = all checks")
	flag.BoolVar(&config.important, "important", false, "Run only critical/important checks (faster)")

	flag.Parse()

	if flag.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %v", flag.Args())
	}

	return &config, nil
}

// validateAndExtract ensures the configuration is valid and
// extracts specified audit checks to perform
func (c *config) validateAndExtract() error {
	if c.search == "" && c.scrape == "" && c.input == "" {
		return fmt.Errorf("neither search prompt, nor scrape prompt, nor input file are specified")
	}

	return nil
}
