// Package audit orchestrates the full pipeline for one audit run:
// crawl → rule checks → AI analysis → merge → stats → persist.
// Sites are processed concurrently (bounded); every stage failure is
// contained so one bad site or page never kills the audit.
package audit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ergonix/auditor/backend/internal/ai"
	"github.com/ergonix/auditor/backend/internal/checks"
	"github.com/ergonix/auditor/backend/internal/crawler"
	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
)

// Config tunes the orchestrator.
type Config struct {
	// SiteConcurrency bounds how many websites run their pipelines at once.
	SiteConcurrency int
}

// Orchestrator runs audits asynchronously and tracks the running set for
// cancellation.
type Orchestrator struct {
	store    database.Store
	crawler  *crawler.Crawler
	engine   *checks.Engine
	analyzer *ai.Analyzer // nil when AI is not configured
	cfg      Config
	log      *slog.Logger

	mu      sync.Mutex
	running map[int64]context.CancelFunc
	wg      sync.WaitGroup
}

// New wires the pipeline. analyzer may be nil (AI disabled).
func New(store database.Store, c *crawler.Crawler, e *checks.Engine,
	analyzer *ai.Analyzer, cfg Config, log *slog.Logger) *Orchestrator {
	if cfg.SiteConcurrency <= 0 {
		cfg.SiteConcurrency = 2
	}
	return &Orchestrator{
		store: store, crawler: c, engine: e, analyzer: analyzer,
		cfg: cfg, log: log, running: map[int64]context.CancelFunc{},
	}
}

// Start launches the audit pipeline in a goroutine and returns immediately.
func (o *Orchestrator) Start(a *models.Audit) {
	ctx, cancel := context.WithCancel(context.Background())
	o.mu.Lock()
	o.running[a.ID] = cancel
	o.mu.Unlock()

	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				o.log.Error("audit panicked", "audit", a.ID, "panic", fmt.Sprint(r))
				a.Status = models.AuditFailed
				a.Error = fmt.Sprintf("internal error: %v", r)
				o.finish(a)
			}
			o.mu.Lock()
			delete(o.running, a.ID)
			o.mu.Unlock()
			cancel()
		}()
		o.run(ctx, a)
	}()
}

// Cancel stops a running audit. Returns false if it wasn't running.
func (o *Orchestrator) Cancel(id int64) bool {
	o.mu.Lock()
	cancel, ok := o.running[id]
	o.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// Wait blocks until all running audits finish (used for graceful shutdown).
func (o *Orchestrator) Wait() { o.wg.Wait() }

// Shutdown cancels every running audit and waits for their goroutines to
// persist a final state.
func (o *Orchestrator) Shutdown() {
	o.mu.Lock()
	for _, cancel := range o.running {
		cancel()
	}
	o.mu.Unlock()
	o.wg.Wait()
}

// run executes the pipeline for every requested website.
func (o *Orchestrator) run(ctx context.Context, a *models.Audit) {
	started := time.Now()
	a.Status = models.AuditRunning
	a.Stage = models.StageCrawling
	a.StartedAt = &started
	a.Sites = make([]models.AuditSite, len(a.Params.Websites))
	for i, w := range a.Params.Websites {
		a.Sites[i] = models.AuditSite{Website: w, Status: "pending"}
	}
	o.persist(a)

	type siteResult struct {
		idx    int
		pages  []*models.Page
		issues []models.Issue
		err    error
	}

	results := make([]siteResult, len(a.Params.Websites))
	sem := make(chan struct{}, o.cfg.SiteConcurrency)
	var wg sync.WaitGroup

	for i, website := range a.Params.Websites {
		wg.Add(1)
		go func(idx int, site string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			pages, issues, err := o.runSite(ctx, a, idx, site)
			results[idx] = siteResult{idx: idx, pages: pages, issues: issues, err: err}
		}(i, website)
	}
	wg.Wait()

	if ctx.Err() != nil {
		a.Status = models.AuditCancelled
		a.Stage = models.StageDone
		a.Error = "cancelled by user"
		o.finish(a)
		return
	}

	// Merge + persist results.
	a.Stage = models.StageReporting
	o.persist(a)

	stats := models.NewAuditStats()
	stats.TotalWebsites = len(a.Params.Websites)
	failures := 0

	var allIssues []models.Issue
	for _, r := range results {
		if r.err != nil {
			failures++
			continue
		}
		for _, p := range r.pages {
			p.AuditID = a.ID
		}
		if err := o.store.SavePages(r.pages); err != nil {
			o.log.Error("saving pages", "audit", a.ID, "err", err)
		}
		stats.TotalPages += len(r.pages)
		allIssues = append(allIssues, r.issues...)
	}
	for i := range allIssues {
		allIssues[i].AuditID = a.ID
	}
	if err := o.store.SaveIssues(allIssues); err != nil {
		o.log.Error("saving issues", "audit", a.ID, "err", err)
		a.Error = "failed to save issues: " + err.Error()
	}

	stats.TotalIssues = len(allIssues)
	tallyIssues(&stats, allIssues)
	stats.DurationMs = time.Since(started).Milliseconds()
	if a.Params.UseAI && o.analyzer == nil {
		stats.AISkipped = true
		stats.Notes = append(stats.Notes, "AI analysis skipped: no API key configured")
	}
	for i := range a.Sites {
		stats.Notes = appendSiteNote(stats.Notes, &a.Sites[i])
	}
	a.Stats = stats

	if failures == len(a.Params.Websites) && failures > 0 {
		a.Status = models.AuditFailed
		if a.Error == "" {
			a.Error = "all websites failed to audit"
		}
	} else {
		a.Status = models.AuditCompleted
	}
	a.Stage = models.StageDone
	o.finish(a)
	o.log.Info("audit finished", "audit", a.ID, "status", a.Status,
		"pages", stats.TotalPages, "issues", stats.TotalIssues,
		"duration", time.Since(started).Round(time.Millisecond))
}

// runSite executes crawl → checks → AI for one website.
func (o *Orchestrator) runSite(ctx context.Context, a *models.Audit, idx int, website string) (
	pages []*models.Page, issues []models.Issue, err error) {

	defer func() {
		if r := recover(); r != nil {
			o.log.Error("site pipeline panicked", "website", website, "panic", fmt.Sprint(r))
			err = fmt.Errorf("internal error auditing %s: %v", website, r)
			o.setSite(a, idx, func(s *models.AuditSite) {
				s.Status = "failed"
				s.Error = err.Error()
			})
		}
	}()

	siteStart := time.Now()
	o.setSite(a, idx, func(s *models.AuditSite) { s.Status = "crawling" })

	pages, err = o.crawler.Crawl(ctx, website, a.Params, func(done int) {
		o.setSite(a, idx, func(s *models.AuditSite) { s.PagesCrawled = done })
	})
	if err != nil {
		o.setSite(a, idx, func(s *models.AuditSite) {
			s.Status = "failed"
			s.Error = "crawl: " + err.Error()
			s.DurationMs = time.Since(siteStart).Milliseconds()
		})
		return nil, nil, err
	}

	o.setSite(a, idx, func(s *models.AuditSite) { s.Status = "checking" })
	issues = o.engine.Run(ctx, &checks.SiteContext{Website: website, Pages: pages})

	if a.Params.UseAI && o.analyzer != nil && ctx.Err() == nil {
		o.setSite(a, idx, func(s *models.AuditSite) { s.Status = "ai_analysis" })
		aiIssues, aiErr := o.analyzer.AnalyzeSite(ctx, website, pages, nil)
		issues = append(issues, aiIssues...)
		if aiErr != nil {
			o.log.Warn("ai analysis failed for site; continuing", "website", website, "err", aiErr)
			o.setSite(a, idx, func(s *models.AuditSite) {
				s.Error = "AI analysis unavailable: " + truncateErr(aiErr)
			})
		}
	}

	o.setSite(a, idx, func(s *models.AuditSite) {
		s.Status = "completed"
		s.IssueCount = len(issues)
		s.DurationMs = time.Since(siteStart).Milliseconds()
	})
	return pages, issues, nil
}

// setSite mutates one site entry under lock and persists progress.
func (o *Orchestrator) setSite(a *models.Audit, idx int, mutate func(*models.AuditSite)) {
	o.mu.Lock()
	mutate(&a.Sites[idx])
	// Derive the audit-level stage from site states.
	stage := models.StageCrawling
	for i := range a.Sites {
		switch a.Sites[i].Status {
		case "ai_analysis":
			stage = models.StageAI
		case "checking":
			if stage != models.StageAI {
				stage = models.StageChecking
			}
		}
	}
	a.Stage = stage
	o.mu.Unlock()
	o.persist(a)
}

func (o *Orchestrator) persist(a *models.Audit) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if err := o.store.UpdateAudit(a); err != nil {
		o.log.Error("persisting audit", "audit", a.ID, "err", err)
	}
}

func (o *Orchestrator) finish(a *models.Audit) {
	now := time.Now()
	a.FinishedAt = &now
	o.persist(a)
}

func appendSiteNote(notes []string, s *models.AuditSite) []string {
	if s.Error != "" {
		return append(notes, fmt.Sprintf("%s: %s", s.Website, s.Error))
	}
	return notes
}

func truncateErr(err error) string {
	s := err.Error()
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}
