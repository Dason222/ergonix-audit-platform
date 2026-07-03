package report

import (
	"encoding/json"
	"io"
)

// JSONExporter emits the full audit (audit, issues, pages) as pretty JSON.
type JSONExporter struct{}

func (JSONExporter) ContentType() string { return "application/json" }
func (JSONExporter) Ext() string         { return "json" }

func (JSONExporter) Export(w io.Writer, fa *FullAudit) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(fa)
}
