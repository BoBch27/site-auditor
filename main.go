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
	config := parseFlags()

	checksToRun, err := config.validateAndExtract()
	if err != nil {
		log.Fatal(err)
	}

	websites, err := config.extractWebsites(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// perform audits in a headless browser
	audits, err := auditWebsites(ctx, websites, checksToRun, config.important)
	if err != nil {
		log.Fatal(err)
	}

	// write audit results
	err = writeResultsToCSV(config.output, audits)
	if err != nil {
		log.Fatal(err)
	}
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

	checksToRun, err := validateAndExtractChecks(c.checks)
	if err != nil {
		return auditChecks{}, err
	}

	err = validatePlacesSearchPrompt(c.search)
	if err != nil {
		return auditChecks{}, err
	}

	return checksToRun, nil
}

// extractWebsites collects websites based on input method
func (c *config) extractWebsites(ctx context.Context) ([]*website, error) {
	var urls []string

	// search for URLs from Google Places
	if c.search != "" {
		placesURLs, err := searchURLsFromGooglePlaces(ctx, c.search)
		if err != nil {
			return nil, err
		}

		urls = append(urls, placesURLs...)
	}

	// scrape URLs from Google Search
	if c.scrape != "" {
		scrapedURLs, err := scrapeURLsFromGoogleSearch(c.scrape)
		if err != nil {
			return nil, err
		}

		urls = append(urls, scrapedURLs...)
	}

	// extract URLs from CSV
	if c.input != "" {
		readURLs, err := readURLsFromCSV(c.input)
		if err != nil {
			return nil, err
		}

		urls = append(urls, readURLs...)
	}

	return c.cleanWebsites(urls), nil
}

// cleanWebsites filters and cleans passed in URLs
func (c *config) cleanWebsites(rawURLs []string) []*website {
	websites := []*website{}
	seen := map[string]bool{}

	for _, url := range rawURLs {
		if url == "" {
			continue
		}

		website, err := newWebsite(url)
		if err != nil {
			fmt.Println(err)
			continue
		}

		// avoid duplicates
		if seen[website.domain] {
			continue
		}

		// avoid domains which contain ignored words
		if website.isIgnored(ignoredBusinessPatterns) {
			continue
		}

		seen[website.domain] = true
		websites = append(websites, website)
	}

	return websites
}

// patterns to ignore when filtering business websites
var ignoredBusinessPatterns = []string{
	"facebook.com", "instagram.com", "twitter.com", "linkedin.com",
	"booksy.com", "treatwell.co.uk", "fresha.com",
	"yelp.com", "yelp.co.uk", "yell.com", "tripadvisor.com",
	"boots.com", "superdrug.com", "directory",
	"google.com", "maps.google.com", "bizmapgo",
}
