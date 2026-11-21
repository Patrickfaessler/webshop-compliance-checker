// Package fixes builds the suggested fixes of the rules from their templates (D-3, F8).
// Values taken from the page are untrusted: Fill HTML-escapes every value it inserts.
package fixes

import (
	"html"
	"strings"
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
