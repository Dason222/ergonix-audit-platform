// Package ai analyses extracted page content with an LLM. It never sends
// raw HTML — only compact structured summaries — and every failure is
// non-fatal: a page that can't be analysed is skipped, and a site whose
// pages all fail simply contributes no AI issues.
package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"

	"github.com/ergonix/auditor/backend/internal/checks"
	"github.com/ergonix/auditor/backend/internal/models"
)

// Config tunes the analyzer. Model is recorded on every finding for
// provenance (which model judged the content).
type Config struct {
	MaxPagesPerSite int
	MaxTextChars    int
	Model           string
}

// Analyzer runs LLM content analysis over a site's crawled pages.
type Analyzer struct {
	client Client
	cfg    Config
	log    *slog.Logger
}

// New builds an Analyzer around any Client implementation.
func New(client Client, cfg Config, log *slog.Logger) *Analyzer {
	if cfg.MaxPagesPerSite <= 0 {
		cfg.MaxPagesPerSite = 8
	}
	if cfg.MaxTextChars <= 0 {
		cfg.MaxTextChars = 4000
	}
	return &Analyzer{client: client, cfg: cfg, log: log}
}

// AnalyzeSite picks the richest pages (up to MaxPagesPerSite), sends each
// summary to the LLM, and returns the merged issues. err is non-nil only
// when every attempted page failed — partial success returns issues, nil.
func (a *Analyzer) AnalyzeSite(ctx context.Context, website string, pages []*models.Page,
	onPage func(done, total int)) ([]models.Issue, error) {

	candidates := make([]*models.Page, 0, len(pages))
	for _, p := range pages {
		if p.OK() && p.IsHTML() && p.VisibleText != "" {
			candidates = append(candidates, p)
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return Richness(candidates[i]) > Richness(candidates[j])
	})
	if len(candidates) > a.cfg.MaxPagesPerSite {
		candidates = candidates[:a.cfg.MaxPagesPerSite]
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	expectedLang := checks.ExpectedLanguage(website)
	system := systemPrompt(website, expectedLang)

	var (
		issues   []models.Issue
		failures int
		lastErr  error
	)
	for i, p := range candidates {
		if ctx.Err() != nil {
			break
		}
		pageIssues, err := a.analyzePage(ctx, system, website, p)
		if err != nil {
			failures++
			lastErr = err
			a.log.Warn("ai analysis failed for page", "url", p.URL, "err", err)
		} else {
			issues = append(issues, pageIssues...)
		}
		if onPage != nil {
			onPage(i+1, len(candidates))
		}
	}

	if failures == len(candidates) && lastErr != nil {
		return issues, lastErr
	}
	return issues, nil
}

func (a *Analyzer) analyzePage(ctx context.Context, system, website string, p *models.Page) ([]models.Issue, error) {
	summary := Summarize(p, a.cfg.MaxTextChars)
	summaryJSON, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	raw, err := a.client.ChatJSON(ctx, system, userPrompt(string(summaryJSON)))
	if err != nil {
		return nil, err
	}
	issues, err := ParseFindings(raw, website, p.URL)
	if a.cfg.Model != "" {
		for i := range issues {
			issues[i].Details["model"] = a.cfg.Model
		}
	}
	return issues, err
}
