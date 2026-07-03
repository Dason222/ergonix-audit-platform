package crawler

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/ergonix/auditor/backend/internal/models"
)

// priceRe matches European e-commerce price formats: "12,99 €", "€12.99",
// "1 299,00 Kč", "49,99 zł", "12.99 EUR", "od 15 €" etc.
var priceRe = regexp.MustCompile(
	`(?:€|EUR|Kč|zł|PLN|CZK)\s*\d{1,3}(?:[ .\x{00A0}]?\d{3})*(?:[.,]\d{1,2})?` +
		`|\d{1,3}(?:[ .\x{00A0}]?\d{3})*(?:[.,]\d{1,2})?\s*(?:€|EUR|Kč|zł|PLN|CZK)`)

var whitespaceRe = regexp.MustCompile(`[ \t\r\n\x{00A0}]+`)

// ExtractPage fills p with everything parseable from the HTML document.
// pageURL must be the final (post-redirect) URL for correct link resolution.
func ExtractPage(p *models.Page, doc *goquery.Document, pageURL *url.URL) {
	p.Title = strings.TrimSpace(doc.Find("head title").First().Text())
	p.MetaDescription, _ = doc.Find(`head meta[name="description"]`).First().Attr("content")
	p.MetaDescription = strings.TrimSpace(p.MetaDescription)
	p.Canonical, _ = doc.Find(`head link[rel="canonical"]`).First().Attr("href")
	p.Canonical = strings.TrimSpace(p.Canonical)
	p.Language, _ = doc.Find("html").First().Attr("lang")
	p.Language = strings.ToLower(strings.TrimSpace(p.Language))

	doc.Find("h1").Each(func(_ int, s *goquery.Selection) {
		p.H1s = append(p.H1s, collapseSpace(s.Text()))
	})
	doc.Find("h2").Each(func(_ int, s *goquery.Selection) {
		if t := collapseSpace(s.Text()); t != "" {
			p.H2s = append(p.H2s, t)
		}
	})

	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, _ := s.Attr("src")
		if src == "" {
			src, _ = s.Attr("data-src")
		}
		alt, hasAlt := s.Attr("alt")
		p.Images = append(p.Images, models.Image{
			Src:    resolveRef(pageURL, src),
			Alt:    strings.TrimSpace(alt),
			HasAlt: hasAlt,
		})
	})

	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		norm := NormalizeURL(pageURL, href)
		if norm == "" {
			return
		}
		rel, _ := s.Attr("rel")
		p.Links = append(p.Links, models.Link{
			Href:     norm,
			Text:     collapseSpace(s.Text()),
			Internal: SameSite(pageURL, norm),
			Nofollow: strings.Contains(strings.ToLower(rel), "nofollow"),
		})
	})

	doc.Find("form").Each(func(_ int, s *goquery.Selection) {
		action, _ := s.Attr("action")
		method, _ := s.Attr("method")
		hasSubmit := s.Find(`input[type="submit"], button[type="submit"]`).Length() > 0
		// A <button> inside a form without an explicit type defaults to submit.
		if !hasSubmit {
			s.Find("button").EachWithBreak(func(_ int, b *goquery.Selection) bool {
				t, ok := b.Attr("type")
				if !ok || strings.EqualFold(t, "submit") {
					hasSubmit = true
					return false
				}
				return true
			})
		}
		p.Forms = append(p.Forms, models.Form{
			Action:    resolveRef(pageURL, action),
			Method:    strings.ToUpper(strings.TrimSpace(orDefault(method, "GET"))),
			Inputs:    s.Find("input, select, textarea").Length(),
			HasSubmit: hasSubmit,
		})
	})

	doc.Find(`button, input[type="button"], input[type="submit"], a[role="button"]`).
		Each(func(_ int, s *goquery.Selection) {
			text := collapseSpace(s.Text())
			if text == "" {
				if v, ok := s.Attr("value"); ok {
					text = collapseSpace(v)
				} else if v, ok := s.Attr("aria-label"); ok {
					text = collapseSpace(v)
				}
			}
			btnType, _ := s.Attr("type")
			p.Buttons = append(p.Buttons, models.Button{
				Text:      text,
				Type:      strings.ToLower(btnType),
				HasAction: buttonHasAction(s),
			})
		})

	doc.Find("script").Each(func(_ int, s *goquery.Selection) {
		if src, ok := s.Attr("src"); ok && src != "" {
			p.Scripts = append(p.Scripts, models.Resource{Src: resolveRef(pageURL, src)})
		} else if len(strings.TrimSpace(s.Text())) > 0 {
			p.Scripts = append(p.Scripts, models.Resource{Inline: true, SizeBytes: int64(len(s.Text()))})
		}
	})
	doc.Find(`link[rel="stylesheet"]`).Each(func(_ int, s *goquery.Selection) {
		if href, ok := s.Attr("href"); ok && href != "" {
			p.Stylesheets = append(p.Stylesheets, models.Resource{Src: resolveRef(pageURL, href)})
		}
	})
	doc.Find("style").Each(func(_ int, s *goquery.Selection) {
		p.Stylesheets = append(p.Stylesheets, models.Resource{Inline: true, SizeBytes: int64(len(s.Text()))})
	})

	p.VisibleText = extractVisibleText(doc)
	p.Prices = uniqueStrings(priceRe.FindAllString(p.VisibleText, 40))
	p.MixedContent = findMixedContent(p, pageURL)
}

// extractVisibleText returns the page's human-visible text with scripts,
// styles and hidden metadata removed, whitespace-collapsed.
func extractVisibleText(doc *goquery.Document) string {
	clone := doc.Selection.Find("body").Clone()
	clone.Find("script, style, noscript, template, svg, iframe").Remove()
	text := clone.Text()
	text = whitespaceRe.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)
	const maxLen = 20000
	if len(text) > maxLen {
		text = text[:maxLen]
	}
	return text
}

// findMixedContent lists http:// asset references on an https page.
func findMixedContent(p *models.Page, pageURL *url.URL) []string {
	if pageURL.Scheme != "https" {
		return nil
	}
	var out []string
	add := func(src string) {
		if strings.HasPrefix(strings.ToLower(src), "http://") {
			out = append(out, src)
		}
	}
	for _, img := range p.Images {
		add(img.Src)
	}
	for _, s := range p.Scripts {
		add(s.Src)
	}
	for _, s := range p.Stylesheets {
		add(s.Src)
	}
	return uniqueStrings(out)
}

// buttonHasAction decides whether a button element does anything: submits a
// form, has a JS handler, or navigates.
func buttonHasAction(s *goquery.Selection) bool {
	if _, ok := s.Attr("onclick"); ok {
		return true
	}
	t, _ := s.Attr("type")
	t = strings.ToLower(t)
	if t == "submit" || t == "reset" {
		return s.ParentsFiltered("form").Length() > 0
	}
	if goquery.NodeName(s) == "a" {
		href, ok := s.Attr("href")
		return ok && href != "" && href != "#"
	}
	if s.ParentsFiltered("form").Length() > 0 && (t == "" || t == "image") {
		return true // implicit submit button inside a form
	}
	// Buttons commonly wired up via JS: give benefit of the doubt only when
	// they carry an id/class/data-* hook a script could target.
	if _, ok := s.Attr("id"); ok {
		return true
	}
	if _, ok := s.Attr("data-action"); ok {
		return true
	}
	for _, attr := range s.Nodes[0].Attr {
		if strings.HasPrefix(attr.Key, "data-") || strings.HasPrefix(attr.Key, "aria-controls") {
			return true
		}
	}
	if cls, ok := s.Attr("class"); ok && cls != "" {
		return true
	}
	return false
}

func resolveRef(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	u, err := base.Parse(ref)
	if err != nil {
		return ref
	}
	return u.String()
}

func collapseSpace(s string) string {
	return strings.TrimSpace(whitespaceRe.ReplaceAllString(s, " "))
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
