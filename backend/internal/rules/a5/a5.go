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
		SummaryDe:     "Nicht alle Bedienelemente sind mit der Tastatur erreichbar oder der Fokus ist nicht sichtbar.",
		ExplanationDe: "Nicht jeder kann eine Maus benutzen. Alle Bedienelemente müssen mit der Tab-Taste erreichbar sein und man muss sehen, wo man gerade ist.",
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
			presses = max(presses, k.Presses) // = the limit, or fewer when the probe ran out of time (A5-9)
		}
		for _, g := range []struct {
			sub   string
			nodes []rules.KeyboardNode
		}{{"unreachable", k.Unreachable}, {"no-focus-style", k.NoFocusStyle}, {"keyboard-trap", k.Traps}, {"not-focusable", k.NotFocusable}} {
			for _, n := range g.nodes {
				findings = append(findings, rules.Finding{PageURL: p.URL, Selector: n.Selector, Snippet: n.Snippet, Sub: g.sub, Fix: fixFor(g.sub, n.Selector)})
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

// fixFor is the A5 fix per sub-rule (spec A5 "Fix templates").
func fixFor(sub, selector string) *rules.Fix {
	switch sub {
	case "no-focus-style":
		return &rules.Fix{TextDe: "Mach sichtbar, welches Element den Fokus hat.",
			// CSS, not HTML: the selector goes in unchanged (it is only ever shown as text)
			Code: selector + ":focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }"}
	case "not-focusable":
		return &rules.Fix{TextDe: `Verwende ein <button>-Element. Alternativ role="button" und tabindex="0" ergänzen und Enter/Leertaste per Skript bedienen.`,
			Code: `<button type="button">…</button>`}
	case "unreachable":
		return &rules.Fix{TextDe: `Mach das Element mit der Tab-Taste erreichbar: entferne tabindex="-1" oder Skripte, die den Fokus wegnehmen.`}
	default: // keyboard-trap
		return &rules.Fix{TextDe: "Sorge dafür, dass man das Element mit der Tab-Taste wieder verlassen kann."}
	}
}
