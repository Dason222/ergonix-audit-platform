package database

import (
	"encoding/json"
	"fmt"

	"github.com/ergonix/auditor/backend/internal/models"
)

// SavePages inserts crawled pages in one transaction. The full Page struct is
// stored as JSON in the data column; hot fields are duplicated into scalar
// columns for cheap listing.
func (d *DB) SavePages(pages []*models.Page) error {
	if len(pages) == 0 {
		return nil
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO pages (audit_id, website, url, status_code, title, response_time_ms, fetch_error, data, crawled_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, p := range pages {
		data, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("marshal page %s: %w", p.URL, err)
		}
		res, err := stmt.Exec(p.AuditID, p.Website, p.URL, p.StatusCode, p.Title,
			p.ResponseTimeMs, p.FetchError, data, fmtTime(p.CrawledAt))
		if err != nil {
			return fmt.Errorf("insert page %s: %w", p.URL, err)
		}
		p.ID, _ = res.LastInsertId()
	}
	return tx.Commit()
}

// ListPages returns all pages of an audit in crawl order.
func (d *DB) ListPages(auditID int64) ([]*models.Page, error) {
	rows, err := d.sql.Query(`SELECT id, data FROM pages WHERE audit_id=? ORDER BY id`, auditID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.Page
	for rows.Next() {
		var (
			id   int64
			data string
		)
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		var p models.Page
		if err := json.Unmarshal([]byte(data), &p); err != nil {
			return nil, fmt.Errorf("page %d: %w", id, err)
		}
		p.ID = id
		out = append(out, &p)
	}
	return out, rows.Err()
}
