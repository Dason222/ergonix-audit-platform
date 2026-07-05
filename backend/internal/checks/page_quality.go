package checks

import (
	"fmt"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// StructuredDataCheck flags product pages that lack Product JSON-LD, which
// powers rich results (price, rating, availability) in search.
type StructuredDataCheck struct{}

func (StructuredDataCheck) Name() string { return "structured-data" }

func (StructuredDataCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	if !strings.Contains(strings.ToLower(p.URL), "/products/") {
		return nil // only judge product pages
	}
	for _, t := range p.StructuredTypes {
		if strings.EqualFold(t, "Product") {
			return nil
		}
	}
	return []models.Issue{issue(p, models.CategorySEO, models.SeverityMedium,
		"Product page without Product structured data",
		fmt.Sprintf("This product page has no Product JSON-LD (found types: %s). Search engines cannot show price, availability or rating rich results.",
			orNone(p.StructuredTypes)),
		"Add schema.org Product JSON-LD with price, availability and rating.")}
}

// HeadingHierarchyCheck flags pages that skip heading levels (e.g. an <h1>
// followed directly by an <h3>), which harms screen-reader navigation and
// document structure.
type HeadingHierarchyCheck struct{}

func (HeadingHierarchyCheck) Name() string { return "heading-hierarchy" }

func (HeadingHierarchyCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	prev := 0
	for _, lvl := range p.HeadingLevels {
		if prev != 0 && lvl > prev+1 {
			return []models.Issue{issue(p, models.CategoryAccessibility, models.SeverityLow,
				"Skipped heading level",
				fmt.Sprintf("The heading outline jumps from h%d to h%d, skipping a level. This confuses screen-reader users navigating by headings.", prev, lvl),
				"Use heading levels sequentially without skipping.")}
		}
		prev = lvl
	}
	return nil
}

// ImageDimensionsCheck flags images missing width/height attributes, which
// cause layout shift (CLS) as they load — a Core Web Vitals penalty.
type ImageDimensionsCheck struct{}

func (ImageDimensionsCheck) Name() string { return "image-dimensions" }

func (ImageDimensionsCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	missing, total := 0, 0
	for _, img := range p.Images {
		if img.Src == "" {
			continue
		}
		total++
		if !img.HasDimensions {
			missing++
		}
	}
	// Only worth reporting when it dominates the page.
	if total < 5 || missing*2 < total {
		return nil
	}
	return []models.Issue{issue(p, models.CategoryPerformance, models.SeverityLow,
		"Images without width/height attributes",
		fmt.Sprintf("%d of %d images lack explicit width/height. The browser cannot reserve space, causing layout shift (poor CLS) as the page loads.",
			missing, total),
		"Set width and height (or aspect-ratio) on images so the browser reserves their space.")}
}

// ViewportZoomCheck flags viewport meta tags that disable pinch-zoom, an
// accessibility barrier for low-vision users.
type ViewportZoomCheck struct{}

func (ViewportZoomCheck) Name() string { return "viewport-zoom" }

func (ViewportZoomCheck) CheckPage(_ *SiteContext, p *models.Page) []models.Issue {
	vc := strings.ToLower(strings.ReplaceAll(p.ViewportContent, " ", ""))
	if !strings.Contains(vc, "user-scalable=no") && !strings.Contains(vc, "maximum-scale=1") {
		return nil
	}
	return []models.Issue{issue(p, models.CategoryAccessibility, models.SeverityLow,
		"Zoom disabled in viewport",
		fmt.Sprintf("The viewport meta (%q) disables pinch-zoom. Low-vision users cannot enlarge the page.", p.ViewportContent),
		"Remove user-scalable=no / maximum-scale=1 from the viewport meta tag.")}
}

func orNone(s []string) string {
	if len(s) == 0 {
		return "none"
	}
	return strings.Join(s, ", ")
}
