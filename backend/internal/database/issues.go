package database

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// IssueFilter narrows ListIssues. Zero values mean "no filter".
type IssueFilter struct {
	AuditID  int64
	Website  string
	Severity string
	Category string
	Source   string
	Search   string
	Limit    int
	Offset   int
}

// SaveIssues inserts issues in one transaction, filling their IDs.
func (d *DB) SaveIssues(issues []models.Issue) error {
	if len(issues) == 0 {
		return nil
	}
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO issues (audit_id, website, page_url, category, source, check_id, severity,
		                     title, description, suggested_fix, confidence, screenshot, details, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i := range issues {
		is := &issues[i]
		details := "{}"
		if is.Details != nil {
			b, err := json.Marshal(is.Details)
			if err != nil {
				return fmt.Errorf("marshal details: %w", err)
			}
			details = string(b)
		}
		res, err := stmt.Exec(is.AuditID, is.Website, is.PageURL, string(is.Category),
			string(is.Source), is.CheckID, string(is.Severity), is.Title, is.Description,
			is.SuggestedFix, is.Confidence, is.Screenshot, details, fmtTime(is.CreatedAt))
		if err != nil {
			return fmt.Errorf("insert issue %q: %w", is.Title, err)
		}
		is.ID, _ = res.LastInsertId()
	}
	return tx.Commit()
}

// ListIssues returns issues matching the filter (ordered by severity, then id)
// plus the total match count ignoring Limit/Offset.
func (d *DB) ListIssues(f IssueFilter) ([]models.Issue, int, error) {
	where, args := issueWhere(f)

	var total int
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM issues `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `SELECT id, audit_id, website, page_url, category, source, check_id, severity,
	             title, description, suggested_fix, confidence, screenshot, details, created_at
	      FROM issues ` + where + `
	      ORDER BY CASE severity
	          WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, id`
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d OFFSET %d", f.Limit, max(f.Offset, 0))
	}

	rows, err := d.sql.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []models.Issue{}
	for rows.Next() {
		var (
			is                              models.Issue
			category, source, severity      string
			details, createdAt              string
		)
		if err := rows.Scan(&is.ID, &is.AuditID, &is.Website, &is.PageURL, &category,
			&source, &is.CheckID, &severity, &is.Title, &is.Description, &is.SuggestedFix,
			&is.Confidence, &is.Screenshot, &details, &createdAt); err != nil {
			return nil, 0, err
		}
		is.Category = models.Category(category)
		is.Source = models.Source(source)
		is.Severity = models.Severity(severity)
		is.CreatedAt = parseTime(createdAt)
		if details != "" && details != "{}" {
			_ = json.Unmarshal([]byte(details), &is.Details)
		}
		out = append(out, is)
	}
	return out, total, rows.Err()
}

func issueWhere(f IssueFilter) (string, []any) {
	var (
		conds []string
		args  []any
	)
	if f.AuditID > 0 {
		conds = append(conds, "audit_id=?")
		args = append(args, f.AuditID)
	}
	if f.Website != "" {
		conds = append(conds, "website=?")
		args = append(args, f.Website)
	}
	if f.Severity != "" {
		conds = append(conds, "severity=?")
		args = append(args, f.Severity)
	}
	if f.Category != "" {
		conds = append(conds, "category=?")
		args = append(args, f.Category)
	}
	if f.Source != "" {
		conds = append(conds, "source=?")
		args = append(args, f.Source)
	}
	if f.Search != "" {
		conds = append(conds, "(title LIKE ? OR description LIKE ? OR page_url LIKE ? OR suggested_fix LIKE ?)")
		pat := "%" + f.Search + "%"
		args = append(args, pat, pat, pat, pat)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conds, " AND "), args
}
