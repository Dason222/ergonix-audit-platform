// Command server runs the Ergonix Website Audit Platform backend.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ergonix/auditor/backend/internal/ai"
	"github.com/ergonix/auditor/backend/internal/api"
	"github.com/ergonix/auditor/backend/internal/audit"
	"github.com/ergonix/auditor/backend/internal/browser"
	"github.com/ergonix/auditor/backend/internal/checks"
	"github.com/ergonix/auditor/backend/internal/config"
	"github.com/ergonix/auditor/backend/internal/crawler"
	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
	"github.com/ergonix/auditor/backend/internal/scheduler"
)

func main() {
	cfg := config.Load()
	log := newLogger(cfg.LogLevel)

	store, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Error("opening database", "path", cfg.DBPath, "err", err)
		os.Exit(1)
	}
	defer store.Close()

	// Browser service: Playwright when enabled and installable, else noop.
	var browserSvc browser.Service = browser.NewNoop()
	if cfg.BrowserEnabled {
		if svc, err := browser.NewPlaywright(log); err != nil {
			log.Warn("Playwright unavailable, continuing HTTP-only", "err", err)
		} else {
			browserSvc = svc
			log.Info("Playwright browser service ready")
		}
	}
	defer browserSvc.Close()

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
			Model:           cfg.AIModel,
		}, log)
		log.Info("AI analysis enabled", "model", cfg.AIModel, "baseURL", cfg.AIBaseURL)
	} else {
		log.Info("AI analysis disabled (no OPENAI_API_KEY)")
	}

	orch := audit.New(store, crawl, engine, analyzer,
		audit.Config{SiteConcurrency: cfg.SiteConcurrency}, log)

	// Automatic recurring audits.
	schedWebsites := cfg.ScheduleWebsites
	if len(schedWebsites) == 0 {
		schedWebsites = cfg.Websites
	}
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	sched := scheduler.New(store, orch, scheduler.Config{
		Enabled:  cfg.ScheduleEnabled,
		Interval: cfg.ScheduleInterval,
		AtStart:  cfg.ScheduleAtStart,
		Websites: schedWebsites,
		Params: models.AuditParams{
			MaxPages:          cfg.DefaultParams.MaxPages,
			MaxDepth:          cfg.DefaultParams.MaxDepth,
			Concurrency:       cfg.DefaultParams.Concurrency,
			RequestTimeoutSec: cfg.DefaultParams.RequestTimeoutSec,
			RetryCount:        cfg.DefaultParams.RetryCount,
			UseAI:             cfg.AIEnabled(),
		},
	}, log)
	sched.Start(schedCtx)

	server := api.NewServer(store, orch, cfg, log)
	httpSrv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("backend listening", "port", cfg.Port)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server", "err", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown: stop accepting requests, cancel audits, flush.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Info("shutting down…")
	schedCancel() // stop scheduling new audits

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Warn("http shutdown", "err", err)
	}
	orch.Shutdown()
	log.Info("bye")
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
