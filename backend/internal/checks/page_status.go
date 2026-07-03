package checks

import (
	"fmt"

	"github.com/ergonix/auditor/backend/internal/models"
)

// StatusCodeCheck records crawled pages that came back as errors: 404s,
// 5xx, or transport failures. It is the only check that runs on non-OK pages.
type StatusCodeCheck struct{}

func (StatusCodeCheck) Name() string { return "status-code" }

func (StatusCodeCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	switch {
	case p.FetchError != "":
		return []models.Issue{issue(p, models.CategoryNetwork, models.SeverityHigh,
			"Page failed to load",
			fmt.Sprintf("Fetching %s failed: %s.", p.URL, p.FetchError),
			"Verify the URL is reachable and the server responds within the timeout.")}
	case p.StatusCode >= 500:
		return []models.Issue{issue(p, models.CategoryNetwork, models.SeverityCritical,
			fmt.Sprintf("Server error (%d)", p.StatusCode),
			fmt.Sprintf("%s returned HTTP %d — an internal server error visible to customers.", p.URL, p.StatusCode),
			"Check server logs for the failing route and fix the underlying error.")}
	case p.StatusCode == 404 || p.StatusCode == 410:
		return []models.Issue{issue(p, models.CategoryNetwork, models.SeverityHigh,
			fmt.Sprintf("Page not found (%d)", p.StatusCode),
			fmt.Sprintf("%s returns HTTP %d but is linked from within the site.", p.URL, p.StatusCode),
			"Fix or remove the links pointing at this URL, or restore/redirect the page.")}
	case p.StatusCode >= 400:
		return []models.Issue{issue(p, models.CategoryNetwork, models.SeverityMedium,
			fmt.Sprintf("Client error (%d)", p.StatusCode),
			fmt.Sprintf("%s returned HTTP %d.", p.URL, p.StatusCode),
			"Investigate why this internally-linked URL rejects requests.")}
	}
	return nil
}
