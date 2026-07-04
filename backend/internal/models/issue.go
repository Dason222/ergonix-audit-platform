package models

import "time"

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
	CreatedAt    time.Time      `json:"createdAt"`
}
