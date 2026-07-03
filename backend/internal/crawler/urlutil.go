package crawler

import (
	"net/url"
	"strings"
)

// trackingParams are query parameters that never change page content and
// would otherwise multiply URLs in the frontier.
var trackingParams = map[string]bool{
	"utm_source": true, "utm_medium": true, "utm_campaign": true,
	"utm_term": true, "utm_content": true, "gclid": true, "fbclid": true,
	"mc_cid": true, "mc_eid": true, "ref": true, "srsltid": true,
}

// NormalizeURL resolves href against base and canonicalizes the result so
// that duplicate pages map to identical strings. Returns "" for links that
// should not be crawled (javascript:, mailto:, tel:, fragments, non-http).
func NormalizeURL(base *url.URL, href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}
	low := strings.ToLower(href)
	for _, p := range []string{"javascript:", "mailto:", "tel:", "data:", "sms:", "ftp:"} {
		if strings.HasPrefix(low, p) {
			return ""
		}
	}
	u, err := base.Parse(href)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Fragment = ""
	u.Host = strings.ToLower(u.Host)
	// Default ports
	u.Host = strings.TrimSuffix(u.Host, ":80")
	if u.Scheme == "https" {
		u.Host = strings.TrimSuffix(u.Host, ":443")
	}
	// Strip tracking params, keep the rest sorted (url.Values.Encode sorts).
	q := u.Query()
	changed := false
	for k := range q {
		if trackingParams[strings.ToLower(k)] {
			q.Del(k)
			changed = true
		}
	}
	if changed || u.RawQuery != "" {
		u.RawQuery = q.Encode()
	}
	if u.Path == "" {
		u.Path = "/"
	}
	// Collapse trailing slash except for root.
	if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
		u.Path = strings.TrimRight(u.Path, "/")
	}
	return u.String()
}

// SameSite reports whether candidate belongs to the same site as root,
// treating "www." as equivalent to the bare host.
func SameSite(root *url.URL, candidate string) bool {
	u, err := url.Parse(candidate)
	if err != nil {
		return false
	}
	return stripWWW(u.Host) == stripWWW(root.Host)
}

func stripWWW(host string) string {
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}

// isBinaryPath filters obvious non-HTML assets before fetching.
func isBinaryPath(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}
	path := strings.ToLower(parsed.Path)
	for _, ext := range []string{
		".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico", ".avif",
		".css", ".js", ".mjs", ".json", ".xml", ".txt",
		".pdf", ".zip", ".gz", ".rar", ".7z",
		".mp4", ".webm", ".mp3", ".wav", ".ogg",
		".woff", ".woff2", ".ttf", ".eot", ".otf",
	} {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
