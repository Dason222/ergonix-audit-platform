package ai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ergonix/auditor/backend/internal/models"
)

// aiFinding mirrors the JSON shape the prompt demands from the model.
type aiFinding struct {
	Type         string  `json:"type"`
	Severity     string  `json:"severity"`
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	SuggestedFix string  `json:"suggestedFix"`
	Confidence   float64 `json:"confidence"`
}

type aiResponse struct {
	Issues []aiFinding `json:"issues"`
}

// typeToCategory maps the 16 prompt type ids onto issue categories.
var typeToCategory = map[string]models.Category{
	"wrong_language":        models.CategoryTranslation,
	"mixed_language":        models.CategoryTranslation,
	"poor_translation":      models.CategoryTranslation,
	"missing_translation":   models.CategoryTranslation,
	"placeholder":           models.CategoryContent,
	"unnatural_wording":     models.CategoryTranslation,
	"grammar":               models.CategoryTranslation,
	"confusing_description": models.CategoryContent,
	"missing_buyer_info":    models.CategoryContent,
	"missing_shipping_info": models.CategoryContent,
	"missing_warranty_info": models.CategoryContent,
	"suspicious_pricing":    models.CategoryLogic,
	"ux_writing":            models.CategoryUI,
	"inconsistency":         models.CategoryContent,
	"country_mismatch":      models.CategoryLogic,
	"trust":                 models.CategoryContent,
}

// ParseFindings validates the model's raw output and converts it into
// issues. Malformed entries are dropped rather than failing the batch.
func ParseFindings(raw, website, pageURL string) ([]models.Issue, error) {
	cleaned := stripFences(raw)

	var resp aiResponse
	if err := json.Unmarshal([]byte(cleaned), &resp); err != nil {
		// Some models wrap the object in stray text; try to slice out the
		// outermost JSON object before giving up.
		if start, end := strings.Index(cleaned, "{"), strings.LastIndex(cleaned, "}"); start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(cleaned[start:end+1]), &resp); err2 != nil {
				return nil, fmt.Errorf("unparseable AI response: %w", err)
			}
		} else {
			return nil, fmt.Errorf("unparseable AI response: %w", err)
		}
	}

	var out []models.Issue
	for _, f := range resp.Issues {
		cat, ok := typeToCategory[strings.ToLower(strings.TrimSpace(f.Type))]
		if !ok || strings.TrimSpace(f.Title) == "" {
			continue // unknown type or empty finding — drop
		}
		sev := models.Severity(strings.ToLower(strings.TrimSpace(f.Severity)))
		if !sev.Valid() {
			sev = models.SeverityLow
		}
		conf := f.Confidence
		if conf <= 0 || conf > 1 {
			conf = 0.5
		}
		aiType := strings.ToLower(strings.TrimSpace(f.Type))
		out = append(out, models.Issue{
			Website:      website,
			PageURL:      pageURL,
			Category:     cat,
			Source:       models.SourceAI,
			CheckID:      "ai:" + aiType,
			Severity:     sev,
			Title:        strings.TrimSpace(f.Title),
			Description:  strings.TrimSpace(f.Description),
			SuggestedFix: strings.TrimSpace(f.SuggestedFix),
			Confidence:   conf,
			Details:      map[string]any{"aiType": aiType},
		})
	}
	return out, nil
}

// stripFences removes markdown code fences some models insist on emitting.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}
