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

// labelFix builds the A3 fix for a form field (spec A3 "Fix templates"). next numbers fields that have
// neither id nor name (feld-1, feld-2, … per page). field may be nil (snippet not parsable).
func labelFix(field *html.Node, next func() int) *rules.Fix {
	attr := func(k string) string {
		if field == nil {
			return ""
		}
		v, _ := domutil.Attr(field, k)
		return strings.TrimSpace(v)
	}
	id, name, ph := attr("id"), attr("name"), attr("placeholder")
	label, note := ph, ""
	if label == "" && name != "" {
		label = humanize(name)
	}
	if label == "" {
		label, note = "Beschriftung", "Beschriftungstext bitte anpassen."
	}
	missingID := id == ""
	if missingID {
		id = name
		if id == "" {
			id = "feld-" + strconv.Itoa(next())
		}
	}
	vals := map[string]string{"id": id, "labelText": label}
	code := fixes.Fill(`<label for="{{id}}">{{labelText}}</label>`, vals)
	if missingID {
		code += "\n" + fixes.Fill(`id="{{id}}" am Feld ergänzen`, vals)
	}
	return &rules.Fix{TextDe: "Gib dem Feld eine sichtbare Beschriftung.", Code: code, NoteDe: note}
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
