package checks

import (
	"strings"
	"testing"

	"github.com/ergonix/auditor/backend/internal/models"
)

// goodPage returns a page that should pass every check.
func goodPage() *models.Page {
	return &models.Page{
		Website:         "https://ergonix.lt",
		URL:             "https://ergonix.lt/products/kede",
		FinalURL:        "https://ergonix.lt/products/kede",
		StatusCode:      200,
		ContentType:     "text/html; charset=utf-8",
		Title:           "Ergonominė biuro kėdė | Ergonix",
		MetaDescription: "Patogi ergonominė kėdė su juosmens atrama, tinkanti ilgam darbui prie kompiuterio.",
		Canonical:       "https://ergonix.lt/products/kede",
		Language:        "lt",
		H1s:             []string{"Ergonominė biuro kėdė"},
		Images:          []models.Image{{Src: "https://ergonix.lt/img/kede.jpg", Alt: "Kėdė", HasAlt: true}},
		Buttons:         []models.Button{{Text: "Į krepšelį", Type: "submit", HasAction: true}},
		Forms:           []models.Form{{Action: "/cart", Method: "POST", Inputs: 2, HasSubmit: true}},
		ResponseTimeMs:  200,
		LoadTimeMs:      900,
	}
}

func sc(pages ...*models.Page) *SiteContext {
	return &SiteContext{
		Website: "https://ergonix.lt",
		Pages:   pages,
		Links:   LinkStatusMap{},
		Cfg:     Defaults(),
	}
}

// runPage is a helper running one page check against one page.
func runPage(t *testing.T, chk PageCheck, p *models.Page) []models.Issue {
	t.Helper()
	return chk.CheckPage(sc(p), p)
}

func TestGoodPageProducesNoIssues(t *testing.T) {
	p := goodPage()
	context := sc(p)
	checksList := []PageCheck{
		&StatusCodeCheck{}, &TitleCheck{}, &MetaDescriptionCheck{}, &H1Check{},
		&ImageAltCheck{}, &LargeImageCheck{}, &SlowResponseCheck{}, &SlowLoadCheck{},
		&MixedContentCheck{}, &HTTPLinkCheck{}, &HardcodedCountryLinkCheck{},
		&EmptyButtonCheck{}, &ButtonActionCheck{}, &FormSubmitCheck{},
		&ConsoleErrorCheck{}, &FailedRequestCheck{}, &LargeBundleCheck{},
		&RedirectCheck{}, &SEOBasicsCheck{},
	}
	for _, chk := range checksList {
		if got := chk.CheckPage(context, p); len(got) != 0 {
			t.Errorf("%s fired on a good page: %+v", chk.Name(), got)
		}
	}
}

func TestStatusCodeCheck(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*models.Page)
		wantSev  models.Severity
		wantSub  string
	}{
		{"404", func(p *models.Page) { p.StatusCode = 404 }, models.SeverityHigh, "not found"},
		{"500", func(p *models.Page) { p.StatusCode = 500 }, models.SeverityCritical, "Server error"},
		{"fetch error", func(p *models.Page) { p.FetchError = "timeout" }, models.SeverityHigh, "failed to load"},
	}
	for _, c := range cases {
		p := goodPage()
		c.mutate(p)
		got := runPage(t, &StatusCodeCheck{}, p)
		if len(got) != 1 {
			t.Fatalf("%s: issues = %d, want 1", c.name, len(got))
		}
		if got[0].Severity != c.wantSev || !strings.Contains(strings.ToLower(got[0].Title), strings.ToLower(c.wantSub)) {
			t.Errorf("%s: got %s %q", c.name, got[0].Severity, got[0].Title)
		}
	}
}

func TestTitleAndMetaAndH1(t *testing.T) {
	p := goodPage()
	p.Title = ""
	if got := runPage(t, &TitleCheck{}, p); len(got) != 1 || got[0].Severity != models.SeverityHigh {
		t.Errorf("missing title: %+v", got)
	}

	p = goodPage()
	p.MetaDescription = " "
	if got := runPage(t, &MetaDescriptionCheck{}, p); len(got) != 1 {
		t.Errorf("missing meta: %+v", got)
	}

	p = goodPage()
	p.H1s = nil
	if got := runPage(t, &H1Check{}, p); len(got) != 1 || !strings.Contains(got[0].Title, "Missing H1") {
		t.Errorf("missing h1: %+v", got)
	}

	p = goodPage()
	p.H1s = []string{"One", "Two"}
	if got := runPage(t, &H1Check{}, p); len(got) != 1 || !strings.Contains(got[0].Title, "Multiple H1") {
		t.Errorf("multiple h1: %+v", got)
	}
}

func TestImageAltAndLargeImage(t *testing.T) {
	p := goodPage()
	p.Images = append(p.Images, models.Image{Src: "x.jpg", HasAlt: false})
	if got := runPage(t, &ImageAltCheck{}, p); len(got) != 1 {
		t.Errorf("missing alt: %+v", got)
	}

	p = goodPage()
	p.Images = []models.Image{{Src: "big.jpg", HasAlt: true, SizeBytes: 900 * 1024}}
	if got := runPage(t, &LargeImageCheck{}, p); len(got) != 1 {
		t.Errorf("large image: %+v", got)
	}
}

func TestPerformanceChecks(t *testing.T) {
	p := goodPage()
	p.ResponseTimeMs = 6000
	got := runPage(t, &SlowResponseCheck{}, p)
	if len(got) != 1 || got[0].Severity != models.SeverityHigh { // 6000 > 3*1500
		t.Errorf("slow response: %+v", got)
	}

	p = goodPage()
	p.LoadTimeMs = 9000
	if got := runPage(t, &SlowLoadCheck{}, p); len(got) != 1 {
		t.Errorf("slow load: %+v", got)
	}

	p = goodPage()
	p.Scripts = []models.Resource{{Src: "app.js", SizeBytes: 900 * 1024}}
	p.Stylesheets = []models.Resource{{Src: "style.css", SizeBytes: 400 * 1024}}
	got = runPage(t, &LargeBundleCheck{}, p)
	if len(got) != 2 {
		t.Errorf("large bundles: %+v", got)
	}
}

func TestSecurityChecks(t *testing.T) {
	p := goodPage()
	p.MixedContent = []string{"http://cdn.example.com/x.js"}
	if got := runPage(t, &MixedContentCheck{}, p); len(got) != 1 || got[0].Severity != models.SeverityHigh {
		t.Errorf("mixed content: %+v", got)
	}

	p = goodPage()
	p.Links = []models.Link{{Href: "http://ergonix.lt/old", Internal: true}}
	if got := runPage(t, &HTTPLinkCheck{}, p); len(got) != 1 {
		t.Errorf("http link: %+v", got)
	}
}

func TestHardcodedCountryLink(t *testing.T) {
	p := goodPage()
	p.Links = []models.Link{
		{Href: "https://ergonix.lv/products/kede", Text: "kėdė", Internal: false},
		{Href: "https://google.com", Internal: false},
	}
	got := runPage(t, &HardcodedCountryLinkCheck{}, p)
	if len(got) != 1 || got[0].Category != models.CategoryLogic {
		t.Errorf("hardcoded country link: %+v", got)
	}
}

func TestUIChecks(t *testing.T) {
	p := goodPage()
	p.Buttons = []models.Button{{Text: "", Type: "button", HasAction: false, Hint: "#cart-toggle .icon-btn"}}
	got := runPage(t, &EmptyButtonCheck{}, p)
	if len(got) != 1 {
		t.Fatalf("empty button: %+v", got)
	}
	if !strings.Contains(got[0].Description, "#cart-toggle") {
		t.Errorf("empty-button issue should name the element: %s", got[0].Description)
	}
	if els, ok := got[0].Details["elements"].([]string); !ok || len(els) != 1 {
		t.Errorf("empty-button details missing elements: %+v", got[0].Details)
	}
	if got := runPage(t, &ButtonActionCheck{}, p); len(got) != 1 {
		t.Errorf("button without action: %+v", got)
	}

	p = goodPage()
	p.Forms = []models.Form{{Action: "/newsletter", Method: "POST", Inputs: 1, HasSubmit: false}}
	if got := runPage(t, &FormSubmitCheck{}, p); len(got) != 1 {
		t.Errorf("form without submit: %+v", got)
	}
}

func TestBrowserDerivedChecks(t *testing.T) {
	p := goodPage()
	p.ConsoleErrors = []string{"TypeError: x is undefined"}
	if got := runPage(t, &ConsoleErrorCheck{}, p); len(got) != 1 || got[0].Category != models.CategoryJavaScript {
		t.Errorf("console errors: %+v", got)
	}

	p = goodPage()
	p.FailedRequests = []string{"https://x/y.js (HTTP 404)"}
	if got := runPage(t, &FailedRequestCheck{}, p); len(got) != 1 || got[0].Category != models.CategoryNetwork {
		t.Errorf("failed requests: %+v", got)
	}
}

func TestRedirectCheck(t *testing.T) {
	p := goodPage()
	p.FetchError = "redirect loop or too many redirects"
	got := runPage(t, &RedirectCheck{}, p)
	if len(got) != 1 || got[0].Severity != models.SeverityCritical {
		t.Errorf("redirect loop: %+v", got)
	}

	p = goodPage()
	p.RedirectChain = []string{
		"https://ergonix.lt/a", "https://ergonix.lt/b", "https://ergonix.lt/c",
		"https://ergonix.lt/d", "https://ergonix.lt/e", "https://ergonix.lt/f",
		"https://ergonix.lt/g",
	}
	got = runPage(t, &RedirectCheck{}, p)
	if len(got) != 1 || !strings.Contains(got[0].Title, "Long redirect chain") {
		t.Errorf("long chain: %+v", got)
	}

	p = goodPage()
	p.RedirectChain = []string{"https://ergonix.lt/out", "https://partner.example.com/lp"}
	got = runPage(t, &RedirectCheck{}, p)
	if len(got) != 1 || !strings.Contains(got[0].Title, "cross-domain") {
		t.Errorf("cross-domain redirect: %+v", got)
	}
}

func TestSEOBasics(t *testing.T) {
	p := goodPage()
	p.Language = "en"
	got := runPage(t, &SEOBasicsCheck{}, p)
	if len(got) != 1 || !strings.Contains(got[0].Title, "lang") {
		t.Errorf("wrong lang: %+v", got)
	}

	p = goodPage()
	p.Canonical = ""
	got = runPage(t, &SEOBasicsCheck{}, p)
	if len(got) != 1 || !strings.Contains(strings.ToLower(got[0].Title), "canonical") {
		t.Errorf("missing canonical: %+v", got)
	}
}

func TestDuplicateChecks(t *testing.T) {
	p1 := goodPage()
	p2 := goodPage()
	p2.URL = "https://ergonix.lt/products/kita"
	p2.FinalURL = p2.URL // same title+meta as p1

	got := (&DuplicateTitleCheck{}).CheckSite(sc(p1, p2))
	if len(got) != 1 || got[0].Category != models.CategorySEO {
		t.Errorf("duplicate title: %+v", got)
	}
	got = (&DuplicateMetaCheck{}).CheckSite(sc(p1, p2))
	if len(got) != 1 {
		t.Errorf("duplicate meta: %+v", got)
	}

	// Same FinalURL => one logical page => no duplicate.
	p3 := goodPage()
	p3.URL = "https://ergonix.lt/en"
	p3.FinalURL = p1.FinalURL
	got = (&DuplicateTitleCheck{}).CheckSite(sc(p1, p3))
	if len(got) != 0 {
		t.Errorf("redirected page should not create duplicate: %+v", got)
	}
}

func TestBrokenLinkCheck(t *testing.T) {
	p := goodPage()
	p.Links = []models.Link{
		{Href: "https://ergonix.lt/dead", Text: "Dead", Internal: true},
		{Href: "https://external.example.com/gone", Text: "Ext", Internal: false},
		{Href: "https://ergonix.lt/ok", Text: "OK", Internal: true},
	}
	context := sc(p)
	context.Links = LinkStatusMap{
		"https://ergonix.lt/dead":            {StatusCode: 404},
		"https://external.example.com/gone":  {Err: "dial tcp: no such host"},
		"https://ergonix.lt/ok":              {StatusCode: 200},
	}
	got := (&BrokenLinkCheck{}).CheckSite(context)
	if len(got) != 2 {
		t.Fatalf("broken links = %d, want 2: %+v", len(got), got)
	}
	var internal, external *models.Issue
	for i := range got {
		if got[i].Details["internal"] == true {
			internal = &got[i]
		} else {
			external = &got[i]
		}
	}
	if internal == nil || internal.Severity != models.SeverityHigh {
		t.Errorf("internal broken link: %+v", internal)
	}
	if external == nil || external.Severity != models.SeverityMedium {
		t.Errorf("external broken link: %+v", external)
	}
}

func TestBrokenLinkExternal403LowConfidence(t *testing.T) {
	p := goodPage()
	p.Links = []models.Link{
		{Href: "https://www.trustpilot.com/review/x", Text: "Reviews", Internal: false},
	}
	context := sc(p)
	context.Links = LinkStatusMap{
		"https://www.trustpilot.com/review/x": {StatusCode: 403},
	}
	got := (&BrokenLinkCheck{}).CheckSite(context)
	if len(got) != 1 {
		t.Fatalf("issues = %d, want 1", len(got))
	}
	if got[0].Confidence != 0.5 {
		t.Errorf("external 403 confidence = %v, want 0.5 (bot protection is likely)", got[0].Confidence)
	}
	if got[0].Severity != models.SeverityLow {
		t.Errorf("external 403 severity = %s, want low", got[0].Severity)
	}
	if !strings.Contains(got[0].Description, "bot protection") {
		t.Errorf("description should mention bot protection: %s", got[0].Description)
	}
}

func TestEngineRunSetsMetadata(t *testing.T) {
	p := goodPage()
	p.Title = "" // trigger one issue
	e := New(Defaults(), discardLogger())
	issues := e.Run(t.Context(), sc(p))
	if len(issues) == 0 {
		t.Fatal("expected at least one issue")
	}
	for _, is := range issues {
		if is.Website != "https://ergonix.lt" || is.Source != models.SourceRule || is.Confidence == 0 {
			t.Errorf("metadata not filled: %+v", is)
		}
		if is.CheckID == "" {
			t.Errorf("issue %q missing CheckID (provenance)", is.Title)
		}
	}
}

func TestLinkResultBroken(t *testing.T) {
	cases := []struct {
		res  LinkResult
		want bool
	}{
		{LinkResult{StatusCode: 200}, false},
		{LinkResult{StatusCode: 404}, true},
		{LinkResult{StatusCode: 500}, true},
		{LinkResult{StatusCode: 429}, false}, // rate limited = inconclusive, never "broken"
		{LinkResult{Err: "dial tcp: no such host"}, true},
	}
	for _, c := range cases {
		if got := c.res.Broken(); got != c.want {
			t.Errorf("Broken(%+v) = %v, want %v", c.res, got, c.want)
		}
	}
}

func TestExpectedLanguage(t *testing.T) {
	cases := map[string]string{
		"https://ergonix.lt": "lt",
		"https://ergonix.lv": "lv",
		"https://ergonix.ee": "et",
		"https://ergonix.cz": "cs",
		"https://ergonix.pl": "pl",
		"https://example.com": "",
	}
	for site, want := range cases {
		if got := ExpectedLanguage(site); got != want {
			t.Errorf("ExpectedLanguage(%s) = %q, want %q", site, got, want)
		}
	}
}
