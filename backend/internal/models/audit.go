package models

import "time"

// AuditParams are the user-supplied knobs for one audit run.
type AuditParams struct {
	Websites          []string `json:"websites"`
	MaxPages          int      `json:"maxPages"`
	MaxDepth          int      `json:"maxDepth"`
	Concurrency       int      `json:"concurrency"`
	RequestTimeoutSec int      `json:"requestTimeoutSec"`
	RetryCount        int      `json:"retryCount"`
	UseAI             bool     `json:"useAI"`
	UseBrowser        bool     `json:"useBrowser"`
}

// DefaultParams returns sensible defaults for an audit.
func DefaultParams() AuditParams {
	return AuditParams{
		MaxPages:          25,
		MaxDepth:          3,
		Concurrency:       4,
		RequestTimeoutSec: 15,
		RetryCount:        2,
		UseAI:             true,
	}
}

// Normalize clamps parameters into safe bounds and fills zero values with defaults.
func (p *AuditParams) Normalize() {
	d := DefaultParams()
	if p.MaxPages <= 0 {
		p.MaxPages = d.MaxPages
	}
	if p.MaxPages > 500 {
		p.MaxPages = 500
	}
	if p.MaxDepth <= 0 {
		p.MaxDepth = d.MaxDepth
	}
	if p.MaxDepth > 10 {
		p.MaxDepth = 10
	}
	if p.Concurrency <= 0 {
		p.Concurrency = d.Concurrency
	}
	if p.Concurrency > 16 {
		p.Concurrency = 16
	}
	if p.RequestTimeoutSec <= 0 {
		p.RequestTimeoutSec = d.RequestTimeoutSec
	}
	if p.RequestTimeoutSec > 120 {
		p.RequestTimeoutSec = 120
	}
	if p.RetryCount < 0 {
		p.RetryCount = 0
	}
	if p.RetryCount > 5 {
		p.RetryCount = 5
	}
}

// AuditSite tracks per-website progress and outcome inside an audit.
type AuditSite struct {
	Website      string `json:"website"`
	Status       string `json:"status"` // pending | crawling | checking | ai_analysis | completed | failed
	PagesCrawled int    `json:"pagesCrawled"`
	IssueCount   int    `json:"issueCount"`
	DurationMs   int64  `json:"durationMs"`
	Error        string `json:"error,omitempty"`
}

// AuditStats is the aggregated result summary persisted with the audit.
type AuditStats struct {
	TotalWebsites int               `json:"totalWebsites"`
	TotalPages    int               `json:"totalPages"`
	TotalIssues   int               `json:"totalIssues"`
	DurationMs    int64             `json:"durationMs"`
	BySeverity    map[Severity]int  `json:"bySeverity"`
	ByCategory    map[Category]int  `json:"byCategory"`
	ByWebsite     map[string]int    `json:"byWebsite"`
	BySource      map[Source]int    `json:"bySource"`
	AISkipped     bool              `json:"aiSkipped,omitempty"`
	Notes         []string          `json:"notes,omitempty"`
}

// NewAuditStats returns stats with initialized maps.
func NewAuditStats() AuditStats {
	return AuditStats{
		BySeverity: map[Severity]int{},
		ByCategory: map[Category]int{},
		ByWebsite:  map[string]int{},
		BySource:   map[Source]int{},
	}
}

// Audit is one audit run over one or more websites.
type Audit struct {
	ID         int64       `json:"id"`
	Status     AuditStatus `json:"status"`
	Stage      string      `json:"stage"`
	Params     AuditParams `json:"params"`
	Sites      []AuditSite `json:"sites"`
	Stats      AuditStats  `json:"stats"`
	Error      string      `json:"error,omitempty"`
	CreatedAt  time.Time   `json:"createdAt"`
	StartedAt  *time.Time  `json:"startedAt,omitempty"`
	FinishedAt *time.Time  `json:"finishedAt,omitempty"`
}

// Running reports whether the audit is still in flight.
func (a *Audit) Running() bool {
	return a.Status == AuditPending || a.Status == AuditRunning
}
