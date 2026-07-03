package checks

import (
	"fmt"
	"sort"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// BrokenLinkCheck reports links (internal and external) whose target is
// dead, attributing each broken target to the pages that link to it.
type BrokenLinkCheck struct{}

func (BrokenLinkCheck) Name() string { return "broken-links" }

func (BrokenLinkCheck) CheckSite(sc *SiteContext) []models.Issue {
	// target URL -> pages that reference it (first few)
	type ref struct {
		internal bool
		text     string
		pages    []string
	}
	refs := map[string]*ref{}
	for _, p := range sc.Pages {
		if !p.OK() || !p.IsHTML() {
			continue
		}
		for _, l := range p.Links {
			r := refs[l.Href]
			if r == nil {
				r = &ref{internal: l.Internal, text: l.Text}
				refs[l.Href] = r
			}
			if len(r.pages) < 3 {
				r.pages = append(r.pages, p.URL)
			}
		}
	}

	var targets []string
	for t := range refs {
		if res, ok := sc.Links[t]; ok && res.Broken() {
			targets = append(targets, t)
		}
	}
	sort.Strings(targets)

	var issues []models.Issue
	for _, t := range targets {
		r := refs[t]
		res := sc.Links[t]

		reason := res.Err
		if reason == "" {
			reason = fmt.Sprintf("HTTP %d", res.StatusCode)
		}

		kind, cat, sev := "external", models.CategoryNetwork, models.SeverityMedium
		if r.internal {
			kind, sev = "internal", models.SeverityHigh
		}
		// Where should the issue live? Attribute to the first referencing page.
		pageURL := ""
		if len(r.pages) > 0 {
			pageURL = r.pages[0]
		}
		issues = append(issues, models.Issue{
			PageURL:  pageURL,
			Category: cat,
			Severity: sev,
			Title:    fmt.Sprintf("Broken %s link", kind),
			Description: fmt.Sprintf("Link %q → %s is broken (%s); referenced from %s.",
				r.text, t, reason, strings.Join(r.pages, ", ")),
			SuggestedFix: "Fix the target URL, restore the destination page, or remove the link.",
			Details:      map[string]any{"target": t, "reason": reason, "internal": r.internal},
		})
	}
	return issues
}
