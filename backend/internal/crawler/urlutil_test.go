package crawler

import (
	"net/url"
	"testing"
)

func mustParse(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestNormalizeURL(t *testing.T) {
	base := mustParse(t, "https://ergonix.lt/collections/kedes")

	cases := []struct {
		href, want string
	}{
		{"/products/chair", "https://ergonix.lt/products/chair"},
		{"https://ergonix.lt/products/chair/", "https://ergonix.lt/products/chair"},
		{"https://ergonix.lt/", "https://ergonix.lt/"},
		{"https://ERGONIX.LT/Products", "https://ergonix.lt/Products"},
		{"https://ergonix.lt:443/x", "https://ergonix.lt/x"},
		{"/x#section", "https://ergonix.lt/x"},
		{"/x?utm_source=fb&page=2", "https://ergonix.lt/x?page=2"},
		{"/x?b=2&a=1", "https://ergonix.lt/x?a=1&b=2"}, // sorted => stable dedup
		{"relative", "https://ergonix.lt/collections/relative"},
		{"javascript:void(0)", ""},
		{"mailto:info@ergonix.lt", ""},
		{"tel:+37060000000", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := NormalizeURL(base, c.href); got != c.want {
			t.Errorf("NormalizeURL(%q) = %q, want %q", c.href, got, c.want)
		}
	}
}

func TestSameSite(t *testing.T) {
	root := mustParse(t, "https://ergonix.lt")
	cases := []struct {
		url  string
		want bool
	}{
		{"https://ergonix.lt/x", true},
		{"https://www.ergonix.lt/x", true},
		{"http://ergonix.lt/x", true},
		{"https://ergonix.lv/x", false},
		{"https://cdn.shopify.com/img.png", false},
	}
	for _, c := range cases {
		if got := SameSite(root, c.url); got != c.want {
			t.Errorf("SameSite(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestIsBinaryPath(t *testing.T) {
	if !isBinaryPath("https://ergonix.lt/cdn/logo.png") {
		t.Error("png should be binary")
	}
	if isBinaryPath("https://ergonix.lt/products/chair") {
		t.Error("product page should not be binary")
	}
}
