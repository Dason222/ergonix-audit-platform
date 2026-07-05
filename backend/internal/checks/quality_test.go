package checks

import (
	"strings"
	"testing"

	"github.com/ergonix/auditor/backend/internal/models"
)

func TestStructuredDataCheck(t *testing.T) {
	p := goodPage()
	p.StructuredTypes = []string{"BreadcrumbList"} // no Product
	got := runPage(t, &StructuredDataCheck{}, p)
	if len(got) != 1 || got[0].Category != models.CategorySEO {
		t.Fatalf("missing product schema: %+v", got)
	}

	// Non-product page is never judged.
	p = goodPage()
	p.URL = "https://ergonix.lt/pages/about"
	p.StructuredTypes = nil
	if got := runPage(t, &StructuredDataCheck{}, p); len(got) != 0 {
		t.Errorf("non-product page flagged: %+v", got)
	}
}

func TestHeadingHierarchyCheck(t *testing.T) {
	p := goodPage()
	p.HeadingLevels = []int{1, 3} // skips h2
	got := runPage(t, &HeadingHierarchyCheck{}, p)
	if len(got) != 1 || !strings.Contains(got[0].Description, "h1 to h3") {
		t.Fatalf("skipped level: %+v", got)
	}

	p.HeadingLevels = []int{1, 2, 3, 2, 3} // going back down is fine
	if got := runPage(t, &HeadingHierarchyCheck{}, p); len(got) != 0 {
		t.Errorf("valid hierarchy flagged: %+v", got)
	}
}

func TestImageDimensionsCheck(t *testing.T) {
	p := goodPage()
	p.Images = nil
	for i := 0; i < 6; i++ {
		p.Images = append(p.Images, models.Image{Src: "x.jpg", HasDimensions: false})
	}
	got := runPage(t, &ImageDimensionsCheck{}, p)
	if len(got) != 1 || got[0].Category != models.CategoryPerformance {
		t.Fatalf("missing dimensions: %+v", got)
	}

	// Few images, or mostly dimensioned → silent.
	p.Images = []models.Image{{Src: "a.jpg"}, {Src: "b.jpg"}}
	if got := runPage(t, &ImageDimensionsCheck{}, p); len(got) != 0 {
		t.Errorf("small page flagged: %+v", got)
	}
}

func TestViewportZoomCheck(t *testing.T) {
	p := goodPage()
	p.ViewportContent = "width=device-width, initial-scale=1, user-scalable=no"
	got := runPage(t, &ViewportZoomCheck{}, p)
	if len(got) != 1 || got[0].Category != models.CategoryAccessibility {
		t.Fatalf("zoom disabled: %+v", got)
	}

	p.ViewportContent = "width=device-width, initial-scale=1"
	if got := runPage(t, &ViewportZoomCheck{}, p); len(got) != 0 {
		t.Errorf("zoomable viewport flagged: %+v", got)
	}
}
