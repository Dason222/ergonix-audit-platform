// Command audit runs a fully automatic website audit from the terminal:
// it crawls real pages, runs rule-based checks and AI content analysis, and
// writes a clear problem list — no UI interaction required.
//
//	audit -sites https://ergonix.lt -pages 20
//	audit -sites all -formats json,html,csv -out ./reports
//	audit -sites https://ergonix.lt -interval 24h        # keep re-checking
//	audit -sites all -fail-on high                       # CI gate
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/ergonix/auditor/backend/internal/ai"
	"github.com/ergonix/auditor/backend/internal/audit"
	"github.com/ergonix/auditor/backend/internal/browser"
	"github.com/ergonix/auditor/backend/internal/checks"
	"github.com/ergonix/auditor/backend/internal/config"
	"github.com/ergonix/auditor/backend/internal/crawler"
	"github.com/ergonix/auditor/backend/internal/models"
	"github.com/ergonix/auditor/backend/internal/report"
)

func main() {
	var (
		sites    = flag.String("sites", "all", `comma-separated website URLs, or "all" for every configured site`)
		pages    = flag.Int("pages", 0, "max pages per site (default from env)")
		depth    = flag.Int("depth", 0, "max crawl depth (default from env)")
		conc     = flag.Int("concurrency", 0, "parallel requests per site (default from env)")
		useAI    = flag.Bool("ai", true, "run AI content analysis (needs OPENAI_API_KEY)")
		outDir   = flag.String("out", "./reports", "directory for report files")
		formats  = flag.String("formats", "json,html", "report formats: json,csv,html,pdf")
		interval = flag.Duration("interval", 0, "re-run every interval (0 = run once)")
		failOn   = flag.String("fail-on", "", "exit code 2 if issues at/above this severity exist (low|medium|high|critical)")
		quiet    = flag.Bool("quiet", false, "only print the summary and issue list")
	)
	flag.Parse()

	cfg := config.Load()
	level := slog.LevelWarn
	if !*quiet {
		level = slog.LevelInfo
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	params := buildParams(cfg, *sites, *pages, *depth, *conc, *useAI)
	if len(params.Websites) == 0 {
		fmt.Fprintln(os.Stderr, "error: no websites to audit")
		os.Exit(1)
	}
	if err := validateFailOn(*failOn); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	runner := buildRunner(cfg, log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exitCode := 0
	for {
		fa := runner.RunOnce(ctx, params, consoleProgress(*quiet))
		printReport(fa, *quiet)

		files, err := writeReports(fa, *outDir, *formats)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error writing reports:", err)
			exitCode = 1
		}
		for _, f := range files {
			fmt.Printf("report written: %s\n", f)
		}

		if code := failOnCode(fa, *failOn); code != 0 {
			exitCode = code
		}
		if fa.Audit.Status == models.AuditFailed {
			exitCode = 1
		}

		if *interval <= 0 || ctx.Err() != nil {
			break
		}
		fmt.Printf("\nnext automatic check in %s (Ctrl+C to stop)\n", *interval)
		select {
		case <-time.After(*interval):
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			break
		}
	}
	os.Exit(exitCode)
}

// buildParams merges CLI flags over configured defaults.
func buildParams(cfg *config.Config, sites string, pages, depth, conc int, useAI bool) models.AuditParams {
	p := models.AuditParams{
		MaxPages:          firstPositive(pages, cfg.DefaultParams.MaxPages),
		MaxDepth:          firstPositive(depth, cfg.DefaultParams.MaxDepth),
		Concurrency:       firstPositive(conc, cfg.DefaultParams.Concurrency),
		RequestTimeoutSec: cfg.DefaultParams.RequestTimeoutSec,
		RetryCount:        cfg.DefaultParams.RetryCount,
		UseAI:             useAI,
	}
	if strings.EqualFold(strings.TrimSpace(sites), "all") {
		p.Websites = cfg.Websites
	} else {
		for _, s := range strings.Split(sites, ",") {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			if !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
				s = "https://" + s
			}
			p.Websites = append(p.Websites, s)
		}
	}
	p.Normalize()
	return p
}

func buildRunner(cfg *config.Config, log *slog.Logger) *audit.Runner {
	var browserSvc browser.Service = browser.NewNoop()
	if cfg.BrowserEnabled {
		if svc, err := browser.NewPlaywright(log); err != nil {
			log.Warn("Playwright unavailable, continuing HTTP-only", "err", err)
		} else {
			browserSvc = svc
		}
	}

	crawl := crawler.New(crawler.Config{
		UserAgent:           cfg.UserAgent,
		CrawlDelay:          cfg.CrawlDelay,
		RespectRobots:       cfg.RespectRobots,
		MaxBodyBytes:        cfg.MaxBodyBytes,
		BrowserPagesPerSite: cfg.BrowserPagesPerSite,
	}, browserSvc, log)

	engine := checks.New(checks.Config{
		LargeImageKB:         cfg.Checks.LargeImageKB,
		SlowResponseMs:       cfg.Checks.SlowResponseMs,
		SlowLoadMs:           cfg.Checks.SlowLoadMs,
		LargeJSKB:            cfg.Checks.LargeJSKB,
		LargeCSSKB:           cfg.Checks.LargeCSSKB,
		MaxRedirects:         cfg.Checks.MaxRedirects,
		LinkProbeConcurrency: cfg.Checks.LinkProbeConcurrency,
		LinkProbeTimeout:     cfg.Checks.LinkProbeTimeout,
		MaxExternalProbes:    cfg.Checks.MaxExternalProbes,
		UserAgent:            cfg.UserAgent,
	}, log)

	var analyzer *ai.Analyzer
	if cfg.AIEnabled() {
		client := ai.NewOpenAIClient(ai.ClientConfig{
			APIKey:  cfg.AIAPIKey,
			BaseURL: cfg.AIBaseURL,
			Model:   cfg.AIModel,
			Timeout: cfg.AITimeout,
		})
		analyzer = ai.New(client, ai.Config{
			MaxPagesPerSite: cfg.AIMaxPages,
			MaxTextChars:    cfg.AIMaxTextChars,
		}, log)
		log.Info("AI analysis enabled", "model", cfg.AIModel)
	} else {
		log.Info("AI analysis disabled (no OPENAI_API_KEY)")
	}

	return &audit.Runner{Crawler: crawl, Engine: engine, Analyzer: analyzer, Log: log}
}

func consoleProgress(quiet bool) audit.Progress {
	if quiet {
		return nil
	}
	return func(website, stage, detail string) {
		fmt.Printf("[%s] %-6s %s\n", website, stage, detail)
	}
}

// printReport writes the human-readable problem list to stdout.
func printReport(fa *report.FullAudit, quiet bool) {
	stats := fa.Audit.Stats

	fmt.Println()
	fmt.Println("================ ERGONIX WEBSITE AUDIT ================")
	fmt.Printf("Status: %-10s  Websites: %d  Pages: %d  Duration: %s\n",
		fa.Audit.Status, stats.TotalWebsites, stats.TotalPages,
		(time.Duration(stats.DurationMs) * time.Millisecond).Round(time.Second))
	fmt.Printf("Issues: %d  (critical %d · high %d · medium %d · low %d)\n",
		stats.TotalIssues,
		stats.BySeverity[models.SeverityCritical], stats.BySeverity[models.SeverityHigh],
		stats.BySeverity[models.SeverityMedium], stats.BySeverity[models.SeverityLow])
	for _, n := range stats.Notes {
		fmt.Println("note:", n)
	}
	fmt.Println("=======================================================")

	issues := make([]models.Issue, len(fa.Issues))
	copy(issues, fa.Issues)
	sort.SliceStable(issues, func(i, j int) bool {
		return models.SeverityRank[issues[i].Severity] < models.SeverityRank[issues[j].Severity]
	})

	limit := len(issues)
	if quiet && limit > 50 {
		limit = 50
	}
	for _, is := range issues[:limit] {
		fmt.Printf("\n[%s] %s · %s · %s\n", strings.ToUpper(string(is.Severity)),
			is.Category, is.Source, is.PageURL)
		fmt.Printf("  %s\n", is.Title)
		if is.Description != "" && is.Description != is.Title {
			fmt.Printf("  %s\n", is.Description)
		}
		if is.SuggestedFix != "" {
			fmt.Printf("  fix: %s\n", is.SuggestedFix)
		}
	}
	if limit < len(issues) {
		fmt.Printf("\n… and %d more issues (see report files)\n", len(issues)-limit)
	}
	fmt.Println()
}

// writeReports exports the audit in each requested format.
func writeReports(fa *report.FullAudit, outDir, formats string) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	stamp := time.Now().Format("20060102-150405")
	var written []string
	for _, f := range strings.Split(formats, ",") {
		f = strings.ToLower(strings.TrimSpace(f))
		if f == "" {
			continue
		}
		exp := report.ForFormat(f)
		if exp == nil {
			return written, fmt.Errorf("unsupported format %q (json, csv, html, pdf)", f)
		}
		path := filepath.Join(outDir, fmt.Sprintf("ergonix-audit-%s.%s", stamp, exp.Ext()))
		file, err := os.Create(path)
		if err != nil {
			return written, err
		}
		err = exp.Export(file, fa)
		file.Close()
		if err != nil {
			return written, fmt.Errorf("export %s: %w", f, err)
		}
		written = append(written, path)
	}
	return written, nil
}

func validateFailOn(s string) error {
	switch s {
	case "", "low", "medium", "high", "critical":
		return nil
	}
	return fmt.Errorf("invalid -fail-on %q (low|medium|high|critical)", s)
}

// failOnCode returns 2 when issues at or above the threshold severity exist.
func failOnCode(fa *report.FullAudit, failOn string) int {
	if failOn == "" {
		return 0
	}
	threshold := models.SeverityRank[models.Severity(failOn)]
	for _, is := range fa.Issues {
		if models.SeverityRank[is.Severity] <= threshold {
			return 2
		}
	}
	return 0
}

func firstPositive(vals ...int) int {
	for _, v := range vals {
		if v > 0 {
			return v
		}
	}
	return 0
}
