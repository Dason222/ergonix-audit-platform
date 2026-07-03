// Package config loads all runtime configuration from environment variables
// (optionally seeded from a .env file), with defaults that let the server run
// with zero configuration.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the full runtime configuration for the backend.
type Config struct {
	// Server
	Port        string
	LogLevel    string
	CORSOrigins []string
	DBPath      string

	// Website registry offered to the UI.
	Websites []string

	// Crawler
	UserAgent         string
	CrawlDelay        time.Duration
	RespectRobots     bool
	MaxBodyBytes      int64
	DefaultParams     DefaultParams
	SiteConcurrency   int // how many websites are audited in parallel

	// Browser (Playwright)
	BrowserEnabled      bool
	BrowserPagesPerSite int

	// AI
	AIAPIKey       string
	AIBaseURL      string
	AIModel        string
	AIMaxPages     int
	AITimeout      time.Duration
	AIMaxTextChars int

	// Check thresholds
	Checks CheckThresholds
}

// DefaultParams are the audit parameter defaults surfaced to the UI.
type DefaultParams struct {
	MaxPages          int
	MaxDepth          int
	Concurrency       int
	RequestTimeoutSec int
	RetryCount        int
}

// CheckThresholds hold the tunable limits used by rule-based checks.
type CheckThresholds struct {
	LargeImageKB         int64
	SlowResponseMs       int64
	SlowLoadMs           int64
	LargeJSKB            int64
	LargeCSSKB           int64
	MaxRedirects         int
	LinkProbeConcurrency int
	LinkProbeTimeout     time.Duration
	MaxExternalProbes    int
}

// Load reads configuration from the environment. If a .env file exists in the
// working directory it is loaded first (existing env vars win).
func Load() *Config {
	_ = godotenv.Load() // absence of .env is fine

	return &Config{
		Port:        env("PORT", "8080"),
		LogLevel:    env("LOG_LEVEL", "info"),
		CORSOrigins: splitCSV(env("CORS_ORIGINS", "http://localhost:5173,http://localhost:3000")),
		DBPath:      env("DB_PATH", "./data/audit.db"),

		Websites: splitCSV(env("WEBSITES",
			"https://ergonix.lt,https://ergonix.lv,https://ergonix.ee,https://ergonix.cz,https://ergonix.pl")),

		UserAgent:       env("CRAWLER_USER_AGENT", "ErgonixAuditBot/1.0 (+internal site audit)"),
		CrawlDelay:      time.Duration(envInt("CRAWL_DELAY_MS", 250)) * time.Millisecond,
		RespectRobots:   envBool("RESPECT_ROBOTS", true),
		MaxBodyBytes:    int64(envInt("MAX_BODY_KB", 3072)) * 1024,
		SiteConcurrency: envInt("SITE_CONCURRENCY", 2),
		DefaultParams: DefaultParams{
			MaxPages:          envInt("DEFAULT_MAX_PAGES", 25),
			MaxDepth:          envInt("DEFAULT_MAX_DEPTH", 3),
			Concurrency:       envInt("DEFAULT_CONCURRENCY", 4),
			RequestTimeoutSec: envInt("DEFAULT_REQUEST_TIMEOUT_SEC", 15),
			RetryCount:        envInt("DEFAULT_RETRY_COUNT", 2),
		},

		BrowserEnabled:      envBool("BROWSER_ENABLED", false),
		BrowserPagesPerSite: envInt("BROWSER_PAGES_PER_SITE", 5),

		AIAPIKey:       env("OPENAI_API_KEY", ""),
		AIBaseURL:      strings.TrimRight(env("OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
		AIModel:        env("OPENAI_MODEL", "gpt-4o-mini"),
		AIMaxPages:     envInt("AI_MAX_PAGES_PER_SITE", 8),
		AITimeout:      time.Duration(envInt("AI_TIMEOUT_SEC", 90)) * time.Second,
		AIMaxTextChars: envInt("AI_MAX_TEXT_CHARS", 4000),

		Checks: CheckThresholds{
			LargeImageKB:         int64(envInt("CHECK_LARGE_IMAGE_KB", 300)),
			SlowResponseMs:       int64(envInt("CHECK_SLOW_RESPONSE_MS", 1500)),
			SlowLoadMs:           int64(envInt("CHECK_SLOW_LOAD_MS", 4000)),
			LargeJSKB:            int64(envInt("CHECK_LARGE_JS_KB", 500)),
			LargeCSSKB:           int64(envInt("CHECK_LARGE_CSS_KB", 150)),
			MaxRedirects:         envInt("CHECK_MAX_REDIRECTS", 5),
			LinkProbeConcurrency: envInt("LINK_PROBE_CONCURRENCY", 4),
			LinkProbeTimeout:     time.Duration(envInt("LINK_PROBE_TIMEOUT_SEC", 10)) * time.Second,
			MaxExternalProbes:    envInt("MAX_EXTERNAL_PROBES", 100),
		},
	}
}

// AIEnabled reports whether an AI backend is configured.
func (c *Config) AIEnabled() bool { return c.AIAPIKey != "" }

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBool(key string, def bool) bool {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

func splitCSV(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}
