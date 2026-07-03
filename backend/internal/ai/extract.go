package ai

import (
	"github.com/ergonix/auditor/backend/internal/models"
)

// PageSummary is the compact, structured representation of a page sent to
// the LLM. Raw HTML is never sent.
type PageSummary struct {
	URL             string   `json:"url"`
	DeclaredLang    string   `json:"declaredLanguage,omitempty"`
	Title           string   `json:"title"`
	MetaDescription string   `json:"metaDescription,omitempty"`
	H1              []string `json:"h1,omitempty"`
	H2              []string `json:"h2,omitempty"`
	Buttons         []string `json:"buttons,omitempty"`
	FormActions     []string `json:"formActions,omitempty"`
	Prices          []string `json:"prices,omitempty"`
	LinkTexts       []string `json:"linkTexts,omitempty"`
	VisibleText     string   `json:"visibleText"`
}

// Summarize builds a PageSummary from a crawled page, truncating the visible
// text to maxTextChars (UTF-8 safe) and capping list fields.
func Summarize(p *models.Page, maxTextChars int) PageSummary {
	s := PageSummary{
		URL:             p.URL,
		DeclaredLang:    p.Language,
		Title:           p.Title,
		MetaDescription: p.MetaDescription,
		H1:              capList(p.H1s, 5),
		H2:              capList(p.H2s, 15),
		Prices:          capList(p.Prices, 15),
		VisibleText:     truncateUTF8(p.VisibleText, maxTextChars),
	}
	for _, b := range p.Buttons {
		if b.Text != "" {
			s.Buttons = append(s.Buttons, b.Text)
		}
	}
	s.Buttons = capList(dedupe(s.Buttons), 20)

	for _, f := range p.Forms {
		if f.Action != "" {
			s.FormActions = append(s.FormActions, f.Action)
		}
	}
	s.FormActions = capList(s.FormActions, 10)

	for _, l := range p.Links {
		if l.Internal && l.Text != "" {
			s.LinkTexts = append(s.LinkTexts, l.Text)
		}
	}
	s.LinkTexts = capList(dedupe(s.LinkTexts), 30)
	return s
}

// Richness scores how much analyzable content a page carries, used to pick
// the most valuable pages when the AI budget is smaller than the crawl.
func Richness(p *models.Page) int {
	score := len(p.VisibleText)
	score += 500 * len(p.Prices) // product-ish pages get priority
	score += 200 * len(p.H1s)
	score += 100 * len(p.Forms)
	if p.Depth == 0 {
		score += 2000 // always favor the homepage
	}
	return score
}

func capList(in []string, n int) []string {
	if len(in) <= n {
		return in
	}
	return in[:n]
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// truncateUTF8 cuts s to at most n bytes without splitting a rune.
func truncateUTF8(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	cut := n
	for cut > 0 && (s[cut]&0xC0) == 0x80 { // don't cut mid-rune
		cut--
	}
	return s[:cut]
}
