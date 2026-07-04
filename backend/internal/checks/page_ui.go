package checks

import (
	"fmt"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// ImageAltCheck flags images that lack an alt attribute entirely.
// (alt="" is valid for decorative images and is not flagged.)
type ImageAltCheck struct{}

func (ImageAltCheck) Name() string { return "image-alt" }

func (ImageAltCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	missing := 0
	example := ""
	for _, img := range p.Images {
		if !img.HasAlt {
			missing++
			if example == "" {
				example = img.Src
			}
		}
	}
	if missing == 0 {
		return nil
	}
	sev := models.SeverityLow
	if missing >= 5 {
		sev = models.SeverityMedium
	}
	return []models.Issue{issue(p, models.CategoryAccessibility, sev,
		"Images without alt attribute",
		fmt.Sprintf("%d image(s) have no alt attribute (e.g. %s). Screen readers cannot describe them.",
			missing, example),
		"Add descriptive alt text to meaningful images, or alt=\"\" to purely decorative ones.")}
}

// EmptyButtonCheck flags buttons with no visible text or accessible label,
// listing element hints so readers can locate each one in the page source.
type EmptyButtonCheck struct{}

func (EmptyButtonCheck) Name() string { return "empty-button" }

func (EmptyButtonCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	var (
		elements []string
		snippets []string
	)
	for _, b := range p.Buttons {
		if b.Text == "" {
			elements = append(elements, buttonLabel(b))
			if b.Snippet != "" {
				snippets = append(snippets, b.Snippet)
			}
		}
	}
	if len(elements) == 0 {
		return nil
	}
	is := issue(p, models.CategoryUI, models.SeverityMedium,
		"Empty buttons",
		fmt.Sprintf("%d button(s) have no text, value or aria-label; users and screen readers see a blank control: %s.",
			len(elements), strings.Join(capStrings(elements, 5), "; ")),
		"Give every button visible text or an aria-label.")
	is.Details = map[string]any{"elements": elements, "snippets": capStrings(snippets, 5)}
	return []models.Issue{is}
}

// ButtonActionCheck flags buttons that appear to do nothing: not inside a
// form, no handler hook, no navigation target.
type ButtonActionCheck struct{}

func (ButtonActionCheck) Name() string { return "button-without-action" }

func (ButtonActionCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	var elements []string
	for _, b := range p.Buttons {
		if !b.HasAction {
			elements = append(elements, buttonLabel(b))
		}
	}
	if len(elements) == 0 {
		return nil
	}
	is := issue(p, models.CategoryLogic, models.SeverityMedium,
		"Buttons without action",
		fmt.Sprintf("%d button(s) have no form, click handler hook, or navigation target: %s.",
			len(elements), strings.Join(capStrings(elements, 5), "; ")),
		"Wire these buttons to a form submit, link, or JavaScript handler — or remove them.")
	is.Details = map[string]any{"elements": elements}
	return []models.Issue{is}
}

// FormSubmitCheck flags forms that cannot be submitted.
type FormSubmitCheck struct{}

func (FormSubmitCheck) Name() string { return "form-without-submit" }

func (FormSubmitCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	var issues []models.Issue
	for _, f := range p.Forms {
		if f.HasSubmit || f.Inputs == 0 {
			continue
		}
		where := f.Hint
		if where == "" {
			where = fmt.Sprintf("action=%q", f.Action)
		}
		is := issue(p, models.CategoryLogic, models.SeverityMedium,
			"Form without submit control",
			fmt.Sprintf("Form %s (action=%q, %d input(s)) has no submit button; users may be unable to send it. Note: forms submitted purely via JavaScript can trip this check — verify in the browser.",
				where, f.Action, f.Inputs),
			"Add a <button type=\"submit\"> or input[type=submit] to the form.")
		is.Details = map[string]any{"element": where}
		if f.Snippet != "" {
			is.Details["snippets"] = []string{f.Snippet}
		}
		issues = append(issues, is)
	}
	if len(issues) > 3 {
		issues = issues[:3]
	}
	return issues
}

// buttonLabel describes one button for a report reader: its text when it has
// any, otherwise the element hint, otherwise its type.
func buttonLabel(b models.Button) string {
	switch {
	case b.Text != "":
		return fmt.Sprintf("%q", b.Text)
	case b.Hint != "":
		return b.Hint
	case b.Type != "":
		return "<button type=" + b.Type + ">"
	default:
		return "<button>"
	}
}

func capStrings(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	out := make([]string, n, n+1)
	copy(out, in[:n])
	return append(out, fmt.Sprintf("… +%d more", len(in)-n))
}
