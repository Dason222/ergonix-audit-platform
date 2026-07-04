package checks

import (
	"fmt"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// FaviconCheck verifies the site has a working favicon: either declared via
// <link rel="icon"> (and the URL loads), or served at the conventional
// /favicon.ico. Also nudges about a missing apple-touch-icon.
type FaviconCheck struct{}

func (FaviconCheck) Name() string { return "favicon" }

func (FaviconCheck) CheckSite(sc *SiteContext) []models.Issue {
	var (
		declared    []string
		seen        = map[string]bool{}
		anyHTML     bool
		hasAppleIco bool
	)
	for _, p := range sc.Pages {
		if !p.OK() || !p.IsHTML() {
			continue
		}
		anyHTML = true
		if p.HasAppleTouchIcon {
			hasAppleIco = true
		}
		for _, f := range p.Favicons {
			if !seen[f] {
				seen[f] = true
				declared = append(declared, f)
			}
		}
	}
	if !anyHTML {
		return nil
	}

	var issues []models.Issue

	if len(declared) == 0 {
		// No icon declared anywhere — fall back to the /favicon.ico convention.
		fallback := FaviconFallbackURL(sc.Website)
		res, probed := sc.Links[fallback]
		if !probed || res.Broken() {
			reason := "and the conventional /favicon.ico does not load either"
			if !probed {
				reason = "and /favicon.ico could not be verified"
			}
			issues = append(issues, models.Issue{
				PageURL:  sc.Website,
				Category: models.CategoryUI,
				Severity: models.SeverityLow,
				Title:    "Missing favicon",
				Description: fmt.Sprintf(
					"No page declares a <link rel=\"icon\">, %s. Browser tabs, bookmarks and search results show a generic icon, which looks unfinished.", reason),
				SuggestedFix: "Add a favicon (declare <link rel=\"icon\"> in the head or serve /favicon.ico).",
			})
		}
	} else {
		// Declared icons must actually load.
		for _, f := range declared {
			// A favicon URL that is itself a template error message can
			// never load — flag it without needing a probe result.
			if containsTemplateError(f) {
				issues = append(issues, models.Issue{
					PageURL:  sc.Website,
					Category: models.CategoryUI,
					Severity: models.SeverityMedium,
					Title:    "Favicon link is broken (template error)",
					Description: fmt.Sprintf(
						"The declared favicon href is a template error message instead of a file: %s. Browser tabs show a generic icon.", f),
					SuggestedFix: "Fix the theme's favicon setting (see the template-error finding for the exact location).",
					Details:      map[string]any{"target": f},
				})
				continue
			}
			if res, ok := sc.Links[f]; ok && res.Broken() {
				reason := res.Err
				if reason == "" {
					reason = fmt.Sprintf("HTTP %d", res.StatusCode)
				}
				issues = append(issues, models.Issue{
					PageURL:  sc.Website,
					Category: models.CategoryUI,
					Severity: models.SeverityMedium,
					Title:    "Favicon link is broken",
					Description: fmt.Sprintf(
						"The declared favicon %s does not load (%s). Browser tabs show a broken/generic icon.", f, reason),
					SuggestedFix: "Fix the favicon URL or upload the missing icon file.",
					Details:      map[string]any{"target": f, "reason": reason},
				})
			}
		}
	}

	if !hasAppleIco {
		issues = append(issues, models.Issue{
			PageURL:  sc.Website,
			Category: models.CategoryUI,
			Severity: models.SeverityLow,
			Title:    "Missing apple-touch-icon",
			Description: "No <link rel=\"apple-touch-icon\"> found. iOS/Android home-screen " +
				"bookmarks will show a screenshot or generic tile instead of the brand icon.",
			SuggestedFix: "Add a 180×180 apple-touch-icon link to the page head.",
		})
	}
	return issues
}

// MobileBasicsCheck verifies template-level head hygiene: the mobile
// viewport meta and an explicit character encoding. One issue per problem
// for the whole site (these come from the shared layout, not single pages).
type MobileBasicsCheck struct{}

func (MobileBasicsCheck) Name() string { return "mobile-basics" }

func (MobileBasicsCheck) CheckSite(sc *SiteContext) []models.Issue {
	var noViewport, noCharset []string
	for _, p := range sc.Pages {
		if !p.OK() || !p.IsHTML() {
			continue
		}
		if !p.HasViewport {
			noViewport = append(noViewport, p.URL)
		}
		if !p.HasCharset {
			noCharset = append(noCharset, p.URL)
		}
	}

	var issues []models.Issue
	if len(noViewport) > 0 {
		issues = append(issues, models.Issue{
			PageURL:  noViewport[0],
			Category: models.CategoryUI,
			Severity: models.SeverityHigh,
			Title:    "Missing mobile viewport meta tag",
			Description: fmt.Sprintf(
				"%d page(s) lack <meta name=\"viewport\">, e.g. %s. Phones render them as zoomed-out desktop pages — a major problem for a store where most traffic is mobile.",
				len(noViewport), strings.Join(capStrings(noViewport, 3), ", ")),
			SuggestedFix: "Add <meta name=\"viewport\" content=\"width=device-width, initial-scale=1\"> to the layout head.",
			Details:      map[string]any{"pages": noViewport},
		})
	}
	if len(noCharset) > 0 {
		issues = append(issues, models.Issue{
			PageURL:  noCharset[0],
			Category: models.CategoryUI,
			Severity: models.SeverityLow,
			Title:    "Missing charset declaration",
			Description: fmt.Sprintf(
				"%d page(s) declare no character encoding, e.g. %s. Lithuanian/Czech/Polish characters may render incorrectly on some clients.",
				len(noCharset), strings.Join(capStrings(noCharset, 3), ", ")),
			SuggestedFix: "Add <meta charset=\"utf-8\"> as the first element of the head.",
			Details:      map[string]any{"pages": noCharset},
		})
	}
	return issues
}
