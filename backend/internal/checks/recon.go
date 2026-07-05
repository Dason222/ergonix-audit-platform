package checks

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ReconData holds results of active, non-crawl probes about a site's
// security posture and infrastructure hygiene.
type ReconData struct {
	// HTTPSRedirect: "" not tested, "ok" http→https, "missing" served over
	// http without redirect, "error:<msg>" probe failed.
	HTTPSRedirect string
	Robots        LinkResult
	Sitemap       LinkResult
	SecurityTxt   LinkResult
	// Exposed maps a sensitive path -> a short evidence string for paths
	// that returned real, sensitive content (not soft-404s).
	Exposed map[string]string
}

// sensitiveProbe is a path to test plus a validator that confirms the
// response body actually is the sensitive resource (guards against catch-all
// 200 pages / SPA fallbacks).
type sensitiveProbe struct {
	path    string
	label   string
	confirm func(body string) bool
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

var sensitiveProbes = []sensitiveProbe{
	{"/.git/config", "Git repository config exposed",
		func(b string) bool { return containsAny(b, "[core]", "repositoryformatversion") }},
	{"/.git/HEAD", "Git repository exposed",
		func(b string) bool { return strings.HasPrefix(strings.TrimSpace(b), "ref:") }},
	{"/.env", "Environment file with secrets exposed",
		func(b string) bool {
			return !strings.Contains(strings.ToLower(b), "<html") &&
				containsAny(b, "APP_", "DB_", "SECRET", "API_KEY", "PASSWORD", "_KEY=")
		}},
	{"/config.php", "PHP config file exposed",
		func(b string) bool { return containsAny(b, "<?php", "define(", "$db", "DB_PASSWORD") }},
	{"/phpinfo.php", "phpinfo() exposed",
		func(b string) bool { return containsAny(b, "phpinfo()", "PHP Version") }},
	{"/.well-known/security.txt", "", nil}, // handled specially (absence, not exposure)
	{"/server-status", "Apache server-status exposed",
		func(b string) bool { return containsAny(b, "Apache Server Status", "Server Version:") }},
	{"/.DS_Store", "macOS .DS_Store exposed",
		func(b string) bool { return strings.Contains(b, "Bud1") }},
	{"/wp-config.php.bak", "WordPress config backup exposed",
		func(b string) bool { return containsAny(b, "DB_PASSWORD", "DB_NAME") }},
	{"/backup.sql", "Database dump exposed",
		func(b string) bool { return containsAny(b, "INSERT INTO", "CREATE TABLE", "DROP TABLE") }},
}

// Recon performs the active probes for one website. It is bounded, paced and
// best-effort: any probe failure is recorded, never fatal.
func (lp *LinkProber) Recon(ctx context.Context, website string) *ReconData {
	rd := &ReconData{Exposed: map[string]string{}}
	root, err := url.Parse(website)
	if err != nil || root.Host == "" {
		return rd
	}
	base := root.Scheme + "://" + root.Host

	// No-redirect client to observe redirect behavior directly.
	noRedirect := &http.Client{
		Timeout:       lp.cfg.LinkProbeTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	follow := &http.Client{Timeout: lp.cfg.LinkProbeTimeout}

	rd.HTTPSRedirect = lp.probeHTTPSRedirect(ctx, noRedirect, root)
	rd.Robots = lp.getStatus(ctx, follow, base+"/robots.txt")
	rd.Sitemap = lp.getStatus(ctx, follow, base+"/sitemap.xml")
	rd.SecurityTxt = lp.getStatus(ctx, follow, base+"/.well-known/security.txt")

	for _, sp := range sensitiveProbes {
		if ctx.Err() != nil {
			break
		}
		if sp.confirm == nil {
			continue // security.txt handled via rd.SecurityTxt
		}
		lp.pace()
		body, status, ct := lp.getBody(ctx, noRedirect, base+sp.path)
		// Only real 200 responses that pass content validation count.
		if status == http.StatusOK && !strings.Contains(strings.ToLower(ct), "text/html") && sp.confirm(body) {
			rd.Exposed[sp.path] = sp.label
		} else if status == http.StatusOK && sp.confirm(body) {
			// Some servers mislabel content type; still require the validator.
			rd.Exposed[sp.path] = sp.label
		}
	}
	return rd
}

// pace enforces the shared polite request interval.
func (lp *LinkProber) pace() {
	time.Sleep(probeDelay)
}

func (lp *LinkProber) probeHTTPSRedirect(ctx context.Context, client *http.Client, root *url.URL) string {
	if root.Scheme != "https" {
		return "" // only meaningful when the canonical site is https
	}
	httpURL := "http://" + root.Host + "/"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", lp.cfg.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "error:" + err.Error()
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		if loc := resp.Header.Get("Location"); strings.HasPrefix(strings.ToLower(loc), "https://") {
			return "ok"
		}
		return "missing" // redirects, but not to https
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return "missing" // serves content over plain http
	}
	return "" // inconclusive (blocked/errored)
}

func (lp *LinkProber) getStatus(ctx context.Context, client *http.Client, u string) LinkResult {
	lp.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return LinkResult{Err: err.Error()}
	}
	req.Header.Set("User-Agent", lp.cfg.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return LinkResult{Err: err.Error()}
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return LinkResult{StatusCode: resp.StatusCode}
}

func (lp *LinkProber) getBody(ctx context.Context, client *http.Client, u string) (body string, status int, contentType string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", 0, ""
	}
	req.Header.Set("User-Agent", lp.cfg.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	return string(b), resp.StatusCode, resp.Header.Get("Content-Type")
}
