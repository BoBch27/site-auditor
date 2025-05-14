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
	requestErrs      []string
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

	// inject error collection script to run on all pages
	err = chromedp.Run(
		browserCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(errScript).Do(ctx)
			return err
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to inject error script: %w", err)
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
				result.requestErrs = append(result.requestErrs, errorInfo)
				resMutex.Unlock()
			}
		}
	})

	// navigate browser to url
	err = chromedp.Run(timeoutCtx, chromedp.Navigate(url))
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to navigate to %s: %w", url, err)
	}

	// wait for page to settle
	err = chromedp.Run(
		timeoutCtx,
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.Sleep(5*time.Second), // precautionary to ensure LCP is calculated
	)
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to wait for %s to load: %w", url, err)
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

	// collect console errors and warnings
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(`window.__console_errors || []`, &result.consoleErrs))
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to evaluate console errors for %s: %w", url, err)
	}

	// collect failed requests
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(`window.__request_errors || []`, &result.requestErrs))
	if err != nil {
		return auditResult{}, fmt.Errorf("failed to evaluate request errors for %s: %w", url, err)
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

// script to capture console errors and warnings, and request errors
const errScript = `(() => {
	window.__console_errors = [];
	window.__request_errors = [];

	// capture request and JS errors
	window.addEventListener('error', (e) => {
		if (e.target && (e.target.src || e.target.href)) { // resource load error (img, script, link)
			const message = (e.target.src || e.target.href) + " (type: " + e.target.tagName + ")";
			window.__request_errors.push("[Resource Load Failed]: " + message);
			return;
		}

		const message = e.message + " at " + e.filename + ":" + e.lineno + ":" + e.colno + " (" + e.error?.stack + ")";
		window.__console_errors.push("[Uncaught Exception]: " + message);
	}, true);
	
	// capture unhandled promise rejections
	window.addEventListener('unhandledrejection', (e) => {
		const message = (e.reason ? e.reason.message : "Unknown") + " (" + e.reason?.stack + ")";
		window.__console_errors.push("[Unhandled Promise Rejection]: " + message);
	});
	
	// override fetch to capture request errors
	const origFetch = fetch;
	fetch = async function(...args) {
		try {
			const res = await origFetch.apply(this, args);
			
			if (res.status >= 400) {
				const message = res.status + " for " + res.url;
				window.__request_errors.push("[HTTP Error]: " + message);
			}
			
			return res;
		} catch (e) {
		 	const message = e.message + " for " + (args ? args[0] : "");
			window.__request_errors.push("[HTTP Error]: " + message);
			throw e;
		}
	};

	// override XMLHttpRequest to capture request errors
	const origOpen = XMLHttpRequest.prototype.open;
  	const origSend = XMLHttpRequest.prototype.send;
  	XMLHttpRequest.prototype.open = function (method, url, async, user, password) {
    	this.__requestUrl = url;
    	return origOpen.apply(this, arguments);
  	};
 	XMLHttpRequest.prototype.send = function (body) {
    	const xhr = this;

    	function logError() {
			if (xhr.status >= 400 || xhr.status === 0) {
				const message = xhr.status + " for " + xhr.__requestUrl;
				window.__request_errors.push("[HTTP Error]: " + message);
			}
    	}

		this.addEventListener("load", logError);
		this.addEventListener("error", logError);
		this.addEventListener("abort", logError);

		return origSend.apply(this, arguments);
	};

	// override console.error to capture console errors
	const originalConsoleError = console.error;
	console.error = (...args) => {
		const message = args.map(String).join(' ');
		window.__console_errors.push("[Error]: " + message);
		originalConsoleError.apply(console, args);
	};
	
	// override console.warn to capture console warnings
	const originalConsoleWarn = console.warn;
	console.warn = (...args) => {
		const message = args.map(String).join(' ');
		window.__console_errors.push("[Warning]: " + message);
		originalConsoleWarn.apply(console, args);
	};
	
	return window.__console_errors;
})();`

// script to collect mobile responsiveness issues
const responsiveScript = `(() => {
	const __responsiveIssues = [];

	// 1 - check for horizontal scrollbar
	const horizontalBar = document.body.scrollWidth > window.innerWidth;
	if (horizontalBar) {
		__responsiveIssues.push("Has horizontal scrollbar");
	}

    // 2 - check overflowing elements
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
    
    // 3 - check for viewport meta tag
    const hasViewport = !!document.querySelector('meta[name="viewport"]');
	if (!hasViewport) {
		__responsiveIssues.push("No viewport tag");
	}
    
    // 4 - check if content adapts to viewport width
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
