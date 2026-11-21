// Package lexicon holds the per-language word lists for the legal rules (D-1 stage 2, specs/06-detection-and-ai.md).
// One file per language; all words are lowercase.
package lexicon

import "strings"

// Kind is a kind of legal page.
type Kind string

const (
	LegalNotice Kind = "legal_notice"
	Privacy     Kind = "privacy"
	Terms       Kind = "terms"
	Withdrawal  Kind = "withdrawal"
)

// Section is one R2 section marker: its German name and the words that signal it.
type Section struct {
	NameDe string
	Words  []string
}

// List is the word list of one language.
type List struct {
	Links         map[Kind][]string // matched against link text and URL path
	Sections      []Section         // R2: exactly six, in spec order
	Withdrawal    []string          // R5 withdrawal vocabulary
	Periods       []string          // R5 period phrases
	Exempt        []string          // R5 exempt-goods keywords
	EmailLabels   []string          // R1
	AddressLabels []string          // R1
	NameLabels    []string          // R1
}

var lists = map[string]List{"de": de, "en": en}

// For returns the list of a language ("de", "en").
func For(lang string) (List, bool) {
	l, ok := lists[lang]
	return l, ok
}

// Hits counts the distinct words of l (link words, labels, section words, withdrawal words) found in text.
func Hits(l List, text string) int {
	t := strings.ToLower(text)
	seen := map[string]bool{}
	add := func(ws []string) {
		for _, w := range ws {
			if !seen[w] && containsWord(t, w) {
				seen[w] = true
			}
		}
	}
	for _, ws := range l.Links {
		add(ws)
	}
	for _, s := range l.Sections {
		add(s.Words)
	}
	add(l.Withdrawal)
	add(l.AddressLabels)
	add(l.NameLabels)
	return len(seen)
}

// containsWord reports whether w occurs in t at a word start (so "agb" does not match inside "tagblatt").
func containsWord(t, w string) bool {
	for i := 0; ; {
		j := strings.Index(t[i:], w)
		if j < 0 {
			return false
		}
		k := i + j
		if k == 0 || !isLetter(t[k-1]) {
			return true
		}
		i = k + 1
	}
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 0x80 }
