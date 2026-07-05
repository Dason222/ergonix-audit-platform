package audit

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
)

func openStore(t *testing.T) *database.DB {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "diff.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func mkAudit(t *testing.T, store database.Store, status models.AuditStatus, sites ...string) *models.Audit {
	t.Helper()
	a := &models.Audit{
		Status: status, Stage: models.StageDone,
		Params: models.AuditParams{Websites: sites},
		Stats:  models.NewAuditStats(), CreatedAt: time.Now(),
	}
	if err := store.CreateAudit(a); err != nil {
		t.Fatal(err)
	}
	return a
}

func issue(auditID int64, site, check, title, target string, sev models.Severity) models.Issue {
	d := map[string]any{}
	if target != "" {
		d["target"] = target
	}
	return models.Issue{
		AuditID: auditID, Website: site, PageURL: site + "/", CheckID: check,
		Category: models.CategorySEO, Source: models.SourceRule, Severity: sev,
		Title: title, Confidence: 1, Details: d, CreatedAt: time.Now(),
	}
}

func TestFingerprintStability(t *testing.T) {
	a := issue(1, "https://ergonix.lt", "broken-links", "Broken internal link", "https://ergonix.lt/x", models.SeverityHigh)
	b := issue(2, "https://ergonix.lt", "broken-links", "Broken internal link", "https://ergonix.lt/x", models.SeverityHigh)
	if a.Fingerprint() != b.Fingerprint() {
		t.Error("same problem across audits must share a fingerprint")
	}
	c := issue(2, "https://ergonix.lt", "broken-links", "Broken internal link", "https://ergonix.lt/y", models.SeverityHigh)
	if a.Fingerprint() == c.Fingerprint() {
		t.Error("different broken targets must differ")
	}
}

func TestComputeDiffNewAndResolved(t *testing.T) {
	store := openStore(t)
	site := "https://ergonix.lt"

	// Previous completed audit with 2 issues.
	prev := mkAudit(t, store, models.AuditCompleted, site)
	prevIssues := []models.Issue{
		issue(prev.ID, site, "missing-title", "Missing page title", "", models.SeverityHigh),
		issue(prev.ID, site, "broken-links", "Broken internal link", site+"/old", models.SeverityHigh),
	}
	if err := store.SaveIssues(prevIssues); err != nil {
		t.Fatal(err)
	}

	// Current audit: keeps the missing-title, drops the old broken link,
	// adds a new broken link.
	cur := mkAudit(t, store, models.AuditRunning, site)
	curIssues := []models.Issue{
		issue(cur.ID, site, "missing-title", "Missing page title", "", models.SeverityHigh),
		issue(cur.ID, site, "broken-links", "Broken internal link", site+"/new", models.SeverityHigh),
	}

	stats := models.NewAuditStats()
	computeDiff(store, cur.ID, cur.WebsiteKey(), &stats, curIssues)

	if stats.PreviousAuditID != prev.ID {
		t.Errorf("previousAuditId = %d, want %d", stats.PreviousAuditID, prev.ID)
	}
	if stats.NewCount != 1 {
		t.Errorf("newCount = %d, want 1", stats.NewCount)
	}
	if stats.ResolvedCount != 1 {
		t.Errorf("resolvedCount = %d, want 1", stats.ResolvedCount)
	}
	// The new broken link must be marked; the persisting title must not.
	var markedNew, markedOld bool
	for _, is := range curIssues {
		if is.CheckID == "broken-links" && is.New {
			markedNew = true
		}
		if is.CheckID == "missing-title" && is.New {
			markedOld = true
		}
	}
	if !markedNew {
		t.Error("new broken link should be marked New")
	}
	if markedOld {
		t.Error("persisting issue should not be marked New")
	}
}

func TestComputeDiffNoPrevious(t *testing.T) {
	store := openStore(t)
	cur := mkAudit(t, store, models.AuditRunning, "https://ergonix.lt")
	issues := []models.Issue{issue(cur.ID, "https://ergonix.lt", "missing-title", "Missing page title", "", models.SeverityHigh)}
	stats := models.NewAuditStats()
	computeDiff(store, cur.ID, cur.WebsiteKey(), &stats, issues)
	// First audit ever: everything is "new" relative to nothing → no diff.
	if stats.NewCount != 0 || stats.ResolvedCount != 0 || stats.PreviousAuditID != 0 {
		t.Errorf("no-previous diff should be empty: %+v", stats)
	}
	if issues[0].New {
		t.Error("first audit issues should not be marked New")
	}
}
