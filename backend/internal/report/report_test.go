package report

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ergonix/auditor/backend/internal/models"
)

func sampleAudit() *FullAudit {
	now := time.Now()
	stats := models.NewAuditStats()
	stats.TotalWebsites = 1
	stats.TotalPages = 3
	stats.TotalIssues = 2
	stats.DurationMs = 42_000
	stats.BySeverity[models.SeverityCritical] = 1
	stats.BySeverity[models.SeverityMedium] = 1
	return &FullAudit{
		Audit: &models.Audit{
			ID: 7, Status: models.AuditCompleted, Stage: models.StageDone,
			Params:    models.AuditParams{Websites: []string{"https://ergonix.lt"}},
			Sites:     []models.AuditSite{{Website: "https://ergonix.lt", Status: "completed", PagesCrawled: 3, IssueCount: 2}},
			Stats:     stats,
			CreatedAt: now,
		},
		Issues: []models.Issue{
			{ID: 1, AuditID: 7, Website: "https://ergonix.lt", PageURL: "https://ergonix.lt/x",
				Category: models.CategoryNetwork, Source: models.SourceRule,
				Severity: models.SeverityCritical, Title: "Server error (500)",
				Description: "Boom", SuggestedFix: "Fix it", Confidence: 1},
			{ID: 2, AuditID: 7, Website: "https://ergonix.lt", PageURL: "https://ergonix.lt/y",
				Category: models.CategoryTranslation, Source: models.SourceAI,
				Severity: models.SeverityMedium, Title: "Mišri kalba \"citata\"",
				Description: "Aprašymas lietuviškai, mygtukai angliškai",
				SuggestedFix: "Išversti mygtukus", Confidence: 0.85},
		},
	}
}

func TestJSONExport(t *testing.T) {
	var buf bytes.Buffer
	if err := (JSONExporter{}).Export(&buf, sampleAudit()); err != nil {
		t.Fatal(err)
	}
	var round FullAudit
	if err := json.Unmarshal(buf.Bytes(), &round); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if round.Audit.ID != 7 || len(round.Issues) != 2 {
		t.Errorf("round trip: %+v", round.Audit)
	}
}

func TestCSVExport(t *testing.T) {
	var buf bytes.Buffer
	if err := (CSVExporter{}).Export(&buf, sampleAudit()); err != nil {
		t.Fatal(err)
	}
	content := buf.String()
	if !strings.HasPrefix(content, "\xEF\xBB\xBF") {
		t.Error("missing UTF-8 BOM")
	}
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(content, "\xEF\xBB\xBF")))
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("invalid CSV (quoting broken?): %v", err)
	}
	if len(rows) != 3 { // header + 2 issues
		t.Fatalf("rows = %d", len(rows))
	}
	if rows[2][6] != "Mišri kalba \"citata\"" {
		t.Errorf("unicode/quote handling: %q", rows[2][6])
	}
}

func TestHTMLExport(t *testing.T) {
	var buf bytes.Buffer
	if err := (HTMLExporter{}).Export(&buf, sampleAudit()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{"<!DOCTYPE html>", "Report #7", "Server error (500)", "Mišri kalba"} {
		if !strings.Contains(out, want) {
			t.Errorf("HTML missing %q", want)
		}
	}
	if strings.Contains(out, "<script") {
		t.Error("report should not contain scripts")
	}
}

func TestPDFExport(t *testing.T) {
	var buf bytes.Buffer
	if err := (PDFExporter{}).Export(&buf, sampleAudit()); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("output is not a PDF")
	}
	if buf.Len() < 1000 {
		t.Errorf("suspiciously small PDF: %d bytes", buf.Len())
	}
}

func TestForFormat(t *testing.T) {
	for _, f := range []string{"json", "csv", "html", "pdf"} {
		if ForFormat(f) == nil {
			t.Errorf("no exporter for %s", f)
		}
	}
	if ForFormat("docx") != nil {
		t.Error("unexpected exporter for docx")
	}
}
