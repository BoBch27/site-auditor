package main

import (
	"context"
	"flag"
	"fmt"
	"log"
)

type config struct {
	search        string
	scrape        string
	input         string
	output        string
	checks        string
	important     bool
	screenshotDir string
}

func main() {
	ctx := context.Background()
	spinner := NewSpinner()

	// parse flags
	spinner.Start("Parsing input...")
	config, err := parseFlags()
	if err != nil {
		log.Fatalf("\n❌ failed flag parsing: %v\n", err)
	}
	spinner.Stop()

	// initialise internal resources - grouped here, since they do internal
	// validations and that should be done before running actual logic
	spinner.Start("Initialising resources...")
	extractors, err := NewExtractors(config.search, config.scrape, config.input)
	if err != nil {
		log.Fatalf("\n❌ failed extractors initialisation: %v\n", err)
	}

	audit, err := newAudit(config.checks, config.important, config.screenshotDir)
	if err != nil {
		log.Fatalf("\n❌ failed audit service initialisation: %v\n", err)
	}

	csvSink, err := NewCSVSink(config.output)
	if err != nil {
		log.Fatalf("\n❌ failed csv output initialisation: %v\n", err)
	}
	spinner.Stop()

	// collect websites from different sources
	spinner.Start("Extracting websites...")
	websites, err := ExtractWebsites(ctx, extractors)
	if err != nil {
		log.Fatalf("\n❌ failed website extracting: %v\n", err)
	}
	spinner.Stop()

	// perform audits in a headless browser
	spinner.Start("Auditing websites...")
	audits, err := audit.run(ctx, websites)
	if err != nil {
		log.Fatalf("\n❌ failed website auditing: %v\n", err)
	}
	spinner.Stop()

	// write audit results to csv
	spinner.Start("Writing results...")
	err = csvSink.WriteResults(audits)
	if err != nil {
		log.Fatalf("\n❌ failed results writing: %v\n", err)
	}
	spinner.Stop()

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
	flag.StringVar(&config.screenshotDir, "screenshot-dir", "screenshots", "Path to folder to store screenshots")

	flag.Parse()

	if flag.NArg() > 0 {
		return nil, fmt.Errorf("unexpected arguments: %v", flag.Args())
	}

	if config.search == "" && config.scrape == "" && config.input == "" {
		return nil, fmt.Errorf("neither search prompt, nor scrape prompt, nor input file are specified")
	}

	return &config, nil
}
