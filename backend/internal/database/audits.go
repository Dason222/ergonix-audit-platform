package database

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ergonix/auditor/backend/internal/models"
)

// CreateAudit inserts a new audit and fills a.ID.
func (d *DB) CreateAudit(a *models.Audit) error {
	params, sites, stats, err := marshalAuditJSON(a)
	if err != nil {
		return err
	}
	res, err := d.sql.Exec(
		`INSERT INTO audits (status, stage, params, sites, stats, error, created_at, started_at, finished_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(a.Status), a.Stage, params, sites, stats, a.Error,
		fmtTime(a.CreatedAt), fmtTimePtr(a.StartedAt), fmtTimePtr(a.FinishedAt),
	)
	if err != nil {
		return fmt.Errorf("create audit: %w", err)
	}
	a.ID, err = res.LastInsertId()
	return err
}

// UpdateAudit persists every mutable field of an audit.
func (d *DB) UpdateAudit(a *models.Audit) error {
	params, sites, stats, err := marshalAuditJSON(a)
	if err != nil {
		return err
	}
	_, err = d.sql.Exec(
		`UPDATE audits SET status=?, stage=?, params=?, sites=?, stats=?, error=?, started_at=?, finished_at=?
		 WHERE id=?`,
		string(a.Status), a.Stage, params, sites, stats, a.Error,
		fmtTimePtr(a.StartedAt), fmtTimePtr(a.FinishedAt), a.ID,
	)
	if err != nil {
		return fmt.Errorf("update audit %d: %w", a.ID, err)
	}
	return nil
}

// GetAudit loads one audit by id, returning ErrNotFound if missing.
func (d *DB) GetAudit(id int64) (*models.Audit, error) {
	row := d.sql.QueryRow(
		`SELECT id, status, stage, params, sites, stats, error, created_at, started_at, finished_at
		 FROM audits WHERE id=?`, id)
	a, err := scanAudit(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return a, err
}

// ListAudits returns audits newest-first plus the total count.
func (d *DB) ListAudits(limit, offset int) ([]*models.Audit, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var total int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM audits`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := d.sql.Query(
		`SELECT id, status, stage, params, sites, stats, error, created_at, started_at, finished_at
		 FROM audits ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*models.Audit
	for rows.Next() {
		a, err := scanAudit(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, a)
	}
	return out, total, rows.Err()
}

// DeleteAudit removes an audit and (via FK cascade) its pages and issues.
func (d *DB) DeleteAudit(id int64) error {
	res, err := d.sql.Exec(`DELETE FROM audits WHERE id=?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func marshalAuditJSON(a *models.Audit) (params, sites, stats []byte, err error) {
	if params, err = json.Marshal(a.Params); err != nil {
		return
	}
	if a.Sites == nil {
		a.Sites = []models.AuditSite{}
	}
	if sites, err = json.Marshal(a.Sites); err != nil {
		return
	}
	stats, err = json.Marshal(a.Stats)
	return
}

type rowScanner interface{ Scan(dest ...any) error }

func scanAudit(r rowScanner) (*models.Audit, error) {
	var (
		a                       models.Audit
		status                  string
		params, sites, stats    string
		createdAt               string
		startedAt, finishedAt   sql.NullString
	)
	err := r.Scan(&a.ID, &status, &a.Stage, &params, &sites, &stats, &a.Error,
		&createdAt, &startedAt, &finishedAt)
	if err != nil {
		return nil, err
	}
	a.Status = models.AuditStatus(status)
	if err := json.Unmarshal([]byte(params), &a.Params); err != nil {
		return nil, fmt.Errorf("audit %d params: %w", a.ID, err)
	}
	if err := json.Unmarshal([]byte(sites), &a.Sites); err != nil {
		return nil, fmt.Errorf("audit %d sites: %w", a.ID, err)
	}
	if err := json.Unmarshal([]byte(stats), &a.Stats); err != nil {
		return nil, fmt.Errorf("audit %d stats: %w", a.ID, err)
	}
	a.CreatedAt = parseTime(createdAt)
	a.StartedAt = parseTimePtr(startedAt)
	a.FinishedAt = parseTimePtr(finishedAt)
	return &a, nil
}
