package checks

import (
	"fmt"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// ConsoleErrorCheck surfaces JavaScript console errors captured by the
// browser pass. Pages without browser data produce nothing.
type ConsoleErrorCheck struct{}

func (ConsoleErrorCheck) Name() string { return "console-errors" }

func (ConsoleErrorCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	if len(p.ConsoleErrors) == 0 {
		return nil
	}
	examples := p.ConsoleErrors
	if len(examples) > 3 {
		examples = examples[:3]
	}
	sev := models.SeverityMedium
	if len(p.ConsoleErrors) >= 5 {
		sev = models.SeverityHigh
	}
	return []models.Issue{issue(p, models.CategoryJavaScript, sev,
		"JavaScript console errors",
		fmt.Sprintf("%d console error(s) during page load, e.g.: %s",
			len(p.ConsoleErrors), strings.Join(examples, " | ")),
		"Open the browser devtools on this page and fix the reported script errors.")}
}

// FailedRequestCheck surfaces network requests that failed or returned 4xx/5xx
// during the browser pass.
type FailedRequestCheck struct{}

func (FailedRequestCheck) Name() string { return "failed-requests" }

func (FailedRequestCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	if len(p.FailedRequests) == 0 {
		return nil
	}
	examples := p.FailedRequests
	if len(examples) > 3 {
		examples = examples[:3]
	}
	return []models.Issue{issue(p, models.CategoryNetwork, models.SeverityMedium,
		"Failed network requests",
		fmt.Sprintf("%d request(s) failed while loading the page, e.g.: %s",
			len(p.FailedRequests), strings.Join(examples, " | ")),
		"Fix or remove references to the failing resources.")}
}

// RedirectCheck flags redirect chains that are too long, loops, and
// unexpected cross-domain redirects of internal URLs.
type RedirectCheck struct{}

func (RedirectCheck) Name() string { return "redirects" }

func (RedirectCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	var issues []models.Issue

	if strings.Contains(p.FetchError, "redirect loop") {
		issues = append(issues, issue(p, models.CategoryNetwork, models.SeverityCritical,
			"Redirect loop",
			fmt.Sprintf("%s never resolves — the server keeps redirecting.", p.URL),
			"Fix the redirect rules so the URL reaches a final page."))
	}

	if n := len(p.RedirectChain); n > 1 {
		hops := n - 1
		if hops > sc.Cfg.MaxRedirects {
			issues = append(issues, issue(p, models.CategoryPerformance, models.SeverityMedium,
				"Long redirect chain",
				fmt.Sprintf("Reaching %s takes %d redirects: %s.", p.URL, hops,
					strings.Join(p.RedirectChain, " → ")),
				"Point links directly at the final URL."))
		}
		// Unexpected redirect: internal URL ends up on a different domain.
		first, last := p.RedirectChain[0], p.RedirectChain[len(p.RedirectChain)-1]
		if hostOf(first) != "" && hostOf(last) != "" &&
			trimWWW(hostOf(first)) != trimWWW(hostOf(last)) {
			issues = append(issues, issue(p, models.CategoryLogic, models.SeverityMedium,
				"Unexpected cross-domain redirect",
				fmt.Sprintf("Internal URL %s redirects off-site to %s.", first, last),
				"Confirm this redirect is intentional; customers may be leaving the store unexpectedly."))
		}
	}
	return issues
}

func hostOf(rawURL string) string {
	if i := strings.Index(rawURL, "://"); i >= 0 {
		rest := rawURL[i+3:]
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			return strings.ToLower(rest[:j])
		}
		return strings.ToLower(rest)
	}
	return ""
}

func trimWWW(h string) string { return strings.TrimPrefix(h, "www.") }
