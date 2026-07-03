package checks

import (
	"fmt"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// SlowResponseCheck flags pages whose HTTP response (TTFB-ish) is slow.
type SlowResponseCheck struct{}

func (SlowResponseCheck) Name() string { return "slow-response" }

func (SlowResponseCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	limit := sc.Cfg.SlowResponseMs
	if p.ResponseTimeMs <= limit {
		return nil
	}
	sev := models.SeverityMedium
	if p.ResponseTimeMs > limit*3 {
		sev = models.SeverityHigh
	}
	return []models.Issue{issue(p, models.CategoryPerformance, sev,
		"Slow server response",
		fmt.Sprintf("The server took %d ms to respond (threshold %d ms).", p.ResponseTimeMs, limit),
		"Investigate backend/app performance for this route; add caching or a CDN where possible.")}
}

// SlowLoadCheck flags pages whose full load (browser load event when
// available, otherwise full download time) is slow.
type SlowLoadCheck struct{}

func (SlowLoadCheck) Name() string { return "slow-load" }

func (SlowLoadCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	limit := sc.Cfg.SlowLoadMs
	if p.LoadTimeMs <= limit {
		return nil
	}
	return []models.Issue{issue(p, models.CategoryPerformance, models.SeverityMedium,
		"Slow page load",
		fmt.Sprintf("The page needed %d ms to finish loading (threshold %d ms).", p.LoadTimeMs, limit),
		"Reduce render-blocking resources, compress images, and defer non-critical scripts.")}
}

// LargeImageCheck flags images whose transfer size exceeds the threshold.
// Sizes come from the browser pass or Content-Length probes; images without
// a known size are skipped.
type LargeImageCheck struct{}

func (LargeImageCheck) Name() string { return "large-image" }

func (LargeImageCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	limit := sc.Cfg.LargeImageKB * 1024
	var issues []models.Issue
	for _, img := range p.Images {
		if img.SizeBytes > limit {
			issues = append(issues, issue(p, models.CategoryPerformance, models.SeverityMedium,
				"Large image",
				fmt.Sprintf("Image %s is %d KB (threshold %d KB).", img.Src, img.SizeBytes/1024, sc.Cfg.LargeImageKB),
				"Serve a resized/WebP version of this image appropriate for its display size."))
			if len(issues) >= 5 { // avoid drowning the report in one gallery page
				break
			}
		}
	}
	return issues
}

// LargeBundleCheck flags oversized JavaScript and CSS assets.
type LargeBundleCheck struct{}

func (LargeBundleCheck) Name() string { return "large-bundle" }

func (LargeBundleCheck) CheckPage(sc *SiteContext, p *models.Page) []models.Issue {
	var issues []models.Issue
	add := func(kind string, r models.Resource, limitKB int64, cat models.Category) {
		if r.SizeBytes <= limitKB*1024 {
			return
		}
		name := r.Src
		if r.Inline {
			name = "inline " + strings.ToLower(kind)
		}
		issues = append(issues, issue(p, cat, models.SeverityMedium,
			fmt.Sprintf("Large %s bundle", kind),
			fmt.Sprintf("%s weighs %d KB (threshold %d KB), slowing first paint.", name, r.SizeBytes/1024, limitKB),
			fmt.Sprintf("Split, minify or lazy-load this %s asset.", strings.ToLower(kind))))
	}
	for _, s := range p.Scripts {
		add("JavaScript", s, sc.Cfg.LargeJSKB, models.CategoryJavaScript)
	}
	for _, s := range p.Stylesheets {
		add("CSS", s, sc.Cfg.LargeCSSKB, models.CategoryPerformance)
	}
	if len(issues) > 5 {
		issues = issues[:5]
	}
	return issues
}
