package a6

import "testing"

// Spec: A6-F — the lang fix uses the detected language; unsure texts get "de" and a note.
func TestA6Fix(t *testing.T) {
	de := `<html><body><p>Willkommen in unserem Shop. Hier finden Sie die besten Tees aus aller Welt, und wir liefern schnell und sicher zu Ihnen nach Hause. Die Versandkosten sind ab einem Bestellwert von 40 Euro frei, und Sie können uns jederzeit fragen.</p></body></html>`
	en := `<html><body><p>Welcome to our shop. Here you will find the best teas from all over the world, and we deliver them fast and safely to your home. Shipping is free for orders over 40 euros, and you can contact us at any time if you have a question.</p></body></html>`
	tests := []struct{ page, code, note string }{
		{de, `<html lang="de">`, ""},
		{en, `<html lang="en">`, ""},
		{`<html><body><p>Shop</p></body></html>`, `<html lang="de">`, "Sprache bitte anpassen."},
	}
	for _, tt := range tests {
		r := check(t, tt.page)
		if len(r.Findings) != 1 {
			t.Fatalf("findings = %+v", r.Findings)
		}
		f := r.Findings[0].Fix
		if f == nil || f.TextDe != "Setze das Attribut lang im html-Element." || f.Code != tt.code || f.NoteDe != tt.note {
			t.Errorf("fix = %+v, want code %q note %q", f, tt.code, tt.note)
		}
	}
}
