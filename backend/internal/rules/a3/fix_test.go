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
	evil := run(t, `<input id="q" placeholder='"><script>x</script>'>`, nil)
	if f := evil.Findings[0].Fix; f == nil || f.Code != `<label for="q">&#34;&gt;&lt;script&gt;x&lt;/script&gt;</label>` {
		t.Errorf("escaping: %+v", f)
	}
}
