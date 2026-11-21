// Package a1 implements rule A1 "Text contrast". Spec: specs/rules/A1-contrast.md
package a1

import (
	"context"
	"math"
	"strconv"
	"strings"

	"compliancechecker/internal/fixes"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/axecheck"
)

type Rule struct{}

func New() Rule { return Rule{} }

func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID: "A1", Name: "Text contrast", NameDe: "Textkontrast", Area: "accessibility", Severity: rules.Warning,
		SummaryDe:     "Text und Hintergrund haben zu wenig Kontrast.",
		ExplanationDe: "Text und Hintergrund sind zu ähnlich in der Farbe. Menschen mit Sehschwäche können den Text schlecht lesen.",
	}
}

// Check relies on axe-core's color-contrast rule (4.5:1, large text 3:1).
// axe "incomplete" results (background images, overlaps) give not_assessable, never a violation (A1 Result mapping).
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	return axecheck.Check([]string{"color-contrast"}, in, func(_ rules.Page, n rules.AxeNode) *rules.Fix { return fixFor(n) }), nil
}

// fixFor builds the A1 fix from axe's color-contrast check data (spec A1 "Fix templates", A1-3).
func fixFor(n rules.AxeNode) *rules.Fix {
	fg, _ := n.Data["fgColor"].(string)
	bg, _ := n.Data["bgColor"].(string)
	min := 4.5
	if e, ok := n.Data["expectedContrastRatio"].(string); ok {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(e, ":1"), 64); err == nil && v > 0 {
			min = v
		}
	}
	hex, ratio, ok := fixes.SuggestColor(fg, bg, min)
	if !ok {
		return &rules.Fix{
			TextDe: "Wähle eine Textfarbe mit ausreichend Kontrast zum Hintergrund.",
			NoteDe: "Hintergrund nicht eindeutig (z. B. Bild) – Kontrast bitte selbst prüfen.",
		}
	}
	// one decimal, rounded down so the text never claims more than the colour reaches
	r := strings.Replace(strconv.FormatFloat(math.Floor(ratio*10)/10, 'f', 1, 64), ".", ",", 1)
	return &rules.Fix{
		TextDe: fixes.Fill("Ändere die Textfarbe von {{fg}} auf {{suggestedFg}} (Kontrast {{ratio}}:1).", map[string]string{"fg": fg, "suggestedFg": hex, "ratio": r}),
		Code:   fixes.Fill("color: {{suggestedFg}};", map[string]string{"suggestedFg": hex}),
	}
}
