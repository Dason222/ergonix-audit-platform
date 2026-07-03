package report

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"

	"github.com/ergonix/auditor/backend/internal/models"
)

// PDFExporter renders a summary + issue table as PDF (pure Go, no browser).
type PDFExporter struct{}

func (PDFExporter) ContentType() string { return "application/pdf" }
func (PDFExporter) Ext() string         { return "pdf" }

var sevColors = map[models.Severity][3]int{
	models.SeverityCritical: {220, 38, 38},
	models.SeverityHigh:     {234, 88, 12},
	models.SeverityMedium:   {217, 119, 6},
	models.SeverityLow:      {101, 163, 13},
}

func (PDFExporter) Export(w io.Writer, fa *FullAudit) error {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	// Unicode font support: use the built-in helper to translate to cp1257
	// is not enough for LT/CZ/PL; fpdf core fonts are latin-1 only, so we
	// transliterate unsupported runes instead of corrupting output.
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 18)
	pdf.Cell(0, 10, tr(fmt.Sprintf("Ergonix Website Audit - Report #%d", fa.Audit.ID)))
	pdf.Ln(9)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(107, 114, 128)
	pdf.Cell(0, 6, tr(fmt.Sprintf("Status: %s   Created: %s   Generated: %s",
		fa.Audit.Status,
		fa.Audit.CreatedAt.Format("2006-01-02 15:04"),
		time.Now().Format("2006-01-02 15:04"))))
	pdf.Ln(12)

	// Summary cards
	stats := fa.Audit.Stats
	pdf.SetTextColor(31, 41, 55)
	pdf.SetFont("Helvetica", "B", 11)
	summary := []struct {
		label string
		value string
	}{
		{"Websites", fmt.Sprint(stats.TotalWebsites)},
		{"Pages", fmt.Sprint(stats.TotalPages)},
		{"Issues", fmt.Sprint(stats.TotalIssues)},
		{"Duration", (time.Duration(stats.DurationMs) * time.Millisecond).Round(time.Second).String()},
		{"Critical", fmt.Sprint(stats.BySeverity[models.SeverityCritical])},
		{"High", fmt.Sprint(stats.BySeverity[models.SeverityHigh])},
		{"Medium", fmt.Sprint(stats.BySeverity[models.SeverityMedium])},
		{"Low", fmt.Sprint(stats.BySeverity[models.SeverityLow])},
	}
	for _, s := range summary {
		pdf.SetFillColor(243, 244, 246)
		pdf.CellFormat(33, 14, "", "1", 0, "", true, 0, "")
		x, y := pdf.GetXY()
		pdf.SetXY(x-33, y+1)
		pdf.SetFont("Helvetica", "B", 12)
		pdf.CellFormat(33, 6, s.value, "", 0, "C", false, 0, "")
		pdf.SetXY(x-33, y+7)
		pdf.SetFont("Helvetica", "", 7)
		pdf.CellFormat(33, 5, s.label, "", 0, "C", false, 0, "")
		pdf.SetXY(x+1, y)
	}
	pdf.Ln(20)

	// Issues table
	pdf.SetFont("Helvetica", "B", 13)
	pdf.Cell(0, 8, tr(fmt.Sprintf("Issues (%d)", len(fa.Issues))))
	pdf.Ln(9)

	headers := []struct {
		title string
		width float64
	}{
		{"Severity", 20}, {"Website", 32}, {"Category", 24}, {"Src", 10},
		{"Page", 60}, {"Issue", 80}, {"Suggested fix", 43},
	}
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetFillColor(17, 24, 39)
	pdf.SetTextColor(249, 250, 251)
	for _, h := range headers {
		pdf.CellFormat(h.width, 7, h.title, "1", 0, "", true, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Helvetica", "", 7)
	for _, is := range fa.Issues {
		// Row height driven by the longest wrapped cell. fpdf's SplitText
		// panics on non-Latin-1 runes with core fonts, so wrap with the
		// byte-safe splitLines helper on already-translated text.
		desc := is.Title
		if is.Description != "" {
			desc = is.Title + ": " + is.Description
		}
		descLines := splitLines(pdf, tr(desc), 78)
		fixLines := splitLines(pdf, tr(is.SuggestedFix), 41)
		urlLines := splitLines(pdf, tr(is.PageURL), 58)
		n := maxInt(len(descLines), maxInt(len(fixLines), maxInt(len(urlLines), 1)))
		if n > 6 {
			n = 6
		}
		rowH := float64(n) * 3.4
		if rowH < 6 {
			rowH = 6
		}

		if pdf.GetY()+rowH > 190 { // manual page-break with header repeat
			pdf.AddPage()
			pdf.SetFont("Helvetica", "B", 8)
			pdf.SetFillColor(17, 24, 39)
			pdf.SetTextColor(249, 250, 251)
			for _, h := range headers {
				pdf.CellFormat(h.width, 7, h.title, "1", 0, "", true, 0, "")
			}
			pdf.Ln(-1)
			pdf.SetFont("Helvetica", "", 7)
		}

		x, y := pdf.GetXY()
		c := sevColors[is.Severity]
		pdf.SetTextColor(c[0], c[1], c[2])
		pdf.SetFont("Helvetica", "B", 7)
		pdf.CellFormat(20, rowH, string(is.Severity), "1", 0, "", false, 0, "")
		pdf.SetFont("Helvetica", "", 7)
		pdf.SetTextColor(31, 41, 55)
		pdf.CellFormat(32, rowH, tr(trimTo(is.Website, 40)), "1", 0, "", false, 0, "")
		pdf.CellFormat(24, rowH, string(is.Category), "1", 0, "", false, 0, "")
		pdf.CellFormat(10, rowH, string(is.Source), "1", 0, "", false, 0, "")

		multiCell := func(width float64, lines []string) {
			cx, cy := pdf.GetXY()
			pdf.Rect(cx, cy, width, rowH, "D")
			pdf.SetXY(cx+1, cy+0.6)
			for i, ln := range lines {
				if i >= n {
					break
				}
				pdf.CellFormat(width-2, 3.4, ln, "", 2, "", false, 0, "")
			}
			pdf.SetXY(cx+width, cy)
		}
		multiCell(60, urlLines)
		multiCell(80, descLines)
		multiCell(43, fixLines)

		pdf.SetXY(x, y+rowH)
	}

	pdf.SetY(-12)
	pdf.SetFont("Helvetica", "I", 7)
	pdf.SetTextColor(156, 163, 175)
	pdf.Cell(0, 5, "Generated by the Ergonix Website Audit Platform")

	return pdf.Output(w)
}

// splitLines word-wraps an already code-page-translated string to the given
// width in mm, measuring with GetStringWidth (byte-safe for core fonts).
// Overlong words are hard-split.
func splitLines(pdf *fpdf.Fpdf, s string, width float64) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var lines []string
	for _, para := range strings.Split(s, "\n") {
		words := strings.Fields(para)
		cur := ""
		flush := func() {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
		}
		for _, w := range words {
			// Hard-split words wider than the column (long URLs).
			for pdf.GetStringWidth(w) > width {
				cut := len(w)
				for cut > 1 && pdf.GetStringWidth(w[:cut]) > width {
					cut--
				}
				candidate := w[:cut]
				if cur != "" && pdf.GetStringWidth(cur+" "+candidate) > width {
					flush()
				}
				if cur == "" {
					lines = append(lines, candidate)
				} else {
					cur += " " + candidate
					flush()
				}
				w = w[cut:]
			}
			if w == "" {
				continue
			}
			switch {
			case cur == "":
				cur = w
			case pdf.GetStringWidth(cur+" "+w) <= width:
				cur += " " + w
			default:
				flush()
				cur = w
			}
		}
		flush()
	}
	return lines
}

func trimTo(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
