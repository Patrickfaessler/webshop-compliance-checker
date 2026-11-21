// Package a5 implements rule A5 "Keyboard operation". Spec: specs/rules/A5-keyboard.md
// The crawler's keyboard probe does the work (internal/crawler/keyboard.go); this rule maps its reports.
package a5

import (
	"context"
	"fmt"

	"compliancechecker/internal/rules"
)

type Rule struct{}

func New() Rule { return Rule{} }

func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID: "A5", Name: "Keyboard operation", NameDe: "Bedienung per Tastatur", Area: "accessibility", Severity: rules.Critical,
		SummaryDe: "Nicht alle Bedienelemente sind mit der Tastatur erreichbar oder der Fokus ist nicht sichtbar.",
	}
}

// Check follows the A5 result mapping: findings → failed; nothing probed → not_assessable;
// limit reached somewhere without findings → passed with a note (A5-7).
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	var findings []rules.Finding
	probed, capped, presses := 0, 0, 0
	for _, p := range in.Pages {
		k := p.Keyboard
		if k == nil {
			continue
		}
		probed++
		if !k.Completed && len(k.Traps) == 0 {
			capped++
			presses = max(presses, k.MaxPresses)
		}
		for _, g := range []struct {
			sub   string
			nodes []rules.KeyboardNode
		}{{"unreachable", k.Unreachable}, {"no-focus-style", k.NoFocusStyle}, {"keyboard-trap", k.Traps}, {"not-focusable", k.NotFocusable}} {
			for _, n := range g.nodes {
				findings = append(findings, rules.Finding{PageURL: p.URL, Selector: n.Selector, Snippet: n.Snippet, Sub: g.sub})
			}
		}
	}
	switch {
	case len(findings) > 0:
		return rules.Result{State: rules.Failed, DecidedBy: rules.DecidedStructural, Findings: findings}, nil
	case probed == 0:
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedStructural}, nil
	case capped > 0:
		return rules.Result{State: rules.Passed, DecidedBy: rules.DecidedStructural, NoteDe: fmt.Sprintf(
			"Erreichbarkeit nur für die ersten %d Bedienelemente geprüft (Limit erreicht auf %d von %d geprüften Seiten).",
			presses, capped, probed)}, nil
	}
	return rules.Result{State: rules.Passed, DecidedBy: rules.DecidedStructural}, nil
}
