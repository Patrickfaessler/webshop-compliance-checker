package a1

import (
	"strings"
	"testing"

	"compliancechecker/internal/rules"
)

// Spec: A1-F, A1-3 — the fix names the old and the suggested colour and its ratio.
func TestA1Fix(t *testing.T) {
	f := fixFor(rules.AxeNode{Data: map[string]any{"fgColor": "#aaaaaa", "bgColor": "#ffffff", "contrastRatio": 2.32, "expectedContrastRatio": "4.5:1"}})
	if f == nil || !strings.HasPrefix(f.TextDe, "Ändere die Textfarbe von #aaaaaa auf #") || !strings.HasSuffix(f.TextDe, ":1).") {
		t.Fatalf("fix = %+v", f)
	}
	if !strings.Contains(f.TextDe, "(Kontrast 4,") || !strings.HasPrefix(f.Code, "color: #") || !strings.HasSuffix(f.Code, ";") || f.NoteDe != "" {
		t.Errorf("fix = %+v", f)
	}
}

// Spec: A1-F — without a usable background the fix does not guess a colour (Review Focus 2).
func TestA1FixUnknownBackground(t *testing.T) {
	for _, d := range []map[string]any{
		{"fgColor": "#999999", "contrastRatio": 0, "expectedContrastRatio": "3:1", "messageKey": "bgImage"},
		{"fgColor": "#999999", "bgColor": "rgba(0, 0, 0, 0)", "expectedContrastRatio": "4.5:1"},
	} {
		f := fixFor(rules.AxeNode{Data: d})
		if f == nil || f.TextDe != "Wähle eine Textfarbe mit ausreichend Kontrast zum Hintergrund." || f.Code != "" ||
			f.NoteDe != "Hintergrund nicht eindeutig (z. B. Bild) – Kontrast bitte selbst prüfen." {
			t.Errorf("data %v: fix = %+v", d, f)
		}
	}
}

// Spec: A1-F — the rule attaches the fix to every finding.
func TestA1CheckAttachesFix(t *testing.T) {
	p := rules.Page{URL: "https://s.at/", Axe: &rules.AxeReport{Violations: []rules.AxeResult{{ID: "color-contrast",
		Nodes: []rules.AxeNode{{Selector: "p", HTML: "<p>", Data: map[string]any{"fgColor": "#aaaaaa", "bgColor": "#ffffff"}}}}}}}
	r, _ := New().Check(nil, rules.Input{Pages: []rules.Page{p}})
	if len(r.Findings) != 1 || r.Findings[0].Fix == nil {
		t.Fatalf("findings = %+v", r.Findings)
	}
}
