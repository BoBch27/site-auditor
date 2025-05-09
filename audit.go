package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type auditResult struct {
	url string
	lcp float64
}

// auditWebsites opens all URLs in a headless browser and executes various checks
// before returning a set of audit results
func auditWebsites(ctx context.Context, urls []string) ([]auditResult, error) {
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
		return nil, fmt.Errorf("failed to initialise browser: %w", err)
	}

	// inject LCP observer to run on all pages
	err = chromedp.Run(
		browserCtx,
		page.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(lcpScript).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to inject LCP script: %w", err)
	}

	results := make([]auditResult, len(urls))
	var errs []error

	for i, url := range urls {
		// audit each website
		res, err := auditWebsite(browserCtx, url)

		results[i] = res
		if err != nil {
			errs = append(errs, err)
		}
	}

	return results, errors.Join(errs...)
}

// auditWebsite opens the URL in a headless browser and executes various checks
// before returning an audit result
func auditWebsite(ctx context.Context, url string) (auditResult, error) {
	result := auditResult{url: url}

	// set context timeout
	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTimeout()

	// navigate browser to url (and wait to settle)
	err := chromedp.Run(
		timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(3*time.Second), // precautionary to ensure LCP is calculated
	)
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	// calculate largest contentful paint time
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(`window.__lcp || 0`, &result.lcp))
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to evaluate LCP for %s: %w", url, err)
	}

	return result, nil
}

// script to collect LCP in the browser
const lcpScript = `(() => {
	window.__lcp = 0;
	
	new PerformanceObserver((list) => {
  		const entries = list.getEntries();
  		const lastEntry = entries[entries.length - 1]; // use latest LCP candidate
  		
		window.__lcp = lastEntry.startTime || 0;
	}).observe({ type: "largest-contentful-paint", buffered: true });
})();`
