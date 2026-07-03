// Package database implements persistence on SQLite (pure-Go driver, no CGO).
// All repository methods live on *DB, which satisfies the Store interface used
// by the API and the audit orchestrator.
package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ergonix/auditor/backend/internal/models"
)

// Store is the persistence contract consumed by the rest of the application.
type Store interface {
	CreateAudit(a *models.Audit) error
	UpdateAudit(a *models.Audit) error
	GetAudit(id int64) (*models.Audit, error)
	ListAudits(limit, offset int) ([]*models.Audit, int, error)
	DeleteAudit(id int64) error

	SavePages(pages []*models.Page) error
	ListPages(auditID int64) ([]*models.Page, error)

	SaveIssues(issues []models.Issue) error
	ListIssues(f IssueFilter) ([]models.Issue, int, error)

	Dashboard() (*DashboardData, error)

	GetSettings() (map[string]string, error)
	SaveSettings(values map[string]string) error

	Close() error
}

// DB wraps the SQLite connection and implements Store.
type DB struct {
	sql *sql.DB
}

var _ Store = (*DB)(nil)

// Open opens (creating if needed) the SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	dsn := "file:" + filepath.ToSlash(path) +
		"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite allows a single writer; serialize all access through one
	// connection to avoid SQLITE_BUSY under concurrent audit goroutines.
	conn.SetMaxOpenConns(1)

	db := &DB{sql: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, err
	}
	return db, nil
}

// Close releases the underlying connection.
func (d *DB) Close() error { return d.sql.Close() }

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = fmt.Errorf("not found")

// --- time helpers: timestamps are stored as RFC3339Nano TEXT ---

func fmtTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func fmtTimePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return fmtTime(*t)
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func parseTimePtr(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t := parseTime(ns.String)
	return &t
}
