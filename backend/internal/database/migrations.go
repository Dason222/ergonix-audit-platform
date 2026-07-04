package database

import (
	"fmt"
	"strings"
)

// migrate applies the schema. Statements are idempotent so this can run on
// every startup; ALTER-style migrations get appended to the list with guards.
func (d *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS audits (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			status      TEXT NOT NULL,
			stage       TEXT NOT NULL DEFAULT '',
			params      TEXT NOT NULL DEFAULT '{}',
			sites       TEXT NOT NULL DEFAULT '[]',
			stats       TEXT NOT NULL DEFAULT '{}',
			error       TEXT NOT NULL DEFAULT '',
			created_at  TEXT NOT NULL,
			started_at  TEXT,
			finished_at TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS pages (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			audit_id         INTEGER NOT NULL REFERENCES audits(id) ON DELETE CASCADE,
			website          TEXT NOT NULL,
			url              TEXT NOT NULL,
			status_code      INTEGER NOT NULL DEFAULT 0,
			title            TEXT NOT NULL DEFAULT '',
			response_time_ms INTEGER NOT NULL DEFAULT 0,
			fetch_error      TEXT NOT NULL DEFAULT '',
			data             TEXT NOT NULL,
			crawled_at       TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pages_audit ON pages(audit_id)`,
		`CREATE TABLE IF NOT EXISTS issues (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			audit_id      INTEGER NOT NULL REFERENCES audits(id) ON DELETE CASCADE,
			website       TEXT NOT NULL,
			page_url      TEXT NOT NULL,
			category      TEXT NOT NULL,
			source        TEXT NOT NULL,
			severity      TEXT NOT NULL,
			title         TEXT NOT NULL,
			description   TEXT NOT NULL DEFAULT '',
			suggested_fix TEXT NOT NULL DEFAULT '',
			confidence    REAL NOT NULL DEFAULT 1,
			screenshot    TEXT NOT NULL DEFAULT '',
			details       TEXT NOT NULL DEFAULT '{}',
			created_at    TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_issues_audit ON issues(audit_id)`,
		`CREATE INDEX IF NOT EXISTS idx_issues_filter ON issues(audit_id, website, severity, category, source)`,
		`CREATE TABLE IF NOT EXISTS settings (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := d.sql.Exec(s); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	// Column additions for databases created by earlier versions.
	if err := d.addColumn("issues", "check_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

// addColumn adds a column if it does not exist yet (idempotent migration).
func (d *DB) addColumn(table, column, ddl string) error {
	_, err := d.sql.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, ddl))
	if err != nil && strings.Contains(err.Error(), "duplicate column") {
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate %s.%s: %w", table, column, err)
	}
	return nil
}
