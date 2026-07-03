package checks

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
)

// LinkResult is the outcome of probing one URL.
type LinkResult struct {
	StatusCode int
	Err        string
}

// Broken reports whether the probe indicates a dead link.
func (r LinkResult) Broken() bool {
	return r.Err != "" || r.StatusCode >= 400
}

// LinkStatusMap maps absolute URL -> probe result.
type LinkStatusMap map[string]LinkResult

// LinkProber resolves the status of links referenced by crawled pages.
// Internal links are answered from the crawl itself where possible; the
// remainder (and external links, up to a cap) are probed over HTTP.
type LinkProber struct {
	cfg Config
	log *slog.Logger
}

// NewLinkProber builds a prober with the given thresholds.
func NewLinkProber(cfg Config, log *slog.Logger) *LinkProber {
	return &LinkProber{cfg: cfg.withDefaults(), log: log}
}

// ProbeSite returns the status of every unique link on the site's pages.
func (lp *LinkProber) ProbeSite(ctx context.Context, sc *SiteContext) LinkStatusMap {
	results := LinkStatusMap{}

	// Crawled pages already know their status — free answers, and they
	// cover most internal links.
	for _, p := range sc.Pages {
		if p.FetchError != "" {
			results[p.URL] = LinkResult{Err: p.FetchError}
		} else {
			results[p.URL] = LinkResult{StatusCode: p.StatusCode}
			if p.FinalURL != "" && p.FinalURL != p.URL {
				results[p.FinalURL] = LinkResult{StatusCode: p.StatusCode}
			}
		}
	}

	var internal, external []string
	seen := map[string]bool{}
	for _, p := range sc.Pages {
		for _, l := range p.Links {
			if l.Href == "" || seen[l.Href] {
				continue
			}
			seen[l.Href] = true
			if _, known := results[l.Href]; known {
				continue
			}
			if l.Internal {
				internal = append(internal, l.Href)
			} else {
				external = append(external, l.Href)
			}
		}
	}
	if len(external) > lp.cfg.MaxExternalProbes {
		lp.log.Info("capping external link probes",
			"total", len(external), "cap", lp.cfg.MaxExternalProbes)
		external = external[:lp.cfg.MaxExternalProbes]
	}

	lp.probe(ctx, append(internal, external...), results)
	return results
}

// probe fetches URLs concurrently with HEAD, falling back to GET when the
// server rejects HEAD (405/501) or errors.
func (lp *LinkProber) probe(ctx context.Context, urls []string, results LinkStatusMap) {
	if len(urls) == 0 {
		return
	}
	client := &http.Client{Timeout: lp.cfg.LinkProbeTimeout}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	sem := make(chan struct{}, lp.cfg.LinkProbeConcurrency)

	for _, u := range urls {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }()
			res := lp.probeOne(ctx, client, u)
			mu.Lock()
			results[u] = res
			mu.Unlock()
		}(u)
	}
	wg.Wait()
}

func (lp *LinkProber) probeOne(ctx context.Context, client *http.Client, u string) LinkResult {
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		req, err := http.NewRequestWithContext(ctx, method, u, nil)
		if err != nil {
			return LinkResult{Err: err.Error()}
		}
		req.Header.Set("User-Agent", lp.cfg.UserAgent)
		resp, err := client.Do(req)
		if err != nil {
			if method == http.MethodHead {
				continue // some servers break on HEAD; retry with GET
			}
			return LinkResult{Err: err.Error()}
		}
		resp.Body.Close()
		if method == http.MethodHead && (resp.StatusCode == http.StatusMethodNotAllowed ||
			resp.StatusCode == http.StatusNotImplemented || resp.StatusCode == http.StatusForbidden) {
			continue
		}
		return LinkResult{StatusCode: resp.StatusCode}
	}
	return LinkResult{Err: "unreachable"}
}
