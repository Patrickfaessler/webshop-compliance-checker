// Package rules defines the contract every check implements.
// Spec: specs/07-architecture.md (Rule interface), specs/04-scoring.md (states, severity).
package rules

import "context"

// State is the outcome of a rule for a scan. Spec: 04-scoring.md.
type State string

const (
	Passed        State = "passed"
	Failed        State = "failed"
	NotApplicable State = "not_applicable"
	NotAssessable State = "not_assessable"
)

// Severity of a rule. Spec: 04-scoring.md.
type Severity string

const (
	Critical Severity = "critical"
	Warning  Severity = "warning"
	Notice   Severity = "notice"
)

// Weight returns the scoring weight g of the severity (S-1).
func (s Severity) Weight() int {
	switch s {
	case Critical:
		return 3
	case Warning:
		return 2
	case Notice:
		return 1
	default:
		return 0
	}
}

// DecidedBy records which detection stage produced a result. Spec: 06-detection-and-ai.md (D-1).
type DecidedBy string

const (
	DecidedStructural DecidedBy = "structural"
	DecidedDictionary DecidedBy = "dictionary"
	DecidedPattern    DecidedBy = "pattern"
	DecidedAI         DecidedBy = "ai"
)

// Meta describes a rule.
type Meta struct {
	ID         string // "A1", "R6"
	Name       string
	Area       string // "accessibility" or "legal"
	Severity   Severity
	AIFallback bool   // whether the rule spec allows the Claude fallback
	SummaryDe  string // one-line German summary shown in the findings list
}

// Page is one crawled, rendered page.
type Page struct {
	URL  string
	HTML string     // rendered DOM serialised
	Axe  *AxeReport // nil if axe-core did not run on this page (no browser)
}

// AxeReport is the subset of an axe-core run that rules need. Spec: specs/rules/A1..A3.
type AxeReport struct {
	Violations   []AxeResult
	Incomplete   []AxeResult
	Inapplicable []string // axe rule ids that found nothing to test on this page
}

// AxeResult is one axe rule with the elements it flagged.
type AxeResult struct {
	ID    string
	Nodes []AxeNode
}

// AxeNode is one flagged element.
type AxeNode struct {
	Selector string
	HTML     string
	Data     map[string]any // check data, e.g. contrast ratio and colours
}

// Input is what a rule receives. It grows as crawler features land
// (browser states, request logs, llm client); see specs/05-crawler.md.
type Input struct {
	Pages []Page
}

// Finding is one violation with its location. Spec: 02-requirements.md F8.
type Finding struct {
	PageURL  string
	Selector string
	Snippet  string
	Evidence string // quoted source text, required for AI-derived results (D-5)
	Sub      string // sub-rule name, e.g. "skipped-level" (A4); optional
}

// Result is the outcome of one rule for one scan.
type Result struct {
	State     State
	DecidedBy DecidedBy
	Findings  []Finding
}

// Rule is implemented by every check module (N5).
type Rule interface {
	Meta() Meta
	Check(ctx context.Context, in Input) (Result, error)
}
