package database

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ergonix/auditor/backend/internal/models"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestAuditRoundTrip(t *testing.T) {
	db := openTestDB(t)

	a := &models.Audit{
		Status:    models.AuditPending,
		Stage:     models.StageQueued,
		Params:    models.AuditParams{Websites: []string{"https://ergonix.lt"}, MaxPages: 10, UseAI: true},
		Stats:     models.NewAuditStats(),
		CreatedAt: time.Now(),
	}
	if err := db.CreateAudit(a); err != nil {
		t.Fatalf("create: %v", err)
	}
	if a.ID == 0 {
		t.Fatal("expected ID to be filled")
	}

	now := time.Now()
	a.Status = models.AuditRunning
	a.Stage = models.StageCrawling
	a.StartedAt = &now
	a.Sites = []models.AuditSite{{Website: "ergonix.lt", Status: "crawling", PagesCrawled: 3}}
	if err := db.UpdateAudit(a); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, err := db.GetAudit(a.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != models.AuditRunning || got.Stage != models.StageCrawling {
		t.Errorf("status/stage = %s/%s, want running/crawling", got.Status, got.Stage)
	}
	if len(got.Sites) != 1 || got.Sites[0].PagesCrawled != 3 {
		t.Errorf("sites not persisted: %+v", got.Sites)
	}
	if got.StartedAt == nil {
		t.Error("startedAt not persisted")
	}
	if got.Params.MaxPages != 10 || !got.Params.UseAI {
		t.Errorf("params not persisted: %+v", got.Params)
	}

	if _, err := db.GetAudit(9999); err != ErrNotFound {
		t.Errorf("missing audit: got %v, want ErrNotFound", err)
	}
}

func TestIssueFilteringAndPages(t *testing.T) {
	db := openTestDB(t)

	a := &models.Audit{Status: models.AuditCompleted, Params: models.DefaultParams(),
		Stats: models.NewAuditStats(), CreatedAt: time.Now()}
	if err := db.CreateAudit(a); err != nil {
		t.Fatal(err)
	}

	pages := []*models.Page{
		{AuditID: a.ID, Website: "ergonix.lt", URL: "https://ergonix.lt/", StatusCode: 200,
			Title: "Home", H1s: []string{"Hello"}, CrawledAt: time.Now()},
		{AuditID: a.ID, Website: "ergonix.lv", URL: "https://ergonix.lv/x", StatusCode: 404,
			CrawledAt: time.Now()},
	}
	if err := db.SavePages(pages); err != nil {
		t.Fatalf("save pages: %v", err)
	}
	gotPages, err := db.ListPages(a.ID)
	if err != nil || len(gotPages) != 2 {
		t.Fatalf("list pages: %v (n=%d)", err, len(gotPages))
	}
	if gotPages[0].H1s[0] != "Hello" {
		t.Errorf("page JSON round trip lost H1s: %+v", gotPages[0])
	}

	issues := []models.Issue{
		{AuditID: a.ID, Website: "ergonix.lt", PageURL: "https://ergonix.lt/", Category: models.CategorySEO,
			Source: models.SourceRule, CheckID: "missing-title", Severity: models.SeverityHigh,
			Title: "Missing title", Confidence: 1, CreatedAt: time.Now()},
		{AuditID: a.ID, Website: "ergonix.lv", PageURL: "https://ergonix.lv/x", Category: models.CategoryNetwork,
			Source: models.SourceRule, Severity: models.SeverityCritical, Title: "404 page", Confidence: 1, CreatedAt: time.Now()},
		{AuditID: a.ID, Website: "ergonix.lt", PageURL: "https://ergonix.lt/", Category: models.CategoryTranslation,
			Source: models.SourceAI, Severity: models.SeverityMedium, Title: "Mixed language content",
			Confidence: 0.8, Details: map[string]any{"lang": "en"}, CreatedAt: time.Now()},
	}
	if err := db.SaveIssues(issues); err != nil {
		t.Fatalf("save issues: %v", err)
	}

	all, total, err := db.ListIssues(IssueFilter{AuditID: a.ID})
	if err != nil || total != 3 {
		t.Fatalf("list all: err=%v total=%d", err, total)
	}
	if all[0].Severity != models.SeverityCritical {
		t.Errorf("expected severity ordering, got first=%s", all[0].Severity)
	}
	for _, is := range all {
		if is.Title == "Missing title" && is.CheckID != "missing-title" {
			t.Errorf("checkId not persisted: %+v", is)
		}
	}

	bySource, total, _ := db.ListIssues(IssueFilter{AuditID: a.ID, Source: "ai"})
	if total != 1 || bySource[0].Category != models.CategoryTranslation {
		t.Errorf("source filter failed: total=%d", total)
	}
	if v, ok := bySource[0].Details["lang"]; !ok || v != "en" {
		t.Errorf("details lost: %+v", bySource[0].Details)
	}

	_, total, _ = db.ListIssues(IssueFilter{AuditID: a.ID, Search: "Mixed"})
	if total != 1 {
		t.Errorf("search filter: total=%d, want 1", total)
	}

	dash, err := db.Dashboard()
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if dash.TotalIssues != 3 || dash.TotalPagesScanned != 2 || dash.TotalWebsitesAudited != 2 {
		t.Errorf("dashboard aggregates wrong: %+v", dash)
	}
	if dash.BySeverity[models.SeverityCritical] != 1 {
		t.Errorf("dashboard severity: %+v", dash.BySeverity)
	}

	// Cascade delete
	if err := db.DeleteAudit(a.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, total, _ := db.ListIssues(IssueFilter{AuditID: a.ID}); total != 0 {
		t.Errorf("issues not cascaded, total=%d", total)
	}
}

func TestSettings(t *testing.T) {
	db := openTestDB(t)
	if err := db.SaveSettings(map[string]string{"aiModel": "gpt-4o-mini", "maxPages": "30"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveSettings(map[string]string{"maxPages": "40"}); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got["maxPages"] != "40" || got["aiModel"] != "gpt-4o-mini" {
		t.Errorf("settings: %+v", got)
	}
}
