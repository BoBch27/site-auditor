package main

import (
	"flag"
	"log"
)

type auditResult struct {
	url string
}

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

	var results []auditResult
	for _, url := range urls {
		results = append(results, auditResult{url: url})
	}

	err = writeResultsToCSV(*output, results)
	if err != nil {
		log.Fatal(err)
	}
}
