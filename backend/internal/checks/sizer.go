package checks

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// sizeAssets fills SizeBytes for image/script/stylesheet assets via HEAD
// Content-Length probes, so the large-image and large-bundle checks have
// real numbers to judge. Bounded and paced like the link prober.
func (lp *LinkProber) sizeAssets(ctx context.Context, sc *SiteContext) {
	const (
		maxImages  = 40
		maxBundles = 20
	)
	want := map[string]bool{}
	var urls []string
	add := func(u string, cap *int) {
		if u == "" || want[u] || *cap <= 0 {
			return
		}
		want[u] = true
		*cap--
		urls = append(urls, u)
	}
	imgCap, bundleCap := maxImages, maxBundles
	for _, p := range sc.Pages {
		for _, img := range p.Images {
			add(img.Src, &imgCap)
		}
		for _, s := range p.Scripts {
			if !s.Inline {
				add(s.Src, &bundleCap)
			}
		}
		for _, s := range p.Stylesheets {
			if !s.Inline {
				add(s.Src, &bundleCap)
			}
		}
	}
	if len(urls) == 0 {
		return
	}

	client := &http.Client{Timeout: lp.cfg.LinkProbeTimeout}
	sizes := map[string]int64{}
	var (
		mu          sync.Mutex
		wg          sync.WaitGroup
		paceMu      sync.Mutex
		nextAllowed time.Time
	)
	pace := func() bool {
		paceMu.Lock()
		wait := time.Until(nextAllowed)
		if wait < 0 {
			wait = 0
		}
		nextAllowed = time.Now().Add(wait + probeDelay)
		paceMu.Unlock()
		select {
		case <-time.After(wait):
			return true
		case <-ctx.Done():
			return false
		}
	}

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
			if !pace() {
				return
			}
			req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", lp.cfg.UserAgent)
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()
			if resp.StatusCode >= 400 {
				return
			}
			if n, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil && n > 0 {
				mu.Lock()
				sizes[u] = n
				mu.Unlock()
			}
		}(u)
	}
	wg.Wait()

	// Write measured sizes back into every referencing page.
	for _, p := range sc.Pages {
		for i := range p.Images {
			if n, ok := sizes[p.Images[i].Src]; ok {
				p.Images[i].SizeBytes = n
			}
		}
		for i := range p.Scripts {
			if n, ok := sizes[p.Scripts[i].Src]; ok {
				p.Scripts[i].SizeBytes = n
			}
		}
		for i := range p.Stylesheets {
			if n, ok := sizes[p.Stylesheets[i].Src]; ok {
				p.Stylesheets[i].SizeBytes = n
			}
		}
	}
}
