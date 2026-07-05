package checks

import (
	"fmt"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// MixedContentCheck flags http:// assets loaded on https pages (detected
// during extraction).
type MixedContentCheck struct{}

func (MixedContentCheck) Name() string { return "mixed-content" }

func (MixedContentCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	if len(p.MixedContent) == 0 {
		return nil
	}
	examples := p.MixedContent
	if len(examples) > 3 {
		examples = examples[:3]
	}
	return []models.Issue{issue(p, models.CategorySecurity, models.SeverityHigh,
		"Mixed content on HTTPS page",
		fmt.Sprintf("%d asset(s) load over insecure http://, e.g. %s. Browsers block or warn about these.",
			len(p.MixedContent), strings.Join(examples, ", ")),
		"Serve all images, scripts and stylesheets over https://.")}
}

// InsecureFormCheck flags forms on HTTPS pages that submit to plain http://
// — credentials/personal data would leave the page unencrypted.
type InsecureFormCheck struct{}

func (InsecureFormCheck) Name() string { return "insecure-form" }

func (InsecureFormCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	if !strings.HasPrefix(strings.ToLower(p.FinalURL), "https://") {
		return nil
	}
	var issues []models.Issue
	for _, f := range p.Forms {
		if strings.HasPrefix(strings.ToLower(f.Action), "http://") {
			issues = append(issues, issue(p, models.CategorySecurity, models.SeverityCritical,
				"Form submits over insecure HTTP",
				fmt.Sprintf("Form %s posts to %s — data entered by the customer would be sent unencrypted.", f.Hint, f.Action),
				"Change the form action to https://."))
		}
	}
	return issues
}

// SecurityHeadersCheck (site-level) inspects the response headers of the
// crawled pages: missing hardening headers and version disclosure.
type SecurityHeadersCheck struct{}

func (SecurityHeadersCheck) Name() string { return "security-headers" }

func (SecurityHeadersCheck) CheckSite(sc *SiteContext) []models.Issue {
	// Use the first successfully fetched HTML page (headers are set by the
	// platform, so one sample represents the site).
	var sample *models.Page
	for _, p := range sc.Pages {
		if p.OK() && p.IsHTML() && p.Headers != nil {
			sample = p
			break
		}
	}
	if sample == nil {
		return nil
	}
	var issues []models.Issue

	var missing []string
	if strings.HasPrefix(strings.ToLower(sample.FinalURL), "https://") &&
		sample.Headers["Strict-Transport-Security"] == "" {
		missing = append(missing, "Strict-Transport-Security (HSTS)")
	}
	if sample.Headers["Content-Security-Policy"] == "" {
		missing = append(missing, "Content-Security-Policy")
	}
	if sample.Headers["X-Content-Type-Options"] == "" {
		missing = append(missing, "X-Content-Type-Options: nosniff")
	}
	// Clickjacking protection: X-Frame-Options or a CSP frame-ancestors.
	if sample.Headers["X-Frame-Options"] == "" &&
		!strings.Contains(strings.ToLower(sample.Headers["Content-Security-Policy"]), "frame-ancestors") {
		missing = append(missing, "X-Frame-Options / frame-ancestors (clickjacking)")
	}
	if sample.Headers["Referrer-Policy"] == "" {
		missing = append(missing, "Referrer-Policy")
	}
	if len(missing) > 0 {
		sev := models.SeverityMedium
		if len(missing) >= 4 {
			sev = models.SeverityHigh
		}
		issues = append(issues, models.Issue{
			PageURL:  sample.URL,
			Category: models.CategorySecurity,
			Severity: sev,
			Title:    "Missing security headers",
			Description: fmt.Sprintf("Responses lack: %s. These headers protect customers against protocol downgrade, clickjacking, XSS and MIME-sniffing attacks.",
				strings.Join(missing, "; ")),
			SuggestedFix: "Configure the missing security headers at the platform/CDN level.",
			Details:      map[string]any{"missing": missing},
		})
	}

	for _, h := range []string{"Server", "X-Powered-By"} {
		if v := sample.Headers[h]; v != "" && strings.ContainsAny(v, "0123456789") {
			issues = append(issues, models.Issue{
				PageURL:  sample.URL,
				Category: models.CategorySecurity,
				Severity: models.SeverityLow,
				Title:    "Server version disclosure",
				Description: fmt.Sprintf("The %s header exposes software/version details (%q), helping attackers pick known exploits.", h, v),
				SuggestedFix: "Strip or genericize the " + h + " header.",
			})
		}
	}
	return issues
}

// TabnabbingCheck flags links that open in a new tab (target="_blank")
// without rel="noopener" — the opened page can hijack the original tab via
// window.opener (reverse tabnabbing / phishing).
type TabnabbingCheck struct{}

func (TabnabbingCheck) Name() string { return "tabnabbing" }

func (TabnabbingCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	var vulnerable []string
	for _, l := range p.Links {
		if l.TargetBlank && !l.NoOpener && !l.Internal {
			vulnerable = append(vulnerable, l.Href)
		}
	}
	if len(vulnerable) == 0 {
		return nil
	}
	is := issue(p, models.CategorySecurity, models.SeverityLow,
		"Links vulnerable to reverse tabnabbing",
		fmt.Sprintf("%d external link(s) use target=\"_blank\" without rel=\"noopener\", e.g. %s. The opened site can redirect this tab to a phishing page.",
			len(vulnerable), strings.Join(capStrings(vulnerable, 3), ", ")),
		"Add rel=\"noopener noreferrer\" to links that open in a new tab.")
	is.Details = map[string]any{"links": vulnerable}
	return []models.Issue{is}
}

// SRICheck flags third-party scripts loaded without Subresource Integrity.
// If the external host is compromised, malicious JS runs on the store with
// no integrity guard.
type SRICheck struct{}

func (SRICheck) Name() string { return "subresource-integrity" }

func (SRICheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	var unprotected []string
	for _, s := range p.Scripts {
		if s.External && !s.HasIntegrity && s.Src != "" {
			unprotected = append(unprotected, s.Src)
		}
	}
	if len(unprotected) == 0 {
		return nil
	}
	is := issue(p, models.CategorySecurity, models.SeverityLow,
		"Third-party scripts without Subresource Integrity",
		fmt.Sprintf("%d external script(s) load without an integrity hash, e.g. %s. A compromise of the CDN would execute arbitrary code on the store.",
			len(unprotected), strings.Join(capStrings(unprotected, 3), ", ")),
		"Add an integrity (SRI) hash and crossorigin attribute to third-party <script> tags where possible.")
	is.Confidence = 0.6 // many CDNs rotate URLs and can't use static SRI
	is.Details = map[string]any{"scripts": unprotected}
	return []models.Issue{is}
}

// HTTPLinkCheck flags plain-http anchor links on https pages (navigation,
// not assets — assets are MixedContentCheck's job).
type HTTPLinkCheck struct{}

func (HTTPLinkCheck) Name() string { return "http-link" }

func (HTTPLinkCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	if !strings.HasPrefix(strings.ToLower(p.FinalURL), "https://") {
		return nil
	}
	var httpLinks []string
	for _, l := range p.Links {
		if strings.HasPrefix(strings.ToLower(l.Href), "http://") {
			httpLinks = append(httpLinks, l.Href)
		}
	}
	if len(httpLinks) == 0 {
		return nil
	}
	examples := httpLinks
	if len(examples) > 3 {
		examples = examples[:3]
	}
	return []models.Issue{issue(p, models.CategorySecurity, models.SeverityMedium,
		"HTTP links on HTTPS page",
		fmt.Sprintf("%d link(s) point to insecure http:// URLs, e.g. %s.",
			len(httpLinks), strings.Join(examples, ", ")),
		"Update the links to https:// (or protocol-relative) targets.")}
}
