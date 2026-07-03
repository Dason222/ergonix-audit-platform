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
