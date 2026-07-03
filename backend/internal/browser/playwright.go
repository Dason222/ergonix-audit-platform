package browser

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"

	"github.com/ergonix/auditor/backend/internal/models"
)

// Playwright implements Service with a headless Chromium instance. One
// browser is shared; each Enrich call gets its own context+page.
type Playwright struct {
	pw      *playwright.Playwright
	browser playwright.Browser
	log     *slog.Logger
	mu      sync.Mutex // playwright-go driver calls are serialized defensively
}

// NewPlaywright starts the Playwright driver and launches headless Chromium.
// The caller should fall back to NewNoop() when this returns an error
// (driver or browsers not installed).
func NewPlaywright(log *slog.Logger) (Service, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("start playwright driver: %w", err)
	}
	b, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args:     []string{"--disable-gpu", "--no-sandbox"},
	})
	if err != nil {
		pw.Stop()
		return nil, fmt.Errorf("launch chromium: %w", err)
	}
	return &Playwright{pw: pw, browser: b, log: log}, nil
}

// Enrich loads the page in Chromium and captures console errors, failed
// requests, and the wall-clock time until the load event.
func (s *Playwright) Enrich(ctx context.Context, p *models.Page) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bctx, err := s.browser.NewContext(playwright.BrowserNewContextOptions{
		UserAgent: playwright.String("ErgonixAuditBot/1.0 (+internal site audit; browser pass)"),
	})
	if err != nil {
		return fmt.Errorf("browser context: %w", err)
	}
	defer bctx.Close()

	page, err := bctx.NewPage()
	if err != nil {
		return fmt.Errorf("new page: %w", err)
	}

	var (
		consoleErrors  []string
		failedRequests []string
	)
	page.OnConsole(func(msg playwright.ConsoleMessage) {
		if msg.Type() == "error" {
			consoleErrors = append(consoleErrors, truncate(msg.Text(), 500))
		}
	})
	page.OnRequestFailed(func(req playwright.Request) {
		failure := ""
		if f := req.Failure(); f != nil {
			failure = f.Error()
		}
		// Aborted requests (ad blockers, page teardown) are not site bugs.
		if failure == "net::ERR_ABORTED" {
			return
		}
		failedRequests = append(failedRequests, fmt.Sprintf("%s (%s)", req.URL(), failure))
	})
	page.OnResponse(func(resp playwright.Response) {
		if resp.Status() >= 400 {
			failedRequests = append(failedRequests,
				fmt.Sprintf("%s (HTTP %d)", resp.URL(), resp.Status()))
		}
	})

	deadline := 30 * time.Second
	if d, ok := ctx.Deadline(); ok {
		if until := time.Until(d); until < deadline {
			deadline = until
		}
	}

	start := time.Now()
	_, err = page.Goto(p.FinalURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateLoad,
		Timeout:   playwright.Float(float64(deadline.Milliseconds())),
	})
	if err != nil {
		return fmt.Errorf("goto %s: %w", p.FinalURL, err)
	}
	// Give late console errors/async requests a moment to surface.
	page.WaitForTimeout(1000)

	p.LoadTimeMs = time.Since(start).Milliseconds()
	p.ConsoleErrors = dedupe(consoleErrors, 20)
	p.FailedRequests = dedupe(failedRequests, 20)
	return nil
}

// Close shuts down the browser and driver.
func (s *Playwright) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.browser.Close(); err != nil {
		s.log.Warn("closing browser", "err", err)
	}
	return s.pw.Stop()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func dedupe(in []string, limit int) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}
