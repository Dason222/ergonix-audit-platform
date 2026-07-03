package models

// Severity classifies how urgent an issue is.
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityHigh     Severity = "high"
	SeverityMedium   Severity = "medium"
	SeverityLow      Severity = "low"
)

// SeverityRank orders severities for sorting; lower rank = more severe.
var SeverityRank = map[Severity]int{
	SeverityCritical: 0,
	SeverityHigh:     1,
	SeverityMedium:   2,
	SeverityLow:      3,
}

func (s Severity) Valid() bool {
	_, ok := SeverityRank[s]
	return ok
}

// Category groups issues by domain.
type Category string

const (
	CategoryTranslation   Category = "Translation"
	CategoryPerformance   Category = "Performance"
	CategoryAccessibility Category = "Accessibility"
	CategorySEO           Category = "SEO"
	CategorySecurity      Category = "Security"
	CategoryContent       Category = "Content"
	CategoryLogic         Category = "Logic"
	CategoryUI            Category = "UI"
	CategoryNetwork       Category = "Network"
	CategoryJavaScript    Category = "JavaScript"
)

// Categories lists every valid category.
var Categories = []Category{
	CategoryTranslation, CategoryPerformance, CategoryAccessibility,
	CategorySEO, CategorySecurity, CategoryContent, CategoryLogic,
	CategoryUI, CategoryNetwork, CategoryJavaScript,
}

func (c Category) Valid() bool {
	for _, v := range Categories {
		if v == c {
			return true
		}
	}
	return false
}

// Source identifies what produced an issue.
type Source string

const (
	SourceRule Source = "rule"
	SourceAI   Source = "ai"
)

// AuditStatus is the lifecycle state of an audit.
type AuditStatus string

const (
	AuditPending   AuditStatus = "pending"
	AuditRunning   AuditStatus = "running"
	AuditCompleted AuditStatus = "completed"
	AuditFailed    AuditStatus = "failed"
	AuditCancelled AuditStatus = "cancelled"
)

// Audit pipeline stages, persisted for progress display.
const (
	StageQueued    = "queued"
	StageCrawling  = "crawling"
	StageChecking  = "checking"
	StageAI        = "ai_analysis"
	StageReporting = "reporting"
	StageDone      = "done"
)
