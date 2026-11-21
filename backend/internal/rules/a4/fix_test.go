package a4

import "testing"

// Spec: A4-F — every heading finding names what to change (text only, no code).
func TestA4Fix(t *testing.T) {
	tests := []struct{ body, sub, text string }{
		{`<h1>T</h1><h3>U</h3>`, "skipped-level", "Ändere <h3> zu <h2>."},
		{`<h2>T</h2>`, "no-h1", "Füge genau eine <h1> mit dem Seitentitel hinzu."},
		{`<h1>T</h1><h1>U</h1>`, "multi-h1", "Behalte nur eine <h1> und mache die anderen zu <h2>."},
		{`<h1>T</h1><h2></h2>`, "empty", "Gib der Überschrift einen Text oder entferne das leere Element."},
	}
	for _, tt := range tests {
		res := run(t, tt.body)
		found := false
		for _, f := range res.Findings {
			if f.Sub != tt.sub {
				continue
			}
			found = true
			if f.Fix == nil || f.Fix.TextDe != tt.text || f.Fix.Code != "" {
				t.Errorf("%s: fix = %+v, want %q", tt.sub, f.Fix, tt.text)
			}
		}
		if !found {
			t.Errorf("%s: no finding in %+v", tt.sub, res.Findings)
		}
	}
}
