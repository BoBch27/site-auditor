package main

import (
	"context"
	"flag"
	"fmt"
	"log"
)

func main() {
	config := parseFlags()

	err := config.validate()
	if err != nil {
		log.Fatal(err)
	}

	// extract urls
	if config.search != "" {
		// scrape URLs from Google search
		config.urls, err = scrapeURLs(config.search)
	} else {
		// extract URLs from CSV
		config.urls, err = readURLsFromCSV(config.input)
	}
	if err != nil {
		log.Fatal(err)
	}

	// perform audits in a headless browser
	audits, err := auditWebsites(context.Background(), config.urls, config.checks)
	if err != nil {
		log.Fatal(err)
	}

	// write audit results
	err = writeResultsToCSV(config.output, audits)
	if err != nil {
		log.Fatal(err)
	}
}

type config struct {
	search string
	input  string
	output string
	checks string
	urls   []string
}

// parseFlags parses command line flags and returns a config
func parseFlags() config {
	var config config

	// define flags
	flag.StringVar(&config.search, "search", "", "Search prompt to scrape URLs for")
	flag.StringVar(&config.input, "input", "", "Path to input CSV file with URLs")
	flag.StringVar(&config.output, "output", "report.csv", "Path to output CSV report")
	flag.StringVar(&config.checks, "checks", "", "Comma-separated checks to run (security,lcp,console,request,headers,mobile,form,tech,screenshot). Empty = all checks")

	flag.Parse()
	return config
}

// validate ensures the configuration is valid
func (c *config) validate() error {
	if c.input == "" && c.search == "" {
		return fmt.Errorf("neither input file nor search prompt are specified")
	}

	if c.input != "" && c.search != "" {
		return fmt.Errorf("only one of input file or search prompt can be specified")
	}

	return nil
}
