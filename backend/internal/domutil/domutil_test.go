package domutil

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

// Spec: RP-3b, RP-A6 — ids are written as escaped CSS identifiers, like CSS.escape.
func TestCSSEscapeIdent(t *testing.T) {
	cases := map[string]string{
		"main":   "main",
		"a.b":    `a\.b`,
		"x:y[0]": `x\:y\[0\]`,
		"1x":     `\31 x`,
		"-1":     `-\31 `,
		"-":      `\-`,
		"a b":    `a\ b`,
		"ü-ok_1": "ü-ok_1",
		"a\x00b": "a�b",
	}
	for in, want := range cases {
		if got := CSSEscapeIdent(in); got != want {
			t.Errorf("CSSEscapeIdent(%q) = %q, want %q", in, got, want)
		}
	}
}

// Spec: RP-3b — Selector uses the escaped id.
func TestSelectorEscapesID(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<html><body><div id="a.b"><img src="x.png"></div></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	var img *html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			img = n
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if got, want := Selector(img), `#a\.b > img`; got != want {
		t.Errorf("Selector = %q, want %q", got, want)
	}
}
