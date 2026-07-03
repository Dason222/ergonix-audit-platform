package audit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ergonix/auditor/backend/internal/ai"
	"github.com/ergonix/auditor/backend/internal/checks"
	"github.com/ergonix/auditor/backend/internal/crawler"
	"github.com/ergonix/auditor/backend/internal/models"
	"github.com/ergonix/auditor/backend/internal/report"
)

// Runner executes the audit pipeline once, standalone — no database, no HTTP
// server. It powers the CLI (cmd/audit) and is what makes fully automatic,
// hands-free checks possible (cron / Task Scheduler / CI).
type Runner struct {
	Crawler  *crawler.Crawler
	Engine   *checks.Engine
	Analyzer *ai.Analyzer // nil = AI stage skipped
	Log      *slog.Logger
}

// Progress receives human-readable pipeline events for console output.
type Progress func(website, stage string, detail string)

// RunOnce crawls every requested website, runs rule checks and (when
// configured) AI analysis, and returns the merged report. Per-site failures
// are contained: a site that cannot be crawled contributes an error note,
// not an aborted run.
func (r *Runner) RunOnce(ctx context.Context, params models.AuditParams, progress Progress) *report.FullAudit {
	if progress == nil {
		progress = func(string, string, string) {}
	}
	started := time.Now()

	a := &models.Audit{
		Status:    models.AuditRunning,
		Stage:     models.StageCrawling,
		Params:    params,
		Stats:     models.NewAuditStats(),
		CreatedAt: started,
		StartedAt: &started,
	}

	var (
		allPages  []*models.Page
		allIssues []models.Issue
		failures  int
	)

	for _, website := range params.Websites {
		if ctx.Err() != nil {
			break
		}
		siteStart := time.Now()
		site := models.AuditSite{Website: website, Status: "crawling"}

		progress(website, "crawl", fmt.Sprintf("up to %d pages, depth %d", params.MaxPages, params.MaxDepth))
		pages, err := r.Crawler.Crawl(ctx, website, params, nil)
		if err != nil {
			failures++
			site.Status = "failed"
			site.Error = "crawl: " + err.Error()
			site.DurationMs = time.Since(siteStart).Milliseconds()
			a.Sites = append(a.Sites, site)
			progress(website, "error", site.Error)
			continue
		}
		site.PagesCrawled = len(pages)
		progress(website, "crawl", fmt.Sprintf("%d pages fetched", len(pages)))

		progress(website, "checks", "running rule-based checks")
		issues := r.Engine.Run(ctx, &checks.SiteContext{Website: website, Pages: pages})
		progress(website, "checks", fmt.Sprintf("%d findings", len(issues)))

		if params.UseAI && r.Analyzer != nil && ctx.Err() == nil {
			progress(website, "ai", "analysing content with AI")
			aiIssues, aiErr := r.Analyzer.AnalyzeSite(ctx, website, pages, nil)
			issues = append(issues, aiIssues...)
			if aiErr != nil {
				site.Error = "AI analysis unavailable: " + truncateErr(aiErr)
				progress(website, "ai", site.Error)
			} else {
				progress(website, "ai", fmt.Sprintf("%d findings", len(aiIssues)))
			}
		}

		site.Status = "completed"
		site.IssueCount = len(issues)
		site.DurationMs = time.Since(siteStart).Milliseconds()
		a.Sites = append(a.Sites, site)
		allPages = append(allPages, pages...)
		allIssues = append(allIssues, issues...)
	}

	// Aggregate.
	a.Stats.TotalWebsites = len(params.Websites)
	a.Stats.TotalPages = len(allPages)
	a.Stats.TotalIssues = len(allIssues)
	tallyIssues(&a.Stats, allIssues)
	a.Stats.DurationMs = time.Since(started).Milliseconds()
	if params.UseAI && r.Analyzer == nil {
		a.Stats.AISkipped = true
		a.Stats.Notes = append(a.Stats.Notes, "AI analysis skipped: no API key configured")
	}
	for i := range a.Sites {
		a.Stats.Notes = appendSiteNote(a.Stats.Notes, &a.Sites[i])
	}

	finished := time.Now()
	a.FinishedAt = &finished
	a.Stage = models.StageDone
	if failures == len(params.Websites) && failures > 0 {
		a.Status = models.AuditFailed
		a.Error = "all websites failed to audit"
	} else if ctx.Err() != nil {
		a.Status = models.AuditCancelled
	} else {
		a.Status = models.AuditCompleted
	}

	return &report.FullAudit{Audit: a, Issues: allIssues, Pages: allPages}
}

// tallyIssues fills the per-severity/category/website/source counters.
func tallyIssues(stats *models.AuditStats, issues []models.Issue) {
	for _, is := range issues {
		stats.BySeverity[is.Severity]++
		stats.ByCategory[is.Category]++
		stats.ByWebsite[is.Website]++
		stats.BySource[is.Source]++
	}
}
