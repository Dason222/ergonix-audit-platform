package checks

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// TitleCheck flags missing, empty, too-short or too-long titles.
type TitleCheck struct{}

func (TitleCheck) Name() string { return "missing-title" }

func (TitleCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	t := strings.TrimSpace(p.Title)
	switch {
	case t == "":
		return []models.Issue{issue(p, models.CategorySEO, models.SeverityHigh,
			"Missing page title",
			"The page has no <title> element or it is empty. Search engines and browser tabs show nothing meaningful.",
			"Add a unique, descriptive <title> (roughly 30–60 characters) to this page.")}
	case len(t) > 70:
		return []models.Issue{issue(p, models.CategorySEO, models.SeverityLow,
			"Page title too long",
			fmt.Sprintf("Title is %d characters; search results truncate around 60.", len(t)),
			"Shorten the title to under ~60 characters keeping the key phrase first.")}
	}
	return nil
}

// MetaDescriptionCheck flags missing or out-of-range meta descriptions.
type MetaDescriptionCheck struct{}

func (MetaDescriptionCheck) Name() string { return "missing-meta-description" }

func (MetaDescriptionCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	d := strings.TrimSpace(p.MetaDescription)
	switch {
	case d == "":
		return []models.Issue{issue(p, models.CategorySEO, models.SeverityMedium,
			"Missing meta description",
			"The page has no <meta name=\"description\">. Search engines will improvise a snippet.",
			"Add a meta description of ~70–160 characters summarizing the page.")}
	case len(d) > 320:
		return []models.Issue{issue(p, models.CategorySEO, models.SeverityLow,
			"Meta description too long",
			fmt.Sprintf("Meta description is %d characters; snippets truncate near 160.", len(d)),
			"Trim the description to roughly 160 characters.")}
	}
	return nil
}

// H1Check flags missing or multiple H1 headings.
type H1Check struct{}

func (H1Check) Name() string { return "h1" }

func (H1Check) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	nonEmpty := 0
	for _, h := range p.H1s {
		if strings.TrimSpace(h) != "" {
			nonEmpty++
		}
	}
	switch {
	case nonEmpty == 0:
		return []models.Issue{issue(p, models.CategorySEO, models.SeverityMedium,
			"Missing H1 heading",
			"The page contains no <h1> element, weakening its content structure for SEO and accessibility.",
			"Add exactly one <h1> describing the page's main topic.")}
	case nonEmpty > 1:
		return []models.Issue{issue(p, models.CategorySEO, models.SeverityLow,
			"Multiple H1 headings",
			fmt.Sprintf("The page has %d <h1> elements; one clear main heading is recommended.", nonEmpty),
			"Keep a single <h1> and demote the others to <h2>/<h3>.")}
	}
	return nil
}

// SEOBasicsCheck covers canonical and lang-attribute problems that don't
// warrant their own check type.
type SEOBasicsCheck struct{}

func (SEOBasicsCheck) Name() string { return "seo-basics" }

func (SEOBasicsCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	var issues []models.Issue

	if strings.TrimSpace(p.Canonical) == "" {
		issues = append(issues, issue(p, models.CategorySEO, models.SeverityLow,
			"Missing canonical link",
			"No <link rel=\"canonical\"> found. Parameterized or duplicate URLs may split ranking signals.",
			"Add a canonical link pointing at the preferred URL of this page."))
	} else if cu, err := url.Parse(p.Canonical); err == nil && cu.IsAbs() {
		if pu, err2 := url.Parse(p.FinalURL); err2 == nil &&
			!strings.EqualFold(strings.TrimPrefix(cu.Host, "www."), strings.TrimPrefix(pu.Host, "www.")) {
			issues = append(issues, issue(p, models.CategorySEO, models.SeverityMedium,
				"Canonical points to a different domain",
				fmt.Sprintf("Canonical is %s but the page lives on %s. This can deindex the page here.", p.Canonical, pu.Host),
				"Point the canonical at this page's own domain unless cross-domain canonicalization is intentional."))
		}
	}

	if p.Language == "" {
		issues = append(issues, issue(p, models.CategorySEO, models.SeverityLow,
			"Missing html lang attribute",
			"The <html> element has no lang attribute; screen readers and search engines must guess the language.",
			"Set <html lang=\"…\"> to the page's language code."))
	} else if expected := ExpectedLanguage(sc.Website); expected != "" &&
		!strings.HasPrefix(strings.ToLower(p.Language), expected) {
		issues = append(issues, issue(p, models.CategorySEO, models.SeverityMedium,
			"Unexpected html lang attribute",
			fmt.Sprintf("Page declares lang=%q but the %s storefront is expected to serve %q.",
				p.Language, sc.Website, expected),
			"Serve the correct localized page or fix the lang attribute."))
	}
	return issues
}

// ExpectedLanguage maps an Ergonix storefront domain to its expected
// primary language code ("" when the domain is not recognized).
func ExpectedLanguage(website string) string {
	host := website
	if u, err := url.Parse(website); err == nil && u.Host != "" {
		host = u.Host
	}
	host = strings.ToLower(strings.TrimPrefix(host, "www."))
	switch {
	case strings.HasSuffix(host, ".lt"):
		return "lt"
	case strings.HasSuffix(host, ".lv"):
		return "lv"
	case strings.HasSuffix(host, ".ee"):
		return "et"
	case strings.HasSuffix(host, ".cz"):
		return "cs"
	case strings.HasSuffix(host, ".pl"):
		return "pl"
	}
	return ""
}

// HardcodedCountryLinkCheck finds internal-looking links that point at a
// sibling country storefront (e.g. an ergonix.lv link inside ergonix.lt),
// which usually means a hardcoded URL survived a copy between stores.
type HardcodedCountryLinkCheck struct{}

func (HardcodedCountryLinkCheck) Name() string { return "hardcoded-country-link" }

var ergonixTLDs = []string{".lt", ".lv", ".ee", ".cz", ".pl"}

func (HardcodedCountryLinkCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	own, err := url.Parse(sc.Website)
	if err != nil {
		return nil
	}
	ownHost := strings.ToLower(strings.TrimPrefix(own.Host, "www."))
	base := strings.TrimSuffix(ownHost, ownSuffix(ownHost))
	if base == ownHost { // not a recognized country domain
		return nil
	}

	seen := map[string]bool{}
	var issues []models.Issue
	for _, l := range p.Links {
		lu, err := url.Parse(l.Href)
		if err != nil {
			continue
		}
		host := strings.ToLower(strings.TrimPrefix(lu.Host, "www."))
		if host == "" || host == ownHost || seen[host] {
			continue
		}
		// A sibling storefront shares the base name with a different TLD.
		for _, tld := range ergonixTLDs {
			if host == base+tld {
				seen[host] = true
				issues = append(issues, issue(p, models.CategoryLogic, models.SeverityMedium,
					"Hardcoded link to another country store",
					fmt.Sprintf("Link %q (%q) points at sibling storefront %s instead of staying on %s.",
						l.Href, l.Text, host, ownHost),
					"Replace the hardcoded cross-country URL with a relative link or the correct local URL."))
				break
			}
		}
	}
	return issues
}

func ownSuffix(host string) string {
	for _, tld := range ergonixTLDs {
		if strings.HasSuffix(host, tld) {
			return tld
		}
	}
	return ""
}
