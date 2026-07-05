package audit

import (
	"fmt"
	"sort"

	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
)

// computeDiff compares issues against the most recent completed audit of the
// same site set, marking new issues (Details["new"] + New field) and filling
// the change counters in stats. Call it BEFORE persisting issues so the New
// marks are saved. Any lookup failure degrades to "no previous audit".
func computeDiff(store database.Store, auditID int64, websiteKey string,
	stats *models.AuditStats, issues []models.Issue) {

	prev, err := store.PreviousCompletedAudit(auditID, websiteKey)
	if err != nil || prev == nil {
		return
	}
	prevIssues, _, err := store.ListIssues(database.IssueFilter{AuditID: prev.ID})
	if err != nil {
		return
	}

	prevSet := make(map[string]bool, len(prevIssues))
	for i := range prevIssues {
		prevSet[prevIssues[i].Fingerprint()] = true
	}
	currentSet := make(map[string]bool, len(issues))
	for i := range issues {
		currentSet[issues[i].Fingerprint()] = true
	}

	stats.PreviousAuditID = prev.ID
	stats.NewBySeverity = map[models.Severity]int{}

	for i := range issues {
		if !prevSet[issues[i].Fingerprint()] {
			issues[i].New = true
			if issues[i].Details == nil {
				issues[i].Details = map[string]any{}
			}
			issues[i].Details["new"] = true
			stats.NewCount++
			stats.NewBySeverity[issues[i].Severity]++
		}
	}

	// Resolved: present before, gone now. Summarize the most severe few.
	var resolved []models.Issue
	for i := range prevIssues {
		if !currentSet[prevIssues[i].Fingerprint()] {
			resolved = append(resolved, prevIssues[i])
		}
	}
	stats.ResolvedCount = len(resolved)
	sort.SliceStable(resolved, func(i, j int) bool {
		return models.SeverityRank[resolved[i].Severity] < models.SeverityRank[resolved[j].Severity]
	})
	for i, is := range resolved {
		if i >= 10 {
			break
		}
		stats.ResolvedSummary = append(stats.ResolvedSummary,
			fmt.Sprintf("[%s] %s (%s)", is.Severity, is.Title, hostShort(is.Website)))
	}
}

func hostShort(website string) string {
	// crude host extraction good enough for a summary line
	s := website
	for _, p := range []string{"https://", "http://", "www."} {
		if len(s) >= len(p) && s[:len(p)] == p {
			s = s[len(p):]
		}
	}
	if i := indexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return s
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
