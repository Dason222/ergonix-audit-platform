// Package crawler implements a polite, concurrent BFS crawler that reads
// real website pages and extracts the structured data every other pipeline
// stage consumes.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/ergonix/auditor/backend/internal/browser"
	"github.com/ergonix/auditor/backend/internal/models"
)

// Config are the crawler-wide settings (per-audit knobs come in AuditParams).
type Config struct {
	UserAgent     string
	CrawlDelay    time.Duration
	RespectRobots bool
	MaxBodyBytes  int64
	// BrowserPagesPerSite caps how many pages per site get the (slow)
	// Playwright enrichment pass.
	BrowserPagesPerSite int
}

// Crawler fetches and extracts pages for one website at a time.
type Crawler struct {
	cfg     Config
	browser browser.Service
	log     *slog.Logger
}

// New builds a Crawler. browser may be a Noop service.
func New(cfg Config, b browser.Service, log *slog.Logger) *Crawler {
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 3 << 20
	}
	if b == nil {
		b = browser.NewNoop()
	}
	return &Crawler{cfg: cfg, browser: b, log: log}
}

type frontierItem struct {
	url   string
	depth int
}

// Crawl walks website breadth-first up to p.MaxPages / p.MaxDepth, calling
// onPage(done) after each fetched page. It returns every visited page,
// including error pages (which downstream checks turn into issues).
func (c *Crawler) Crawl(ctx context.Context, website string, p models.AuditParams,
	onPage func(done int)) ([]*models.Page, error) {

	root, err := url.Parse(website)
	if err != nil || root.Host == "" {
		return nil, fmt.Errorf("invalid website URL %q: %w", website, err)
	}

	client := c.newClient(time.Duration(p.RequestTimeoutSec) * time.Second)

	var robots *robotsRules
	if c.cfg.RespectRobots {
		robots = fetchRobots(ctx, client, root, c.cfg.UserAgent)
	} else {
		robots = &robotsRules{}
	}

	var (
		mu       sync.Mutex
		visited  = map[string]bool{}
		pages    []*models.Page
		queue    []frontierItem
		inFlight int
	)

	start := NormalizeURL(root, root.String())
	if start == "" {
		return nil, fmt.Errorf("cannot normalize start URL %q", website)
	}
	queue = append(queue, frontierItem{url: start, depth: 0})
	visited[start] = true

	// Site-wide request pacing: CrawlDelay is the minimum spacing between
	// ANY two requests to the site, regardless of worker count. Without
	// this, N workers each sleeping independently still hit the shop at
	// N× the intended rate and trip its rate limiter.
	var (
		paceMu      sync.Mutex
		nextAllowed time.Time
	)
	pace := func() error {
		paceMu.Lock()
		wait := time.Until(nextAllowed)
		if wait < 0 {
			wait = 0
		}
		nextAllowed = time.Now().Add(wait + c.cfg.CrawlDelay)
		paceMu.Unlock()
		if wait == 0 {
			return ctx.Err()
		}
		select {
		case <-time.After(wait):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Worker pool over a shared frontier. cond broadcasts when the queue
	// changes or work finishes so idle workers can either pick up new URLs
	// or detect completion (queue empty + nothing in flight).
	cond := sync.NewCond(&mu)
	done := func() bool { return len(pages) >= p.MaxPages || ctx.Err() != nil }

	// Wake all workers when the context is cancelled so they can exit.
	stopWake := context.AfterFunc(ctx, func() { cond.Broadcast() })
	defer stopWake()

	worker := func() {
		for {
			mu.Lock()
			for len(queue) == 0 && inFlight > 0 && !done() {
				cond.Wait()
			}
			if done() || (len(queue) == 0 && inFlight == 0) {
				mu.Unlock()
				return
			}
			item := queue[0]
			queue = queue[1:]
			inFlight++
			mu.Unlock()

			page := c.fetchPage(ctx, client, root, website, item, p, pace)

			mu.Lock()
			inFlight--
			if len(pages) < p.MaxPages {
				pages = append(pages, page)
				count := len(pages)
				if onPage != nil {
					// Call outside the lock? onPage is a cheap progress
					// counter update; holding the lock keeps count exact.
					onPage(count)
				}
				if page.OK() && page.IsHTML() && item.depth < p.MaxDepth {
					for _, link := range page.Links {
						if !link.Internal || link.Nofollow {
							continue
						}
						u := link.Href
						if visited[u] || isBinaryPath(u) {
							continue
						}
						if c.cfg.RespectRobots && !robots.Allowed(u) {
							continue
						}
						visited[u] = true
						queue = append(queue, frontierItem{url: u, depth: item.depth + 1})
					}
				}
			}
			cond.Broadcast()
			mu.Unlock()
		}
	}

	conc := p.Concurrency
	if conc < 1 {
		conc = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < conc; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			worker()
		}()
	}
	wg.Wait()

	if ctx.Err() != nil && len(pages) == 0 {
		return pages, ctx.Err()
	}

	c.enrichWithBrowser(ctx, pages)
	return pages, nil
}

// fetchPage downloads one URL with retries and extracts its content.
// pace enforces the site-wide politeness interval before every attempt.
func (c *Crawler) fetchPage(ctx context.Context, client *http.Client, root *url.URL,
	website string, item frontierItem, p models.AuditParams, pace func() error) *models.Page {

	page := &models.Page{
		Website:   website,
		URL:       item.url,
		FinalURL:  item.url,
		Depth:     item.depth,
		CrawledAt: time.Now(),
	}

	var lastErr error
	for attempt := 0; attempt <= p.RetryCount; attempt++ {
		if pace != nil {
			if err := pace(); err != nil {
				page.FetchError = "cancelled"
				return page
			}
		}
		if attempt > 0 {
			backoff := time.Duration(attempt) * 500 * time.Millisecond
			if page.StatusCode == http.StatusTooManyRequests {
				// Rate limited: back off much harder before retrying.
				backoff = time.Duration(attempt) * 3 * time.Second
			}
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				page.FetchError = "cancelled"
				return page
			}
		}
		if err := c.doFetch(ctx, client, root, page); err != nil {
			lastErr = err
			// Retry on transport errors, 5xx and 429; not on other 4xx.
			if page.StatusCode >= 400 && page.StatusCode < 500 &&
				page.StatusCode != http.StatusTooManyRequests {
				break
			}
			continue
		}
		return page
	}
	if lastErr != nil && page.FetchError == "" {
		page.FetchError = lastErr.Error()
	}
	return page
}

// doFetch performs a single GET, recording redirects, timing, and content.
func (c *Crawler) doFetch(ctx context.Context, client *http.Client, root *url.URL, page *models.Page) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, page.URL, nil)
	if err != nil {
		page.FetchError = err.Error()
		return err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "*")

	start := time.Now()
	resp, err := client.Do(req)
	page.ResponseTimeMs = time.Since(start).Milliseconds()
	if err != nil {
		if errors.Is(err, errTooManyRedirects) || strings.Contains(err.Error(), errTooManyRedirects.Error()) {
			page.FetchError = "redirect loop or too many redirects"
			page.RedirectChain = redirectChainFromError(req)
			return nil // recorded as a page-level finding, not retried
		}
		page.FetchError = err.Error()
		return err
	}
	defer resp.Body.Close()

	page.StatusCode = resp.StatusCode
	page.ContentType = resp.Header.Get("Content-Type")
	page.FinalURL = resp.Request.URL.String()
	page.RedirectChain = redirectChain(resp)

	body, err := io.ReadAll(io.LimitReader(resp.Body, c.cfg.MaxBodyBytes))
	if err != nil {
		page.FetchError = "read body: " + err.Error()
		return err
	}
	page.ContentLength = int64(len(body))
	// LoadTimeMs approximates full download time HTTP-only; the browser
	// pass overwrites it with real load-event timing when enabled.
	page.LoadTimeMs = time.Since(start).Milliseconds()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("rate limited (429)")
	}
	if !page.IsHTML() {
		return nil
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		page.FetchError = "parse html: " + err.Error()
		return nil
	}
	finalURL, err := url.Parse(page.FinalURL)
	if err != nil {
		finalURL = root
	}
	ExtractPage(page, doc, finalURL)
	return nil
}

var errTooManyRedirects = errors.New("stopped after 10 redirects")

// newClient builds an HTTP client that records redirect hops via the request
// chain and refuses to follow more than 10.
func (c *Crawler) newClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errTooManyRedirects
			}
			return nil
		},
	}
}

// redirectChain reconstructs the URL hops that led to the final response.
func redirectChain(resp *http.Response) []string {
	var chain []string
	for r := resp.Request; r != nil; {
		chain = append([]string{r.URL.String()}, chain...)
		if r.Response == nil {
			break
		}
		r = r.Response.Request
	}
	if len(chain) <= 1 {
		return nil
	}
	return chain
}

func redirectChainFromError(req *http.Request) []string {
	return []string{req.URL.String()}
}

// enrichWithBrowser runs the Playwright pass over the first N successfully
// fetched HTML pages. Failures are logged and ignored.
func (c *Crawler) enrichWithBrowser(ctx context.Context, pages []*models.Page) {
	if _, isNoop := c.browser.(browser.Noop); isNoop || c.cfg.BrowserPagesPerSite <= 0 {
		return
	}
	enriched := 0
	for _, p := range pages {
		if enriched >= c.cfg.BrowserPagesPerSite || ctx.Err() != nil {
			return
		}
		if !p.OK() || !p.IsHTML() {
			continue
		}
		if err := c.browser.Enrich(ctx, p); err != nil {
			c.log.Warn("browser enrichment failed", "url", p.FinalURL, "err", err)
		}
		enriched++
	}
}
