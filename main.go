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

	// parse flags
	flag.Parse()

	urls, err := readURLsFromCSV(*input)
	if err != nil {
		log.Fatal(err)
	}

	audits, err := auditWebsites(context.Background(), urls)
	if err != nil {
		log.Fatal(err)
	}

	err = writeResultsToCSV(*output, audits)
	if err != nil {
		log.Fatal(err)
	}
}
