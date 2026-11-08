// Package a1 implements rule A1 "Text contrast". Spec: specs/rules/A1-contrast.md
package a1

import (
	"context"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/axecheck"
)

type Rule struct{}

func New() Rule { return Rule{} }

func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID: "A1", Name: "Text contrast", NameDe: "Textkontrast", Area: "accessibility", Severity: rules.Warning,
		SummaryDe: "Text und Hintergrund haben zu wenig Kontrast.",
	}
}

// Check relies on axe-core's color-contrast rule (4.5:1, large text 3:1).
// axe "incomplete" results (background images, overlaps) give not_assessable, never a violation (A1 Result mapping).
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	return axecheck.Check([]string{"color-contrast"}, in), nil
}
