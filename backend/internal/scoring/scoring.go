// Package scoring implements specs/04-scoring.md. Pure functions only (S-4).
package scoring

import (
	"math"

	"compliancechecker/internal/rules"
)

// Outcome is one rule's severity and state for a scan.
type Outcome struct {
	Severity rules.Severity
	State    rules.State
}

// Score returns the weighted share of passed rules in percent (S-1).
// ok is false if no rule is passed or failed ("nicht berechenbar").
// not_applicable and not_assessable are ignored.
// Spec: S-1, S-4.
func Score(outcomes []Outcome) (score int, ok bool) {
	passed, total := 0, 0
	for _, o := range outcomes {
		w := o.Severity.Weight()
		switch o.State {
		case rules.Passed:
			passed += w
			total += w
		case rules.Failed:
			total += w
		}
	}
	if total == 0 {
		return 0, false
	}
	return int(math.Round(100 * float64(passed) / float64(total))), true
}
