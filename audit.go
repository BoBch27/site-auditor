package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/target"
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

	// determine concurrency limit - use number based on system resources
	concurrencyLimit := runtime.NumCPU() * 2 // reasonable default: 2 tabs per CPU core
	if concurrencyLimit > 4 {
		concurrencyLimit = 4 // cap at 4 for browser stability
	}

	semaphore := make(chan struct{}, concurrencyLimit)

	var wg sync.WaitGroup
	results := make([]auditResult, len(urls))
	errs := make([]error, len(urls))

	for i, url := range urls {
		wg.Add(1)

		// acquire semaphore (blocks if max goroutines are already running)
		semaphore <- struct{}{}

		// audit each website
		go func(i int, url string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release semaphore when done

			res, err := auditWebsite(browserCtx, url)

			results[i] = res
			errs[i] = err
		}(i, url)
	}

	wg.Wait()
	close(semaphore)

	return results, nil
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

// auditWebsite opens the URL in a headless browser and executes various checks
// before returning an audit result
func auditWebsite(ctx context.Context, url string) (auditResult, error) {
	result := auditResult{url: url}

	// create tab context
	tabCtx, cancelTab, err := newTabContext(ctx)
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to initialise tab for %s: %w", url, err)
	}
	defer cancelTab()

	// set context timeout
	timeoutCtx, cancelTimeout := context.WithTimeout(tabCtx, 60*time.Second)
	defer cancelTimeout()

	// inject LCP observer before page loads
	err = chromedp.Run(
		timeoutCtx,
		page.Enable(),
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(lcpScript).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to inject script for %s: %w", url, err)
	}

	// extract context data
	ctxData := chromedp.FromContext(timeoutCtx)
	if ctxData == nil || ctxData.Target == nil {
		return auditResult{}, errors.New("tab not initialised in context")
	}

	// navigate browser to url (and wait to settle)
	err = chromedp.Run(
		timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		target.ActivateTarget(ctxData.Target.TargetID), // focus tab for LCP observer script to run
		chromedp.Sleep(3*time.Second),                  //precautionary to ensure LCP is calculated
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

// newTabContext opens a new tab in the same window and returns a chromedp context for it
// to be used instead of chromedp.NewContext(ctx), as that opens a new window
func newTabContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	// extract context data
	ctxData := chromedp.FromContext(ctx)
	if ctxData == nil || ctxData.Browser == nil {
		return nil, nil, errors.New("browser not initialised in context")
	}

	// set executor (browser) for context
	execCtx := cdp.WithExecutor(ctx, ctxData.Browser)

	// create new tab in existing window
	targetID, err := target.CreateTarget("about:blank").
		WithNewWindow(false). // IMPORTANT: ensures it's a new tab, not a window
		Do(execCtx)
	if err != nil {
		return nil, nil, err
	}

	// create new chromedp context attached to new tab
	tabCtx, cancelTab := chromedp.NewContext(ctx, chromedp.WithTargetID(targetID))
	return tabCtx, cancelTab, nil
}
