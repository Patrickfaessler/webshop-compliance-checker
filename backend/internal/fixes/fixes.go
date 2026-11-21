// Package fixes builds the suggested fixes of the rules from their templates (D-3, F8).
// Values taken from the page are untrusted: Fill HTML-escapes every value it inserts.
package fixes

import (
	"html"
	"strings"

	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Attr HTML-escapes s (&, <, >, ", ') so it is safe inside markup and attribute values.
func Attr(s string) string { return html.EscapeString(s) }

// Fill replaces each {{key}} in tmpl with the escaped value. Unknown keys stay as they are.
func Fill(tmpl string, vals map[string]string) string {
	pairs := make([]string, 0, 2*len(vals))
	for k, v := range vals {
		pairs = append(pairs, "{{"+k+"}}", Attr(v))
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}

// Tag parses the first element of an HTML snippet (e.g. axe's node HTML) so its attributes can be read.
// It returns nil if the snippet holds no element.
func Tag(snippet string) *xhtml.Node {
	nodes, err := xhtml.ParseFragment(strings.NewReader(snippet), &xhtml.Node{Type: xhtml.ElementNode, Data: "body", DataAtom: atom.Body})
	if err != nil {
		return nil
	}
	for _, n := range nodes {
		if n.Type == xhtml.ElementNode {
			return n
		}
	}
	return nil
}
