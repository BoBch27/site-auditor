package main

import (
	"flag"
	"fmt"
)

func main() {
	// define flags
	input := flag.String("input", "websites.csv", "Path to input CSV file with URLs")
	output := flag.String("output", "report.csv", "Path to output CSV report")

	// parse flags
	flag.Parse()

	fmt.Println(*input, *output)
}
