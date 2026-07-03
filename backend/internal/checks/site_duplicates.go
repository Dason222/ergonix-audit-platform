package checks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// DuplicateTitleCheck flags groups of pages sharing the same <title>.
type DuplicateTitleCheck struct{}

func (DuplicateTitleCheck) Name() string { return "duplicate-title" }

func (DuplicateTitleCheck) CheckSite(sc *SiteContext) []models.Issue {
	return duplicateCheck(sc, "title",
		func(p *models.Page) string { return p.Title },
		"Duplicate page title",
		"Search engines struggle to distinguish these pages; only one may rank.",
		"Give each page a unique, descriptive title.")
}

// DuplicateMetaCheck flags groups of pages sharing the same meta description.
type DuplicateMetaCheck struct{}

func (DuplicateMetaCheck) Name() string { return "duplicate-meta" }

func (DuplicateMetaCheck) CheckSite(sc *SiteContext) []models.Issue {
	return duplicateCheck(sc, "meta description",
		func(p *models.Page) string { return p.MetaDescription },
		"Duplicate meta description",
		"Identical descriptions across pages waste snippet space and hint at template gaps.",
		"Write a unique meta description per page.")
}

func duplicateCheck(sc *SiteContext, what string, get func(*models.Page) string,
	title, why, fix string) []models.Issue {

	groups := map[string][]*models.Page{}
	seenFinal := map[string]bool{}
	for _, p := range sc.Pages {
		if !p.OK() || !p.IsHTML() {
			continue
		}
		// Two crawl URLs redirecting to the same final page are one page,
		// not a duplicate-content problem.
		final := p.FinalURL
		if final == "" {
			final = p.URL
		}
		if seenFinal[final] {
			continue
		}
		seenFinal[final] = true
		// Empty values are handled by the missing-title/meta checks.
		v := strings.TrimSpace(get(p))
		if v == "" {
			continue
		}
		groups[v] = append(groups[v], p)
	}

	var keys []string
	for v, pages := range groups {
		if len(pages) > 1 {
			keys = append(keys, v)
		}
	}
	sort.Strings(keys)

	var issues []models.Issue
	for _, v := range keys {
		pages := groups[v]
		urls := make([]string, 0, len(pages))
		for _, p := range pages {
			urls = append(urls, p.URL)
		}
		display := v
		if len(display) > 80 {
			display = display[:80] + "…"
		}
		issues = append(issues, models.Issue{
			PageURL:  urls[0],
			Category: models.CategorySEO,
			Severity: models.SeverityMedium,
			Title:    title,
			Description: fmt.Sprintf("%d pages share the %s %q: %s. %s",
				len(pages), what, display, strings.Join(urls, ", "), why),
			SuggestedFix: fix,
			Details:      map[string]any{"value": v, "pages": urls},
		})
	}
	return issues
}
