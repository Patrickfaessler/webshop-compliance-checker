package legal

import (
	"testing"

	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

func de() lexicon.List { l, _ := lexicon.For("de"); return l }
func en() lexicon.List { l, _ := lexicon.For("en"); return l }

// Spec: R1-4 — EU VAT formats; malformed numbers are not counted.
func TestVATNumbers(t *testing.T) {
	valid := map[string]string{
		"UID: DE123456789":         "DE123456789",
		"UID-Nr. ATU12345678 FN 1": "ATU12345678",
		"ATU 1234 5678":            "ATU12345678",
		"VAT NL123456789B01":       "NL123456789B01",
		"TVA FRXX123456789":        "FRXX123456789",
		"VAT IE1234567T":           "IE1234567T",
		"P.IVA IT12345678901.":     "IT12345678901",
	}
	for text, want := range valid {
		if got := VATNumbers(text); len(got) != 1 || got[0] != want {
			t.Errorf("%q: got %v, want [%s]", text, got, want)
		}
	}
	for _, text := range []string{"DE12345678", "ATU1234567", "XX123456789", "DE1234567890", "CODE123456789"} {
		if got := VATNumbers(text); len(got) != 0 {
			t.Errorf("%q: got %v, want none", text, got)
		}
	}
}

// Spec: R1-1 — street line plus EU postal code is an address; partial data is unsure; nothing is missing.
func TestAddress(t *testing.T) {
	found := []string{
		"Hauptstraße 12, 6020 Innsbruck",
		"Musterweg 3/4 A-1010 Wien",
		"ul. Marszałkowska 10, 00-950 Warszawa",
		"Rua Augusta 5, 1100-053 Lisboa",
		"Damrak 1, 1012 AB Amsterdam",
		"Drottninggatan 5, 111 51 Stockholm",
	}
	for _, s := range found {
		if f := Address(s, de()); f.State != Found || f.Quote == "" {
			t.Errorf("%q: %+v, want Found", s, f)
		}
	}
	if f := Address("12 High Street, London SW1A 1AA", en()); f.State != Found {
		t.Errorf("UK: %+v, want Found", f)
	}
	for _, s := range []string{"Postfach 12, 1010 Wien", "6020 Innsbruck", "Anschrift: siehe unten", "Hauptstraße 12"} {
		if f := Address(s, de()); f.State != Unsure {
			t.Errorf("%q: %+v, want Unsure", s, f)
		}
	}
	for _, s := range []string{"Wir sind ein Familienbetrieb.", "Telefon: 0512 123456", "Firmenbuch: FN 123456a, UID: ATU12345678"} {
		if f := Address(s, de()); f.State != Missing {
			t.Errorf("%q: %+v, want Missing", s, f)
		}
	}
}

// Spec: R1-6, R1-7 — e-mail with obfuscations; a label with only an image is unsure.
func TestEmail(t *testing.T) {
	pg := func(body string) rules.Page { return rules.Page{HTML: `<html><body>` + body + `</body></html>`} }
	for _, body := range []string{"<p>info@shop.at</p>", "<p>info (at) shop.at</p>", "<p>info[at]shop.at</p>", "<p>E-Mail: info @ shop . at</p>",
		`<p><a href="mailto:x@y.at">Schreiben Sie uns</a></p>`} {
		p := pg(body)
		if f := Email(ContentText(p), p, de(), false); f.State != Found {
			t.Errorf("%s: %+v, want Found", body, f)
		}
	}
	for _, body := range []string{`<p>E-Mail: <img src="mail.png" alt=""></p>`, `<p>E-Mail:</p><p>Telefon 0512</p>`} {
		p := pg(body)
		if f := Email(ContentText(p), p, de(), false); f.State != Unsure {
			t.Errorf("%s: %+v, want Unsure", body, f)
		}
	}
	for _, body := range []string{"<p>Telefon: 0512 123</p>", `<p>Telefon 0512</p><footer><form><label>E-Mail für den Newsletter</label><input></form></footer>`} {
		p := pg(body)
		if f := Email(ContentText(p), p, de(), false); f.State != Missing {
			t.Errorf("%s: %+v, want Missing", body, f)
		}
	}
}

// Spec: R1-8 — legal form or label names the provider; a plain name is unsure, never missing.
func TestProviderName(t *testing.T) {
	for _, s := range []string{"Radwerk Alpin GmbH", "Inhaber: Max Muster", "Seifen Lavendel e.U., Graz", "Shop Ltd", "Muster GmbH & Co. KG"} {
		l := de()
		if s == "Shop Ltd" {
			l = en()
		}
		if f := ProviderName(s, l); f.State != Found {
			t.Errorf("%q: %+v, want Found", s, f)
		}
	}
	for _, s := range []string{"Max Muster", "AGB und Datenschutz", ""} {
		if f := ProviderName(s, de()); f.State != Unsure {
			t.Errorf("%q: %+v, want Unsure", s, f)
		}
	}
}

// Spec: R1 Logic 5 — register numbers.
func TestRegisterNumber(t *testing.T) {
	for _, s := range []string{"Firmenbuch: FN 123456a", "Amtsgericht München HRB 12345", "Company number 01234567"} {
		if _, ok := RegisterNumber(s); !ok {
			t.Errorf("%q: not found", s)
		}
	}
	if _, ok := RegisterNumber("Firmenbuchgericht Innsbruck"); ok {
		t.Error("court alone is no register number")
	}
}
