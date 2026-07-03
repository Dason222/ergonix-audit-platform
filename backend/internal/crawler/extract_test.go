package crawler

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"github.com/ergonix/auditor/backend/internal/models"
)

const fixtureHTML = `<!DOCTYPE html>
<html lang="lt">
<head>
  <title>Ergonominė kėdė | Ergonix</title>
  <meta name="description" content="Geriausia ergonominė kėdė namams ir biurui.">
  <link rel="canonical" href="https://ergonix.lt/products/kede">
  <link rel="stylesheet" href="/assets/main.css">
  <style>.x{color:red}</style>
</head>
<body>
  <h1>Ergonominė kėdė</h1>
  <h2>Aprašymas</h2>
  <h2>  </h2>
  <img src="/img/kede.jpg" alt="Kėdė">
  <img src="/img/no-alt.jpg">
  <img src="http://insecure.example.com/pix.png" alt="">
  <a href="/products/kita-kede">Kita kėdė</a>
  <a href="https://ergonix.lv/products/kede">LV versija</a>
  <a href="/blog?utm_source=x" rel="nofollow">Blogas</a>
  <a href="javascript:void(0)">JS link</a>
  <form action="/search" method="get">
    <input type="text" name="q">
    <button type="submit">Ieškoti</button>
  </form>
  <form action="/subscribe">
    <input type="email" name="email">
  </form>
  <button type="button"></button>
  <button onclick="addToCart()">Į krepšelį</button>
  <p>Kaina: 129,99 € su PVM. Sena kaina 1 299,00 €.</p>
  <script src="/assets/app.js"></script>
  <script>console.log("inline")</script>
</body>
</html>`

func extractFixture(t *testing.T) *models.Page {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(fixtureHTML))
	if err != nil {
		t.Fatal(err)
	}
	p := &models.Page{URL: "https://ergonix.lt/products/kede"}
	ExtractPage(p, doc, mustParse(t, "https://ergonix.lt/products/kede"))
	return p
}

func TestExtractBasics(t *testing.T) {
	p := extractFixture(t)

	if p.Title != "Ergonominė kėdė | Ergonix" {
		t.Errorf("title = %q", p.Title)
	}
	if p.MetaDescription == "" || p.Canonical != "https://ergonix.lt/products/kede" {
		t.Errorf("meta=%q canonical=%q", p.MetaDescription, p.Canonical)
	}
	if p.Language != "lt" {
		t.Errorf("lang = %q", p.Language)
	}
	if len(p.H1s) != 1 || p.H1s[0] != "Ergonominė kėdė" {
		t.Errorf("h1s = %v", p.H1s)
	}
	if len(p.H2s) != 1 { // empty h2 skipped
		t.Errorf("h2s = %v", p.H2s)
	}
}

func TestExtractImagesAltDetection(t *testing.T) {
	p := extractFixture(t)
	if len(p.Images) != 3 {
		t.Fatalf("images = %d, want 3", len(p.Images))
	}
	if !p.Images[0].HasAlt || p.Images[0].Alt != "Kėdė" {
		t.Errorf("img0: %+v", p.Images[0])
	}
	if p.Images[1].HasAlt {
		t.Errorf("img1 should have no alt: %+v", p.Images[1])
	}
	if !p.Images[2].HasAlt || p.Images[2].Alt != "" {
		t.Errorf("img2 should have empty alt: %+v", p.Images[2])
	}
}

func TestExtractLinks(t *testing.T) {
	p := extractFixture(t)
	if len(p.Links) != 3 { // js link dropped
		t.Fatalf("links = %d, want 3: %+v", len(p.Links), p.Links)
	}
	if !p.Links[0].Internal {
		t.Error("first link should be internal")
	}
	if p.Links[1].Internal {
		t.Error("ergonix.lv is a different site")
	}
	if !p.Links[2].Nofollow {
		t.Error("nofollow flag lost")
	}
	if strings.Contains(p.Links[2].Href, "utm_source") {
		t.Error("tracking param should be stripped")
	}
}

func TestExtractFormsAndButtons(t *testing.T) {
	p := extractFixture(t)
	if len(p.Forms) != 2 {
		t.Fatalf("forms = %d", len(p.Forms))
	}
	if !p.Forms[0].HasSubmit || p.Forms[0].Method != "GET" {
		t.Errorf("form0: %+v", p.Forms[0])
	}
	if p.Forms[1].HasSubmit {
		t.Errorf("form1 has no submit: %+v", p.Forms[1])
	}

	// button[type=submit], empty button[type=button], onclick button
	if len(p.Buttons) != 3 {
		t.Fatalf("buttons = %d: %+v", len(p.Buttons), p.Buttons)
	}
	var empty, onclick *models.Button
	for i := range p.Buttons {
		switch p.Buttons[i].Text {
		case "":
			empty = &p.Buttons[i]
		case "Į krepšelį":
			onclick = &p.Buttons[i]
		}
	}
	if empty == nil || empty.HasAction {
		t.Errorf("empty type=button with no hooks should have no action: %+v", empty)
	}
	if onclick == nil || !onclick.HasAction {
		t.Errorf("onclick button should have action: %+v", onclick)
	}
}

func TestExtractPricesAndMixedContent(t *testing.T) {
	p := extractFixture(t)
	if len(p.Prices) != 2 {
		t.Errorf("prices = %v, want 2 matches", p.Prices)
	}
	if len(p.MixedContent) != 1 || !strings.Contains(p.MixedContent[0], "insecure.example.com") {
		t.Errorf("mixed content = %v", p.MixedContent)
	}
	if len(p.Scripts) != 2 || len(p.Stylesheets) != 2 {
		t.Errorf("scripts=%d stylesheets=%d", len(p.Scripts), len(p.Stylesheets))
	}
	if !strings.Contains(p.VisibleText, "Kaina") || strings.Contains(p.VisibleText, "console.log") {
		t.Errorf("visible text wrong: %q", p.VisibleText[:min(120, len(p.VisibleText))])
	}
}
