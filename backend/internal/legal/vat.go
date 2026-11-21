package legal

import (
	"regexp"
	"strings"
)

// vatBody is the format of the part after the country prefix, per EU member state plus XI (Northern Ireland).
var vatBody = map[string]*regexp.Regexp{}

func init() {
	for cc, re := range map[string]string{
		"AT": `U\d{8}`, "BE": `[01]\d{9}`, "BG": `\d{9,10}`, "CY": `\d{8}[A-Z]`, "CZ": `\d{8,10}`, "DE": `\d{9}`,
		"DK": `\d{8}`, "EE": `\d{9}`, "EL": `\d{9}`, "ES": `[A-Z0-9]\d{7}[A-Z0-9]`, "FI": `\d{8}`,
		"FR": `[A-HJ-NP-Z0-9]{2}\d{9}`, "HR": `\d{11}`, "HU": `\d{8}`, "IE": `\d{7}[A-W][A-I]?|\d[A-Z+*]\d{5}[A-W]`,
		"IT": `\d{11}`, "LT": `\d{9}|\d{12}`, "LU": `\d{8}`, "LV": `\d{11}`, "MT": `\d{8}`, "NL": `\d{9}B\d{2}`,
		"PL": `\d{10}`, "PT": `\d{9}`, "RO": `\d{2,10}`, "SE": `\d{12}`, "SI": `\d{8}`, "SK": `\d{10}`,
		"XI": `\d{9}|\d{12}|GD\d{3}|HA\d{3}`,
	} {
		vatBody[cc] = regexp.MustCompile(`^(?:` + re + `)$`)
	}
}

// reVATCandidate is a country prefix followed by groups of upper-case letters/digits, separated by space or dot.
var reVATCandidate = regexp.MustCompile(`\b(AT|BE|BG|CY|CZ|DE|DK|EE|EL|ES|FI|FR|HR|HU|IE|IT|LT|LU|LV|MT|NL|PL|PT|RO|SE|SI|SK|XI) ?((?:[A-Z0-9+*]+[ .]?){1,6})`)

// VATNumbers returns the VAT ids of valid EU format in text, without spaces or dots (R1-4). A group is never cut in
// the middle: "DE1234567890" is not read as DE123456789.
func VATNumbers(text string) []string {
	var out []string
	for _, m := range reVATCandidate.FindAllStringSubmatch(text, -1) {
		groups := strings.FieldsFunc(m[2], func(r rune) bool { return r == ' ' || r == '.' })
		for k := len(groups); k > 0; k-- {
			body := strings.Join(groups[:k], "")
			if vatBody[m[1]].MatchString(body) {
				out = append(out, m[1]+body)
				break
			}
		}
	}
	return out
}
