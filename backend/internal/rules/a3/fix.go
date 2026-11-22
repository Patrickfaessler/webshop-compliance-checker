package a3

import (
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/fixes"
	"compliancechecker/internal/rules"
)

// labelFix builds the A3 fix for a form field (spec A3 "Fix templates"). next numbers fields that need a new
// id (feld-1, feld-2, … per page); taken reports ids that already exist on the page. field is nil when the
// snippet could not be read (e.g. cut off by axe): then no code is suggested.
func labelFix(field *html.Node, next func() int, taken func(string) bool) *rules.Fix {
	const text = "Gib dem Feld eine sichtbare Beschriftung."
	if field == nil {
		return &rules.Fix{TextDe: text, NoteDe: `Beschriftung mit <label for="…"> und der id des Feldes ergänzen.`}
	}
	attr := func(k string) string {
		v, _ := domutil.Attr(field, k)
		return strings.TrimSpace(v)
	}
	id, name, ph := attr("id"), attr("name"), attr("placeholder")
	typ := strings.ToLower(attr("type"))
	group := typ == "radio" || typ == "checkbox" // the name is shared by the whole group
	label, note := ph, ""
	if group {
		label, note = attr("value"), "Optionsgruppen zusätzlich mit <fieldset> und <legend> beschriften."
	}
	if label == "" && !group && letters(name) >= 3 {
		label = humanize(name)
	}
	if label == "" && typ == "search" {
		label = "Suche"
	}
	if label == "" {
		label = "Beschriftung"
		if note == "" {
			note = "Beschriftungstext bitte anpassen."
		}
	}
	missingID := id == ""
	if missingID {
		if name != "" && !group && !taken(name) {
			id = name
		} else {
			id = "feld-" + strconv.Itoa(next())
		}
	}
	vals := map[string]string{"id": id, "labelText": label}
	code := fixes.Fill(`<label for="{{id}}">{{labelText}}</label>`, vals)
	if missingID {
		code += "\n" + fixes.Fill(`id="{{id}}" am Feld ergänzen`, vals)
	}
	return &rules.Fix{TextDe: text, Code: code, NoteDe: note}
}

// humanize turns a field name into a label: "e_mail" → "E mail".
func humanize(name string) string {
	s := strings.Join(strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' }), " ")
	r := []rune(s)
	if len(r) > 0 {
		r[0] = unicode.ToUpper(r[0])
	}
	return string(r)
}

// letters counts the letters in a field name; names with fewer than 3 make no useful label ("q").
func letters(name string) int {
	n := 0
	for _, r := range name {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}
