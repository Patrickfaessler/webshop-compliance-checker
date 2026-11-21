package fixes

import (
	"strings"
	"unicode"
)

// stopwords are short function words per language for DetectLang (A6 fix).
var stopwords = map[string][]string{
	"de": strings.Fields("der die das und ist nicht mit sich des auf für ein eine einem einen dem den zu von im in wir sie ihr ihre unser unsere bei oder auch wenn aus nach"),
	"en": strings.Fields("the and is are not with for a an of to in on at we you your our or also if from by this that it be have"),
	"fr": strings.Fields("le la les et est pas avec pour un une des du de dans sur nous vous votre notre ou aussi si par ce qui que au"),
	"it": strings.Fields("il lo la gli le e è non con per un una dei del di nel sul noi voi vostro nostro o anche se da questo che al"),
	"es": strings.Fields("el la los las y es no con para un una de del en sobre nosotros usted su nuestro o también si por este que al"),
	"nl": strings.Fields("de het een en is niet met voor van in op wij u uw onze ons of ook als uit door dit dat bij zijn"),
}

// DetectLang guesses the language of text from stopword counts. It is sure when the top language has at
// least 8 hits and at least twice as many as the second.
func DetectLang(text string) (lang string, sure bool) {
	counts := map[string]int{}
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) })
	for _, w := range words {
		for l, list := range stopwords {
			for _, s := range list {
				if w == s {
					counts[l]++
					break
				}
			}
		}
	}
	best, second := "", 0
	for _, l := range []string{"de", "en", "fr", "it", "es", "nl"} { // fixed order: deterministic ties
		if best == "" || counts[l] > counts[best] {
			if best != "" {
				second = max(second, counts[best])
			}
			best = l
		} else {
			second = max(second, counts[l])
		}
	}
	if counts[best] >= 8 && counts[best] >= 2*second {
		return best, true
	}
	return best, false
}
