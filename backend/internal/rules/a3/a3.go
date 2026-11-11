// Package a3 implements rule A3 "Form labels". Spec: specs/rules/A3-form-labels.md
package a3

import (
	"context"
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/axecheck"
)

var axeIDs = []string{"label", "select-name", "aria-input-field-name"}

type Rule struct{}

func New() Rule { return Rule{} }

func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID: "A3", Name: "Form labels", NameDe: "Beschriftung von Formularfeldern", Area: "accessibility", Severity: rules.Critical,
		SummaryDe: "Ein Formularfeld hat keine Beschriftung, die Screenreader vorlesen können.",
	}
}

// Check combines axe-core with a DOM check: axe accepts a placeholder as a name, but the spec (A3-2)
// does not, because it disappears while typing and is often too faint.
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	res := axecheck.Check(axeIDs, in)
	for _, p := range in.Pages {
		res.Findings = append(res.Findings, placeholderOnly(p)...)
	}
	if len(res.Findings) > 0 {
		res.State = rules.Failed
	}
	return res, nil
}

func placeholderOnly(p rules.Page) []rules.Finding {
	doc := domutil.Parse(p.HTML)

	forLabels := map[string]bool{} // ids targeted by <label for=...> with text
	domutil.Walk(doc, func(n *html.Node) bool {
		if domutil.IsElement(n, "label") {
			if id, ok := domutil.Attr(n, "for"); ok && strings.TrimSpace(domutil.Text(n)) != "" {
				forLabels[id] = true
			}
		}
		return true
	})

	var out []rules.Finding
	domutil.Walk(doc, func(n *html.Node) bool {
		if !isTextField(n) || domutil.Hidden(n) {
			return true
		}
		if ph, _ := domutil.Attr(n, "placeholder"); strings.TrimSpace(ph) == "" {
			return true // no placeholder: plain missing labels are axe's job
		}
		if hasName(n, forLabels) {
			return true
		}
		out = append(out, rules.Finding{
			PageURL: p.URL, Selector: domutil.Selector(n), Snippet: domutil.OpeningTag(n), Sub: "placeholder-only",
		})
		return true
	})
	return out
}

func isTextField(n *html.Node) bool {
	if domutil.IsElement(n, "textarea") {
		return true
	}
	if !domutil.IsElement(n, "input") {
		return false
	}
	t, _ := domutil.Attr(n, "type")
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "", "text", "search", "email", "tel", "url", "password", "number":
		return true
	}
	return false
}

// hasName reports an accessible name other than the placeholder.
func hasName(n *html.Node, forLabels map[string]bool) bool {
	for _, key := range []string{"aria-label", "aria-labelledby", "title"} {
		if v, _ := domutil.Attr(n, key); strings.TrimSpace(v) != "" {
			return true
		}
	}
	if id, ok := domutil.Attr(n, "id"); ok && forLabels[id] {
		return true
	}
	for p := n.Parent; p != nil; p = p.Parent { // wrapping <label>
		if domutil.IsElement(p, "label") && strings.TrimSpace(domutil.Text(p)) != "" {
			return true
		}
	}
	return false
}
