package r1

import (
	"context"
	"strings"
	"testing"

	"compliancechecker/internal/rules"
)

const start = "https://s.at/"

func shop(impressum string) rules.Input {
	return rules.Input{Pages: []rules.Page{
		{URL: start, RequestedURL: start, HTML: `<html lang="de"><body><main>Willkommen</main><footer><a href="/impressum">Impressum</a></footer></body></html>`},
		{URL: start + "impressum", RequestedURL: start + "impressum", HTML: impressum},
	}}
}

func check(t *testing.T, in rules.Input) rules.Result {
	t.Helper()
	res, err := New().Check(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	// Spec: R1-F — every finding carries a fix with German text.
	for _, f := range res.Findings {
		if f.Fix == nil || f.Fix.TextDe == "" {
			t.Errorf("finding %s has no fix", f.Sub)
		}
	}
	return res
}

func page(body string) string {
	return `<html lang="de"><body><main><h1>Impressum</h1>` + body + `</main><footer><a href="/agb">AGB</a></footer></body></html>`
}

const complete = `<p>Radwerk Alpin GmbH</p><p>Hauptstraße 12, 6020 Innsbruck</p><p>E-Mail: info@radwerk.at</p><p>Firmenbuch: FN 123456a</p><p>UID: ATU12345678</p>`

// Spec: R1-1 — all three fields present.
func TestR1Complete(t *testing.T) {
	res := check(t, shop(page(complete)))
	if res.State != rules.Passed || !strings.Contains(res.NoteDe, "Nur geprüft, ob die Angaben vorhanden sind.") || strings.Contains(res.NoteDe, "UID-Nummer") {
		t.Errorf("got %+v", res)
	}
}

// Spec: R1-2 — missing e-mail fails naming "E-Mail".
func TestR1MissingEmail(t *testing.T) {
	res := check(t, shop(page(`<p>Radwerk Alpin GmbH</p><p>Hauptstraße 12, 6020 Innsbruck</p><p>Telefon 0512 123456</p>`)))
	if res.State != rules.Failed || len(res.Findings) != 1 {
		t.Fatalf("got %+v", res)
	}
	f := res.Findings[0]
	if f.Sub != "missing-email" || f.PageURL != start+"impressum" || f.Selector != "" ||
		!strings.Contains(f.Fix.Code, "E-Mail: {{E-Mail}}") || strings.Contains(f.Fix.Code, "Firmenname") ||
		f.Fix.TextDe != "Ergänze die fehlenden Angaben im Impressum." {
		t.Errorf("finding = %+v fix = %+v", f, f.Fix)
	}
}

// Spec: R1-3 — no legal link fails on the start page.
func TestR1NoLink(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{{URL: start, HTML: `<html lang="de"><body><a href="/produkt">Lampe</a></body></html>`}}}
	res := check(t, in)
	if res.State != rules.Failed || len(res.Findings) != 1 {
		t.Fatalf("got %+v", res)
	}
	f := res.Findings[0]
	if f.Sub != "no-legal-notice-page" || f.PageURL != start || f.Fix.TextDe != "Lege eine Seite „Impressum“ an und verlinke sie im Footer jeder Seite." ||
		!strings.Contains(f.Fix.Code, "Firmenname: {{Firmenname}}") || !strings.Contains(f.Fix.Code, "E-Mail: {{E-Mail}}") {
		t.Errorf("finding = %+v fix = %+v", f, f.Fix)
	}
}

// Spec: R1-7 — an e-mail shown only as an image cannot be judged.
func TestR1EmailImage(t *testing.T) {
	res := check(t, shop(page(`<p>Radwerk Alpin GmbH</p><p>Hauptstraße 12, 6020 Innsbruck</p><p>E-Mail: <img src="mail.png" alt=""></p>`)))
	if res.State != rules.NotAssessable || !strings.Contains(res.NoteDe, "Bitte manuell prüfen: E-Mail.") {
		t.Errorf("got %+v", res)
	}
}

// Spec: R1-8 — a plain name without legal form never fails.
func TestR1NameWithoutLegalForm(t *testing.T) {
	res := check(t, shop(page(`<p>Max Muster</p><p>Hauptstraße 12, 6020 Innsbruck</p><p>info@muster.at</p>`)))
	if res.State != rules.NotAssessable || !strings.Contains(res.NoteDe, "Bitte manuell prüfen: Firmenname.") {
		t.Errorf("got %+v", res)
	}
}

// Spec: R1-9 — a legal notice on another domain is linked but not loaded.
func TestR1LinkNotLoaded(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{{URL: start, HTML: `<html lang="de"><body><footer><a href="https://legal.example/s.at">Impressum</a></footer></body></html>`}}}
	res := check(t, in)
	if res.State != rules.NotAssessable || res.NoteDe != "Das Impressum ist verlinkt, konnte aber nicht geladen werden." || len(res.Findings) != 0 {
		t.Errorf("got %+v", res)
	}
}

// Spec: R1-10 — an e-mail only in the Impressum page's footer counts.
func TestR1FooterEmailCounts(t *testing.T) {
	html := `<html lang="de"><body><main><h1>Impressum</h1><p>Radwerk Alpin GmbH</p><p>Hauptstraße 12, 6020 Innsbruck</p></main><footer>Kontakt: <a href="mailto:info@radwerk.at">Schreiben Sie uns</a></footer></body></html>`
	if res := check(t, shop(html)); res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}

// Spec: R1 Logic 5 — absent VAT and register only add a note.
func TestR1ConditionalNote(t *testing.T) {
	res := check(t, shop(page(`<p>Radwerk Alpin GmbH</p><p>Hauptstraße 12, 6020 Innsbruck</p><p>info@radwerk.at</p>`)))
	if res.State != rules.Passed || !strings.Contains(res.NoteDe, "UID-Nummer bzw. Firmenbuchnummer prüfen, ob zutreffend.") {
		t.Errorf("got %+v", res)
	}
}

// Spec: R1 Logic 2 — other languages are not assessed.
func TestR1French(t *testing.T) {
	fr := `<html lang="fr"><body><main><h1>Mentions légales</h1><p>Société Exemple, rue de la Paix</p></main></body></html>`
	res := check(t, shop(fr))
	if res.State != rules.NotAssessable || !strings.Contains(res.NoteDe, "Seite bitte manuell prüfen (Sprache oder Aufbau nicht erkannt).") {
		t.Errorf("got %+v", res)
	}
}

// Spec: R1-F, F8 — meta carries the German explanation and the legal area.
func TestR1Meta(t *testing.T) {
	m := New().Meta()
	if m.ID != "R1" || m.Area != "legal" || m.Severity != rules.Critical || m.ExplanationDe == "" || !m.AIFallback {
		t.Errorf("meta = %+v", m)
	}
}

// Spec: R1-11 — an Impressum embedded in an iframe is not judged as missing.
func TestR1Embedded(t *testing.T) {
	res := check(t, shop(page(`<iframe src="https://legal.example/impressum"></iframe>`)))
	if res.State != rules.NotAssessable || !strings.Contains(res.NoteDe, "eingebettet (iframe)") {
		t.Errorf("got %+v", res)
	}
}

// Spec: R1-13 — a modal Impressum link is not assessable, not failed.
func TestR1ModalLink(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{{URL: start, HTML: `<html lang="de"><body><footer><a href="#">Impressum</a></footer></body></html>`}}}
	if res := check(t, in); res.State != rules.NotAssessable {
		t.Errorf("got %+v", res)
	}
}
