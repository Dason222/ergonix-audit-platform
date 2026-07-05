// Package scheduler runs audits automatically on a recurring interval, so
// the platform detects problems on its own without anyone clicking "Audit".
// Its configuration can be changed at runtime via Reconfigure (from Settings).
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/ergonix/auditor/backend/internal/audit"
	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
)

// Config controls automatic auditing.
type Config struct {
	Enabled  bool
	Interval time.Duration
	AtStart  bool               // run once shortly after boot
	Websites []string           // sites to audit (empty = handled by caller)
	Params   models.AuditParams // crawl params for scheduled runs
}

// Scheduler periodically enqueues audits via the orchestrator. The control
// loop runs for the process lifetime; enabling/disabling is a config change,
// not a start/stop, so settings can toggle it live.
type Scheduler struct {
	store database.Store
	orch  *audit.Orchestrator
	log   *slog.Logger

	mu          sync.Mutex
	cfg         Config
	reconfig    chan struct{}
	lastTrigger time.Time // when a scheduled audit was last started
}

// New builds a Scheduler with an initial config.
func New(store database.Store, orch *audit.Orchestrator, cfg Config, log *slog.Logger) *Scheduler {
	return &Scheduler{store: store, orch: orch, cfg: cfg, log: log, reconfig: make(chan struct{}, 1)}
}

// Reconfigure replaces the schedule configuration and wakes the loop so the
// change takes effect immediately.
func (s *Scheduler) Reconfigure(cfg Config) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
	s.log.Info("scheduler reconfigured",
		"enabled", cfg.Enabled, "interval", cfg.Interval.String(), "websites", len(cfg.Websites))
	select {
	case s.reconfig <- struct{}{}:
	default:
	}
}

func (s *Scheduler) snapshot() Config {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg
}

// Start launches the always-on control loop.
func (s *Scheduler) Start(ctx context.Context) {
	s.seedLastTrigger()
	go s.loop(ctx)
}

// seedLastTrigger initializes lastTrigger from the most recent scheduled
// audit's creation time, so a restart resumes the cadence instead of
// treating the schedule as never-run.
func (s *Scheduler) seedLastTrigger() {
	audits, _, err := s.store.ListAudits(50, 0)
	if err != nil {
		return
	}
	for _, a := range audits { // newest first
		if a.Trigger == "scheduled" {
			s.mu.Lock()
			s.lastTrigger = a.CreatedAt
			s.mu.Unlock()
			return
		}
	}
}

func (s *Scheduler) loop(ctx context.Context) {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	s.arm(timer, true)

	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			s.log.Info("scheduler stopped")
			return
		case <-s.reconfig:
			s.arm(timer, false)
		case <-timer.C:
			s.trigger(ctx)
			s.arm(timer, false)
		}
	}
}

// arm (re)schedules the timer based on the current config. A disabled or
// site-less config leaves the timer stopped (loop only wakes on reconfig).
func (s *Scheduler) arm(timer *time.Timer, initial bool) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	cfg := s.snapshot()
	if !cfg.Enabled || cfg.Interval <= 0 || len(cfg.Websites) == 0 {
		return
	}
	if initial && cfg.AtStart {
		timer.Reset(5 * time.Second) // explicit "run once on boot"
		return
	}
	timer.Reset(s.nextDelay(cfg.Interval))
}

// nextDelay computes how long until the next scheduled run, based on when a
// scheduled audit was LAST STARTED (not when it finished). This is what
// prevents runaway re-triggering: once trigger() records lastTrigger=now,
// the next run is a full interval away regardless of how long the audit runs.
func (s *Scheduler) nextDelay(interval time.Duration) time.Duration {
	s.mu.Lock()
	last := s.lastTrigger
	s.mu.Unlock()

	const soon = 5 * time.Second // grace so the server is fully up
	if last.IsZero() {
		return soon // never run — start shortly after enabling
	}
	if elapsed := time.Since(last); elapsed < interval {
		return interval - elapsed
	}
	return soon // overdue — run shortly
}

// scheduledInFlight reports whether a scheduled audit is still pending or
// running, so we never stack a second one on top of it.
func (s *Scheduler) scheduledInFlight() bool {
	audits, _, err := s.store.ListAudits(20, 0)
	if err != nil {
		return false
	}
	for _, a := range audits {
		if a.Trigger == "scheduled" && a.Running() {
			return true
		}
	}
	return false
}

// trigger creates and starts one scheduled audit.
func (s *Scheduler) trigger(ctx context.Context) {
	cfg := s.snapshot()
	if ctx.Err() != nil || !cfg.Enabled || len(cfg.Websites) == 0 {
		return
	}
	// Never stack scheduled audits: if one is still in flight, skip this
	// tick and try again next interval.
	if s.scheduledInFlight() {
		s.log.Info("scheduler: previous scheduled audit still running, skipping this tick")
		s.mu.Lock()
		s.lastTrigger = nowUTC() // reset the clock so we wait a full interval
		s.mu.Unlock()
		return
	}
	s.mu.Lock()
	s.lastTrigger = nowUTC()
	s.mu.Unlock()

	params := cfg.Params
	params.Websites = append([]string(nil), cfg.Websites...)
	params.Normalize()

	a := &models.Audit{
		Status:    models.AuditPending,
		Stage:     models.StageQueued,
		Trigger:   "scheduled",
		Params:    params,
		Stats:     models.NewAuditStats(),
		CreatedAt: nowUTC(),
	}
	for _, w := range params.Websites {
		a.Sites = append(a.Sites, models.AuditSite{Website: w, Status: "pending"})
	}
	if err := s.store.CreateAudit(a); err != nil {
		s.log.Error("scheduler: create audit", "err", err)
		return
	}
	s.log.Info("scheduled audit started", "audit", a.ID, "websites", len(params.Websites))
	run := *a
	s.orch.Start(&run)
}

// nowUTC exists so tests can reason about timestamps without a global clock.
func nowUTC() time.Time { return time.Now().UTC() }
