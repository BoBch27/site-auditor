package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type auditResult struct {
	url              string
	lcp              float64
	consoleErrs      []string
	requestErrs      []string
	missingHeaders   []string
	responsiveIssues []string
	formIssues       []string
	auditErrs        []string
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
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("disable-features", "PreloadMediaEngagementData,PreloadMediaEngagementData2,SpeculativePreconnect,NoStatePrefetch"),
	)

	// create context with ExecAllocator
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	// create browser context
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	// open headless browser with a blank page
	err := chromedp.Run(browserCtx, chromedp.Navigate("about:blank"))
	if err != nil {
		return nil, fmt.Errorf("failed to initialise browser: %w", err)
	}

	// wait for browser to initialise
	err = chromedp.Run(browserCtx, chromedp.Sleep(1*time.Second))
	if err != nil {
		return nil, fmt.Errorf("failed to wait for browser initialisation: %w", err)
	}

	urlsNo := len(urls)
	results := make([]auditResult, urlsNo)

	for i, url := range urls {
		// audit each website
		fmt.Printf("auditing site %d/%d (%s)\n", i+1, urlsNo, url)
		results[i] = auditWebsite(browserCtx, url)

	}

	return results, nil
}

// auditWebsite opens the URL in a headless browser and executes various checks
// before returning an audit result
func auditWebsite(ctx context.Context, url string) auditResult {
	result := auditResult{url: url}

	// create new window context
	windowCtx, cancelWindow := chromedp.NewContext(ctx)
	defer cancelWindow()

	// set context timeout
	timeoutCtx, cancelTimeout := context.WithTimeout(windowCtx, 60*time.Second)
	defer cancelTimeout()

	// open window with blank page
	err := chromedp.Run(timeoutCtx, chromedp.Navigate("about:blank"))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to initialise window: %s", err.Error()),
		)

		return result
	}

	// wait for window to initialise
	err = chromedp.Run(timeoutCtx, chromedp.Sleep(1*time.Second))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to wait for window initialisation: %s", err.Error()),
		)

		return result
	}

	// enable additional chromedp domains
	err = chromedp.Run(
		timeoutCtx,
		network.Enable(),
		page.Enable(),
	)
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to enable additional domains: %s", err.Error()),
		)

		return result
	}

	// inject LCP observer to run on page
	err = chromedp.Run(
		timeoutCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(lcpScript).Do(ctx)
			return err
		}),
	)
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to inject LCP script: %s", err.Error()),
		)

		return result
	}

	// inject error collection script to run on page
	err = chromedp.Run(
		timeoutCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(errScript).Do(ctx)
			return err
		}),
	)
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to inject error script: %s", err.Error()),
		)

		return result
	}

	// emulate mobile device (iPhone)
	err = chromedp.Run(
		timeoutCtx,
		emulation.SetUserAgentOverride("Mozilla/5.0 (iPhone; CPU iPhone OS 15_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/15.0 Mobile/15E148 Safari/604.1"),
		emulation.SetDeviceMetricsOverride(375, 667, 2.0, true),
		emulation.SetTouchEmulationEnabled(true),
	)
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to emulate mobile device: %s", err.Error()),
		)

		return result
	}

	// clear network cache and cookies before each run
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		err := network.ClearBrowserCache().Do(ctx)
		if err != nil {
			return err
		}

		err = network.ClearBrowserCookies().Do(ctx)
		return err
	}))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to clear browser cache and cookies: %s", err.Error()),
		)

		return result
	}

	// navigate browser to url
	nr, err := chromedp.RunResponse(timeoutCtx, chromedp.Navigate(url))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to navigate: %s", err.Error()),
		)

		return result
	}
	if nr.Status >= 400 { // if main document request failed
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to navigate: HTTP Status - %d", nr.Status),
		)

		return result
	}

	// check for missing security headers
	for _, header := range securityHeaders {
		found := false
		for key := range nr.Headers {
			if strings.EqualFold(key, header) {
				found = true
				break
			}
		}
		if !found {
			result.missingHeaders = append(result.missingHeaders, header)
		}
	}

	// wait for page to settle
	err = chromedp.Run(
		timeoutCtx,
		chromedp.WaitReady("body", chromedp.ByQuery),
		waitNetworkIdle(500*time.Millisecond, 30*time.Second),
	)
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to wait to load: %s", err.Error()),
		)

		return result
	}

	// calculate largest contentful paint time
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(`window.__lcp || 0`, &result.lcp))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to evaluate LCP: %s", err.Error()),
		)

		return result
	}

	// capture mobile responsiveness issues
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(responsiveScript, &result.responsiveIssues))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to evaluate mobile responsiveness: %s", err.Error()),
		)

		return result
	}

	// collect console errors and warnings
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(`window.__console_errors || []`, &result.consoleErrs))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to evaluate console errors: %s", err.Error()),
		)

		return result
	}

	// collect failed requests
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(`window.__request_errors || []`, &result.requestErrs))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to evaluate request errors: %s", err.Error()),
		)

		return result
	}

	// capture form issues
	err = chromedp.Run(timeoutCtx, chromedp.Evaluate(formValidationScript, &result.formIssues))
	if err != nil {
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to evaluate form issues: %s", err.Error()),
		)

		return result
	}

	return result
}

// waitNetworkIdle returns a chromedp.Action that waits until network is idle,
// similar to Puppeteer's "networkidle0".
func waitNetworkIdle(idleTime, maxWait time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		activeRequests := make(map[network.RequestID]string)
		idleTimer := time.NewTimer(idleTime)
		idleTimer.Stop()
		staticSiteTimer := time.NewTimer(1 * time.Second) // short timer for static site detection

		// common domains to ignore (analytics, tracking, favicons)
		ignored := []string{
			"google-analytics.com", "googletagmanager.com", "doubleclick.net",
			"facebook.net", "hotjar.com", "favicon.ico", "google.com/gen_204",
			"amazon-adsystem.com", "googlesyndication.com", "adsystem.amazon",
			"facebook.com/tr", "linkedin.com/px", "twitter.com/i/adsct",
			"pinterest.com/ct", "tiktok.com/i18n", "snapchat.com/p",
			"analytics", "tracking", "metrics", "telemetry", "audioeye",
			"interactions", "events", "status",
		}
		isIgnored := func(url string) bool {
			urlLower := strings.ToLower(url)
			for _, domain := range ignored {
				if strings.Contains(urlLower, domain) {
					return true
				}
			}
			return false
		}

		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *network.EventRequestWillBeSent:
				if !isIgnored(ev.Request.URL) &&
					ev.Type != network.ResourceTypeOther &&
					ev.Type != network.ResourceTypePing &&
					ev.Type != network.ResourceTypeWebSocket &&
					ev.Type != network.ResourceTypeEventSource {
					staticSiteTimer.Stop() // we have requests - not a static site
					activeRequests[ev.RequestID] = ev.Request.URL
					idleTimer.Stop()
				}
			case *network.EventLoadingFinished:
				if _, ok := activeRequests[ev.RequestID]; ok {
					delete(activeRequests, ev.RequestID)
					if len(activeRequests) == 0 {
						idleTimer.Reset(idleTime)
					}
				}
			case *network.EventLoadingFailed:
				if _, ok := activeRequests[ev.RequestID]; ok {
					delete(activeRequests, ev.RequestID)
					if len(activeRequests) == 0 {
						idleTimer.Reset(idleTime)
					}
				}
			}
		})

		timeout := time.NewTimer(maxWait)
		defer timeout.Stop()
		defer idleTimer.Stop()
		defer staticSiteTimer.Stop()

		select {
		case <-idleTimer.C:
			return nil // network became idle
		case <-staticSiteTimer.C:
			// likely a static site - wait for DOM to be ready
			return chromedp.WaitReady("body", chromedp.ByQuery).Do(ctx)
		case <-timeout.C:
			return context.DeadlineExceeded
		case <-ctx.Done():
			return ctx.Err()
		}
	})
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

// script to collect LCP time
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

	// capture resource and JS errors
	window.addEventListener('error', (e) => {
		if (e.target && (e.target.src || e.target.href)) {
			const message = (e.target.src || e.target.href) + " (type: " + e.target.tagName + ")";
			window.__request_errors.push("[Resource Load Failed]: " + message);
			return;
		}

		const message = e.message + " at " + e.filename + ":" + e.lineno + ":" + e.colno + " (" + e.error?.stack + ")";
		window.__console_errors.push("[Uncaught JS Error]: " + message);
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

	// check for horizontal scrollbar
	const horizontalBar = document.body.scrollWidth > window.innerWidth;
	if (horizontalBar) {
		__responsiveIssues.push("Has horizontal scrollbar");
	}

    // check overflowing elements
    const els = Array.from(document.querySelectorAll("*"));
    const overflowingEls = els
        .filter(el => el.scrollWidth > el.clientWidth + 5)
        .map((el, index) => {
			const overflow = (el.scrollWidth - el.clientWidth).toString();
			const tag = el.tagName.toLowerCase();
			const selector = el.id ? (tag + '#' + el.id) : 
				el.className ? (tag + '.' + el.className) : 
				(tag + ':nth-of-type(' + (index + 1) + ')');

			return selector + " (overflow: " + overflow + ")";
		})
        .slice(0, 3)
		.forEach(el => {
			__responsiveIssues.push("Overflowing element: " + el);
		});
    
    // check for viewport meta tag
    const hasViewport = !!document.querySelector('meta[name="viewport"]');
	if (!hasViewport) {
		__responsiveIssues.push("No viewport tag");
	}
    
    // check if content adapts to viewport width
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

// script to collect form issues
const formValidationScript = `(() => {
    const __formIssues = [];
    
    // iterate over all forms in the document
    document.querySelectorAll('form').forEach((form, formIndex) => {
        const formSelector = form.id ? 
            'form#' + form.id : 
            'form:nth-of-type(' + (formIndex + 1) + ')';
        
        // check for form action and method
        const formAction = form.getAttribute('action') || form.getAttribute('onsubmit');
        const formMethod = (form.getAttribute('method') || 'get').toLowerCase();
		const hasJsAttr = (form.hasAttribute('data-action') || form.hasAttribute('ng-submit') || 
			form.hasAttribute('v-on:submit') || form.hasAttribute('@submit'));
		const hasHtmxAttr = (form.hasAttribute("hx-get") || form.hasAttribute("hx-post") || 
			form.hasAttribute("hx-put") || form.hasAttribute("hx-patch") || form.hasAttribute("hx-delete"));
        
        if (!formAction && !hasJsAttr && !hasHtmxAttr) {
            __formIssues.push(formSelector + " is missing action attribute or JavaScript submit handler");
        }
        
        // check GET vs POST usage
        const hasFileInput = !!form.querySelector('input[type="file"]');
        const hasPasswordInput = !!form.querySelector('input[type="password"]');
        const hasLargeTextarea = Array.from(form.querySelectorAll('textarea'))
            .some(textarea => textarea.value.length > 2000);
            
        // forms with files, passwords, or large data should use POST
        if (formMethod === 'get' && (hasFileInput || hasPasswordInput || hasLargeTextarea)) {
			__formIssues.push(formSelector + " should use POST method for sensitive or large data submission");
        }

		// check for proper enctype for file uploads
		if (hasFileInput && form.getAttribute('enctype') !== 'multipart/form-data') {
			__formIssues.push(formSelector + " is missing proper enctype='multipart/form-data'");
		}
        
        // check for CSRF protection on non-GET forms
        if (formMethod !== 'get') {
            const possibleCsrfTokens = form.querySelectorAll('input[name*="csrf"], input[name*="token"], input[name="_token"], input[name="authenticity_token"]');
            if (possibleCsrfTokens.length === 0) {
				__formIssues.push(
					formSelector + " uses " + formMethod.toUpperCase() + " but appears to be missing CSRF protection"
				);
            }
        }
        
        // check if form has a submit button
        const hasSubmitButton = !!form.querySelector('button[type="submit"], input[type="submit"]');
        if (!hasSubmitButton) {
			__formIssues.push(formSelector + " is missing a submit button");
        }

		// check for duplicate IDs within the form
		const idMap = new Map();
		Array.from(form.querySelectorAll('[id]')).forEach(el => {
			const id = el.id;
			if (idMap.has(id)) {
				__formIssues.push(formSelector + " has duplicate IDs (" + id + ")");
			} else {
				idMap.set(id, true);
			}
		});
        
        // find all input elements excluding hidden and submit types
        const inputs = form.querySelectorAll('input:not([type="hidden"]):not([type="submit"]), select, textarea');
        inputs.forEach((input, inputIndex) => {
			const tag = input.tagName.toLowerCase()
            const inputSelector = input.id ? tag + '#' + input.id : 
                input.name ? 
                    tag + '[name="' + input.name + '"]' : 
                    tag + ':nth-of-type(' + (inputIndex + 1) + ')';
            
            // check for label association
            const hasLabel = input.id ? 
                !!document.querySelector('label[for="' + input.id + '"]') : 
                input.closest('label') !== null;
            if (!hasLabel) {
				__formIssues.push(inputSelector + " (in " + formSelector + ") lacks associated label");
            }
            
            // check for name attribute (crucial for form submission)
            if (!input.name && input.type !== 'button' && input.type !== 'submit') {
				__formIssues.push(
					inputSelector + " (in " + formSelector + ") is missing name attribute (required for form submission)"
				);
            }
            
            // check for accessibility attributes
            if (!input.getAttribute('aria-label') && !input.getAttribute('aria-labelledby') && !hasLabel) {
				__formIssues.push(inputSelector + " (in " + formSelector + ") lacks accessible name");
            }
            
            // password field specific checks
            if (input.type === 'password') {
                // check if form is served over HTTPS (simple check, more robust would be via headers)
                if (window.location.protocol !== 'https:') {
					__formIssues.push(
						inputSelector + " (in " + formSelector + ") is a password field not served over HTTPS"
					);
                }
            }

			// check for required fields without validation
			if (input.required) {
				const hasValidation = (
					input.hasAttribute('pattern') || 
					input.hasAttribute('min') || 
					input.hasAttribute('max') ||
					input.hasAttribute('minlength') || 
					input.hasAttribute('maxlength') ||
					input.type === 'email' ||
					input.type === 'url' ||
					input.type === 'number' ||
					input.type === 'date'
				);

				if (!hasValidation && input.type === 'text') {
					__formIssues.push(inputSelector + " (in " + formSelector + ") has no validation");
				}
			}
        });
    });
    
    return __formIssues;
})();`
