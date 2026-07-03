package audit

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ergonix/auditor/backend/internal/ai"
	"github.com/ergonix/auditor/backend/internal/checks"
	"github.com/ergonix/auditor/backend/internal/crawler"
	"github.com/ergonix/auditor/backend/internal/models"
)

func discard() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fixtureSite serves two pages: a healthy home page linking to a page with
// deliberate problems (no title, no h1, broken link).
func fixtureSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html lang="lt"><head><title>Namai</title>
			<meta name="description" content="Ergonomiški baldai namams ir biurui."></head>
			<body><h1>Sveiki</h1><a href="/broken-page">Kėdės</a><a href="/dingo">Dingęs</a>
			<p>Kaina: 129,99 €</p></body></html>`)
	})
	mux.HandleFunc("/broken-page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html lang="lt"><head></head><body><p>Puslapis be pavadinimo</p></body></html>`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func fixtureRunner(analyzer *ai.Analyzer) *Runner {
	return &Runner{
		Crawler:  crawler.New(crawler.Config{UserAgent: "test-bot"}, nil, discard()),
		Engine:   checks.New(checks.Defaults(), discard()),
		Analyzer: analyzer,
		Log:      discard(),
	}
}

func runnerParams(site string) models.AuditParams {
	p := models.AuditParams{
		Websites: []string{site}, MaxPages: 10, MaxDepth: 2,
		Concurrency: 2, RequestTimeoutSec: 5, UseAI: false,
	}
	return p
}

func TestRunnerRunOnce(t *testing.T) {
	srv := fixtureSite(t)
	r := fixtureRunner(nil)

	var stages []string
	fa := r.RunOnce(context.Background(), runnerParams(srv.URL),
		func(_, stage, _ string) { stages = append(stages, stage) })

	if fa.Audit.Status != models.AuditCompleted {
		t.Fatalf("status = %s (%s)", fa.Audit.Status, fa.Audit.Error)
	}
	if fa.Audit.Stats.TotalPages != 3 { // /, /broken-page, /dingo(404)
		t.Errorf("pages = %d, want 3", fa.Audit.Stats.TotalPages)
	}
	if fa.Audit.Stats.TotalIssues == 0 || len(fa.Issues) != fa.Audit.Stats.TotalIssues {
		t.Errorf("issues: stats=%d len=%d", fa.Audit.Stats.TotalIssues, len(fa.Issues))
	}
	// The 404 page and the missing title must be among the findings.
	var saw404, sawTitle bool
	for _, is := range fa.Issues {
		if strings.Contains(is.Title, "not found") {
			saw404 = true
		}
		if strings.Contains(is.Title, "Missing page title") {
			sawTitle = true
		}
	}
	if !saw404 || !sawTitle {
		t.Errorf("expected 404 + missing-title findings (saw404=%v sawTitle=%v)", saw404, sawTitle)
	}
	if fa.Audit.Stats.BySeverity[models.SeverityHigh] == 0 {
		t.Errorf("severity tally empty: %+v", fa.Audit.Stats.BySeverity)
	}
	if len(fa.Audit.Sites) != 1 || fa.Audit.Sites[0].Status != "completed" {
		t.Errorf("sites: %+v", fa.Audit.Sites)
	}
	joined := strings.Join(stages, ",")
	if !strings.Contains(joined, "crawl") || !strings.Contains(joined, "checks") {
		t.Errorf("progress stages: %v", stages)
	}
}

func TestRunnerAISkippedNote(t *testing.T) {
	srv := fixtureSite(t)
	r := fixtureRunner(nil)
	p := runnerParams(srv.URL)
	p.UseAI = true // requested but no analyzer configured

	fa := r.RunOnce(context.Background(), p, nil)
	if !fa.Audit.Stats.AISkipped {
		t.Error("expected aiSkipped=true when AI requested without analyzer")
	}
}

func TestRunnerAllSitesFailed(t *testing.T) {
	r := fixtureRunner(nil)
	p := runnerParams("http://127.0.0.1:1") // nothing listens here
	p.RetryCount = 0
	p.RequestTimeoutSec = 2

	fa := r.RunOnce(context.Background(), p, nil)
	// A dead host still yields one recorded error page, so the site
	// "completes" with a failed-to-load finding — the audit must never
	// be empty-handed.
	if fa.Audit.Stats.TotalIssues == 0 && fa.Audit.Status == models.AuditCompleted {
		t.Errorf("dead site produced no findings and no failure: %+v", fa.Audit)
	}
}
