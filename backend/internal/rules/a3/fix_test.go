package a3

import (
	"testing"

	"compliancechecker/internal/rules"
)

// Spec: A3-F — the label text comes from placeholder or name; a missing id is named; nothing derivable → note.
func TestA3Fix(t *testing.T) {
	ph := run(t, `<input id="plz" placeholder="PLZ">`, nil)
	if len(ph.Findings) != 1 || ph.Findings[0].Fix == nil || ph.Findings[0].Fix.Code != `<label for="plz">PLZ</label>` ||
		ph.Findings[0].Fix.TextDe != "Gib dem Feld eine sichtbare Beschriftung." {
		t.Fatalf("placeholder: %+v", ph.Findings)
	}
	axe := func(tag string) *rules.AxeReport {
		return &rules.AxeReport{Violations: []rules.AxeResult{{ID: "label", Nodes: []rules.AxeNode{{Selector: "input", HTML: tag}}}}}
	}
	nm := run(t, `<input name="e_mail">`, axe(`<input name="e_mail">`))
	if f := nm.Findings[0].Fix; f == nil || f.Code != "<label for=\"e_mail\">E mail</label>\nid=\"e_mail\" am Feld ergänzen" {
		t.Errorf("name: %+v", f)
	}
	none := run(t, `<input type="text">`, axe(`<input type="text">`))
	if f := none.Findings[0].Fix; f == nil || f.Code != "<label for=\"feld-1\">Beschriftung</label>\nid=\"feld-1\" am Feld ergänzen" ||
		f.NoteDe != "Beschriftungstext bitte anpassen." {
		t.Errorf("none: %+v", f)
	}
	search := run(t, `<input type="search" name="q">`, axe(`<input type="search" name="q">`))
	if f := search.Findings[0].Fix; f == nil || f.Code != "<label for=\"q\">Suche</label>\nid=\"q\" am Feld ergänzen" {
		t.Errorf("short name, search: %+v", f)
	}
	short := run(t, `<input type="text" name="nl">`, axe(`<input type="text" name="nl">`))
	if f := short.Findings[0].Fix; f == nil || f.Code != "<label for=\"nl\">Beschriftung</label>\nid=\"nl\" am Feld ergänzen" ||
		f.NoteDe != "Beschriftungstext bitte anpassen." {
		t.Errorf("short name: %+v", f)
	}
	evil := run(t, `<input id="q" placeholder='"><script>x</script>'>`, nil)
	if f := evil.Findings[0].Fix; f == nil || f.Code != `<label for="q">&#34;&gt;&lt;script&gt;x&lt;/script&gt;</label>` {
		t.Errorf("escaping: %+v", f)
	}
}

// Spec: A3-F — a cut-off snippet gives no id advice; radios do not reuse the group name; taken ids are not reused.
func TestA3FixEdgeCases(t *testing.T) {
	axe := func(tag string) *rules.AxeReport {
		return &rules.AxeReport{Violations: []rules.AxeResult{{ID: "label", Nodes: []rules.AxeNode{{Selector: "input", HTML: tag}}}}}
	}
	cut := run(t, `<input id="q" placeholder="Menge > 1">`, axe(`<input id="q" placeholder="Menge `))
	for _, f := range cut.Findings {
		if f.Sub == "label" && (f.Fix == nil || f.Fix.Code != "" || f.Fix.NoteDe != `Beschriftung mit <label for="…"> und der id des Feldes ergänzen.`) {
			t.Errorf("cut snippet: fix = %+v", f.Fix)
		}
	}
	radio := run(t, `<input type="radio" name="color" value="rot">`, axe(`<input type="radio" name="color" value="rot">`))
	if f := radio.Findings[0].Fix; f == nil || f.Code != "<label for=\"feld-1\">rot</label>\nid=\"feld-1\" am Feld ergänzen" ||
		f.NoteDe != "Optionsgruppen zusätzlich mit <fieldset> und <legend> beschriften." {
		t.Errorf("radio: fix = %+v", f)
	}
	taken := run(t, `<div id="email"></div><input name="email">`, axe(`<input name="email">`))
	if f := taken.Findings[0].Fix; f == nil || f.Code != "<label for=\"feld-1\">Email</label>\nid=\"feld-1\" am Feld ergänzen" {
		t.Errorf("taken id: fix = %+v", f)
	}
}
