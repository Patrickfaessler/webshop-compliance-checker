package r2

import (
	"context"
	"strings"
	"testing"

	"compliancechecker/internal/rules"
)

const start = "https://s.at/"

func shop(lang, privacy string) rules.Input {
	return rules.Input{Pages: []rules.Page{
		{URL: start, RequestedURL: start, HTML: `<html lang="` + lang + `"><body><main>Shop</main><footer><a href="/datenschutz">Datenschutz</a></footer></body></html>`},
		{URL: start + "datenschutz", RequestedURL: start + "datenschutz", HTML: `<html lang="` + lang + `"><body><main>` + privacy + `</main><footer>Newsletter</footer></body></html>`},
	}}
}

func check(t *testing.T, in rules.Input) rules.Result {
	t.Helper()
	res, err := New().Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	// Spec: R2-F — every finding carries a fix with German text.
	for _, f := range res.Findings {
		if f.Fix == nil || f.Fix.TextDe == "" {
			t.Errorf("finding %s has no fix", f.Sub)
		}
	}
	return res
}

const filler = `<p>Wir verarbeiten Ihre Daten nur, soweit dies für den Betrieb dieses Online-Shops nötig ist. Diese Erklärung beschreibt, welche Daten wir bei einem Besuch und bei einer Bestellung verarbeiten und wie wir sie schützen.</p>`

var sectionsDe = []string{
	`<h2>Verantwortlicher</h2><p>Verantwortlich ist die Teehaus GmbH, Teegasse 7, 1070 Wien.</p>`,
	`<h2>Zwecke und Rechtsgrundlage</h2><p>Wir verarbeiten Daten zur Vertragserfüllung (Art. 6 Abs. 1 lit. b DSGVO).</p>`,
	`<h2>Empfänger</h2><p>Empfänger sind unser Zahlungsdienst und der Versanddienst.</p>`,
	`<h2>Speicherdauer</h2><p>Wir speichern Bestelldaten sieben Jahre (Aufbewahrungspflicht).</p>`,
	`<h2>Ihre Rechte</h2><p>Sie haben das Recht auf Auskunft, Berichtigung, Löschung und Widerspruch.</p>`,
	`<h2>Beschwerderecht</h2><p>Sie können sich bei der Datenschutzbehörde beschweren.</p>`,
}

func privacy(n int) string {
	return `<h1>Datenschutzerklärung</h1>` + filler + strings.Join(sectionsDe[:n], "")
}

// Spec: R2-1 — all six sections present.
func TestR2Complete(t *testing.T) {
	res := check(t, shop("de", privacy(6)))
	if res.State != rules.Passed || res.NoteDe != "Nur geprüft, ob die Abschnitte vorhanden sind, nicht ob der Inhalt stimmt." {
		t.Errorf("got %+v", res)
	}
}

// Spec: R2-2 — no privacy link.
func TestR2NoLink(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{{URL: start, HTML: `<html lang="de"><body><a href="/agb">AGB</a></body></html>`}}}
	res := check(t, in)
	if res.State != rules.Failed || len(res.Findings) != 1 || res.Findings[0].Sub != "no-privacy-page" || res.Findings[0].PageURL != start ||
		res.Findings[0].Fix.TextDe != "Lege eine Datenschutzerklärung an und verlinke sie im Footer jeder Seite." ||
		strings.Count(res.Findings[0].Fix.Code, "- ") != 6 {
		t.Errorf("got %+v", res)
	}
}

// Spec: R2-3 — a heading and filler only: all six missing.
func TestR2HeadingOnly(t *testing.T) {
	res := check(t, shop("de", `<h1>Datenschutz</h1>`+filler+filler+filler))
	if res.State != rules.Failed || len(res.Findings) != 1 || res.Findings[0].Sub != "incomplete-sections" {
		t.Fatalf("got %+v", res)
	}
	want := "- Verantwortlicher\n- Zwecke und Rechtsgrundlage\n- Empfänger\n- Speicherdauer\n- Rechte der betroffenen Personen\n- Beschwerderecht bei der Aufsichtsbehörde"
	if f := res.Findings[0].Fix; f.Code != want || f.TextDe != "Ergänze die fehlenden Abschnitte der Datenschutzerklärung." || f.NoteDe != "Ein Datenschutz-Generator oder eine Fachperson kann helfen." {
		t.Errorf("fix = %+v", f)
	}
}

// Spec: R2-4 — English privacy policy.
func TestR2English(t *testing.T) {
	en := `<h1>Privacy policy</h1><p>This policy explains how we handle personal data when you visit and order from this online shop and how we protect it.</p>
<h2>Data controller</h2><p>The controller is Northwind Goods Ltd.</p><h2>Purpose and legal basis</h2><p>We process data to perform the contract (Art. 6(1)(b) GDPR).</p>
<h2>Recipients</h2><p>Recipients are our payment and shipping providers.</p><h2>Retention</h2><p>We keep order data for seven years.</p>
<h2>Your rights</h2><p>You have the right of access, rectification and erasure.</p><h2>Complaints</h2><p>You may lodge a complaint with a supervisory authority.</p>`
	in := shop("en", en)
	in.Pages[0].HTML = `<html lang="en"><body><footer><a href="/datenschutz">Privacy policy</a></footer></body></html>`
	if res := check(t, in); res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}

// Spec: R2-5 — 500 characters or less fails with all six missing.
func TestR2Short(t *testing.T) {
	res := check(t, shop("de", `<h1>Datenschutz</h1><p>Wir nehmen Datenschutz ernst. Verantwortlich: Teehaus GmbH.</p>`))
	if res.State != rules.Failed || len(res.Findings) != 1 || strings.Count(res.Findings[0].Fix.Code, "- ") != 6 {
		t.Errorf("got %+v", res)
	}
}

// Spec: R2-6 — numbered and uppercase headings count.
func TestR2NumberedHeadings(t *testing.T) {
	body := `<h1>DATENSCHUTZERKLÄRUNG</h1>` + filler + filler + `<h2>1. VERANTWORTLICHER</h2><p>Teehaus GmbH.</p><h2>2. Zweck der Verarbeitung</h2><p>Bestellung.</p>
<h2>3. Empfänger</h2><p>Versand.</p><h2>4. Speicherdauer</h2><p>Sieben Jahre.</p><h2>§ 5 Ihre Rechte</h2><p>Siehe unten.</p><p><strong>VI. Beschwerde</strong></p><p>Bei der Behörde.</p>`
	if res := check(t, shop("de", body)); res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}

// Spec: R2 result mapping — four of six fail and list exactly the missing two; five of six pass.
func TestR2Threshold(t *testing.T) {
	res := check(t, shop("de", privacy(4)))
	if res.State != rules.Failed || res.Findings[0].Fix.Code != "- Rechte der betroffenen Personen\n- Beschwerderecht bei der Aufsichtsbehörde" {
		t.Errorf("4/6: got %+v", res)
	}
	if res := check(t, shop("de", privacy(5))); res.State != rules.Passed {
		t.Errorf("5/6: got %+v", res)
	}
}

// Spec: R2 result mapping — linked but not loaded.
func TestR2NotLoaded(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{{URL: start, HTML: `<html lang="de"><body><footer><a href="https://other.example/datenschutz">Datenschutz</a></footer></body></html>`}}}
	if res := check(t, in); res.State != rules.NotAssessable || res.NoteDe != "Die Datenschutzerklärung ist verlinkt, konnte aber nicht geladen werden." {
		t.Errorf("got %+v", res)
	}
}

// Spec: F8 — meta.
func TestR2Meta(t *testing.T) {
	if m := New().Meta(); m.ID != "R2" || m.Area != "legal" || m.Severity != rules.Warning || m.ExplanationDe == "" {
		t.Errorf("meta = %+v", m)
	}
}
