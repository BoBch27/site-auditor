package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// searchScraper is responsible for querying Google with specified search prompt
// in a headless browser, and extract found URLs - it satisfies the extractor interface
type searchScraper struct {
	searchPrompt string
}

// newSearchScraper creates a new searchScraper instance
func newSearchScraper(searchPrompt string) *searchScraper {
	return &searchScraper{searchPrompt}
}

// extract queries Google with the specified search prompt in a
// headless browser, and extracts the returned result URLs
func (s *searchScraper) extract(_ context.Context) ([]string, error) {
	if s.searchPrompt == "" {
		return nil, nil
	}

	urls := []string{}
	searchQuery := url.QueryEscape(s.searchPrompt)

	for page := range 9 { // scrape first 10 pages (0 -> 9)
		start := page * 10
		searchPath := fmt.Sprintf("/search?q=%s&start=%d", searchQuery, start)

		// send request and get doc
		doc, err := getDoc(searchPath)
		if err != nil {
			return nil, err
		}

		// grab links
		doc.Find("div.yuRUbf a").Each(func(i int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if !exists || !strings.HasPrefix(href, "http") || strings.Contains(href, "google.com") {
				return
			}

			urls = append(urls, href)
		})

		// random 30-60 second wait to simulate human behaviour
		time.Sleep(time.Duration(rand.Intn(31)+30) * time.Second)
	}

	return urls, nil
}

// getDoc sends an HTTP Get request to Google, checks if there's a redirect link
// and sends a request to it if so, before returning a parsed HTML document
func getDoc(searchPath string) (*goquery.Document, error) {
	req, _ := http.NewRequest("GET", "https://google.com"+searchPath, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Safari/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("non-200 response: %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse html response: %w", err)
	}

	// check for Google's redirect link
	redirectLink, exists := doc.Find("div#yvlrue a").First().Attr("href")
	if exists {
		// send request to redirect link
		doc, err = getDoc(redirectLink)
	}

	return doc, err
}
