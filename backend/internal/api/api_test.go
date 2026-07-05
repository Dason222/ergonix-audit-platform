package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ergonix/auditor/backend/internal/audit"
	"github.com/ergonix/auditor/backend/internal/checks"
	"github.com/ergonix/auditor/backend/internal/config"
	"github.com/ergonix/auditor/backend/internal/crawler"
	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
	"github.com/ergonix/auditor/backend/internal/scheduler"
	"github.com/ergonix/auditor/backend/internal/settings"
)

// newTestServer wires a full real stack (SQLite temp DB, crawler, checks,
// no AI) behind the Gin router.
func newTestServer(t *testing.T) (*httptest.Server, database.Store) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, err := database.Open(filepath.Join(t.TempDir(), "api-test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	crawl := crawler.New(crawler.Config{UserAgent: "test-bot", CrawlDelay: 0}, nil, log)
	engine := checks.New(checks.Defaults(), log)
	orch := audit.New(store, crawl, engine, nil, audit.Config{SiteConcurrency: 2}, log)

	cfg := &config.Config{
		Websites:         []string{"https://ergonix.lt", "https://ergonix.lv"},
		CORSOrigins:      []string{"http://localhost:5173"},
		AIModel:          "gpt-4o-mini",
		AIBaseURL:        "https://api.openai.com/v1",
		ScheduleInterval: 24 * time.Hour,
	}
	sched := scheduler.New(store, orch, scheduler.Config{}, log)
	sm, err := settings.New(cfg, store, orch, sched, log)
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(store, orch, sm, cfg, log)
	ts := httptest.NewServer(srv.Router())
	t.Cleanup(ts.Close)
	return ts, store
}

// newTargetSite serves a tiny website with one deliberate 404 link and a
// page missing its title, so an end-to-end audit produces issues.
func newTargetSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html lang="lt"><head><title>Namai</title>
			<meta name="description" content="Aprašymas"></head>
			<body><h1>Sveiki</h1><a href="/kede">Kėdė</a><a href="/dinges">Dingęs</a></body></html>`)
	})
	mux.HandleFunc("/kede", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html lang="lt"><head></head><body><p>Be pavadinimo</p></body></html>`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func postJSON(t *testing.T, url string, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decode[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}

func TestCreateAuditValidation(t *testing.T) {
	ts, _ := newTestServer(t)

	resp := postJSON(t, ts.URL+"/api/audits", `{"websites":[]}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("empty websites: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = postJSON(t, ts.URL+"/api/audits", `{"websites":["not a url ::"]}`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid url: status %d", resp.StatusCode)
	}
	resp.Body.Close()

	resp = postJSON(t, ts.URL+"/api/audits", `not json`)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad json: status %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestFullAuditLifecycle(t *testing.T) {
	ts, _ := newTestServer(t)
	target := newTargetSite(t)

	// Create audit against the local target site.
	resp := postJSON(t, ts.URL+"/api/audits",
		fmt.Sprintf(`{"websites":[%q],"maxPages":10,"maxDepth":2,"concurrency":2,"useAI":false}`, target.URL))
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create: status %d: %s", resp.StatusCode, body)
	}
	created := decode[models.Audit](t, resp)
	if created.ID == 0 || created.Status != models.AuditPending {
		t.Fatalf("created: %+v", created)
	}

	// Poll until the audit completes (crawl of 3 URLs is fast).
	var final models.Audit
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("audit did not finish; last: %+v", final)
		}
		r, err := http.Get(fmt.Sprintf("%s/api/audits/%d", ts.URL, created.ID))
		if err != nil {
			t.Fatal(err)
		}
		final = decode[models.Audit](t, r)
		if final.Status == models.AuditCompleted || final.Status == models.AuditFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if final.Status != models.AuditCompleted {
		t.Fatalf("audit failed: %+v", final)
	}
	if final.Stats.TotalPages != 3 { // home, /kede, /dinges(404)
		t.Errorf("pages = %d, want 3", final.Stats.TotalPages)
	}
	if final.Stats.TotalIssues == 0 {
		t.Error("expected issues (404 + missing title/h1/meta)")
	}

	// Issues endpoint + filters.
	r, _ := http.Get(fmt.Sprintf("%s/api/audits/%d/issues", ts.URL, created.ID))
	issuesResp := decode[struct {
		Issues []models.Issue `json:"issues"`
		Total  int            `json:"total"`
	}](t, r)
	if issuesResp.Total == 0 || len(issuesResp.Issues) != issuesResp.Total {
		t.Fatalf("issues: %+v", issuesResp)
	}
	r, _ = http.Get(fmt.Sprintf("%s/api/audits/%d/issues?search=title", ts.URL, created.ID))
	filtered := decode[struct {
		Issues []models.Issue `json:"issues"`
		Total  int            `json:"total"`
	}](t, r)
	if filtered.Total == 0 || filtered.Total >= issuesResp.Total {
		t.Errorf("search filter: %d of %d", filtered.Total, issuesResp.Total)
	}

	// Exports.
	for format, wantType := range map[string]string{
		"json": "application/json", "csv": "text/csv", "html": "text/html", "pdf": "application/pdf",
	} {
		r, err := http.Get(fmt.Sprintf("%s/api/audits/%d/export/%s", ts.URL, created.ID, format))
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(r.Body)
		r.Body.Close()
		if r.StatusCode != 200 || !strings.Contains(r.Header.Get("Content-Type"), wantType) {
			t.Errorf("export %s: status=%d type=%s", format, r.StatusCode, r.Header.Get("Content-Type"))
		}
		if len(body) == 0 {
			t.Errorf("export %s: empty body", format)
		}
	}
	r, _ = http.Get(fmt.Sprintf("%s/api/audits/%d/export/docx", ts.URL, created.ID))
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("unsupported format: status %d", r.StatusCode)
	}
	r.Body.Close()

	// Dashboard aggregates.
	r, _ = http.Get(ts.URL + "/api/dashboard")
	dash := decode[database.DashboardData](t, r)
	if dash.TotalAudits != 1 || dash.TotalPagesScanned != 3 {
		t.Errorf("dashboard: %+v", dash)
	}

	// List + delete.
	r, _ = http.Get(ts.URL + "/api/audits")
	list := decode[struct {
		Audits []models.Audit `json:"audits"`
		Total  int            `json:"total"`
	}](t, r)
	if list.Total != 1 {
		t.Errorf("list total = %d", list.Total)
	}

	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/audits/%d", ts.URL, created.ID), nil)
	dr, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	dr.Body.Close()
	if dr.StatusCode != http.StatusNoContent {
		t.Errorf("delete: status %d", dr.StatusCode)
	}
	r, _ = http.Get(fmt.Sprintf("%s/api/audits/%d", ts.URL, created.ID))
	if r.StatusCode != http.StatusNotFound {
		t.Errorf("after delete: status %d", r.StatusCode)
	}
	r.Body.Close()
}

func TestMetaEndpoints(t *testing.T) {
	ts, _ := newTestServer(t)

	r, _ := http.Get(ts.URL + "/api/websites")
	sites := decode[struct {
		Websites []string `json:"websites"`
	}](t, r)
	if len(sites.Websites) != 2 {
		t.Errorf("websites: %+v", sites)
	}

	resp := postJSON(t, ts.URL+"/api/audits/abc/cancel", ``)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("bad id: %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Settings round trip: enable schedule + set AI key, then read back.
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings",
		strings.NewReader(`{"scheduleEnabled":true,"scheduleIntervalHours":12,"aiApiKey":"sk-test-1234567890","aiModel":"gpt-4o"}`))
	req.Header.Set("Content-Type", "application/json")
	pr, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	pr.Body.Close()
	if pr.StatusCode != 200 {
		t.Errorf("put settings: %d", pr.StatusCode)
	}
	r, _ = http.Get(ts.URL + "/api/settings")
	got := decode[struct {
		AI struct {
			Enabled    bool   `json:"enabled"`
			KeySet     bool   `json:"keySet"`
			KeyPreview string `json:"keyPreview"`
			Model      string `json:"model"`
		} `json:"ai"`
		Schedule struct {
			Enabled       bool `json:"enabled"`
			IntervalHours int  `json:"intervalHours"`
		} `json:"schedule"`
	}](t, r)
	if !got.Schedule.Enabled || got.Schedule.IntervalHours != 12 {
		t.Errorf("schedule not persisted: %+v", got.Schedule)
	}
	if !got.AI.Enabled || !got.AI.KeySet || got.AI.Model != "gpt-4o" {
		t.Errorf("ai not persisted: %+v", got.AI)
	}
	if strings.Contains(got.AI.KeyPreview, "567890") { // full key must not leak
		t.Errorf("AI key leaked in preview: %q", got.AI.KeyPreview)
	}

	// Interval out of range is rejected.
	req2, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/settings",
		strings.NewReader(`{"scheduleIntervalHours":9999}`))
	req2.Header.Set("Content-Type", "application/json")
	pr2, _ := http.DefaultClient.Do(req2)
	pr2.Body.Close()
	if pr2.StatusCode != http.StatusBadRequest {
		t.Errorf("out-of-range interval accepted: %d", pr2.StatusCode)
	}

	// Health.
	r, _ = http.Get(ts.URL + "/healthz")
	if r.StatusCode != 200 {
		t.Errorf("healthz: %d", r.StatusCode)
	}
	r.Body.Close()
}
