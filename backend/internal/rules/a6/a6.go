// Package a6 implements rule A6 "Page language declared".
// Spec: specs/rules/A6-html-lang.md
package a6

import (
	"context"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/rules"
)

// langTag accepts a well-formed BCP 47 shape: primary subtag of 2-3 letters,
// followed by subtags of 1-8 alphanumerics (A6-1, A6-4). It does not check the IANA registry.
var langTag = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{1,8})*$`)

// Rule is the A6 check.
type Rule struct{}

// New returns the rule.
func New() Rule { return Rule{} }

// Meta implements rules.Rule.
func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID:         "A6",
		Name:       "Page language declared",
		Area:       "accessibility",
		Severity:   rules.Notice,
		AIFallback: false,
		SummaryDe:  "Die Seite gibt ihre Sprache nicht an (lang-Attribut fehlt oder ist ungültig).",
	}
}

// Check implements rules.Rule. Fully deterministic (decided_by: structural).
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	res := rules.Result{DecidedBy: rules.DecidedStructural}

	checked := 0
	for _, p := range in.Pages {
		htmlEl, ok := findHTMLElement(p.HTML)
		if !ok {
			continue // not an HTML document; ignored
		}
		checked++
		lang, _ := attr(htmlEl, "lang")
		if !valid(lang) {
			res.Findings = append(res.Findings, rules.Finding{
				PageURL:  p.URL,
				Selector: "html",
				Snippet:  openingTag(htmlEl),
			})
		}
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

func valid(lang string) bool {
	return langTag.MatchString(strings.TrimSpace(lang))
}

func findHTMLElement(src string) (*html.Node, bool) {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return nil, false
	}
	var found *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if found != nil {
			return
		}
		if n.Type == html.ElementNode && n.Data == "html" {
			found = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return found, found != nil
}

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// openingTag rebuilds the opening tag of n as shown in the finding snippet.
func openingTag(n *html.Node) string {
	var b strings.Builder
	b.WriteString("<")
	b.WriteString(n.Data)
	for _, a := range n.Attr {
		b.WriteString(" ")
		b.WriteString(a.Key)
		b.WriteString(`="`)
		b.WriteString(a.Val)
		b.WriteString(`"`)
	}
	b.WriteString(">")
	return b.String()
}
