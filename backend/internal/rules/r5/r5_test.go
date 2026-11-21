package r5

import (
	"context"
	"strings"
	"testing"

	"compliancechecker/internal/rules"
)

const start = "https://s.at/"

func pg(path, lang, main string) rules.Page {
	return rules.Page{URL: start + path, RequestedURL: start + path,
		HTML: `<html lang="` + lang + `"><body><main>` + main + `</main><footer>Kontakt · Newsletter</footer></body></html>`}
}

func startPage(footer string) rules.Page {
	return rules.Page{URL: start, RequestedURL: start, HTML: `<html lang="de"><body><main>Willkommen</main><footer>` + footer + `</footer></body></html>`}
}

func check(t *testing.T, pages ...rules.Page) rules.Result {
	t.Helper()
	res, err := New().Check(context.Background(), rules.Input{Pages: pages})
	if err != nil {
		t.Fatal(err)
	}
	// Spec: R5-F — every finding carries a fix with German text.
	for _, f := range res.Findings {
		if f.Fix == nil || f.Fix.TextDe == "" {
			t.Errorf("finding %s has no fix", f.Sub)
		}
	}
	return res
}

const terms = `<h1>Allgemeine Geschäftsbedingungen</h1><p>§ 1 Geltungsbereich. Diese AGB gelten für alle Bestellungen in unserem Shop.</p><p>§ 2 Vertragsschluss. Der Vertrag kommt mit der Auftragsbestätigung zustande.</p>`
const withdrawal = `<h1>Widerrufsbelehrung</h1><h2>Widerrufsrecht</h2><p>Sie haben das Recht, binnen vierzehn Tagen ohne Angabe von Gründen diesen Vertrag zu widerrufen.</p>`
const both = `<a href="/agb">AGB</a> <a href="/widerruf">Widerrufsbelehrung</a>`

// Spec: R5-1 — terms and withdrawal page with the period.
func TestR5Both(t *testing.T) {
	res := check(t, startPage(both), pg("agb", "de", terms), pg("widerruf", "de", withdrawal))
	if res.State != rules.Passed || !strings.HasPrefix(res.NoteDe, "Annahme: Der Shop verkauft an Verbraucher.") {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5-2 — no terms link.
func TestR5NoTerms(t *testing.T) {
	res := check(t, startPage(`<a href="/widerruf">Widerrufsbelehrung</a>`), pg("widerruf", "de", withdrawal))
	if res.State != rules.Failed || len(res.Findings) != 1 || res.Findings[0].Sub != "no-terms" || res.Findings[0].PageURL != start ||
		res.Findings[0].Fix.TextDe != "Lege AGB an und verlinke sie im Footer und im Bestellprozess." || res.Findings[0].Fix.Code != "" {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5-3 — withdrawal information inside the terms.
func TestR5WithdrawalInTerms(t *testing.T) {
	res := check(t, startPage(`<a href="/agb">AGB</a>`), pg("agb", "de", terms+`<h2>§ 5 Widerrufsrecht</h2><p>Verbraucher können den Vertrag binnen 14 Tagen widerrufen.</p>`))
	if res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5-4 — English shop.
func TestR5English(t *testing.T) {
	en := rules.Page{URL: start, HTML: `<html lang="en"><body><footer><a href="/terms">Terms and conditions</a> <a href="/withdrawal">Right of withdrawal</a></footer></body></html>`}
	res := check(t, en, pg("terms", "en", `<h1>Terms and conditions</h1><p>These terms apply to all orders placed in our shop.</p>`),
		pg("withdrawal", "en", `<h1>Right of withdrawal</h1><p>You have the right to withdraw from this contract within 14 days without giving any reason.</p>`))
	if res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5-5 — withdrawal words without a period.
func TestR5NoPeriod(t *testing.T) {
	res := check(t, startPage(both), pg("agb", "de", terms), pg("widerruf", "de", `<h1>Widerrufsbelehrung</h1><p>Sie können Ihre Bestellung widerrufen. Details folgen.</p>`))
	if res.State != rules.NotAssessable || !strings.Contains(res.NoteDe, "Widerrufsfrist nicht gefunden – bitte manuell prüfen.") {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5-6 — Austrian wording.
func TestR5Ruecktritt(t *testing.T) {
	res := check(t, startPage(`<a href="/agb">AGB</a>`), pg("agb", "de", terms+`<h2>Rücktrittsrecht</h2><p>Sie können innerhalb von 14 Tagen ohne Angabe von Gründen vom Vertrag zurücktreten (Rücktrittsrecht nach FAGG).</p>`))
	if res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5-7 — an English terms page on a German shop is read as English.
func TestR5EnglishTermsOnGermanShop(t *testing.T) {
	res := check(t, startPage(`<a href="/agb">AGB</a>`), pg("agb", "en", `<h1>Terms and conditions</h1><p>These terms apply to all orders.</p><h2>Right of withdrawal</h2><p>Consumers may withdraw from the contract within fourteen days.</p>`))
	if res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5 result mapping — no withdrawal words anywhere fails; a linked but not loaded withdrawal page is not assessable.
func TestR5FooterLinkNotWithdrawalInfo(t *testing.T) {
	res := check(t, startPage(`<a href="/agb">AGB</a>`), pg("agb", "de", terms))
	if res.State != rules.Failed || len(res.Findings) != 1 || res.Findings[0].Sub != "no-withdrawal-info" || res.Findings[0].PageURL != start+"agb" {
		t.Fatalf("no link: got %+v", res)
	}
	if f := res.Findings[0].Fix; f.TextDe != "Ergänze eine Widerrufsbelehrung mit der 14-tägigen Frist." || !strings.HasPrefix(f.Code, "Widerrufsbelehrung\n") ||
		!strings.Contains(f.NoteDe, "Anhang I der Richtlinie 2011/83/EU") {
		t.Errorf("fix = %+v", f)
	}
	res = check(t, startPage(both), pg("agb", "de", terms))
	if res.State != rules.NotAssessable || !strings.Contains(res.NoteDe, "konnten aber nicht geladen werden") {
		t.Errorf("not loaded: got %+v", res)
	}
}

// Spec: R5 result mapping — both failures give two findings.
func TestR5BothMissing(t *testing.T) {
	res := check(t, startPage(`<a href="/kontakt">Kontakt</a>`))
	if res.State != rules.Failed || len(res.Findings) != 2 {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5 notes — exempt goods add a note.
func TestR5ExemptNote(t *testing.T) {
	res := check(t, startPage(both), pg("agb", "de", terms+`<p>Für digitale Inhalte gelten Sonderregeln.</p>`), pg("widerruf", "de", withdrawal))
	if res.State != rules.Passed || !strings.Contains(res.NoteDe, "Ausnahmen vom Widerrufsrecht prüfen.") {
		t.Errorf("got %+v", res)
	}
}

// Spec: F8 — meta.
func TestR5Meta(t *testing.T) {
	if m := New().Meta(); m.ID != "R5" || m.Area != "legal" || m.Severity != rules.Warning || m.ExplanationDe == "" {
		t.Errorf("meta = %+v", m)
	}
}

// Spec: R5-8 — a form link before the real Widerrufsbelehrung does not hide it.
func TestR5FormLinkFirst(t *testing.T) {
	res := check(t, startPage(`<a href="/agb">AGB</a> <a href="/formular">Widerrufsformular</a> <a href="/widerruf">Widerrufsbelehrung</a>`),
		pg("agb", "de", terms), pg("formular", "de", `<h1>Widerrufsformular</h1><form><label>Name</label><input></form>`), pg("widerruf", "de", withdrawal))
	if res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}

// Spec: R5-9 — a combined "AGB & Widerrufsbelehrung" link on a neutral URL counts as the terms link.
func TestR5CombinedLink(t *testing.T) {
	res := check(t, startPage(`<a href="/?page_id=7">AGB &amp; Widerrufsbelehrung</a>`), pg("?page_id=7", "de", terms+`<h2>Widerrufsrecht</h2><p>Sie können binnen 14 Tagen widerrufen.</p>`))
	if res.State != rules.Passed {
		t.Errorf("got %+v", res)
	}
}
