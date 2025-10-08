package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
)

// audit handles auditting of websites in a headless browser
type audit struct {
	checksStr     string
	checks        auditChecks
	important     bool
	screenshotDir string
}
type auditChecks struct {
	secure           auditCheck[bool]
	lcp              auditCheck[float64]
	consoleErrs      auditCheck[[]string]
	requestErrs      auditCheck[[]string]
	missingHeaders   auditCheck[[]string]
	responsiveIssues auditCheck[[]string]
	formIssues       auditCheck[[]string]
	techStack        auditCheck[[]string]
	screenshot       auditCheck[bool]
}
type auditCheck[T interface{}] struct {
	enabled bool
	result  T
}

// newAudit creates a new audit instance
func newAudit(checksStr string, important bool) (*audit, error) {
	audit := audit{checksStr: checksStr, important: important, screenshotDir: "screenshots"}

	err := audit.parseAndValidateChecks()
	if err != nil {
		return nil, fmt.Errorf("failed to parse audit checks: %w", err)
	}

	err = audit.validateAndCreateScreenshotDir()
	if err != nil {
		return nil, fmt.Errorf("failed screenshot directory validation/creation: %w", err)
	}

	return &audit, nil
}

// parseChecks validates and specifies which audit checks to run, based on
// provided comma-separated string
func (a *audit) parseAndValidateChecks() error {
	// can't enable both important and specified checks, since they're predefined
	if a.important && a.checksStr != "" {
		return fmt.Errorf("important checks are predefined")
	}

	// set predefined important checks
	if a.important {
		a.checks = auditChecks{
			secure:           auditCheck[bool]{enabled: true},
			responsiveIssues: auditCheck[[]string]{enabled: true},
			formIssues:       auditCheck[[]string]{enabled: true},
			techStack:        auditCheck[[]string]{enabled: true},
		}
		return nil
	}

	// if no checks specified, set all enabled
	if a.checksStr == "" {
		a.checks = auditChecks{
			secure:           auditCheck[bool]{enabled: true},
			lcp:              auditCheck[float64]{enabled: true},
			consoleErrs:      auditCheck[[]string]{enabled: true},
			requestErrs:      auditCheck[[]string]{enabled: true},
			missingHeaders:   auditCheck[[]string]{enabled: true},
			responsiveIssues: auditCheck[[]string]{enabled: true},
			formIssues:       auditCheck[[]string]{enabled: true},
			techStack:        auditCheck[[]string]{enabled: true},
			screenshot:       auditCheck[bool]{enabled: true},
		}
		return nil
	}

	// all checks are disabled by default when initialising audit instance
	// enable specified ones
	for check := range strings.SplitSeq(a.checksStr, ",") {
		check = strings.TrimSpace(check)
		switch check {
		case "security":
			a.checks.secure.enabled = true
		case "lcp":
			a.checks.lcp.enabled = true
		case "console":
			a.checks.consoleErrs.enabled = true
		case "request":
			a.checks.requestErrs.enabled = true
		case "headers":
			a.checks.missingHeaders.enabled = true
		case "mobile":
			a.checks.responsiveIssues.enabled = true
		case "form":
			a.checks.formIssues.enabled = true
		case "tech":
			a.checks.techStack.enabled = true
		case "screenshot":
			a.checks.screenshot.enabled = true
		default:
			return fmt.Errorf("unknown check: %s", check)
		}
	}

	return nil
}

// validateAndCreateScreenshotDir checks whether screenshots are enabled,
// and ensures screenshot directory exists (or if not, create it)
func (a *audit) validateAndCreateScreenshotDir() error {
	if !a.checks.screenshot.enabled {
		return nil // not capturing screenshots
	}

	// ensure directory exists, else create it
	err := os.MkdirAll(a.screenshotDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create screenshot directory: %w", err)
	}

	return nil
}

// auditResult holds audit results data useful for output
type auditResult struct {
	website   string
	checks    auditChecks
	auditErrs []string
}

// run opens all sites in a headless browser and executes various checks
// before returning a set of audit results
func (a *audit) run(ctx context.Context, websites []*website) ([]auditResult, error) {
	if len(websites) == 0 {
		return nil, fmt.Errorf("no websites to audit")
	}

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
		return nil, fmt.Errorf("failed to open browser: %w", err)
	}

	sitesNo := len(websites)
	results := make([]auditResult, sitesNo)

	for i, website := range websites {
		// audit each website
		fmt.Printf("\r - auditing site %d/%d (%s)\n", i+1, sitesNo, website.domain)
		results[i] = a.runSingle(browserCtx, website)
	}

	return results, nil
}

// runSingle opens the site in a headless browser and executes various checks
// before returning an audit result
func (a *audit) runSingle(ctx context.Context, website *website) auditResult {
	result := auditResult{website: website.domain, checks: a.checks}

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

		if a.checks.lcp.enabled {
			_, err = page.AddScriptToEvaluateOnNewDocument(lcpScript).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to inject LCP script: %w", err)
			}
		}

		if a.checks.consoleErrs.enabled || a.checks.requestErrs.enabled {
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

	// emulate mobile device
	err = chromedp.Run(
		timeoutCtx,
		chromedp.Emulate(chromedp.Device(device.IPhone13)),
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

	// force site to load over http in order to check if it auto redirects
	// (if security check is enabled)
	websiteScheme := website.scheme
	if a.checks.secure.enabled {
		websiteScheme = "http"
	}

	// navigate to site and wait to settle
	nr, err := chromedp.RunResponse(timeoutCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		err := chromedp.Navigate(websiteScheme + "://" + website.domain + "/").Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to navigate: %w", err)
		}

		err = chromedp.WaitReady("body", chromedp.ByQuery).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to wait for \"body\": %w", err)
		}

		err = a.waitNetworkIdle(500*time.Millisecond, 10*time.Second).Do(ctx)
		if err != nil {
			// don't return error if check has timed out
			if !errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("failed to wait for page to be idle: %w", err)
			}

			fmt.Println("⚠️ page's idle check timed out")
		}

		err = chromedp.Sleep(1 * time.Second).Do(ctx)
		if err != nil {
			return fmt.Errorf("failed to wait for page to settle: %w", err)
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
	if a.checks.missingHeaders.enabled {
		result.checks.missingHeaders.result = a.checkSecurityHeaders(nr.Headers)
	}

	// perform checks
	err = chromedp.Run(timeoutCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		// capture site security (is HTTPS)
		if a.checks.secure.enabled {
			err := chromedp.Evaluate(securityScript, &result.checks.secure.result).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate security: %w", err)
			}
		}

		// calculate largest contentful paint time
		if a.checks.lcp.enabled {
			err := chromedp.Evaluate(`window.__lcp || 0`, &result.checks.lcp.result).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate LCP: %w", err)
			}
		}

		// capture mobile responsiveness issues
		if a.checks.responsiveIssues.enabled {
			script := fmt.Sprintf("%s(%t)", responsiveScript, a.important)
			err = chromedp.Evaluate(script, &result.checks.responsiveIssues.result).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate mobile responsiveness: %w", err)
			}
		}

		// collect console errors and warnings
		if a.checks.consoleErrs.enabled {
			err = chromedp.Evaluate(`window.__console_errors || []`, &result.checks.consoleErrs.result).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate console errors: %w", err)
			}
		}

		// collect failed requests
		if a.checks.requestErrs.enabled {
			err = chromedp.Evaluate(`window.__request_errors || []`, &result.checks.requestErrs.result).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate request errors: %w", err)
			}
		}

		// capture form issues
		if a.checks.formIssues.enabled {
			script := fmt.Sprintf("%s(%t)", formScript, a.important)
			err = chromedp.Evaluate(script, &result.checks.formIssues.result).Do(ctx)
			if err != nil {
				return fmt.Errorf("failed to evaluate form issues: %w", err)
			}
		}

		// capture common frontend technologies used
		if a.checks.techStack.enabled {
			// if important is enabled, only run check if important issues are found
			hasImportantIssues := len(result.checks.responsiveIssues.result) > 0 ||
				len(result.checks.formIssues.result) > 0

			if !a.important || hasImportantIssues {
				err = chromedp.Evaluate(techScript, &result.checks.techStack.result).Do(ctx)
				if err != nil {
					return fmt.Errorf("failed to detect tech stack: %w", err)
				}
			}
		}

		return nil
	}))
	if err != nil {
		result.auditErrs = append(result.auditErrs, err.Error())
		return result
	}

	// capture full page screenshot
	if a.checks.screenshot.enabled {
		result.checks.screenshot.result, err = a.captureScreenshot(timeoutCtx, website.domain)
		if err != nil {
			result.auditErrs = append(result.auditErrs, err.Error())
			return result
		}
	}

	return result
}

// waitNetworkIdle returns a chromedp.Action that waits until network is idle,
// similar to Puppeteer's "networkidle0"
func (a *audit) waitNetworkIdle(idleTime, maxWait time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		activeRequests := make(map[network.RequestID]string)
		idleTimer := time.NewTimer(idleTime)
		idleTimer.Stop()
		staticSiteTimer := time.NewTimer(1 * time.Second) // short timer for static site detection

		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *network.EventRequestWillBeSent:
				if !isIgnoredResource(strings.ToLower(ev.Request.URL), ignoredIdlePatterns) {
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

// patterns to ignore during idle check (analytics, tracking, chats, favicons)
var ignoredIdlePatterns = []string{
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

// checkSecurityHeaders looks for missing security headers from
// the page's main document request
func (a *audit) checkSecurityHeaders(resHeaders network.Headers) []string {
	// important security headers to check
	securityHeaders := []string{
		"Content-Security-Policy",
		"Strict-Transport-Security",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Permissions-Policy",
		"Referrer-Policy",
	}

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

// captureScreenshot takes a full page screenshot and saves it
// to disk
func (a *audit) captureScreenshot(ctx context.Context, domain string) (bool, error) {
	var screenshot []byte

	err := chromedp.Run(ctx, chromedp.FullScreenshot(&screenshot, 90))
	if err != nil {
		return false, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	// sanitise domain for filesystem
	safeDomain := a.sanitiseFilename(domain)
	filename := filepath.Join(a.screenshotDir, fmt.Sprintf("screenshot_%s.jpg", safeDomain))
	err = os.WriteFile(filename, screenshot, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to write screenshot: %w", err)
	}

	return true, nil
}

// sanitiseFilename removes characters that could cause filesystem issues
func (a *audit) sanitiseFilename(s string) string {
	// replace problematic characters
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, ":", "_")

	return s
}
