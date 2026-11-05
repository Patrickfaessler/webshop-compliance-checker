// Package domutil has small helpers for rules that inspect rendered HTML.
package domutil

import (
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// Parse parses an HTML document. Parsing never fails for real-world markup; on error it returns nil.
func Parse(src string) *html.Node {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return nil
	}
	return doc
}

// Walk visits n and its descendants in document order. If fn returns false the subtree is skipped.
func Walk(n *html.Node, fn func(*html.Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		Walk(c, fn)
	}
}

// Attr returns the attribute value and whether it is present.
func Attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val, true
		}
	}
	return "", false
}

// IsElement reports whether n is an element with one of the given tag names.
func IsElement(n *html.Node, tags ...string) bool {
	if n.Type != html.ElementNode {
		return false
	}
	for _, t := range tags {
		if n.Data == t {
			return true
		}
	}
	return false
}

// Text returns the concatenated text content of n.
func Text(n *html.Node) string {
	var b strings.Builder
	Walk(n, func(c *html.Node) bool {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
		return true
	})
	return b.String()
}

// Hidden reports whether n (or an ancestor) is hidden from all users by markup we can see without
// computed styles: `hidden` attribute, aria-hidden="true" or inline display:none / visibility:hidden.
// Limitation: stylesheet rules are not evaluated.
func Hidden(n *html.Node) bool {
	for p := n; p != nil; p = p.Parent {
		if p.Type != html.ElementNode {
			continue
		}
		if _, ok := Attr(p, "hidden"); ok {
			return true
		}
		if v, _ := Attr(p, "aria-hidden"); strings.EqualFold(strings.TrimSpace(v), "true") {
			return true
		}
		if st, ok := Attr(p, "style"); ok {
			s := strings.ToLower(strings.ReplaceAll(st, " ", ""))
			if strings.Contains(s, "display:none") || strings.Contains(s, "visibility:hidden") {
				return true
			}
		}
	}
	return false
}

// Selector builds a CSS selector path such as `html > body > main > h3:nth-of-type(2)`.
// An element with an id gets `#id` and ends the path.
func Selector(n *html.Node) string {
	var parts []string
	for p := n; p != nil && p.Type == html.ElementNode; p = p.Parent {
		if id, ok := Attr(p, "id"); ok && id != "" {
			parts = append(parts, "#"+id)
			break
		}
		seg := p.Data
		if p.Parent != nil && p.Data != "html" && p.Data != "body" && p.Data != "head" {
			idx, same := 0, 0
			for s := p.Parent.FirstChild; s != nil; s = s.NextSibling {
				if s.Type == html.ElementNode && s.Data == p.Data {
					same++
					if s == p {
						idx = same
					}
				}
			}
			if same > 1 {
				seg = fmt.Sprintf("%s:nth-of-type(%d)", p.Data, idx)
			}
		}
		parts = append(parts, seg)
	}
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, " > ")
}

// OpeningTag rebuilds the opening tag of n for finding snippets.
func OpeningTag(n *html.Node) string {
	var b strings.Builder
	b.WriteString("<" + n.Data)
	for _, a := range n.Attr {
		b.WriteString(fmt.Sprintf(` %s="%s"`, a.Key, a.Val))
	}
	b.WriteString(">")
	return b.String()
}
