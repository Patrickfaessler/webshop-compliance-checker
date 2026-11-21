// Package r5 implements rule R5 "Terms and withdrawal information present"
// (specs/rules/R5-terms-and-withdrawal-info.md). Presence only; deterministic version without the AI fallback.
package r5

import (
	"context"
	"strings"

	"compliancechecker/internal/legal"
	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

// Rule is R5.
type Rule struct{}

// New returns R5.
func New() Rule { return Rule{} }

// Meta implements rules.Rule.
func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID:            "R5",
		Name:          "Terms and withdrawal information present",
		NameDe:        "AGB und Widerrufsinformation vorhanden",
		Area:          "legal",
		Severity:      rules.Warning,
		AIFallback:    true,
		SummaryDe:     "AGB oder Widerrufsbelehrung fehlen.",
		ExplanationDe: "Kunden müssen vor dem Kauf lesen können, unter welchen Bedingungen sie kaufen und wie sie innerhalb von 14 Tagen widerrufen können.",
	}
}

const (
	noteB2C       = "Annahme: Der Shop verkauft an Verbraucher."
	notePeriod    = "Widerrufsfrist nicht gefunden – bitte manuell prüfen."
	noteNotLoaded = "Die AGB bzw. die Widerrufsbelehrung sind verlinkt, konnten aber nicht geladen werden."
	noteExempt    = "Ausnahmen vom Widerrufsrecht prüfen."
	window        = 300 // characters between withdrawal word and period phrase
	fixNoTerms    = "Lege AGB an und verlinke sie im Footer und im Bestellprozess."
	fixNoWdText   = "Ergänze eine Widerrufsbelehrung mit der 14-tägigen Frist."
	fixNoWdCode   = "Widerrufsbelehrung\n" +
		"Widerrufsrecht: Sie haben das Recht, binnen vierzehn Tagen ohne Angabe von Gründen diesen Vertrag zu widerrufen.\n" +
		"Widerrufsfrist: 14 Tage ab dem Tag, an dem Sie oder ein von Ihnen benannter Dritter die Waren in Besitz genommen haben.\n" +
		"Ausübung: Senden Sie uns ({{Name}}, {{Anschrift}}, {{E-Mail}}) eine eindeutige Erklärung.\n" +
		"Folgen des Widerrufs: {{Rückzahlung und Rücksendung}}"
	fixNoWdNote = "Link-Text im Footer: „Widerrufsbelehrung“. Den vollständigen Mustertext finden Sie in Anhang I der Richtlinie 2011/83/EU."
)

// what one page says about withdrawal
type finding int

const (
	none      finding = iota // no withdrawal word
	wordsOnly                // withdrawal words, no period nearby
	info                     // withdrawal word within window of a period phrase
	unknown                  // language not supported
)

// Check implements rules.Rule.
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	if len(in.Pages) == 0 {
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedStructural}, nil
	}
	startURL := in.Pages[0].URL
	terms := legal.FindLinked(in, lexicon.Terms)
	wds := legal.FindAllLinked(in, lexicon.Withdrawal)
	var wd legal.Link
	if len(wds) > 0 {
		wd = wds[0]
	}
	res := rules.Result{DecidedBy: rules.DecidedDictionary}
	notes := []string{noteB2C}

	if !terms.Found {
		res.Findings = append(res.Findings, rules.Finding{PageURL: startURL, Sub: "no-terms", Fix: &rules.Fix{TextDe: fixNoTerms}})
	}

	// withdrawal information: every crawled withdrawal candidate, then the terms; the best result wins (R5-8)
	var pages []*rules.Page
	seen := map[*rules.Page]bool{}
	for _, l := range append(wds, terms) {
		if l.Loaded && !seen[l.Page] {
			seen[l.Page] = true
			pages = append(pages, l.Page)
		}
	}
	best, sawUnknown, exempt, embedded := none, false, false, false
	for _, p := range pages {
		embedded = embedded || legal.Embedded(*p)
		got, ex := scan(*p)
		exempt = exempt || ex
		if got == unknown {
			sawUnknown = true
			continue
		}
		if got > best {
			best = got
		}
	}
	notLoaded := (wd.Found && !wd.Loaded) || (terms.Found && !terms.Loaded)
	assessable := true
	switch {
	case best == info:
	case best == wordsOnly:
		assessable = false
		notes = append(notes, notePeriod)
	case notLoaded:
		assessable = false
		notes = append(notes, noteNotLoaded)
	case embedded:
		assessable = false
		notes = append(notes, legal.NoteEmbedded)
	case sawUnknown:
		assessable = false
		notes = append(notes, legal.NoteLanguage)
	default:
		url := startURL
		if len(pages) > 0 {
			url = pages[len(pages)-1].URL
		}
		res.Findings = append(res.Findings, rules.Finding{PageURL: url, Sub: "no-withdrawal-info",
			Fix: &rules.Fix{TextDe: fixNoWdText, Code: fixNoWdCode, NoteDe: fixNoWdNote}})
	}
	if exempt {
		notes = append(notes, noteExempt)
	}
	switch {
	case len(res.Findings) > 0:
		res.State = rules.Failed
	case !assessable:
		res.State = rules.NotAssessable
	default:
		res.State = rules.Passed
	}
	res.NoteDe = strings.Join(notes, " ")
	return res, nil
}

// scan reads one page in its own language: what it says about withdrawal, and whether it names exempt goods.
func scan(p rules.Page) (finding, bool) {
	lang, ok := legal.Language(p)
	if !ok {
		return unknown, false
	}
	l, _ := lexicon.For(lang)
	text := strings.ToLower(legal.ContentText(p))
	exempt := false
	for _, w := range l.Exempt {
		if lexicon.ContainsWord(text, w) {
			exempt = true
		}
	}
	words, periods := positions(text, l.Withdrawal), positions(text, l.Periods)
	if len(words) == 0 {
		return none, exempt
	}
	for _, w := range words {
		for _, q := range periods {
			if d := w - q; d <= window && d >= -window {
				return info, exempt
			}
		}
	}
	return wordsOnly, exempt
}

// positions lists the byte offsets where a word of ws starts (at a word start) in text.
func positions(text string, ws []string) []int {
	var out []int
	for _, w := range ws {
		for i := 0; ; {
			j := strings.Index(text[i:], w)
			if j < 0 {
				break
			}
			k := i + j
			if k == 0 || !isLetter(text[k-1]) {
				out = append(out, k)
			}
			i = k + len(w)
		}
	}
	return out
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 0x80 }
