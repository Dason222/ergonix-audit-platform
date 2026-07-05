// Package checks implements the rule-based validation engine. Checks come in
// two granularities: PageCheck runs once per crawled page, SiteCheck runs
// once per website with the full page set (duplicates, link graph, …).
package checks

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ergonix/auditor/backend/internal/models"
)

// SiteContext carries everything checks may inspect for one website.
type SiteContext struct {
	Website string
	Pages   []*models.Page
	// Links maps absolute URL -> probe result for every unique link found
	// on the site. Populated by the engine before checks run.
	Links LinkStatusMap
	// Recon holds active security/infrastructure probe results.
	Recon *ReconData
	Cfg   Config
}

// PageCheck inspects a single page.
type PageCheck interface {
	Name() string
	CheckPage(sc *SiteContext, p *models.Page) []models.Issue
}

// SiteCheck inspects the whole site at once.
type SiteCheck interface {
	Name() string
	CheckSite(sc *SiteContext) []models.Issue
}

// Engine owns the registered checks and runs them with panic isolation.
type Engine struct {
	cfg        Config
	log        *slog.Logger
	prober     *LinkProber
	pageChecks []PageCheck
	siteChecks []SiteCheck
}

// New creates an Engine with the default check set registered.
func New(cfg Config, log *slog.Logger) *Engine {
	e := &Engine{cfg: cfg, log: log, prober: NewLinkProber(cfg, log)}
	e.registerDefaults()
	return e
}

func (e *Engine) registerDefaults() {
	e.pageChecks = []PageCheck{
		&StatusCodeCheck{},
		&TitleCheck{},
		&MetaDescriptionCheck{},
		&H1Check{},
		&ImageAltCheck{},
		&LargeImageCheck{},
		&SlowResponseCheck{},
		&SlowLoadCheck{},
		&MixedContentCheck{},
		&HTTPLinkCheck{},
		&HardcodedCountryLinkCheck{},
		&EmptyButtonCheck{},
		&ButtonActionCheck{},
		&FormSubmitCheck{},
		&ConsoleErrorCheck{},
		&FailedRequestCheck{},
		&LargeBundleCheck{},
		&RedirectCheck{},
		&SEOBasicsCheck{},
		&NoindexCheck{},
		&OGTagsCheck{},
		&CurrencyCheck{},
		&TemplateErrorCheck{},
		&ZeroPriceCheck{},
		&InsecureFormCheck{},
		&WrongLanguageCheck{},
		&TabnabbingCheck{},
		&SRICheck{},
		&StructuredDataCheck{},
		&HeadingHierarchyCheck{},
		&ImageDimensionsCheck{},
		&ViewportZoomCheck{},
	}
	e.siteChecks = []SiteCheck{
		&BrokenLinkCheck{},
		&DuplicateTitleCheck{},
		&DuplicateMetaCheck{},
		&HreflangCheck{},
		&FaviconCheck{},
		&MobileBasicsCheck{},
		&SecurityHeadersCheck{},
		&SensitiveFileCheck{},
		&HTTPSRedirectCheck{},
		&RobotsSitemapCheck{},
		&CookieSecurityCheck{},
	}
}

// Run probes links then executes all checks, returning the merged issues.
// A panicking check is logged and skipped; it never aborts the audit.
func (e *Engine) Run(ctx context.Context, sc *SiteContext) []models.Issue {
	if sc.Cfg == (Config{}) {
		sc.Cfg = e.cfg
	}
	sc.Links = e.prober.ProbeSite(ctx, sc)
	e.prober.sizeAssets(ctx, sc)
	if sc.Recon == nil {
		sc.Recon = e.prober.Recon(ctx, sc.Website)
	}

	var issues []models.Issue
	for _, p := range sc.Pages {
		for _, chk := range e.pageChecks {
			out := e.safeRunPage(chk, sc, p)
			for i := range out {
				out[i].CheckID = chk.Name()
			}
			issues = append(issues, out...)
		}
	}
	for _, chk := range e.siteChecks {
		out := e.safeRunSite(chk, sc)
		for i := range out {
			out[i].CheckID = chk.Name()
		}
		issues = append(issues, out...)
	}

	now := time.Now()
	for i := range issues {
		issues[i].Website = sc.Website
		issues[i].Source = models.SourceRule
		if issues[i].Confidence == 0 {
			issues[i].Confidence = 1
		}
		issues[i].CreatedAt = now
	}
	return issues
}

func (e *Engine) safeRunPage(chk PageCheck, sc *SiteContext, p *models.Page) (out []models.Issue) {
	defer func() {
		if r := recover(); r != nil {
			e.log.Error("check panicked", "check", chk.Name(), "page", p.URL, "panic", fmt.Sprint(r))
			out = nil
		}
	}()
	// Skip content checks on pages that failed to fetch or aren't HTML;
	// StatusCodeCheck still needs to see error pages.
	if _, always := chk.(*StatusCodeCheck); !always && (!p.OK() || !p.IsHTML()) {
		return nil
	}
	return chk.CheckPage(sc, p)
}

func (e *Engine) safeRunSite(chk SiteCheck, sc *SiteContext) (out []models.Issue) {
	defer func() {
		if r := recover(); r != nil {
			e.log.Error("site check panicked", "check", chk.Name(), "panic", fmt.Sprint(r))
			out = nil
		}
	}()
	return chk.CheckSite(sc)
}

// issue is a small helper for constructing issues inside checks.
func issue(p *models.Page, cat models.Category, sev models.Severity,
	title, desc, fix string) models.Issue {
	pageURL := ""
	if p != nil {
		pageURL = p.URL
	}
	return models.Issue{
		PageURL:      pageURL,
		Category:     cat,
		Severity:     sev,
		Title:        title,
		Description:  desc,
		SuggestedFix: fix,
	}
}
