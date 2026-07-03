// Package report renders a finished audit into exportable formats:
// JSON, CSV, standalone HTML, and PDF.
package report

import (
	"io"

	"github.com/ergonix/auditor/backend/internal/models"
)

// FullAudit bundles everything an export needs.
type FullAudit struct {
	Audit  *models.Audit   `json:"audit"`
	Issues []models.Issue  `json:"issues"`
	Pages  []*models.Page  `json:"pages,omitempty"`
}

// Exporter renders a FullAudit to a writer.
type Exporter interface {
	ContentType() string
	Ext() string
	Export(w io.Writer, fa *FullAudit) error
}

// ForFormat returns the exporter for a format string, or nil.
func ForFormat(format string) Exporter {
	switch format {
	case "json":
		return JSONExporter{}
	case "csv":
		return CSVExporter{}
	case "html":
		return HTMLExporter{}
	case "pdf":
		return PDFExporter{}
	}
	return nil
}
