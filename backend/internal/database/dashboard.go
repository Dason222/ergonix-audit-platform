package database

import (
	"github.com/ergonix/auditor/backend/internal/models"
)

// TimePoint is one audit's issue counts for the issues-over-time chart.
type TimePoint struct {
	AuditID  int64  `json:"auditId"`
	Date     string `json:"date"` // RFC3339
	Total    int    `json:"total"`
	Critical int    `json:"critical"`
	High     int    `json:"high"`
	Medium   int    `json:"medium"`
	Low      int    `json:"low"`
}

// DashboardData aggregates everything the dashboard page needs in one call.
type DashboardData struct {
	TotalAudits          int                      `json:"totalAudits"`
	TotalWebsitesAudited int                      `json:"totalWebsitesAudited"`
	TotalPagesScanned    int                      `json:"totalPagesScanned"`
	TotalIssues          int                      `json:"totalIssues"`
	AvgAuditDurationMs   int64                    `json:"avgAuditDurationMs"`
	BySeverity           map[models.Severity]int  `json:"bySeverity"`
	ByCategory           map[models.Category]int  `json:"byCategory"`
	ByWebsite            map[string]int           `json:"byWebsite"`
	IssuesOverTime       []TimePoint              `json:"issuesOverTime"`
	RecentAudits         []*models.Audit          `json:"recentAudits"`
}

// Dashboard computes all-time aggregates across audits and issues.
func (d *DB) Dashboard() (*DashboardData, error) {
	data := &DashboardData{
		BySeverity:     map[models.Severity]int{},
		ByCategory:     map[models.Category]int{},
		ByWebsite:      map[string]int{},
		IssuesOverTime: []TimePoint{},
	}

	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM audits`).Scan(&data.TotalAudits); err != nil {
		return nil, err
	}
	if err := d.sql.QueryRow(`SELECT COUNT(DISTINCT website) FROM pages`).Scan(&data.TotalWebsitesAudited); err != nil {
		return nil, err
	}
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM pages`).Scan(&data.TotalPagesScanned); err != nil {
		return nil, err
	}
	if err := d.sql.QueryRow(`SELECT COUNT(*) FROM issues`).Scan(&data.TotalIssues); err != nil {
		return nil, err
	}

	// Average duration of finished audits (stored in stats JSON; recompute
	// from timestamps to stay independent of stats format).
	var avgMs float64
	err := d.sql.QueryRow(
		`SELECT COALESCE(AVG((julianday(finished_at) - julianday(started_at)) * 86400000), 0)
		 FROM audits WHERE started_at IS NOT NULL AND finished_at IS NOT NULL`).
		Scan(&avgMs)
	if err != nil {
		return nil, err
	}
	data.AvgAuditDurationMs = int64(avgMs)

	if err := d.countInto(`SELECT severity, COUNT(*) FROM issues GROUP BY severity`, func(k string, n int) {
		data.BySeverity[models.Severity(k)] = n
	}); err != nil {
		return nil, err
	}
	if err := d.countInto(`SELECT category, COUNT(*) FROM issues GROUP BY category`, func(k string, n int) {
		data.ByCategory[models.Category(k)] = n
	}); err != nil {
		return nil, err
	}
	if err := d.countInto(`SELECT website, COUNT(*) FROM issues GROUP BY website`, func(k string, n int) {
		data.ByWebsite[k] = n
	}); err != nil {
		return nil, err
	}

	// Issues over time: one point per audit that produced issues or finished.
	rows, err := d.sql.Query(
		`SELECT a.id, a.created_at,
		        COUNT(i.id),
		        SUM(CASE WHEN i.severity='critical' THEN 1 ELSE 0 END),
		        SUM(CASE WHEN i.severity='high'     THEN 1 ELSE 0 END),
		        SUM(CASE WHEN i.severity='medium'   THEN 1 ELSE 0 END),
		        SUM(CASE WHEN i.severity='low'      THEN 1 ELSE 0 END)
		 FROM audits a LEFT JOIN issues i ON i.audit_id = a.id
		 WHERE a.status IN ('completed','failed')
		 GROUP BY a.id ORDER BY a.id ASC LIMIT 60`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			tp   TimePoint
			date string
			c, h, m, l *int
		)
		if err := rows.Scan(&tp.AuditID, &date, &tp.Total, &c, &h, &m, &l); err != nil {
			return nil, err
		}
		tp.Date = date
		tp.Critical, tp.High, tp.Medium, tp.Low = deref(c), deref(h), deref(m), deref(l)
		data.IssuesOverTime = append(data.IssuesOverTime, tp)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	recent, _, err := d.ListAudits(5, 0)
	if err != nil {
		return nil, err
	}
	data.RecentAudits = recent
	if data.RecentAudits == nil {
		data.RecentAudits = []*models.Audit{}
	}
	return data, nil
}

func (d *DB) countInto(query string, set func(key string, n int)) error {
	rows, err := d.sql.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			k string
			n int
		)
		if err := rows.Scan(&k, &n); err != nil {
			return err
		}
		set(k, n)
	}
	return rows.Err()
}

func deref(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
