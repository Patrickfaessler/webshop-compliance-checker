// Package a4 implements rule A4 "Heading structure". Spec: specs/rules/A4-heading-structure.md
package a4

import (
	"context"
	"strconv"
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/rules"
)

type Rule struct{}

func New() Rule { return Rule{} }

func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID: "A4", Name: "Heading structure", NameDe: "Überschriftenstruktur", Area: "accessibility", Severity: rules.Warning,
		SummaryDe: "Die Überschriften der Seite sind nicht sauber gegliedert.",
	}
}

type heading struct {
	node  *html.Node
	level int
}

// Check is fully deterministic (decided_by: structural). Sub-rules: no-h1, multi-h1, skipped-level, empty.
// Limitation: hiding is detected from markup only (hidden, aria-hidden, inline style), not from stylesheets.
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	res := rules.Result{DecidedBy: rules.DecidedStructural}
	checked := 0
	for _, p := range in.Pages {
		doc := domutil.Parse(p.HTML)
		if doc == nil || !hasHTMLElement(doc) {
			continue
		}
		checked++
		res.Findings = append(res.Findings, checkPage(p, doc)...)
	}
	switch {
	case checked == 0:
		res.State = rules.NotAssessable
	case len(res.Findings) > 0:
		res.State = rules.Failed
	default:
		res.State = rules.Passed
	}
	return res, nil
}

func hasHTMLElement(doc *html.Node) bool {
	found := false
	domutil.Walk(doc, func(n *html.Node) bool {
		if domutil.IsElement(n, "html") {
			found = true
		}
		return !found
	})
	return found
}

func checkPage(p rules.Page, doc *html.Node) []rules.Finding {
	var hs []heading
	domutil.Walk(doc, func(n *html.Node) bool {
		if n.Type == html.ElementNode && domutil.Hidden(n) {
			return false
		}
		if lvl, ok := level(n); ok {
			hs = append(hs, heading{n, lvl})
		}
		return true
	})

	var out []rules.Finding
	add := func(n *html.Node, sub string) {
		out = append(out, rules.Finding{PageURL: p.URL, Selector: domutil.Selector(n), Snippet: domutil.OpeningTag(n), Sub: sub})
	}

	h1s := 0
	for i, h := range hs {
		if h.level == 1 {
			h1s++
			if h1s > 1 {
				add(h.node, "multi-h1")
			}
		}
		if i > 0 && h.level > hs[i-1].level+1 {
			add(h.node, "skipped-level")
		}
		if isEmpty(h.node) {
			add(h.node, "empty")
		}
	}
	if h1s == 0 {
		out = append(out, rules.Finding{
			PageURL: p.URL, Selector: "html", Snippet: "Keine h1-Überschrift gefunden", Sub: "no-h1",
		})
	}
	return out
}

// level returns the heading level of n, from h1-h6 or role=heading with aria-level (default 2).
func level(n *html.Node) (int, bool) {
	if n.Type != html.ElementNode {
		return 0, false
	}
	if len(n.Data) == 2 && n.Data[0] == 'h' && n.Data[1] >= '1' && n.Data[1] <= '6' {
		return int(n.Data[1] - '0'), true
	}
	if role, _ := domutil.Attr(n, "role"); strings.EqualFold(strings.TrimSpace(role), "heading") {
		if v, ok := domutil.Attr(n, "aria-level"); ok {
			if l, err := strconv.Atoi(strings.TrimSpace(v)); err == nil && l >= 1 && l <= 6 {
				return l, true
			}
		}
		return 2, true
	}
	return 0, false
}

// isEmpty: no text, no image with alt text and no aria-label.
func isEmpty(n *html.Node) bool {
	if strings.TrimSpace(domutil.Text(n)) != "" {
		return false
	}
	if v, _ := domutil.Attr(n, "aria-label"); strings.TrimSpace(v) != "" {
		return false
	}
	hasAlt := false
	domutil.Walk(n, func(c *html.Node) bool {
		if domutil.IsElement(c, "img") {
			if alt, _ := domutil.Attr(c, "alt"); strings.TrimSpace(alt) != "" {
				hasAlt = true
			}
		}
		return true
	})
	return !hasAlt
}
