package legal

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

// FieldState is how sure a validator is that a field is on the page (R1 Logic 4).
type FieldState int

const (
	Missing FieldState = iota // confidently absent
	Unsure                    // maybe there, maybe not: manual check
	Found
)

// Field is a validator result; Quote is the matched text, at most 120 runes.
type Field struct {
	State FieldState
	Quote string
}

func quote(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > 120 {
		return string(r[:120])
	}
	return s
}

var (
	atSep  = `(?:\s*@\s*|\s*[\(\[\{]\s*(?:at|@)\s*[\)\]\}]\s*)`
	dotSep = `(?:\s*\.\s*|\s*[\(\[\{]\s*(?:dot|punkt)\s*[\)\]\}]\s*)`
	reMail = regexp.MustCompile(`(?i)[\p{L}0-9._%+-]+` + atSep + `[\p{L}0-9-]+(?:` + dotSep + `[\p{L}0-9-]+)*` + dotSep + `\p{L}{2,24}`)
	// "info at shop dot at" (R1-12)
	reMailWords = regexp.MustCompile(`(?i)\b[a-z0-9._-]+\s+(?:at|ät)\s+[\p{L}0-9-]+(?:\s+(?:dot|punkt)\s+[\p{L}0-9-]+)+`)
	// e-mail-like text that is not an address: a reversed address, "@" or at/dot words
	reMailLike = regexp.MustCompile(`(?i)@|\b(?:at|ät|dot|punkt)\b`)
)

// Email finds an e-mail address in text, including common obfuscations, or a mailto link in the page (outside
// header/nav/footer/aside/form unless whole is true). Without an address, an e-mail label that is followed only
// by an image or by nothing makes the field Unsure (R1-7).
func Email(text string, page rules.Page, l lexicon.List, whole bool) Field {
	if m := reMail.FindString(text); m != "" {
		return Field{Found, quote(m)}
	}
	if m := reMailWords.FindString(text); m != "" {
		return Field{Found, quote(m)}
	}
	if strings.Contains(page.HTML, "data-cfemail") || strings.Contains(page.HTML, "/cdn-cgi/l/email-protection") {
		return Field{State: Unsure} // Cloudflare e-mail protection: the address is there, encoded (R1-12)
	}
	unsure, mailto := false, false
	walkContent(domutil.Parse(page.HTML), whole, func(n *html.Node) {
		if domutil.IsElement(n, "a") {
			if h, _ := domutil.Attr(n, "href"); strings.HasPrefix(strings.ToLower(strings.TrimSpace(h)), "mailto:") && strings.Contains(h, "@") {
				mailto = true
			}
			return
		}
		if n.Type != html.TextNode || n.Parent == nil {
			return
		}
		t := strings.ToLower(n.Data)
		for _, w := range l.EmailLabels {
			if !lexicon.ContainsWord(t, w) {
				continue
			}
			parent := n.Parent
			rest := strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(after(strings.ToLower(collapse(domutil.Text(parent))), w)), ":-–"))
			if hasImg(parent) || rest == "" || reMailLike.MatchString(rest) {
				unsure = true
			}
			return
		}
	})
	if mailto {
		return Field{Found, "mailto"}
	}
	if unsure {
		return Field{State: Unsure}
	}
	return Field{State: Missing}
}

func after(s, w string) string {
	if i := strings.Index(s, w); i >= 0 {
		return s[i+len(w):]
	}
	return s
}

func hasImg(n *html.Node) bool {
	found := false
	domutil.Walk(n, func(c *html.Node) bool {
		if domutil.IsElement(c, "img", "svg", "canvas") {
			found = true
		}
		return !found
	})
	return found
}

// walkContent visits the nodes outside script/style and, unless whole, outside header/nav/footer/aside/form.
func walkContent(doc *html.Node, whole bool, fn func(*html.Node)) {
	domutil.Walk(doc, func(n *html.Node) bool {
		if domutil.IsElement(n, "script", "style", "noscript", "template", "head") || (!whole && domutil.IsElement(n, boilerplate...)) {
			return false
		}
		fn(n)
		return true
	})
}

var (
	// a capitalised street name (at least 3 letters) plus house number, e.g. "Hauptstraße 12", "Musterweg 3/4", "Damrak 1"
	// further words must be capitalised or short particles ("Rue de la Paix 5"); a 4-digit year is no house number
	reStreet = regexp.MustCompile(`(\p{Lu}\p{Ll}{2,}(?:[ -](?:\p{Lu}\p{Ll}+|de|la|le|du|des|der|del|di|da|am|an|im|zum|zur|von|van|\d{1,2}\.))*\.?)\s+(\d{1,4})[a-z]?(?:\s*[/-]\s*\d{1,4}[a-z]?)*\b`)
	// English order, e.g. "12 High Street"
	reStreetEn = regexp.MustCompile(`\b\d{1,5}[A-Za-z]?\s+(?:\p{Lu}\p{Ll}+\s+)+(?:Street|St\.?|Road|Rd\.?|Avenue|Ave\.?|Lane|Way|Drive|Place|Square|Row|Hill)\b`)
	// not streets: words that precede numbers in an Impressum
	notStreet = map[string]bool{"postfach": true, "telefon": true, "tel": true, "fax": true, "mobil": true, "handy": true,
		"seite": true, "stand": true, "version": true, "iban": true, "kontonummer": true, "firmenbuch": true, "box": true}
	rePostal = []*regexp.Regexp{
		regexp.MustCompile(`(?:\b[A-Z]{1,2}-)?\b\d{4,5}\s+\p{Lu}\p{L}+`), // DE/AT/CH/FR/IT/ES/BE/DK/… with optional A-/D- prefix
		regexp.MustCompile(`\b\d{2}-\d{3}\s+\p{Lu}\p{L}+`),               // PL
		regexp.MustCompile(`\b\d{4}-\d{3}\b`),                            // PT
		regexp.MustCompile(`\b\d{3} \d{2}\s+\p{Lu}\p{L}+`),               // SE, CZ, SK, GR
		regexp.MustCompile(`\b\d{4} ?[A-Z]{2}\s+\p{Lu}\p{L}+`),           // NL
	}
	rePostalEn = []*regexp.Regexp{
		regexp.MustCompile(`\b[A-Z]\d[\dW] ?[A-Z\d]{4}\b`),         // IE Eircode
		regexp.MustCompile(`\b[A-Z]{3} ?\d{4}\b`),                  // MT
		regexp.MustCompile(`\b[A-Z]{1,2}\d[A-Z\d]? ?\d[A-Z]{2}\b`), // UK-style
	}
	rePOBox = regexp.MustCompile(`(?i)\b(?:postfach|p\.?\s?o\.?\s?box)\b`)
)

// Address looks for a geographic postal address (R1 Logic 4): a street line and a postal code from the EU
// format table → Found; one of the two, a PO box alone, or an address label → Unsure; none of these → Missing.
func Address(text string, l lexicon.List) Field {
	street := ""
	for _, m := range reStreet.FindAllStringSubmatch(text, -1) {
		if n := m[2]; len(n) == 4 && (n[:2] == "18" || n[:2] == "19" || n[:2] == "20") {
			continue // a year, e.g. "seit 1998"
		}
		if !notStreet[strings.ToLower(strings.TrimSuffix(m[1], "."))] {
			street = m[0]
			break
		}
	}
	if street == "" {
		street = reStreetEn.FindString(text)
	}
	postal := ""
	pats := rePostal
	if l.Lang == "en" { // Irish, Maltese and UK-style codes, only on English pages
		pats = append(append([]*regexp.Regexp{}, rePostal...), rePostalEn...)
	}
	for _, re := range pats {
		if m := re.FindString(text); m != "" {
			postal = m
			break
		}
	}
	switch {
	case street != "" && postal != "":
		return Field{Found, quote(street + " … " + postal)}
	case street != "" || postal != "" || rePOBox.MatchString(text):
		return Field{State: Unsure, Quote: quote(street + postal)}
	}
	t := strings.ToLower(text)
	for _, w := range l.AddressLabels {
		if lexicon.ContainsWord(t, w) {
			return Field{State: Unsure}
		}
	}
	return Field{State: Missing}
}

var reLegalForm = regexp.MustCompile(`(?:^|[^\p{L}])(GmbH & Co\. KG|GmbH|gGmbH|AG|KG|OG|OHG|e\.U\.|e\.K\.|UG|SE|Ltd\.?|LLC|LLP|PLC|plc|SARL|SAS|SA|B\.V\.|N\.V\.|S\.L\.|S\.r\.l\.|S\.p\.A\.|Sp\. z o\.o\.|s\.r\.o\.|ApS|AB|Oy)(?:$|[^\p{L}])`)

// ProviderName finds the provider by a legal-form suffix or a name label followed by ":" (R1 Logic 4). Without
// either it is Unsure, never Missing: a sole trader's plain name cannot be told from other text (R1-8).
func ProviderName(text string, l lexicon.List) Field {
	if m := reLegalForm.FindStringIndex(text); m != nil {
		start := m[0] - 60
		if start < 0 {
			start = 0
		}
		return Field{Found, quote(text[start:m[1]])}
	}
	t := strings.ToLower(text)
	for _, w := range l.NameLabels {
		i := strings.Index(t, w+":")
		if i >= 0 && (i == 0 || !isLetter(t[i-1])) {
			rest := strings.TrimSpace(text[i+len(w)+1:])
			if rest != "" {
				return Field{Found, quote(text[i:])}
			}
		}
	}
	return Field{State: Unsure}
}

func isLetter(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= 0x80 }

var reRegister = regexp.MustCompile(`\bFN\s*\d{1,6}\s?[a-z]\b|\bHR[AB]\s*\d{1,6}\b|(?i:\bcompany (?:number|no\.?|registration number)\b)[:\s]*[A-Z]{0,2}\d{6,8}\b`)

// RegisterNumber finds a commercial register number (FN 123456a, HRB 12345, Company number 01234567).
func RegisterNumber(text string) (string, bool) {
	m := reRegister.FindString(text)
	return m, m != ""
}
