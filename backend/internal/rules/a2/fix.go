package a2

import (
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/fixes"
	"compliancechecker/internal/rules"
)

const fixText = "Beschreibe das Bild in wenigen Worten, z. B. mit dem Produktnamen."

// fixFor builds the A2 fix for an image element (spec A2 "Fix templates"); img may be nil.
func fixFor(img *html.Node) *rules.Fix {
	if name := productName(img); name != "" {
		return &rules.Fix{TextDe: fixText, Code: fixes.Fill(`alt="{{productName}}"`, map[string]string{"productName": name})}
	}
	return &rules.Fix{TextDe: fixText, Code: `alt="Beschreibung des Bildes"`,
		NoteDe: `Beschreibung bitte anpassen. Rein dekorative Bilder bekommen alt="".`}
}

// productName is the text of the nearest heading or [itemprop=name] in the image's card: the closest of at
// most 4 ancestors that contains one, stopping before body/main so a page heading is not used. At most 80 runes.
func productName(img *html.Node) string {
	if img == nil {
		return ""
	}
	n := img.Parent
	for depth := 0; n != nil && depth < 4; depth, n = depth+1, n.Parent {
		if domutil.IsElement(n, "body", "main", "html", "header", "footer", "nav") {
			return ""
		}
		var found string
		domutil.Walk(n, func(c *html.Node) bool {
			if found != "" {
				return false
			}
			if ip, _ := domutil.Attr(c, "itemprop"); domutil.IsElement(c, "h1", "h2", "h3", "h4") || (c.Type == html.ElementNode && ip == "name") {
				found = strings.Join(strings.Fields(domutil.Text(c)), " ")
				return false
			}
			return true
		})
		if found != "" {
			if r := []rune(found); len(r) > 80 {
				found = string(r[:80])
			}
			return found
		}
	}
	return ""
}

// imgBySrc finds the first img with the src of the axe snippet in the page DOM.
func imgBySrc(doc *html.Node, snippet string) *html.Node {
	t := fixes.Tag(snippet)
	if t == nil || !domutil.IsElement(t, "img") {
		return nil
	}
	src, ok := domutil.Attr(t, "src")
	if !ok {
		return nil
	}
	var hit *html.Node
	domutil.Walk(doc, func(n *html.Node) bool {
		if hit != nil {
			return false
		}
		if s, ok := domutil.Attr(n, "src"); ok && s == src && domutil.IsElement(n, "img") {
			hit = n
			return false
		}
		return true
	})
	return hit
}
