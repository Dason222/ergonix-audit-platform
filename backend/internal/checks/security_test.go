package checks

import (
	"strings"
	"testing"

	"github.com/ergonix/auditor/backend/internal/models"
)

func TestSensitiveFileCheck(t *testing.T) {
	p := goodPage()
	ctx := sc(p)
	ctx.Recon = &ReconData{Exposed: map[string]string{
		"/.git/config": "Git repository config exposed",
		"/.env":        "Environment file with secrets exposed",
	}}
	got := (&SensitiveFileCheck{}).CheckSite(ctx)
	if len(got) != 2 {
		t.Fatalf("issues = %d, want 2", len(got))
	}
	for _, is := range got {
		if is.Severity != models.SeverityCritical || is.Category != models.CategorySecurity {
			t.Errorf("expected critical security issue: %+v", is)
		}
	}
	// No exposure → silent.
	ctx.Recon = &ReconData{Exposed: map[string]string{}}
	if got := (&SensitiveFileCheck{}).CheckSite(ctx); len(got) != 0 {
		t.Errorf("clean site flagged: %+v", got)
	}
}

func TestHTTPSRedirectCheck(t *testing.T) {
	p := goodPage()
	ctx := sc(p)
	ctx.Recon = &ReconData{HTTPSRedirect: "missing"}
	got := (&HTTPSRedirectCheck{}).CheckSite(ctx)
	if len(got) != 1 || got[0].Severity != models.SeverityHigh {
		t.Fatalf("missing redirect: %+v", got)
	}

	ctx.Recon = &ReconData{HTTPSRedirect: "ok"}
	if got := (&HTTPSRedirectCheck{}).CheckSite(ctx); len(got) != 0 {
		t.Errorf("ok redirect flagged: %+v", got)
	}
}

func TestRobotsSitemapCheck(t *testing.T) {
	p := goodPage()
	ctx := sc(p)
	ctx.Recon = &ReconData{Robots: LinkResult{StatusCode: 404}, Sitemap: LinkResult{StatusCode: 404}}
	got := (&RobotsSitemapCheck{}).CheckSite(ctx)
	if len(got) != 2 {
		t.Fatalf("issues = %d, want 2", len(got))
	}

	ctx.Recon = &ReconData{Robots: LinkResult{StatusCode: 200}, Sitemap: LinkResult{StatusCode: 200}}
	if got := (&RobotsSitemapCheck{}).CheckSite(ctx); len(got) != 0 {
		t.Errorf("healthy flagged: %+v", got)
	}
}

func TestCookieSecurityCheck(t *testing.T) {
	p := goodPage()
	p.Cookies = []string{
		"session=abc; Path=/; HttpOnly",                        // missing Secure + SameSite
		"cart=xyz; Path=/; Secure; HttpOnly; SameSite=Lax",     // fine
	}
	got := (&CookieSecurityCheck{}).CheckSite(sc(p))
	if len(got) != 2 {
		t.Fatalf("issues = %d: %+v", len(got), got)
	}
	var secure, sameSite bool
	for _, is := range got {
		if strings.Contains(is.Title, "Secure") {
			secure = true
		}
		if strings.Contains(is.Title, "SameSite") {
			sameSite = true
		}
	}
	if !secure || !sameSite {
		t.Errorf("expected both Secure and SameSite findings: %+v", got)
	}

	p.Cookies = []string{"cart=xyz; Secure; HttpOnly; SameSite=Lax"}
	if got := (&CookieSecurityCheck{}).CheckSite(sc(p)); len(got) != 0 {
		t.Errorf("hardened cookie flagged: %+v", got)
	}
}

func TestSecurityHeadersExpanded(t *testing.T) {
	p := goodPage()
	p.Headers = map[string]string{} // everything missing
	got := (&SecurityHeadersCheck{}).CheckSite(sc(p))
	if len(got) != 1 || got[0].Severity != models.SeverityHigh {
		t.Fatalf("all headers missing should be high: %+v", got)
	}
	for _, want := range []string{"HSTS", "clickjacking", "Referrer-Policy"} {
		if !strings.Contains(got[0].Description, want) {
			t.Errorf("missing %q in: %s", want, got[0].Description)
		}
	}

	// Clickjacking satisfied by CSP frame-ancestors instead of XFO.
	p.Headers = map[string]string{
		"Strict-Transport-Security": "max-age=1",
		"Content-Security-Policy":   "frame-ancestors 'self'",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "no-referrer",
	}
	if got := (&SecurityHeadersCheck{}).CheckSite(sc(p)); len(got) != 0 {
		t.Errorf("frame-ancestors should satisfy clickjacking: %+v", got)
	}
}

func TestTabnabbingCheck(t *testing.T) {
	p := goodPage()
	p.Links = []models.Link{
		{Href: "https://facebook.com/ergonix", Internal: false, TargetBlank: true, NoOpener: false},
		{Href: "https://instagram.com/ergonix", Internal: false, TargetBlank: true, NoOpener: true}, // safe
	}
	got := runPage(t, &TabnabbingCheck{}, p)
	if len(got) != 1 || got[0].Category != models.CategorySecurity {
		t.Fatalf("tabnabbing: %+v", got)
	}
	if !strings.Contains(got[0].Description, "facebook.com") {
		t.Errorf("should name the vulnerable link: %s", got[0].Description)
	}
}

func TestSRICheck(t *testing.T) {
	p := goodPage()
	p.Scripts = []models.Resource{
		{Src: "https://cdn.thirdparty.com/lib.js", External: true, HasIntegrity: false},
		{Src: "https://cdn.thirdparty.com/ok.js", External: true, HasIntegrity: true},
		{Src: "https://ergonix.lt/app.js", External: false, HasIntegrity: false}, // same-site, fine
	}
	got := runPage(t, &SRICheck{}, p)
	if len(got) != 1 || got[0].Confidence != 0.6 {
		t.Fatalf("SRI: %+v", got)
	}
	if !strings.Contains(got[0].Description, "cdn.thirdparty.com/lib.js") {
		t.Errorf("should name the unprotected script: %s", got[0].Description)
	}
}
