package checks

import (
	"fmt"
	"net/http"
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
		desc := fmt.Sprintf("Link %q → %s is broken (%s); referenced from %s.",
			r.text, t, reason, strings.Join(r.pages, ", "))
		fix := "Fix the target URL, restore the destination page, or remove the link."
		confidence := 0.0 // engine default (1.0)
		// External sites often answer automated probes with 403 (bot
		// protection, e.g. Trustpilot) while working fine in a browser —
		// report it, but honestly, at reduced confidence.
		if !r.internal && res.StatusCode == http.StatusForbidden {
			confidence = 0.5
			desc += " The target may simply be blocking automated checks (bot protection)."
			fix = "Open the link in a normal browser to verify; fix or remove it only if it is genuinely dead."
		}
		// Where should the issue live? Attribute to the first referencing page.
		pageURL := ""
		if len(r.pages) > 0 {
			pageURL = r.pages[0]
		}
		issues = append(issues, models.Issue{
			PageURL:      pageURL,
			Category:     cat,
			Severity:     sev,
			Title:        fmt.Sprintf("Broken %s link", kind),
			Description:  desc,
			SuggestedFix: fix,
			Confidence:   confidence,
			Details:      map[string]any{"target": t, "reason": reason, "internal": r.internal},
		})
	}
	return issues
}
