package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
func auditWebsites(ctx context.Context, urls []string, specifiedChecks string) ([]auditResult, error) {
	checksToRun := extractChecksToRun(specifiedChecks)

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

	// open browser with a blank page and wait to initialise,
	// done so performance metrics aren’t skewed by cold start overhead
	err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		err := chromedp.Navigate("about:blank").Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialise browser: %w", err)
		}

		err = chromedp.Sleep(1 * time.Second).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to wait for browser initialisation: %w", err)
		}

		return nil
	}))
	if err != nil {
		return nil, err
	}

	urlsNo := len(urls)
	results := make([]auditResult, urlsNo)

	for i, url := range urls {
		// audit each website
		fmt.Printf("auditing site %d/%d (%s)\n", i+1, urlsNo, url)
		results[i] = auditWebsite(browserCtx, url, checksToRun)

	}

	return results, nil
}

type auditCheck string

const (
	lcp              auditCheck = "lcp"
	consoleErrs      auditCheck = "console"
	requestErrs      auditCheck = "request"
	missingHeaders   auditCheck = "headers"
	responsiveIssues auditCheck = "mobile"
	formIssues       auditCheck = "form"
)

// extractChecksToRun takes in a comma-separated string and extracts
// different audit checks to run
func extractChecksToRun(checks string) []auditCheck {
	if checks == "" {
		return nil
	}

	checksToRun := []auditCheck{}

	auditChecks := strings.SplitSeq(checks, ",")
	for s := range auditChecks {
		checksToRun = append(checksToRun, auditCheck(s))
	}

	return checksToRun
}

// auditWebsite opens the URL in a headless browser and executes various checks
// before returning an audit result
func auditWebsite(ctx context.Context, url string, checksToRun []auditCheck) auditResult {
	result := auditResult{url: url}

	// create new window context
	windowCtx, cancelWindow := chromedp.NewContext(ctx)
	defer cancelWindow()

	// set context timeout
	timeoutCtx, cancelTimeout := context.WithTimeout(windowCtx, 60*time.Second)
	defer cancelTimeout()

	// open window with blank page and wait to initialise,
	// done so performance metrics aren’t skewed by cold start overhead
	err := chromedp.Run(timeoutCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		err := chromedp.Navigate("about:blank").Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to initialise window: %w", err)
		}

		err = chromedp.Sleep(1 * time.Second).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to wait for window initialisation: %w", err)
		}

		return nil
	}))
	if err != nil {
		result.auditErrs = append(result.auditErrs, err.Error())
		return result
	}

	// enable page domain and inject JS scripts to run on page
	err = chromedp.Run(timeoutCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		err := page.Enable().Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to enable page domain: %w", err)
		}

		if checksToRun == nil || slices.Contains(checksToRun, lcp) {
			_, err = page.AddScriptToEvaluateOnNewDocument(lcpScript).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to inject LCP script: %w", err)
			}
		}

		if checksToRun == nil || slices.Contains(checksToRun, consoleErrs) || slices.Contains(checksToRun, requestErrs) {
			_, err = page.AddScriptToEvaluateOnNewDocument(errScript).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to inject error script: %w", err)
			}
		}

		return nil
	}))
	if err != nil {
		result.auditErrs = append(result.auditErrs, err.Error())
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

	// enable network domain, and clear cache and cookies
	err = chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		err := network.Enable().Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to enable network domain: %w", err)
		}

		err = network.ClearBrowserCache().Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to clear browser cache: %w", err)
		}

		err = network.ClearBrowserCookies().Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to clear browser cookies: %w", err)
		}

		return nil
	}))
	if err != nil {
		result.auditErrs = append(result.auditErrs, err.Error())
		return result
	}

	// navigate to url and wait to settle
	nr, err := chromedp.RunResponse(timeoutCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		err := chromedp.Navigate(url).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to navigate: %w", err)
		}

		err = chromedp.WaitReady("body", chromedp.ByQuery).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to wait for \"body\": %w", err)
		}

		err = waitNetworkIdle(500*time.Millisecond, 10*time.Second).Do(ctx)
		if err != nil {
			// don't return error if check has timed out
			if !errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("failed to wait for page to be idle: %w", err)
			}

			fmt.Println("[warning]: page's idle check timed out")
		}

		return nil
	}))
	if err != nil {
		result.auditErrs = append(result.auditErrs, err.Error())
		return result
	}
	if nr.Status >= 400 { // if main document request failed
		result.auditErrs = append(
			result.auditErrs,
			fmt.Sprintf("failed to navigate: HTTP Status - %d", nr.Status),
		)

		return result
	}

	// capture missing security headers
	if checksToRun == nil || slices.Contains(checksToRun, missingHeaders) {
		result.missingHeaders = checkSecurityHeaders(nr.Headers)
	}

	// perform checks
	err = chromedp.Run(timeoutCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		// calculate largest contentful paint time
		if checksToRun == nil || slices.Contains(checksToRun, lcp) {
			err := chromedp.Evaluate(`window.__lcp || 0`, &result.lcp).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate LCP: %w", err)
			}
		}

		// capture mobile responsiveness issues
		if checksToRun == nil || slices.Contains(checksToRun, responsiveIssues) {
			err = chromedp.Evaluate(responsiveScript, &result.responsiveIssues).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate mobile responsiveness: %w", err)
			}
		}

		// collect console errors and warnings
		if checksToRun == nil || slices.Contains(checksToRun, consoleErrs) {
			err = chromedp.Evaluate(`window.__console_errors || []`, &result.consoleErrs).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate console errors: %w", err)
			}
		}

		// collect failed requests
		if checksToRun == nil || slices.Contains(checksToRun, requestErrs) {
			err = chromedp.Evaluate(`window.__request_errors || []`, &result.requestErrs).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate request errors: %w", err)
			}
		}

		// capture form issues
		if checksToRun == nil || slices.Contains(checksToRun, formIssues) {
			err = chromedp.Evaluate(formValidationScript, &result.formIssues).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate form issues: %w", err)
			}
		}

		return nil
	}))
	if err != nil {
		result.auditErrs = append(result.auditErrs, err.Error())
		return result
	}

	return result
}

// waitNetworkIdle returns a chromedp.Action that waits until network is idle,
// similar to Puppeteer's "networkidle0"
func waitNetworkIdle(idleTime, maxWait time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		activeRequests := make(map[network.RequestID]string)
		idleTimer := time.NewTimer(idleTime)
		idleTimer.Stop()
		staticSiteTimer := time.NewTimer(1 * time.Second) // short timer for static site detection

		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *network.EventRequestWillBeSent:
				if !isIgnoredURL(ev.Request.URL) {
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

// domains to ignore (analytics, tracking, chats, favicons)
var ignoredDomains = []string{
	"google-analytics.com", "googletagmanager.com", "doubleclick.net",
	"facebook.net", "hotjar.com", "favicon.ico", "google.com/gen_204",
	"amazon-adsystem.com", "googlesyndication.com", "adsystem.amazon",
	"facebook.com/tr", "linkedin.com/px", "twitter.com/i/adsct",
	"pinterest.com/ct", "tiktok.com/i18n", "snapchat.com/p",
	"scorecardresearch.com", "newrelic.com", "cloudflareinsights.com",
	"segment.io", "sentry.io", "monorail-edge.shopify.com",
	"shopifycloud.com", "intercom.io", "zendesk.com", "drift.com",
	"crisp.chat", "tawk.to", "livechat.com", "freshchat.com",
	"helpscout.net", "olark.com", "liveperson.net", "pusher.com",
	"analytics", "telemetry",
}

// isIgnoredURL checks if the given URL contains one of the above domains
// and patterns to helps avoid waiting on analytics, tracking, and
// non-critical resources
func isIgnoredURL(url string) bool {
	urlLower := strings.ToLower(url)

	// (web workers, service workers, generated content, etc.)
	if strings.HasPrefix(urlLower, "blob:") {
		return true
	}

	// (inline content)
	if strings.HasPrefix(urlLower, "data:") {
		return true
	}

	for _, domain := range ignoredDomains {
		if strings.Contains(urlLower, domain) {
			return true
		}
	}

	return false
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

// checkSecurityHeaders looks for missing security headers from
// the page's main document request
func checkSecurityHeaders(resHeaders network.Headers) []string {
	missingHeaders := []string{}

	for _, header := range securityHeaders {
		found := false

		for key := range resHeaders {
			if strings.EqualFold(key, header) {
				found = true
				break
			}
		}

		if !found {
			missingHeaders = append(missingHeaders, header)
		}
	}

	return missingHeaders
}
