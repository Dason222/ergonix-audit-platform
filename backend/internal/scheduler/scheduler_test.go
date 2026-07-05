package scheduler

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/ergonix/auditor/backend/internal/audit"
	"github.com/ergonix/auditor/backend/internal/checks"
	"github.com/ergonix/auditor/backend/internal/crawler"
	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
)

func newTestScheduler(t *testing.T, cfg Config) (*Scheduler, database.Store) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	store, err := database.Open(filepath.Join(t.TempDir(), "sched.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	crawl := crawler.New(crawler.Config{UserAgent: "t"}, nil, log)
	engine := checks.New(checks.Defaults(), log)
	orch := audit.New(store, crawl, engine, nil, audit.Config{}, log)
	return New(store, orch, cfg, log), store
}

// TestNextDelayNoRunaway is the regression guard: after a trigger records
// lastTrigger=now, the next delay must be a full interval — never seconds.
func TestNextDelayNoRunaway(t *testing.T) {
	s, _ := newTestScheduler(t, Config{Enabled: true, Interval: time.Hour, Websites: []string{"https://x"}})

	// Never run yet → soon.
	if d := s.nextDelay(time.Hour); d > 10*time.Second {
		t.Errorf("first run delay = %s, want ~5s", d)
	}

	// Simulate a trigger just happening.
	s.mu.Lock()
	s.lastTrigger = nowUTC()
	s.mu.Unlock()

	// Now the next delay must be ~a full interval, NOT seconds. This is the
	// bug that caused audits to spawn every few seconds.
	d := s.nextDelay(time.Hour)
	if d < 50*time.Minute {
		t.Fatalf("delay after trigger = %s, want ~1h (runaway regression!)", d)
	}
}

func TestSeedLastTriggerFromDB(t *testing.T) {
	s, store := newTestScheduler(t, Config{Enabled: true, Interval: time.Hour})
	created := time.Now().Add(-20 * time.Minute)
	a := &models.Audit{
		Status: models.AuditCompleted, Trigger: "scheduled",
		Params: models.AuditParams{Websites: []string{"https://x"}},
		Stats:  models.NewAuditStats(), CreatedAt: created,
	}
	if err := store.CreateAudit(a); err != nil {
		t.Fatal(err)
	}
	s.seedLastTrigger()

	// A scheduled audit ran 20 min ago on a 1h interval → next run ~40 min.
	d := s.nextDelay(time.Hour)
	if d < 35*time.Minute || d > 45*time.Minute {
		t.Errorf("resumed delay = %s, want ~40m", d)
	}
}

func TestScheduledInFlightGuard(t *testing.T) {
	s, store := newTestScheduler(t, Config{Enabled: true, Interval: time.Hour, Websites: []string{"https://x"}})
	running := &models.Audit{
		Status: models.AuditRunning, Trigger: "scheduled",
		Params: models.AuditParams{Websites: []string{"https://x"}},
		Stats:  models.NewAuditStats(), CreatedAt: time.Now(),
	}
	if err := store.CreateAudit(running); err != nil {
		t.Fatal(err)
	}
	if !s.scheduledInFlight() {
		t.Fatal("running scheduled audit should be detected as in-flight")
	}

	// trigger() must not create a second audit while one is in flight.
	s.trigger(context.Background())
	audits, total, _ := store.ListAudits(50, 0)
	if total != 1 {
		urls := make([]string, len(audits))
		for i, a := range audits {
			urls[i] = string(a.Status)
		}
		t.Errorf("trigger stacked a second audit: total=%d (%v)", total, urls)
	}
}

func TestDisabledNeverTriggers(t *testing.T) {
	s, store := newTestScheduler(t, Config{Enabled: false, Interval: time.Hour, Websites: []string{"https://x"}})
	s.trigger(context.Background())
	if _, total, _ := store.ListAudits(50, 0); total != 0 {
		t.Errorf("disabled scheduler created an audit: total=%d", total)
	}
}
