package checks

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// templateErrorPatterns are high-precision signatures of a template engine
// failing in production output (Shopify Liquid and its i18n system).
var templateErrorPatterns = []string{"liquid error", "translation missing"}

// containsTemplateError reports whether s (URL-decoded when possible)
// carries a template-engine error signature.
func containsTemplateError(s string) bool {
	if d, err := url.QueryUnescape(s); err == nil {
		s = d
	}
	low := strings.ToLower(s)
	for _, pat := range templateErrorPatterns {
		if strings.Contains(low, pat) {
			return true
		}
	}
	return false
}

// TemplateErrorCheck finds template rendering errors that leaked into the
// page: error text in the title/description/body, or error messages baked
// into asset URLs (e.g. a favicon href of "Liquid error (layout/theme …)").
type TemplateErrorCheck struct{}

func (TemplateErrorCheck) Name() string { return "template-error" }

func (TemplateErrorCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	var found []string
	add := func(where, value string) {
		if len(found) >= 5 || !containsTemplateError(value) {
			return
		}
		if d, err := url.QueryUnescape(value); err == nil {
			value = d
		}
		if len(value) > 140 {
			value = value[:140] + "…"
		}
		found = append(found, fmt.Sprintf("%s: %q", where, value))
	}

	add("page title", p.Title)
	add("meta description", p.MetaDescription)
	for _, f := range p.Favicons {
		add("favicon URL", f)
	}
	for _, img := range p.Images {
		add("image URL", img.Src)
	}
	for _, s := range p.Scripts {
		add("script URL", s.Src)
	}
	for _, s := range p.Stylesheets {
		add("stylesheet URL", s.Src)
	}
	// Visible text last: quote the error with some surrounding context.
	low := strings.ToLower(p.VisibleText)
	for _, pat := range templateErrorPatterns {
		if idx := strings.Index(low, pat); idx >= 0 && len(found) < 5 {
			start := idx - 40
			if start < 0 {
				start = 0
			}
			end := idx + 100
			if end > len(p.VisibleText) {
				end = len(p.VisibleText)
			}
			found = append(found, fmt.Sprintf("visible text: %q", "…"+p.VisibleText[start:end]+"…"))
		}
	}

	if len(found) == 0 {
		return nil
	}
	is := issue(p, models.CategoryContent, models.SeverityHigh,
		"Template rendering error in page output",
		fmt.Sprintf("The page contains template-engine error output — the theme is failing in production. Found: %s.",
			strings.Join(found, "; ")),
		"Fix the theme template at the location named in the error message.")
	is.Details = map[string]any{"occurrences": found}
	return []models.Issue{is}
}
