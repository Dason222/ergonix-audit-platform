package models

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"time"
)

// Issue is one finding produced by a rule-based check or the AI analyzer.
type Issue struct {
	ID           int64          `json:"id"`
	AuditID      int64          `json:"auditId"`
	Website      string         `json:"website"`
	PageURL      string         `json:"pageUrl"`
	Category     Category       `json:"category"`
	Source       Source         `json:"source"`
	// CheckID names the producer: a rule check id (e.g. "empty-button")
	// or "ai:<type>" for AI findings (e.g. "ai:wrong_language").
	CheckID      string         `json:"checkId,omitempty"`
	Severity     Severity       `json:"severity"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	SuggestedFix string         `json:"suggestedFix"`
	Confidence   float64        `json:"confidence"` // 0..1
	Screenshot   string         `json:"screenshot,omitempty"`
	Details      map[string]any `json:"details,omitempty"`
	// New is true when this finding was not present in the previous audit of
	// the same site(s). Computed at audit time, persisted in Details.
	New       bool      `json:"new,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Fingerprint returns a stable identity for an issue across audits, so the
// same problem on the same page is recognized run-to-run. It deliberately
// ignores volatile detail (counts, timings) and query strings.
func (i *Issue) Fingerprint() string {
	page := i.PageURL
	if u, err := url.Parse(i.PageURL); err == nil && u.Host != "" {
		page = u.Host + u.Path
	}
	// A per-target discriminator keeps distinct broken links / duplicate
	// groups from collapsing into one fingerprint.
	disc := ""
	if i.Details != nil {
		for _, k := range []string{"target", "value", "path", "element"} {
			if v, ok := i.Details[k].(string); ok && v != "" {
				disc = v
				break
			}
		}
	}
	raw := fmt.Sprintf("%s|%s|%s|%s|%s", i.Website, i.CheckID, i.Category, page, disc)
	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}
