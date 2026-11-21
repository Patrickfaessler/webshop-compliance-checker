package legal

import (
	"strings"
	"testing"

	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

var germanNav = `<header><nav>` + strings.Repeat(`<a href="/k">Startseite</a> <a href="/k">Neuheiten und Angebote für die ganze Familie</a> <a href="/k">Über uns und unsere Geschichte</a> <a href="/k">Versand und Zahlung</a> `, 12) + `</nav></header>`

// Spec: R2-7 — language is read from the content, not from the German menu around it.
func TestLanguageUsesContent(t *testing.T) {
	p := rules.Page{HTML: `<html lang="de"><body>` + germanNav + `<main><h1>Privacy policy</h1><p>This policy explains how we handle your personal data when you visit and order from our shop, which data we collect and how long we keep it. You can contact us at any time about your rights.</p></main><footer>Impressum · Datenschutz · AGB · Versand</footer></body></html>`}
	if l, ok := Language(p); !ok || l != "en" {
		t.Errorf("got (%q,%v), want (en,true)", l, ok)
	}
}

// Spec: R2-9 — the best-ranked link decides; a "#" settings link or a teaser never replaces an off-site policy.
func TestFindLinkedRanking(t *testing.T) {
	offsite := `<footer><a href="https://www.iubenda.com/privacy-policy/1">Datenschutzerklärung</a> <a href="#">Datenschutz-Einstellungen</a></footer>`
	in := rules.Input{Pages: []rules.Page{page("https://s.at/", offsite)}}
	if l := FindLinked(in, lexicon.Privacy); !l.Found || l.Loaded {
		t.Errorf("settings link: got %+v", l)
	}
	in = rules.Input{Pages: []rules.Page{
		page("https://s.at/", `<main><a href="/blog/datenschutz-tipps">Datenschutz im Alltag: unsere Tipps</a></main><footer><a href="https://legal.example/p">Datenschutz</a></footer>`),
		page("https://s.at/blog/datenschutz-tipps", ``),
	}}
	if l := FindLinked(in, lexicon.Privacy); !l.Found || l.Loaded {
		t.Errorf("teaser: got %+v", l)
	}
	in = rules.Input{Pages: []rules.Page{
		page("https://s.at/", `<main><a href="/blog/5-tipps">Datenschutz: 5 Tipps für Ihren Alltag</a></main><div class="site-footer"><a href="/datenschutz">Datenschutz</a></div>`),
		page("https://s.at/blog/5-tipps", ``), page("https://s.at/datenschutz", ``),
	}}
	if l := FindLinked(in, lexicon.Privacy); !l.Loaded || l.Page.URL != "https://s.at/datenschutz" {
		t.Errorf("div footer: got %+v", l)
	}
}

// Spec: R1-13 — modal links (#, javascript:, fragment without target) are found but not loaded.
func TestFindLinkedModal(t *testing.T) {
	for _, a := range []string{`<a href="#">Impressum</a>`, `<a href="javascript:void(0)">Impressum</a>`, `<a href="#legal">Impressum</a>`} {
		in := rules.Input{Pages: []rules.Page{page("https://s.at/", `<footer>`+a+`</footer>`)}}
		if l := FindLinked(in, lexicon.LegalNotice); !l.Found || l.Loaded {
			t.Errorf("%s: got %+v", a, l)
		}
	}
}

// Spec: R1-13, R5-8, R5-9 — common labels: "Rechtliches", refund policies, combined AGB & Widerruf, more terms names.
func TestFindLinkedLabels(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{page("https://s.at/", `<footer><a href="/p/3">Rechtliches</a></footer>`), page("https://s.at/p/3", ``)}}
	for _, k := range []lexicon.Kind{lexicon.LegalNotice, lexicon.Privacy} {
		if l := FindLinked(in, k); !l.Loaded {
			t.Errorf("Rechtliches %s: got %+v", k, l)
		}
	}
	in = rules.Input{Pages: []rules.Page{page("https://s.at/", `<footer><a href="/policies/refund-policy">Rückerstattungsrichtlinie</a> <a href="/?page_id=7">AGB &amp; Widerrufsbelehrung</a></footer>`),
		page("https://s.at/policies/refund-policy", ``), page("https://s.at/?page_id=7", ``)}}
	if l := FindLinked(in, lexicon.Withdrawal); !l.Loaded || l.Page.URL != "https://s.at/policies/refund-policy" {
		t.Errorf("refund: got %+v", l)
	}
	if l := FindLinked(in, lexicon.Terms); !l.Loaded || l.Page.URL != "https://s.at/?page_id=7" {
		t.Errorf("combined: got %+v", l)
	}
	for _, text := range []string{"Verkaufsbedingungen", "Lieferbedingungen", "Conditions of sale", "Nutzungsbedingungen"} {
		in := rules.Input{Pages: []rules.Page{page("https://s.at/", `<footer><a href="/x">`+text+`</a></footer>`), page("https://s.at/x", ``)}}
		if l := FindLinked(in, lexicon.Terms); !l.Loaded {
			t.Errorf("%s: got %+v", text, l)
		}
	}
	// a crawled page whose heading is "Impressum" counts even without a link
	in = rules.Input{Pages: []rules.Page{page("https://s.at/", ``), page("https://s.at/p/9", `<h1>Impressum</h1>`)}}
	if l := FindLinked(in, lexicon.LegalNotice); !l.Loaded || l.Page.URL != "https://s.at/p/9" {
		t.Errorf("heading: got %+v", l)
	}
}

// Spec: R5-8 — all withdrawal candidates, best first.
func TestFindAllLinked(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{page("https://s.at/", `<footer><a href="/formular">Widerrufsformular</a> <a href="/widerruf">Widerrufsbelehrung</a></footer>`),
		page("https://s.at/formular", ``), page("https://s.at/widerruf", ``)}}
	all := FindAllLinked(in, lexicon.Withdrawal)
	if len(all) != 2 || !all[0].Loaded || !all[1].Loaded {
		t.Errorf("got %+v", all)
	}
}

// Spec: R1-12 — unusual but valid spellings are found; unreadable ones are unsure, not missing.
func TestEmailSpellings(t *testing.T) {
	pg := func(body string) rules.Page { return rules.Page{HTML: `<html><body>` + body + `</body></html>`} }
	for _, body := range []string{"<p>E-Mail: info@müller-shop.de</p>", "<p>E-Mail: info(@)shop.at</p>", "<p>E-Mail: info at shop dot at</p>"} {
		p := pg(body)
		if f := Email(ContentText(p), p, de(), false); f.State != Found {
			t.Errorf("%s: %+v, want Found", body, f)
		}
	}
	for _, body := range []string{`<p>E-Mail: <span style="unicode-bidi:bidi-override;direction:rtl">ta.pohs@ofni</span></p>`,
		`<p>E-Mail: <a href="/cdn-cgi/l/email-protection#abc" class="__cf_email__" data-cfemail="abc">[email&#160;protected]</a></p>`} {
		p := pg(body)
		if f := Email(ContentText(p), p, de(), false); f.State != Unsure {
			t.Errorf("%s: %+v, want Unsure", body, f)
		}
	}
}

// Spec: R1-11, R2-8 — an iframe in the content marks embedded legal text.
func TestEmbedded(t *testing.T) {
	if !Embedded(rules.Page{HTML: `<body><main><h1>Datenschutz</h1><iframe src="https://legal.example/x"></iframe></main></body>`}) {
		t.Error("iframe not seen")
	}
	if Embedded(rules.Page{HTML: `<body><main><h1>Datenschutz</h1></main><footer><iframe src="map"></iframe></footer></body>`}) {
		t.Error("footer iframe is not the legal text")
	}
}
