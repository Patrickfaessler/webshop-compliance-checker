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

// svgFix is the fix for svg-img-alt: an SVG graphic is named with aria-label, not alt.
func svgFix() *rules.Fix {
	return &rules.Fix{TextDe: "Gib der Grafik einen kurzen Namen.", Code: `aria-label="Beschreibung der Grafik"`,
		NoteDe: "Beschreibung bitte anpassen."}
}

// productName is the text of the nearest heading or [itemprop=name] in the image's card. It looks at up to
// 4 ancestors and stops at one with more than one image (a grid, no longer one card) and at page regions
// (body, main, header, footer, nav). A name that comes before the image only counts inside the image's
// direct parent, so a section heading above a card is not taken. At most 80 runes.
func productName(img *html.Node) string {
	if img == nil {
		return ""
	}
	n := img.Parent
	for depth := 0; n != nil && depth < 4; depth, n = depth+1, n.Parent {
		if domutil.IsElement(n, "body", "main", "html", "header", "footer", "nav") || countImgs(n) > 1 {
			return ""
		}
		var found string
		after := false // document order: has the walk passed the image?
		domutil.Walk(n, func(c *html.Node) bool {
			if found != "" {
				return false
			}
			if c == img {
				after = true
				return true
			}
			if ip, _ := domutil.Attr(c, "itemprop"); domutil.IsElement(c, "h1", "h2", "h3", "h4") || (c.Type == html.ElementNode && ip == "name") {
				if after || depth == 0 {
					found = strings.Join(strings.Fields(domutil.Text(c)), " ")
				}
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

func countImgs(n *html.Node) int {
	c := 0
	domutil.Walk(n, func(x *html.Node) bool {
		if domutil.IsElement(x, "img") {
			c++
		}
		return true
	})
	return c
}

// imgBySrc finds the img with the src of the axe snippet in the page DOM. It returns nil when no image or
// more than one image has that src (lazy-load placeholders): then the image cannot be told apart.
func imgBySrc(doc *html.Node, snippet string) *html.Node {
	t := fixes.Tag(snippet)
	if t == nil || !domutil.IsElement(t, "img") {
		return nil
	}
	src, ok := domutil.Attr(t, "src")
	if !ok {
		return nil
	}
	var hits []*html.Node
	domutil.Walk(doc, func(n *html.Node) bool {
		if s, ok := domutil.Attr(n, "src"); ok && s == src && domutil.IsElement(n, "img") {
			hits = append(hits, n)
		}
		return true
	})
	if len(hits) != 1 {
		return nil
	}
	return hits[0]
}
