package ai

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ergonix/auditor/backend/internal/models"
)

func TestParseFindingsHappyPath(t *testing.T) {
	raw := `{"issues":[
		{"type":"wrong_language","severity":"high","title":"English text on Lithuanian store",
		 "description":"Button says 'Add to cart'","suggestedFix":"Translate to 'Į krepšelį'","confidence":0.9},
		{"type":"suspicious_pricing","severity":"critical","title":"Price is 0.00 EUR",
		 "description":"Product shows 0,00 €","suggestedFix":"Fix the price feed","confidence":0.95}
	]}`
	issues, err := ParseFindings(raw, "https://ergonix.lt", "https://ergonix.lt/p")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 {
		t.Fatalf("issues = %d", len(issues))
	}
	if issues[0].Category != models.CategoryTranslation || issues[0].Source != models.SourceAI {
		t.Errorf("issue0: %+v", issues[0])
	}
	if issues[1].Category != models.CategoryLogic || issues[1].Severity != models.SeverityCritical {
		t.Errorf("issue1: %+v", issues[1])
	}
}

func TestParseFindingsToleratesFencesAndJunk(t *testing.T) {
	raw := "```json\n{\"issues\":[{\"type\":\"grammar\",\"severity\":\"low\",\"title\":\"Typo\",\"description\":\"x\",\"suggestedFix\":\"y\",\"confidence\":0.7}]}\n```"
	issues, err := ParseFindings(raw, "w", "u")
	if err != nil || len(issues) != 1 {
		t.Fatalf("fenced: err=%v n=%d", err, len(issues))
	}

	raw = `Here is my analysis: {"issues":[]} hope it helps`
	issues, err = ParseFindings(raw, "w", "u")
	if err != nil || len(issues) != 0 {
		t.Fatalf("wrapped: err=%v n=%d", err, len(issues))
	}
}

func TestParseFindingsDropsInvalidEntries(t *testing.T) {
	raw := `{"issues":[
		{"type":"nonsense_type","severity":"high","title":"Should be dropped"},
		{"type":"grammar","severity":"weird","title":"Bad severity becomes low","confidence":7}
	]}`
	issues, err := ParseFindings(raw, "w", "u")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("issues = %d, want 1", len(issues))
	}
	if issues[0].Severity != models.SeverityLow || issues[0].Confidence != 0.5 {
		t.Errorf("sanitized wrong: %+v", issues[0])
	}
}

func TestParseFindingsRejectsGarbage(t *testing.T) {
	if _, err := ParseFindings("total nonsense, no json at all", "w", "u"); err == nil {
		t.Error("expected error for garbage input")
	}
}

func TestSummarizeBudgetsAndCaps(t *testing.T) {
	p := &models.Page{
		URL:         "https://ergonix.lt/p",
		Language:    "lt",
		Title:       "T",
		VisibleText: strings.Repeat("ą", 3000), // 2-byte runes
	}
	for i := 0; i < 50; i++ {
		p.Buttons = append(p.Buttons, models.Button{Text: "Pirkti"})
		p.Links = append(p.Links, models.Link{Internal: true, Text: "Nuoroda"})
	}
	s := Summarize(p, 1001)
	if len(s.VisibleText) > 1001 {
		t.Errorf("text not truncated: %d", len(s.VisibleText))
	}
	if !strings.HasSuffix(s.VisibleText, "ą") {
		t.Error("truncation split a UTF-8 rune")
	}
	if len(s.Buttons) != 1 { // deduped
		t.Errorf("buttons = %d", len(s.Buttons))
	}
}

// fakeClient returns queued responses or errors in order.
type fakeClient struct {
	responses []string
	errs      []error
	calls     int
}

func (f *fakeClient) ChatJSON(ctx context.Context, system, user string) (string, error) {
	i := f.calls
	f.calls++
	var err error
	if i < len(f.errs) {
		err = f.errs[i]
	}
	resp := `{"issues":[]}`
	if i < len(f.responses) && f.responses[i] != "" {
		resp = f.responses[i]
	}
	return resp, err
}

func testPages() []*models.Page {
	mk := func(url, text string, prices int, depth int) *models.Page {
		p := &models.Page{URL: url, FinalURL: url, StatusCode: 200, VisibleText: text, Depth: depth}
		for i := 0; i < prices; i++ {
			p.Prices = append(p.Prices, "9,99 €")
		}
		return p
	}
	return []*models.Page{
		mk("https://x.lt/", "home page text", 0, 0),
		mk("https://x.lt/product", strings.Repeat("rich product text ", 100), 3, 1),
		mk("https://x.lt/thin", "x", 0, 2),
	}
}

func TestAnalyzeSiteCapsAndPrioritizes(t *testing.T) {
	fc := &fakeClient{responses: []string{
		`{"issues":[{"type":"grammar","severity":"low","title":"Typo","confidence":0.8}]}`,
		`{"issues":[]}`,
	}}
	a := New(fc, Config{MaxPagesPerSite: 2}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	var progress [][2]int
	issues, err := a.AnalyzeSite(context.Background(), "https://ergonix.lt", testPages(),
		func(done, total int) { progress = append(progress, [2]int{done, total}) })
	if err != nil {
		t.Fatal(err)
	}
	if fc.calls != 2 {
		t.Errorf("calls = %d, want 2 (capped)", fc.calls)
	}
	if len(issues) != 1 || issues[0].Source != models.SourceAI {
		t.Errorf("issues: %+v", issues)
	}
	if len(progress) != 2 || progress[1] != [2]int{2, 2} {
		t.Errorf("progress: %v", progress)
	}
}

func TestAnalyzeSiteToleratesPartialFailure(t *testing.T) {
	fc := &fakeClient{
		errs: []error{errors.New("boom"), nil},
		responses: []string{"",
			`{"issues":[{"type":"trust","severity":"medium","title":"No contact info","confidence":0.6}]}`},
	}
	a := New(fc, Config{MaxPagesPerSite: 2}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	issues, err := a.AnalyzeSite(context.Background(), "https://ergonix.lt", testPages(), nil)
	if err != nil {
		t.Fatalf("partial failure must not error: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("issues = %d, want 1", len(issues))
	}
}

func TestAnalyzeSiteAllFailedReturnsError(t *testing.T) {
	fc := &fakeClient{errs: []error{errors.New("a"), errors.New("b"), errors.New("c")}}
	a := New(fc, Config{MaxPagesPerSite: 3}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_, err := a.AnalyzeSite(context.Background(), "https://ergonix.lt", testPages(), nil)
	if err == nil {
		t.Error("expected error when every page fails")
	}
}
