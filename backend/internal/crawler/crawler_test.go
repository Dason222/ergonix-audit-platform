package crawler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ergonix/auditor/backend/internal/models"
)

// newTestSite serves a small site: / -> /a, /b ; /a -> /b, /missing ; /b -> /
// plus /missing (404) and /loop which redirects to itself.
func newTestSite(t *testing.T) (*httptest.Server, *int64) {
	t.Helper()
	var hits int64
	mux := http.NewServeMux()
	page := func(title, links string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt64(&hits, 1)
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `<html lang="lt"><head><title>%s</title></head><body>%s</body></html>`, title, links)
		}
	}
	mux.HandleFunc("/{$}", page("Home", `<a href="/a">A</a> <a href="/b">B</a> <a href="/a">A dup</a>`))
	mux.HandleFunc("/a", page("A", `<a href="/b">B</a> <a href="/missing">Broken</a>`))
	mux.HandleFunc("/b", page("B", `<a href="/">Home</a> <a href="/c">C</a>`))
	mux.HandleFunc("/c", page("C", ``))
	mux.HandleFunc("/missing", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&hits, 1)
		http.NotFound(w, r)
	})
	mux.HandleFunc("/loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop", http.StatusFound)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &hits
}

func testParams() models.AuditParams {
	return models.AuditParams{
		MaxPages: 10, MaxDepth: 3, Concurrency: 2,
		RequestTimeoutSec: 5, RetryCount: 0,
	}
}

func newTestCrawler() *Crawler {
	return New(Config{
		UserAgent:     "ErgonixAuditBot/1.0-test",
		CrawlDelay:    0,
		RespectRobots: false,
	}, nil, slog.Default())
}

func TestCrawlBFSDedupAnd404(t *testing.T) {
	srv, _ := newTestSite(t)
	c := newTestCrawler()

	var progress int32
	pages, err := c.Crawl(context.Background(), srv.URL, testParams(), func(done int) {
		atomic.StoreInt32(&progress, int32(done))
	})
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}

	// Expect exactly /, /a, /b, /missing, /c — each once.
	if len(pages) != 5 {
		urls := make([]string, len(pages))
		for i, p := range pages {
			urls[i] = fmt.Sprintf("%s(%d)", p.URL, p.StatusCode)
		}
		t.Fatalf("pages = %d, want 5: %v", len(pages), urls)
	}
	byURL := map[string]*models.Page{}
	for _, p := range pages {
		if byURL[p.URL] != nil {
			t.Errorf("duplicate crawl of %s", p.URL)
		}
		byURL[p.URL] = p
	}
	if p := byURL[srv.URL+"/missing"]; p == nil || p.StatusCode != 404 {
		t.Errorf("missing page not recorded as 404: %+v", p)
	}
	if atomic.LoadInt32(&progress) != 5 {
		t.Errorf("progress callback final = %d, want 5", progress)
	}
}

func TestCrawlRespectsMaxPages(t *testing.T) {
	srv, _ := newTestSite(t)
	c := newTestCrawler()
	p := testParams()
	p.MaxPages = 2

	pages, err := c.Crawl(context.Background(), srv.URL, p, nil)
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}
	if len(pages) != 2 {
		t.Errorf("pages = %d, want 2", len(pages))
	}
}

func TestCrawlRespectsMaxDepth(t *testing.T) {
	srv, _ := newTestSite(t)
	c := newTestCrawler()
	p := testParams()
	p.MaxDepth = 1 // home (0) + direct children (1); /missing and /c are at depth 2

	pages, err := c.Crawl(context.Background(), srv.URL, p, nil)
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}
	if len(pages) != 3 { // /, /a, /b
		t.Errorf("pages = %d, want 3", len(pages))
	}
}

func TestCrawlRedirectLoop(t *testing.T) {
	srv, _ := newTestSite(t)
	c := newTestCrawler()
	p := testParams()

	// Crawl the loop URL directly as the "website".
	pages, err := c.Crawl(context.Background(), srv.URL+"/loop", p, nil)
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("pages = %d", len(pages))
	}
	if pages[0].FetchError == "" {
		t.Errorf("redirect loop should record a fetch error, got %+v", pages[0])
	}
}

func TestCrawlHonorsCancellation(t *testing.T) {
	srv, _ := newTestSite(t)
	c := New(Config{UserAgent: "t", CrawlDelay: 50 * time.Millisecond}, nil, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before start
	pages, _ := c.Crawl(ctx, srv.URL, testParams(), nil)
	if len(pages) > 1 {
		t.Errorf("cancelled crawl should stop early, got %d pages", len(pages))
	}
}

func TestCrawlRespectsRobots(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "User-agent: *\nDisallow: /a\n")
	})
	mux.HandleFunc("/{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>H</title></head><body><a href="/a">A</a><a href="/b">B</a></body></html>`)
	})
	mux.HandleFunc("/a", func(w http.ResponseWriter, r *http.Request) {
		t.Error("crawler fetched robots-disallowed /a")
	})
	mux.HandleFunc("/b", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>B</title></head><body></body></html>`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(Config{UserAgent: "ErgonixAuditBot/1.0", RespectRobots: true}, nil, slog.Default())
	pages, err := c.Crawl(context.Background(), srv.URL, testParams(), nil)
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}
	if len(pages) != 2 { // / and /b only
		t.Errorf("pages = %d, want 2", len(pages))
	}
}
