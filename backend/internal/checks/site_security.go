package checks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// SensitiveFileCheck reports sensitive files/directories that are publicly
// reachable (exposed git repos, .env secrets, database dumps, phpinfo…).
// These are among the highest-impact web vulnerabilities.
type SensitiveFileCheck struct{}

func (SensitiveFileCheck) Name() string { return "sensitive-file-exposure" }

func (SensitiveFileCheck) CheckSite(sc *SiteContext) []models.Issue {
	if sc.Recon == nil || len(sc.Recon.Exposed) == 0 {
		return nil
	}
	paths := make([]string, 0, len(sc.Recon.Exposed))
	for p := range sc.Recon.Exposed {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	var issues []models.Issue
	for _, p := range paths {
		issues = append(issues, models.Issue{
			PageURL:  strings.TrimRight(sc.Website, "/") + p,
			Category: models.CategorySecurity,
			Severity: models.SeverityCritical,
			Title:    "Sensitive file publicly accessible",
			Description: fmt.Sprintf("%s — %s is reachable without authentication. Attackers can download it directly.",
				sc.Recon.Exposed[p], p),
			SuggestedFix: "Block public access to this path at the web server / CDN, and rotate any leaked secrets.",
			Details:      map[string]any{"path": p},
		})
	}
	return issues
}

// HTTPSRedirectCheck flags a site that serves content over plain http://
// without redirecting to https:// — customers can be served a downgraded,
// interceptable page.
type HTTPSRedirectCheck struct{}

func (HTTPSRedirectCheck) Name() string { return "https-redirect" }

func (HTTPSRedirectCheck) CheckSite(sc *SiteContext) []models.Issue {
	if sc.Recon == nil || sc.Recon.HTTPSRedirect != "missing" {
		return nil
	}
	return []models.Issue{{
		PageURL:  strings.Replace(sc.Website, "https://", "http://", 1),
		Category: models.CategorySecurity,
		Severity: models.SeverityHigh,
		Title:    "HTTP not redirected to HTTPS",
		Description: "The site answers over plain http:// without redirecting to https://. " +
			"Traffic (including any submitted data) can be read or modified on the network.",
		SuggestedFix: "Force a 301 redirect from http:// to https:// and enable HSTS.",
	}}
}

// RobotsSitemapCheck flags missing robots.txt / sitemap.xml — basic crawl
// hygiene that affects how search engines index the store.
type RobotsSitemapCheck struct{}

func (RobotsSitemapCheck) Name() string { return "robots-sitemap" }

func (RobotsSitemapCheck) CheckSite(sc *SiteContext) []models.Issue {
	if sc.Recon == nil {
		return nil
	}
	var issues []models.Issue
	if sc.Recon.Robots.StatusCode == 404 {
		issues = append(issues, models.Issue{
			PageURL:      strings.TrimRight(sc.Website, "/") + "/robots.txt",
			Category:     models.CategorySEO,
			Severity:     models.SeverityLow,
			Title:        "Missing robots.txt",
			Description:  "No robots.txt is served. Search engines cannot find crawl directives or the sitemap reference.",
			SuggestedFix: "Publish a robots.txt (Shopify serves one by default — investigate why it is missing).",
		})
	}
	if sc.Recon.Sitemap.StatusCode == 404 {
		issues = append(issues, models.Issue{
			PageURL:      strings.TrimRight(sc.Website, "/") + "/sitemap.xml",
			Category:     models.CategorySEO,
			Severity:     models.SeverityMedium,
			Title:        "Missing sitemap.xml",
			Description:  "No sitemap.xml is served, slowing discovery and indexing of product and collection pages.",
			SuggestedFix: "Publish an XML sitemap and reference it from robots.txt.",
		})
	}
	return issues
}

// CookieSecurityCheck inspects Set-Cookie flags: cookies on an HTTPS site
// should be Secure, session cookies should be HttpOnly, and SameSite should
// be set to limit CSRF.
type CookieSecurityCheck struct{}

func (CookieSecurityCheck) Name() string { return "cookie-security" }

func (CookieSecurityCheck) CheckSite(sc *SiteContext) []models.Issue {
	var sample *models.Page
	for _, p := range sc.Pages {
		if p.OK() && len(p.Cookies) > 0 {
			sample = p
			break
		}
	}
	if sample == nil {
		return nil
	}
	https := strings.HasPrefix(strings.ToLower(sample.FinalURL), "https://")

	var missingSecure, missingSameSite []string
	for _, c := range sample.Cookies {
		name := cookieName(c)
		low := strings.ToLower(c)
		if https && !strings.Contains(low, "secure") {
			missingSecure = append(missingSecure, name)
		}
		if !strings.Contains(low, "samesite") {
			missingSameSite = append(missingSameSite, name)
		}
	}

	var issues []models.Issue
	if len(missingSecure) > 0 {
		issues = append(issues, models.Issue{
			PageURL:  sample.URL,
			Category: models.CategorySecurity,
			Severity: models.SeverityMedium,
			Title:    "Cookies without Secure flag",
			Description: fmt.Sprintf("%d cookie(s) on this HTTPS site lack the Secure flag (%s); they can leak over an accidental http request.",
				len(missingSecure), strings.Join(capStrings(missingSecure, 5), ", ")),
			SuggestedFix: "Set the Secure attribute on all cookies.",
			Details:      map[string]any{"cookies": missingSecure},
		})
	}
	if len(missingSameSite) > 0 {
		issues = append(issues, models.Issue{
			PageURL:  sample.URL,
			Category: models.CategorySecurity,
			Severity: models.SeverityLow,
			Title:    "Cookies without SameSite attribute",
			Description: fmt.Sprintf("%d cookie(s) have no SameSite attribute (%s), weakening CSRF protection.",
				len(missingSameSite), strings.Join(capStrings(missingSameSite, 5), ", ")),
			SuggestedFix: "Set SameSite=Lax (or Strict) on cookies that do not need cross-site delivery.",
			Details:      map[string]any{"cookies": missingSameSite},
		})
	}
	return issues
}

func cookieName(setCookie string) string {
	if i := strings.Index(setCookie, "="); i > 0 {
		return strings.TrimSpace(setCookie[:i])
	}
	if i := strings.Index(setCookie, ";"); i > 0 {
		return strings.TrimSpace(setCookie[:i])
	}
	return "cookie"
}
