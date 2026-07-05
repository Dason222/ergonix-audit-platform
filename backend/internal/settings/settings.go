// Package settings makes AI and scheduler configuration editable at runtime
// from the UI. Persisted settings (in the DB) override the .env defaults, and
// changes are applied live: the AI analyzer is rebuilt and the scheduler
// reconfigured without a restart.
package settings

import (
	"fmt"
	"log/slog"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ergonix/auditor/backend/internal/ai"
	"github.com/ergonix/auditor/backend/internal/audit"
	"github.com/ergonix/auditor/backend/internal/config"
	"github.com/ergonix/auditor/backend/internal/database"
	"github.com/ergonix/auditor/backend/internal/models"
	"github.com/ergonix/auditor/backend/internal/scheduler"
)

// Persisted setting keys.
const (
	keyAIKey          = "ai.apiKey"
	keyAIBaseURL      = "ai.baseUrl"
	keyAIModel        = "ai.model"
	keySchedEnabled   = "schedule.enabled"
	keySchedInterval  = "schedule.intervalHours"
	keySchedWebsites  = "schedule.websites"
)

// Effective is the resolved configuration after overlaying DB settings on
// the .env defaults.
type Effective struct {
	AIKey            string
	AIBaseURL        string
	AIModel          string
	ScheduleEnabled  bool
	ScheduleInterval time.Duration
	ScheduleWebsites []string // resolved (empty falls back to all configured)
}

// AIEnabled reports whether a usable AI key is configured.
func (e Effective) AIEnabled() bool { return strings.TrimSpace(e.AIKey) != "" }

// Update carries the editable fields from an API request. Pointer/omission
// semantics: a nil pointer means "leave unchanged".
type Update struct {
	AIKey            *string  `json:"aiApiKey"`
	ClearAIKey       bool     `json:"clearAiKey"`
	AIBaseURL        *string  `json:"aiBaseUrl"`
	AIModel          *string  `json:"aiModel"`
	ScheduleEnabled  *bool    `json:"scheduleEnabled"`
	ScheduleInterval *int     `json:"scheduleIntervalHours"`
	ScheduleWebsites *[]string `json:"scheduleWebsites"`
}

// Manager owns the effective settings and applies changes to the live
// orchestrator and scheduler.
type Manager struct {
	base  *config.Config
	store database.Store
	orch  *audit.Orchestrator
	sched *scheduler.Scheduler
	log   *slog.Logger

	mu  sync.RWMutex
	eff Effective
}

// New builds the Manager and loads the effective settings from the DB.
func New(base *config.Config, store database.Store, orch *audit.Orchestrator,
	sched *scheduler.Scheduler, log *slog.Logger) (*Manager, error) {
	m := &Manager{base: base, store: store, orch: orch, sched: sched, log: log}
	if err := m.reload(); err != nil {
		return nil, err
	}
	return m, nil
}

// Effective returns a copy of the current resolved settings.
func (m *Manager) Effective() Effective {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.eff
}

// reload reads DB settings, merges over env, and applies them live.
func (m *Manager) reload() error {
	stored, err := m.store.GetSettings()
	if err != nil {
		return err
	}
	eff := m.merge(stored)
	m.mu.Lock()
	m.eff = eff
	m.mu.Unlock()
	m.apply(eff)
	return nil
}

// merge overlays stored values on the env defaults.
func (m *Manager) merge(stored map[string]string) Effective {
	eff := Effective{
		AIKey:            m.base.AIAPIKey,
		AIBaseURL:        m.base.AIBaseURL,
		AIModel:          m.base.AIModel,
		ScheduleEnabled:  m.base.ScheduleEnabled,
		ScheduleInterval: m.base.ScheduleInterval,
		ScheduleWebsites: m.base.ScheduleWebsites,
	}
	if v, ok := stored[keyAIKey]; ok {
		eff.AIKey = v
	}
	if v := stored[keyAIBaseURL]; v != "" {
		eff.AIBaseURL = strings.TrimRight(v, "/")
	}
	if v := stored[keyAIModel]; v != "" {
		eff.AIModel = v
	}
	if v, ok := stored[keySchedEnabled]; ok {
		eff.ScheduleEnabled = v == "true"
	}
	if v := stored[keySchedInterval]; v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			eff.ScheduleInterval = time.Duration(n) * time.Hour
		}
	}
	if v, ok := stored[keySchedWebsites]; ok {
		eff.ScheduleWebsites = splitCSV(v)
	}
	return eff
}

// apply pushes the effective settings into the running components.
func (m *Manager) apply(eff Effective) {
	// AI analyzer: rebuild or disable.
	if eff.AIEnabled() {
		client := ai.NewOpenAIClient(ai.ClientConfig{
			APIKey:  eff.AIKey,
			BaseURL: eff.AIBaseURL,
			Model:   eff.AIModel,
			Timeout: m.base.AITimeout,
		})
		m.orch.SetAnalyzer(ai.New(client, ai.Config{
			MaxPagesPerSite: m.base.AIMaxPages,
			MaxTextChars:    m.base.AIMaxTextChars,
			Model:           eff.AIModel,
		}, m.log))
	} else {
		m.orch.SetAnalyzer(nil)
	}

	// Scheduler: reconfigure with resolved website list.
	sites := eff.ScheduleWebsites
	if len(sites) == 0 {
		sites = m.base.Websites
	}
	if m.sched != nil {
		m.sched.Reconfigure(scheduler.Config{
			Enabled:  eff.ScheduleEnabled,
			Interval: eff.ScheduleInterval,
			Websites: sites,
			Params: models.AuditParams{
				MaxPages:          m.base.DefaultParams.MaxPages,
				MaxDepth:          m.base.DefaultParams.MaxDepth,
				Concurrency:       m.base.DefaultParams.Concurrency,
				RequestTimeoutSec: m.base.DefaultParams.RequestTimeoutSec,
				RetryCount:        m.base.DefaultParams.RetryCount,
				UseAI:             eff.AIEnabled(),
			},
		})
	}
}

// Apply re-reads and applies settings (used at startup after wiring).
func (m *Manager) Apply() error { return m.reload() }

// Save validates the update, persists changed keys, and applies live.
func (m *Manager) Save(u Update) error {
	toStore := map[string]string{}

	if u.ClearAIKey {
		toStore[keyAIKey] = ""
	} else if u.AIKey != nil && strings.TrimSpace(*u.AIKey) != "" {
		toStore[keyAIKey] = strings.TrimSpace(*u.AIKey)
	}
	if u.AIBaseURL != nil {
		v := strings.TrimSpace(*u.AIBaseURL)
		if v != "" {
			pu, err := url.Parse(v)
			if err != nil || (pu.Scheme != "http" && pu.Scheme != "https") {
				return fmt.Errorf("aiBaseUrl must be a valid http(s) URL")
			}
		}
		toStore[keyAIBaseURL] = strings.TrimRight(v, "/")
	}
	if u.AIModel != nil {
		toStore[keyAIModel] = strings.TrimSpace(*u.AIModel)
	}
	if u.ScheduleEnabled != nil {
		toStore[keySchedEnabled] = strconv.FormatBool(*u.ScheduleEnabled)
	}
	if u.ScheduleInterval != nil {
		n := *u.ScheduleInterval
		if n < 1 || n > 168 {
			return fmt.Errorf("scheduleIntervalHours must be between 1 and 168")
		}
		toStore[keySchedInterval] = strconv.Itoa(n)
	}
	if u.ScheduleWebsites != nil {
		toStore[keySchedWebsites] = strings.Join(*u.ScheduleWebsites, ",")
	}

	if len(toStore) == 0 {
		return nil
	}
	if err := m.store.SaveSettings(toStore); err != nil {
		return err
	}
	return m.reload()
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

// MaskKey returns a safe preview of a secret (first 3 + last 4 chars).
func MaskKey(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "••••"
	}
	return key[:3] + "••••" + key[len(key)-4:]
}
