package main

import (
	"context"
	"flag"
	"log"
)

func main() {
	// define flags
	input := flag.String("input", "websites.csv", "Path to input CSV file with URLs")
	output := flag.String("output", "report.csv", "Path to output CSV report")
	checks := flag.String("checks", "", "Specify which checks to run")

	// parse flags
	flag.Parse()

	// extract urls
	urls, err := readURLsFromCSV(*input)
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
