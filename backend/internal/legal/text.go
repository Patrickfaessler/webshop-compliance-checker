// Package legal holds the helpers shared by the legal rules R1, R2, R5: finding legal pages among the crawled
// pages, their text, their language and field validators (specs/rules/R*.md).
package legal

import (
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/fixes"
	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

// boilerplate subtrees left out of the content text (R1 Logic 3).
var boilerplate = []string{"header", "nav", "footer", "aside", "form"}

// ContentText is the visible text without header, nav, footer, aside and forms, whitespace collapsed. If that
// leaves nothing, it is the full visible text.
func ContentText(p rules.Page) string {
	doc := domutil.Parse(p.HTML)
	var b strings.Builder
	domutil.Walk(doc, func(n *html.Node) bool {
		if domutil.IsElement(n, "script", "style", "noscript", "template", "svg", "head") || domutil.IsElement(n, boilerplate...) {
			return false
		}
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		return true
	})
	if t := collapse(b.String()); t != "" {
		return t
	}
	return collapse(domutil.VisibleText(doc))
}

// Embedded reports whether the content (outside header/nav/footer/aside/form) holds an iframe: the legal text may
// be loaded into it and is then not in the captured page (R1-11, R2-8).
func Embedded(p rules.Page) bool {
	found := false
	walkContent(domutil.Parse(p.HTML), false, func(n *html.Node) {
		if domutil.IsElement(n, "iframe") {
			found = true
		}
	})
	return found
}

// FullText is the whole visible text of the page, whitespace collapsed.
func FullText(p rules.Page) string { return collapse(domutil.VisibleText(domutil.Parse(p.HTML))) }

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// Language picks the word list for a page: the stopword detector on the content text when sure, else <html lang>
// if de/en, else the language whose lexicon has ≥ 2 hits and more than the other, else the detector on the whole
// page. ok is false when none applies.
// The content text decides first, so a German menu around an English policy does not (R2-7).
func Language(p rules.Page) (string, bool) {
	text := ContentText(p)
	if l, sure := fixes.DetectLang(text); sure {
		if _, ok := lexicon.For(l); ok {
			return l, true
		}
		return l, false
	}
	if l := htmlLang(p.HTML); l == "de" || l == "en" {
		return l, true
	}
	de, _ := lexicon.For("de")
	en, _ := lexicon.For("en")
	hd, he := lexicon.Hits(de, text), lexicon.Hits(en, text)
	switch {
	case hd >= 2 && hd > he:
		return "de", true
	case he >= 2 && he > hd:
		return "en", true
	}
	if l, sure := fixes.DetectLang(FullText(p)); sure { // a terse content text: the whole page may still tell
		_, ok := lexicon.For(l)
		return l, ok
	}
	return "", false
}

// htmlLang is the primary subtag of <html lang>, lowercased; "" if absent.
func htmlLang(src string) string {
	var lang string
	domutil.Walk(domutil.Parse(src), func(n *html.Node) bool {
		if domutil.IsElement(n, "html") {
			lang, _ = domutil.Attr(n, "lang")
			return false
		}
		return true
	})
	lang = strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexAny(lang, "-_"); i >= 0 {
		lang = lang[:i]
	}
	return lang
}
