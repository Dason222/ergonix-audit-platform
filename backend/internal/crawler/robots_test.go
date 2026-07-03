package crawler

import (
	"strings"
	"testing"
)

const sampleRobots = `
# comment
User-agent: *
Disallow: /admin
Disallow: /cart
Allow: /admin/public

User-agent: BadBot
Disallow: /
`

func TestParseRobotsWildcardGroup(t *testing.T) {
	r := parseRobots(strings.NewReader(sampleRobots), "ErgonixAuditBot/1.0")

	cases := []struct {
		url  string
		want bool
	}{
		{"https://ergonix.lt/", true},
		{"https://ergonix.lt/products/chair", true},
		{"https://ergonix.lt/admin", false},
		{"https://ergonix.lt/admin/settings", false},
		{"https://ergonix.lt/admin/public", true}, // longest match wins
		{"https://ergonix.lt/cart", false},
	}
	for _, c := range cases {
		if got := r.Allowed(c.url); got != c.want {
			t.Errorf("Allowed(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestParseRobotsSpecificAgent(t *testing.T) {
	txt := `
User-agent: ergonixauditbot
Disallow: /private

User-agent: *
Disallow: /
`
	r := parseRobots(strings.NewReader(txt), "ErgonixAuditBot/1.0")
	if !r.Allowed("https://ergonix.lt/products") {
		t.Error("specific agent group should apply (allow /products)")
	}
	if r.Allowed("https://ergonix.lt/private/x") {
		t.Error("/private should be disallowed for our agent")
	}
}

func TestRobotsWildcardPatterns(t *testing.T) {
	txt := `
User-agent: *
Disallow: /*.json$
Disallow: /search*sort=
`
	r := parseRobots(strings.NewReader(txt), "bot")
	if r.Allowed("https://x.lt/products.json") {
		t.Error("*.json$ should block .json URLs")
	}
	if !r.Allowed("https://x.lt/products.json.html") {
		t.Error("$ anchor should not block .json.html")
	}
	if r.Allowed("https://x.lt/search?q=a&sort=price") {
		t.Error("wildcard middle should match query strings")
	}
}

func TestEmptyRobotsAllowsEverything(t *testing.T) {
	r := &robotsRules{}
	if !r.Allowed("https://ergonix.lt/anything") {
		t.Error("empty rules must allow everything")
	}
}
