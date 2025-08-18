package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
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

	urls, err := config.extractURLs(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// perform audits in a headless browser
	audits, err := auditWebsites(ctx, urls, checksToRun, config.important)
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

// extractURLs collects URLs based on input method
func (c *config) extractURLs(ctx context.Context) ([]string, error) {
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

	return c.extractCleanURLs(urls), nil
}

// extractCleanURLs filters and cleans URLs
func (c *config) extractCleanURLs(rawURLs []string) []string {
	urls := []string{}
	seen := map[string]bool{}

	for _, url := range rawURLs {
		if url == "" {
			continue
		}

		scheme, domain, err := extractUrlParts(url)
		if err != nil {
			fmt.Println(err)
			continue
		}

		// avoid duplicates
		if seen[domain] {
			continue
		}

		// avoid domains which contain ignored words
		if c.isIgnoredDomain(domain) {
			continue
		}

		seen[domain] = true
		urls = append(urls, scheme+"://"+domain+"/")
	}

	return urls
}

// domains to ignore when filtering business websites
var ignoredBusinessDomains = []string{
	"facebook.com", "instagram.com", "twitter.com", "linkedin.com",
	"booksy.com", "treatwell.co.uk", "fresha.com",
	"yelp.com", "yelp.co.uk", "yell.com", "tripadvisor.com",
	"boots.com", "superdrug.com", "directory",
	"google.com", "maps.google.com", "bizmapgo",
}

// isIgnoredDomain checks if a domain should be filtered out
func (c *config) isIgnoredDomain(domain string) bool {
	for _, ignored := range ignoredBusinessDomains {
		if strings.Contains(domain, ignored) {
			return true
		}
	}

	return false
}
