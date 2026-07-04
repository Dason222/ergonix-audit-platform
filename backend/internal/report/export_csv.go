package report

import (
	"encoding/csv"
	"fmt"
	"io"
)

// CSVExporter emits the issue list as CSV (one row per issue).
type CSVExporter struct{}

func (CSVExporter) ContentType() string { return "text/csv; charset=utf-8" }
func (CSVExporter) Ext() string         { return "csv" }

func (CSVExporter) Export(w io.Writer, fa *FullAudit) error {
	// UTF-8 BOM so Excel opens Lithuanian/Czech/Polish characters correctly.
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"ID", "Website", "Page URL", "Category", "Source", "Check", "Severity",
		"Title", "Description", "Suggested Fix", "Confidence",
	}); err != nil {
		return err
	}
	for _, is := range fa.Issues {
		if err := cw.Write([]string{
			fmt.Sprint(is.ID), is.Website, is.PageURL, string(is.Category),
			string(is.Source), is.CheckID, string(is.Severity), is.Title, is.Description,
			is.SuggestedFix, fmt.Sprintf("%.2f", is.Confidence),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
