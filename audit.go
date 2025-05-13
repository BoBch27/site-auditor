package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

type auditResult struct {
	url              string
	lcp              float64
	consoleErrs      []string
	missingHeaders   []string
	responsiveIssues []string
}

// auditWebsites opens all URLs in a headless browser and executes various checks
// before returning a set of audit results
func auditWebsites(ctx context.Context, urls []string) ([]auditResult, error) {
	// setup browser options
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-cache", true),
		chromedp.Flag("incognito", true),
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

	// enable additional chromedp domains
	err = chromedp.Run(
		browserCtx,
		runtime.Enable(),
		network.Enable(),
		page.Enable(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to enable additional domains: %w", err)
	}

	// inject LCP observer to run on all pages
	err = chromedp.Run(
		browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(lcpScript).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to inject LCP script: %w", err)
	}

	// emulate mobile device (iPhone)
	err = chromedp.Run(
		browserCtx,
		emulation.SetUserAgentOverride("Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1"),
		emulation.SetDeviceMetricsOverride(375, 667, 2.0, true),
		emulation.SetTouchEmulationEnabled(true),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to emulate mobile device: %w", err)
	}

	urlsNo := len(urls)
	results := make([]auditResult, urlsNo)
	var errs []error

	for i, url := range urls {
		// audit each website
		fmt.Printf("auditing site %d/%d (%s)\n", i+1, urlsNo, url)
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
	var resMutex sync.Mutex
	var headerChecked bool

	// set context timeout
	timeoutCtx, cancelTimeout := context.WithTimeout(ctx, 60*time.Second)
	defer cancelTimeout()

	// clear network cache and cookies before each run
	err := chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		err := network.ClearBrowserCache().Do(ctx)
		if err != nil {
			return err
		}

		err = network.ClearBrowserCookies().Do(ctx)
		return err
	}))
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to clear browser cache and cookies: %w", err)
	}

	// subscribe and listen to target events
	chromedp.ListenTarget(timeoutCtx, func(ev interface{}) {
		switch msg := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			if msg.Type == runtime.APITypeError || msg.Type == runtime.APITypeWarning {
				for _, arg := range msg.Args {
					errInfo := fmt.Sprintf("[%s]: %s", msg.Type, arg.Value)

					resMutex.Lock()
					result.consoleErrs = append(result.consoleErrs, errInfo)
					resMutex.Unlock()
				}
			}
		case *runtime.EventExceptionThrown:
			exceptionDetails := msg.ExceptionDetails

			errorInfo := fmt.Sprintf(
				"Uncaught Exception: %s at %s:%d:%d (Script ID: %s)",
				exceptionDetails.Text,
				exceptionDetails.URL,
				exceptionDetails.LineNumber,
				exceptionDetails.ColumnNumber,
				exceptionDetails.ScriptID,
			)

			if exceptionDetails.StackTrace != nil {
				for _, callFrame := range exceptionDetails.StackTrace.CallFrames {
					errorInfo += fmt.Sprintf(
						"\n  at %s (%s:%d:%d)",
						callFrame.FunctionName,
						callFrame.URL,
						callFrame.LineNumber,
						callFrame.ColumnNumber,
					)
				}
			}

			resMutex.Lock()
			result.consoleErrs = append(result.consoleErrs, errorInfo)
			resMutex.Unlock()
		case *network.EventResponseReceived:
			// only check headers for main document/page response
			if msg.Type == network.ResourceTypeDocument {
				if headerChecked {
					return
				}

				headerChecked = true

				for _, header := range securityHeaders {
					found := false

					// headers in chromedp are case-insensitive, but we need to check different formats
					for key := range msg.Response.Headers {
						if strings.EqualFold(key, header) {
							found = true
							break
						}
					}

					if !found {
						resMutex.Lock()
						result.missingHeaders = append(result.missingHeaders, header)
						resMutex.Unlock()
					}
				}
			}

			if msg.Response.Status >= 400 {
				errorInfo := fmt.Sprintf(
					"HTTP Error: %d for %s",
					msg.Response.Status,
					msg.Response.URL,
				)

				resMutex.Lock()
				result.consoleErrs = append(result.consoleErrs, errorInfo)
				resMutex.Unlock()
			}
		case *network.EventLoadingFailed:
			errorInfo := fmt.Sprintf(
				"Request Failed: %s for %s (Reason: %s)",
				msg.ErrorText,
				msg.RequestID,
				msg.Type,
			)

			resMutex.Lock()
			result.consoleErrs = append(result.consoleErrs, errorInfo)
			resMutex.Unlock()
		}
	})

	// navigate browser to url (and wait to settle)
	err = chromedp.Run(
		timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // precautionary to ensure LCP is calculated
	)
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	// calculate largest contentful paint time
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(`window.__lcp || 0`, &result.lcp))
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to evaluate LCP for %s: %w", url, err)
	}

	// capture mobile responsiveness issues
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(responsiveScript, &result.responsiveIssues))
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to evaluate website responsiveness for %s: %w", url, err)
	}

	return result, nil
}

// important security headers to check
var securityHeaders = []string{
	"Content-Security-Policy",
	"Strict-Transport-Security",
	"X-Content-Type-Options",
	"X-Frame-Options",
	"Permissions-Policy",
	"Referrer-Policy",
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

// script to collect mobile responsiveness issues
const responsiveScript = `(() => {
	const __responsiveIssues = [];

    // 1 - check overflowing elements
    const els = Array.from(document.querySelectorAll("*"));
    const overflowingEls = els
        .filter(el => el.scrollWidth > el.clientWidth + 5)
        .map(el => {
			const tag = "tag: " + el.tagName + "; ";
			const id = "id: " + el.id + "; ";
			const className = "class: " + el.className + "; ";
			const overflow = "overflow: " + (el.scrollWidth - el.clientWidth).toString();
			return tag + id + className + overflow;
		})
        .slice(0, 3)
		.forEach(el => {
			__responsiveIssues.push("Overflowing element: " + el);
		});
    
    // 2 - check for viewport meta tag
    const hasViewport = !!document.querySelector('meta[name="viewport"]');
	if (!hasViewport) {
		__responsiveIssues.push("No viewport tag");
	}
    
    // 3 - check if content adapts to viewport width
    const mainContent = document.querySelector('main, #main, .main, #content, .content, body > div');
    const mainWidth = mainContent ? mainContent.offsetWidth : 0;
    const windowWidth = window.innerWidth;
    const widthRatio = mainWidth / windowWidth;
    
    // responsive sites typically have content that takes up
    // 90-100 percent of viewport on mobile (not fixed pixel width)
    const adaptiveLayout = widthRatio > 0.9;
	if (!adaptiveLayout) {
		__responsiveIssues.push("Not adaptive layout");
	}

	return __responsiveIssues;
})()`
