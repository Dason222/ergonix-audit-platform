package checks

import (
	"fmt"

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

// EmptyButtonCheck flags buttons with no visible text or accessible label.
type EmptyButtonCheck struct{}

func (EmptyButtonCheck) Name() string { return "empty-button" }

func (EmptyButtonCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	empty := 0
	for _, b := range p.Buttons {
		if b.Text == "" {
			empty++
		}
	}
	if empty == 0 {
		return nil
	}
	return []models.Issue{issue(p, models.CategoryUI, models.SeverityMedium,
		"Empty buttons",
		fmt.Sprintf("%d button(s) have no text, value or aria-label; users and screen readers see a blank control.", empty),
		"Give every button visible text or an aria-label.")}
}

// ButtonActionCheck flags buttons that appear to do nothing: not inside a
// form, no handler hook, no navigation target.
type ButtonActionCheck struct{}

func (ButtonActionCheck) Name() string { return "button-without-action" }

func (ButtonActionCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	dead := 0
	example := ""
	for _, b := range p.Buttons {
		if !b.HasAction {
			dead++
			if example == "" {
				example = b.Text
			}
		}
	}
	if dead == 0 {
		return nil
	}
	desc := fmt.Sprintf("%d button(s) have no form, click handler hook, or navigation target.", dead)
	if example != "" {
		desc = fmt.Sprintf("%d button(s) appear inert, e.g. %q — no form, click handler hook, or navigation target.", dead, example)
	}
	return []models.Issue{issue(p, models.CategoryLogic, models.SeverityMedium,
		"Buttons without action", desc,
		"Wire these buttons to a form submit, link, or JavaScript handler — or remove them.")}
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
		issues = append(issues, issue(p, models.CategoryLogic, models.SeverityMedium,
			"Form without submit control",
			fmt.Sprintf("Form (action=%q, %d input(s)) has no submit button; users may be unable to send it.",
				f.Action, f.Inputs),
			"Add a <button type=\"submit\"> or input[type=submit] to the form."))
	}
	if len(issues) > 3 {
		issues = issues[:3]
	}
	return issues
}
