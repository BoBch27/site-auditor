package main

import (
	"context"
	"flag"
	"log"
)

func main() {
	// define flags
	search := flag.String("search", "", "Search prompt to scrape URLs for")
	input := flag.String("input", "", "Path to input CSV file with URLs")
	output := flag.String("output", "report.csv", "Path to output CSV report")
	checks := flag.String("checks", "", "Specify which checks to run")

	// parse flags
	flag.Parse()

	// check specified flags
	if *input == "" {
		if *search == "" {
			log.Fatal("neither input file nor search prompt are specified")
		}
	} else {
		if *search != "" {
			log.Fatal("only one of input file or search prompt can be specified")
		}
	}

	// extract urls
	// get URLs
	var urls []string
	var err error
	if *search != "" {
		// scrape URLs from Google search
		urls, err = scrapeURLs(*search)
	} else {
		// extract URLs from CSV
		urls, err = readURLsFromCSV(*input)
	}
	if err != nil {
		log.Fatal(err)
	}

	// perform audits in a headless browser
	audits, err := auditWebsites(context.Background(), urls, *checks)
	if err != nil {
		log.Fatal(err)
	}

	// write audit results
	err = writeResultsToCSV(*output, audits)
	if err != nil {
		log.Fatal(err)
	}
}
