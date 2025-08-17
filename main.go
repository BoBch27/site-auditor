package main

import (
	"context"
	"flag"
	"fmt"
	"log"
)

type config struct {
	search    string
	input     string
	output    string
	checks    string
	important bool
	urls      []string
}

func main() {
	config := parseFlags()

	err := config.validate()
	if err != nil {
		log.Fatal(err)
	}

	err = config.extractURLs()
	if err != nil {
		log.Fatal(err)
	}

	// perform audits in a headless browser
	audits, err := auditWebsites(context.Background(), config)
	if err != nil {
		log.Fatal(err)
	}

	// write audit results
	err = writeResultsToCSV(config, audits)
	if err != nil {
		log.Fatal(err)
	}
}

// parseFlags parses command line flags and returns a config
func parseFlags() config {
	var config config

	// define flags
	flag.StringVar(&config.search, "search", "", "Search prompt to scrape URLs for")
	flag.StringVar(&config.input, "input", "", "Path to input CSV file with URLs")
	flag.StringVar(&config.output, "output", "report.csv", "Path to output CSV report")
	flag.StringVar(&config.checks, "checks", "", "Comma-separated checks to run (security,lcp,console,request,headers,mobile,form,tech,screenshot). Empty = all checks")
	flag.BoolVar(&config.important, "important", false, "Run only critical/important checks (faster)")

	flag.Parse()
	return config
}

// validate ensures the configuration is valid
func (c *config) validate() error {
	if c.input == "" && c.search == "" {
		return fmt.Errorf("neither input file nor search prompt are specified")
	}

	return nil
}

// extractURLs populates the URLs field based on input method
func (c *config) extractURLs() error {
	// scrape URLs from Google search
	if c.search != "" {
		scrapedURLs, err := scrapeURLs(c.search)
		if err != nil {
			return err
		}

		c.urls = append(c.urls, scrapedURLs...)
	}

	// extract URLs from CSV
	if c.input != "" {
		readURLs, err := readURLsFromCSV(c.input)
		if err != nil {
			return err
		}

		c.urls = append(c.urls, readURLs...)
	}

	return nil
}
