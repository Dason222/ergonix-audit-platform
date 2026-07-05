// Package scheduler runs audits automatically on a recurring interval, so
// the platform detects problems on its own without anyone clicking "Audit".
package scheduler

import (
	"context"
	"log/slog"
	"time"

	"github.com/ergonix/auditor/backend/internal/audit"
	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
)

// Config controls automatic auditing.
type Config struct {
	Enabled  bool
	Interval time.Duration
	AtStart  bool          // run once shortly after boot
	Websites []string      // sites to audit (empty handled by caller)
	Params   models.AuditParams
}

// Scheduler periodically enqueues audits via the orchestrator.
type Scheduler struct {
	store database.Store
	orch  *audit.Orchestrator
	cfg   Config
	log   *slog.Logger
}

// New builds a Scheduler.
func New(store database.Store, orch *audit.Orchestrator, cfg Config, log *slog.Logger) *Scheduler {
	return &Scheduler{store: store, orch: orch, cfg: cfg, log: log}
}

// Start launches the scheduling loop in a goroutine. It returns immediately;
// the loop stops when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	if !s.cfg.Enabled || len(s.cfg.Websites) == 0 || s.cfg.Interval <= 0 {
		s.log.Info("automatic auditing disabled")
		return
	}
	s.log.Info("automatic auditing enabled",
		"interval", s.cfg.Interval.String(),
		"websites", len(s.cfg.Websites),
		"atStart", s.cfg.AtStart)

	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	// Decide whether to run immediately: either AtStart is set, or the most
	// recent scheduled audit is older than the interval (survives restarts).
	if s.cfg.AtStart || s.overdue() {
		// small delay so the server is fully up first
		select {
		case <-time.After(5 * time.Second):
			s.trigger(ctx)
		case <-ctx.Done():
			return
		}
	}

	ticker := time.NewTicker(s.cfg.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.trigger(ctx)
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return
		}
	}
}

// overdue reports whether no scheduled audit has completed within the last
// interval, so a restart doesn't skip a due run.
func (s *Scheduler) overdue() bool {
	audits, _, err := s.store.ListAudits(50, 0)
	if err != nil {
		return false
	}
	for _, a := range audits {
		if a.Trigger == "scheduled" && a.FinishedAt != nil {
			return time.Since(*a.FinishedAt) >= s.cfg.Interval
		}
	}
	return true // never run a scheduled audit before
}

// trigger creates and starts one scheduled audit.
func (s *Scheduler) trigger(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	params := s.cfg.Params
	params.Websites = append([]string(nil), s.cfg.Websites...)
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
