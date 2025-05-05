package main

import (
	"context"
	"log"
	"time"

	"github.com/chromedp/chromedp"
)

type auditResult struct {
	url string
}

// auditWebsite opens the URL in a headless browser and executes various checks
// before returning an audit result
func auditWebsites(ctx context.Context, urls []string) ([]auditResult, error) {
	var results []auditResult

	// setup browser options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
	)

	// create context with ExecAllocator
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	// create browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithLogf(log.Printf))
	defer cancelBrowser()

	// open headless browser with a blank page
	err := chromedp.Run(browserCtx, chromedp.Navigate("about:blank"))
	if err != nil {
		return nil, err
	}

	for _, url := range urls {
		result := auditResult{url}

		// set context timeout
		timeoutCtx, cancelTimeout := context.WithTimeout(browserCtx, 60*time.Second)
		defer cancelTimeout()

		// navigate browser to url
		err = chromedp.Run(timeoutCtx, chromedp.Navigate(url))
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	return results, nil
}
