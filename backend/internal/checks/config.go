package checks

import "time"

// Config holds the tunable thresholds for rule-based checks. Zero values are
// replaced by Defaults() when the engine is constructed.
type Config struct {
	LargeImageKB   int64
	SlowResponseMs int64
	SlowLoadMs     int64
	LargeJSKB      int64
	LargeCSSKB     int64
	MaxRedirects   int

	LinkProbeConcurrency int
	LinkProbeTimeout     time.Duration
	MaxExternalProbes    int
	UserAgent            string
}

// Defaults returns the standard thresholds used when nothing is configured.
func Defaults() Config {
	return Config{
		LargeImageKB:         300,
		SlowResponseMs:       1500,
		SlowLoadMs:           4000,
		LargeJSKB:            500,
		LargeCSSKB:           150,
		MaxRedirects:         5,
		LinkProbeConcurrency: 8,
		LinkProbeTimeout:     10 * time.Second,
		MaxExternalProbes:    100,
		UserAgent:            "ErgonixAuditBot/1.0 (+internal site audit)",
	}
}

// withDefaults fills zero fields from Defaults().
func (c Config) withDefaults() Config {
	d := Defaults()
	if c.LargeImageKB <= 0 {
		c.LargeImageKB = d.LargeImageKB
	}
	if c.SlowResponseMs <= 0 {
		c.SlowResponseMs = d.SlowResponseMs
	}
	if c.SlowLoadMs <= 0 {
		c.SlowLoadMs = d.SlowLoadMs
	}
	if c.LargeJSKB <= 0 {
		c.LargeJSKB = d.LargeJSKB
	}
	if c.LargeCSSKB <= 0 {
		c.LargeCSSKB = d.LargeCSSKB
	}
	if c.MaxRedirects <= 0 {
		c.MaxRedirects = d.MaxRedirects
	}
	if c.LinkProbeConcurrency <= 0 {
		c.LinkProbeConcurrency = d.LinkProbeConcurrency
	}
	if c.LinkProbeTimeout <= 0 {
		c.LinkProbeTimeout = d.LinkProbeTimeout
	}
	if c.MaxExternalProbes <= 0 {
		c.MaxExternalProbes = d.MaxExternalProbes
	}
	if c.UserAgent == "" {
		c.UserAgent = d.UserAgent
	}
	return c
}
