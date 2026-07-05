package models

import (
	"strings"
	"time"
)

// Image is one <img> found on a page. HasAlt distinguishes a missing alt
// attribute (accessibility issue) from an intentionally empty alt="".
type Image struct {
	Src           string `json:"src"`
	Alt           string `json:"alt"`
	HasAlt        bool   `json:"hasAlt"`
	HasDimensions bool   `json:"hasDimensions,omitempty"` // width+height attrs set (CLS)
	SizeBytes     int64  `json:"sizeBytes,omitempty"`
}

// Link is one <a href> found on a page.
type Link struct {
	Href        string `json:"href"`
	Text        string `json:"text"`
	Internal    bool   `json:"internal"`
	Nofollow    bool   `json:"nofollow,omitempty"`
	TargetBlank bool   `json:"targetBlank,omitempty"` // opens in a new tab
	NoOpener    bool   `json:"noOpener,omitempty"`    // rel includes noopener/noreferrer
}

// Form is one <form> found on a page. Hint identifies the element in the
// page source (id/class/input names); Snippet is its truncated raw HTML so
// findings can show exactly what the scraper saw.
type Form struct {
	Action    string `json:"action"`
	Method    string `json:"method"`
	Inputs    int    `json:"inputs"`
	HasSubmit bool   `json:"hasSubmit"`
	Hint      string `json:"hint,omitempty"`
	Snippet   string `json:"snippet,omitempty"`
}

// Button is one <button> (or input[type=button|submit]) found on a page.
// Hint identifies the element (id/class/aria-label); Snippet is its
// truncated raw HTML.
type Button struct {
	Text      string `json:"text"`
	Type      string `json:"type"`
	HasAction bool   `json:"hasAction"`
	Hint      string `json:"hint,omitempty"`
	Snippet   string `json:"snippet,omitempty"`
}

// Resource is an external script or stylesheet reference.
type Resource struct {
	Src          string `json:"src"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	Inline       bool   `json:"inline,omitempty"`
	External     bool   `json:"external,omitempty"`     // loaded from a different host
	HasIntegrity bool   `json:"hasIntegrity,omitempty"` // has an SRI integrity attribute
}

// Page holds everything the crawler extracted from a single URL.
type Page struct {
	ID       int64  `json:"id"`
	AuditID  int64  `json:"auditId"`
	Website  string `json:"website"`
	URL      string `json:"url"`
	FinalURL string `json:"finalUrl"`
	Depth    int    `json:"depth"`

	StatusCode      int      `json:"statusCode"`
	Title           string   `json:"title"`
	MetaDescription string   `json:"metaDescription"`
	Canonical       string   `json:"canonical"`
	Language        string   `json:"language"`
	MetaRobots      string   `json:"metaRobots,omitempty"`
	Hreflangs       []string `json:"hreflangs,omitempty"`
	OGProperties    []string `json:"ogProperties,omitempty"`
	Favicons        []string `json:"favicons,omitempty"` // resolved icon URLs from <link rel*="icon">
	HasAppleTouchIcon bool   `json:"hasAppleTouchIcon,omitempty"`
	HasViewport     bool     `json:"hasViewport,omitempty"`
	ViewportContent string   `json:"viewportContent,omitempty"`
	HasCharset      bool     `json:"hasCharset,omitempty"`
	HeadingLevels   []int    `json:"headingLevels,omitempty"`      // h1..h6 levels in document order
	StructuredTypes []string `json:"structuredTypes,omitempty"`    // @type values from JSON-LD
	PasswordFields  int      `json:"passwordFields,omitempty"`

	H1s         []string   `json:"h1s"`
	H2s         []string   `json:"h2s"`
	Images      []Image    `json:"images"`
	Links       []Link     `json:"links"`
	Forms       []Form     `json:"forms"`
	Buttons     []Button   `json:"buttons"`
	VisibleText string     `json:"visibleText"`
	Prices      []string   `json:"prices"`
	Scripts     []Resource `json:"scripts"`
	Stylesheets []Resource `json:"stylesheets"`

	ResponseTimeMs int64  `json:"responseTimeMs"`
	LoadTimeMs     int64  `json:"loadTimeMs,omitempty"`
	ContentLength  int64  `json:"contentLength"`
	ContentType    string `json:"contentType"`

	// Headers holds security-relevant response headers (whitelisted).
	Headers        map[string]string `json:"headers,omitempty"`
	// Cookies holds raw Set-Cookie header values from the response.
	Cookies        []string `json:"cookies,omitempty"`
	RedirectChain  []string `json:"redirectChain,omitempty"`
	ConsoleErrors  []string `json:"consoleErrors,omitempty"`
	FailedRequests []string `json:"failedRequests,omitempty"`
	MixedContent   []string `json:"mixedContent,omitempty"`

	FetchError string    `json:"fetchError,omitempty"`
	CrawledAt  time.Time `json:"crawledAt"`
}

// OK reports whether the page was fetched successfully with a 2xx status.
func (p *Page) OK() bool {
	return p.FetchError == "" && p.StatusCode >= 200 && p.StatusCode < 300
}

// IsHTML reports whether the fetched resource was an HTML document.
func (p *Page) IsHTML() bool {
	ct := strings.ToLower(p.ContentType)
	return ct == "" || strings.Contains(ct, "text/html") || strings.Contains(ct, "xhtml")
}
