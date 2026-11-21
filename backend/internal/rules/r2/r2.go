// Package r2 implements rule R2 "Privacy notice present" (specs/rules/R2-privacy-notice.md). Presence only;
// deterministic version without the AI fallback.
package r2

import (
	"context"
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/legal"
	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

// Rule is R2.
type Rule struct{}

// New returns R2.
func New() Rule { return Rule{} }

// Meta implements rules.Rule.
func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID:            "R2",
		Name:          "Privacy notice present",
		NameDe:        "Datenschutzerklärung vorhanden",
		Area:          "legal",
		Severity:      rules.Warning,
		AIFallback:    true,
		SummaryDe:     "Die Datenschutzerklärung fehlt oder ihr fehlen Pflichtabschnitte.",
		ExplanationDe: "Wer personenbezogene Daten verarbeitet, muss darüber informieren: wer verantwortlich ist, wozu, wie lange und welche Rechte Kunden haben.",
	}
}

const (
	notePassed    = "Nur geprüft, ob die Abschnitte vorhanden sind, nicht ob der Inhalt stimmt."
	noteNotLoaded = "Die Datenschutzerklärung ist verlinkt, konnte aber nicht geladen werden."
	fixText       = "Ergänze die fehlenden Abschnitte der Datenschutzerklärung."
	fixTextNoPage = "Lege eine Datenschutzerklärung an und verlinke sie im Footer jeder Seite."
	fixNote       = "Ein Datenschutz-Generator oder eine Fachperson kann helfen."
	minChars      = 500 // R2-5
	minSections   = 5   // of 6
	minTextHits   = 2
)

// Check implements rules.Rule.
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	if len(in.Pages) == 0 {
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedStructural}, nil
	}
	de, _ := lexicon.For("de") // the German section names and order are the same in every list
	link := legal.FindLinked(in, lexicon.Privacy)
	switch {
	case !link.Found:
		return failed(in.Pages[0].URL, "no-privacy-page", fixTextNoPage, de.Sections, rules.DecidedStructural), nil
	case !link.Loaded:
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedStructural, NoteDe: noteNotLoaded}, nil
	}
	p := *link.Page
	lang, ok := legal.Language(p)
	if !ok {
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedDictionary, NoteDe: legal.NoteLanguage}, nil
	}
	l, _ := lexicon.For(lang)
	text := strings.ToLower(legal.ContentText(p))
	embedded := func() rules.Result {
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedStructural, NoteDe: legal.NoteEmbedded}
	}
	if len([]rune(text)) <= minChars {
		if legal.Embedded(p) {
			return embedded(), nil // R2-8
		}
		return failed(p.URL, "incomplete-sections", fixText, de.Sections, rules.DecidedDictionary), nil
	}
	heads := headings(p)
	var missing []lexicon.Section
	for i, s := range l.Sections {
		if !present(s, heads, text) {
			missing = append(missing, de.Sections[i])
		}
	}
	if len(l.Sections)-len(missing) < minSections {
		if legal.Embedded(p) {
			return embedded(), nil
		}
		return failed(p.URL, "incomplete-sections", fixText, missing, rules.DecidedDictionary), nil
	}
	return rules.Result{State: rules.Passed, DecidedBy: rules.DecidedDictionary, NoteDe: notePassed}, nil
}

func failed(url, sub, text string, missing []lexicon.Section, by rules.DecidedBy) rules.Result {
	var lines []string
	for _, s := range missing {
		lines = append(lines, "- "+s.NameDe)
	}
	return rules.Result{State: rules.Failed, DecidedBy: by, Findings: []rules.Finding{{
		PageURL: url, Sub: sub, Fix: &rules.Fix{TextDe: text, Code: strings.Join(lines, "\n"), NoteDe: fixNote},
	}}}
}

// present: one heading containing a section word, or at least two hits in the content text (R2 Logic 4).
func present(s lexicon.Section, heads []string, text string) bool {
	for _, h := range heads {
		for _, w := range s.Words {
			if lexicon.ContainsWord(h, w) {
				return true
			}
		}
	}
	hits := 0
	for _, w := range s.Words {
		for i := 0; ; {
			j := strings.Index(text[i:], w)
			if j < 0 {
				break
			}
			k := i + j
			if k == 0 || !isLetter(text[k-1]) {
				hits++
			}
			i = k + len(w)
		}
	}
	return hits >= minTextHits
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 0x80 }

// headings returns the lowercased heading texts of the content (h1–h6, dt, a p whose only element child is
// strong/b).
func headings(p rules.Page) []string {
	var out []string
	domutil.Walk(domutil.Parse(p.HTML), func(n *html.Node) bool {
		if domutil.IsElement(n, "header", "nav", "footer", "aside", "form", "script", "style") {
			return false
		}
		if domutil.IsElement(n, "h1", "h2", "h3", "h4", "h5", "h6", "dt") || (domutil.IsElement(n, "p") && onlyStrong(n)) {
			t := strings.ToLower(strings.Join(strings.Fields(domutil.Text(n)), " "))
			out = append(out, t) // "1. VERANTWORTLICHER": word-start matching ignores the numbering
			return false
		}
		return true
	})
	return out
}

func onlyStrong(p *html.Node) bool {
	var el *html.Node
	for c := p.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == html.ElementNode:
			if el != nil {
				return false
			}
			el = c
		case c.Type == html.TextNode && strings.TrimSpace(c.Data) != "":
			return false
		}
	}
	return el != nil && domutil.IsElement(el, "strong", "b")
}
