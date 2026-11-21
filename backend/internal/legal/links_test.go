package legal

import (
	"testing"

	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

func page(url, body string) rules.Page {
	return rules.Page{URL: url, RequestedURL: url, HTML: `<html lang="de"><body>` + body + `</body></html>`}
}

// Spec: R1-3 — no matching link and no matching page.
func TestFindLinkedNone(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{page("https://s.at/", `<a href="/produkt">Lampe</a>`), page("https://s.at/produkt", ``)}}
	if l := FindLinked(in, lexicon.LegalNotice); l.Found {
		t.Errorf("found %+v", l)
	}
}

// Spec: R1-1 — footer link to a crawled Impressum.
func TestFindLinkedFooter(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{
		page("https://s.at/", `<main><a href="/produkt">Lampe</a></main><footer><a href="/impressum">Impressum</a></footer>`),
		page("https://s.at/impressum", `<h1>Impressum</h1>`),
	}}
	l := FindLinked(in, lexicon.LegalNotice)
	if !l.Found || !l.Loaded || l.Page == nil || l.Page.URL != "https://s.at/impressum" || l.Href != "https://s.at/impressum" {
		t.Errorf("got %+v", l)
	}
}

// Spec: C-2, R1-1 — a link that redirected is matched by the requested URL.
func TestFindLinkedRedirect(t *testing.T) {
	imp := rules.Page{URL: "https://s.at/de/impressum/", RequestedURL: "https://s.at/impressum", HTML: `<h1>Impressum</h1>`}
	in := rules.Input{Pages: []rules.Page{page("https://s.at/", `<footer><a href="/impressum">Impressum</a></footer>`), imp}}
	if l := FindLinked(in, lexicon.LegalNotice); !l.Loaded || l.Page.URL != imp.URL {
		t.Errorf("got %+v", l)
	}
	// trailing slash difference alone
	in.Pages[1] = page("https://s.at/impressum/", `<h1>Impressum</h1>`)
	if l := FindLinked(in, lexicon.LegalNotice); !l.Loaded {
		t.Errorf("trailing slash: got %+v", l)
	}
}

// Spec: R1-9 — a link to another domain is found but not loaded.
func TestFindLinkedOffsite(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{page("https://s.at/", `<footer><a href="https://legal-texts.example/s.at/impressum">Impressum</a></footer>`)}}
	l := FindLinked(in, lexicon.LegalNotice)
	if !l.Found || l.Loaded || l.Page != nil || l.Href != "https://legal-texts.example/s.at/impressum" {
		t.Errorf("got %+v", l)
	}
}

// Spec: R1-1 — a same-page anchor points at the start page itself.
func TestFindLinkedFragment(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{page("https://s.at/", `<footer><a href="#impressum">Impressum</a></footer><section id="impressum">…</section>`)}}
	l := FindLinked(in, lexicon.LegalNotice)
	if !l.Loaded || l.Page != &in.Pages[0] {
		t.Errorf("got %+v", l)
	}
}

// Spec: R5-1 — terms and withdrawal links are told apart; English link text works; a matching path without a link counts.
func TestFindLinkedKinds(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{
		page("https://s.at/", `<footer><a href="/widerruf">Widerrufsbelehrung</a> <a href="/agb">AGB</a> <a href="/p/9">Text in English: Terms &amp; Conditions</a></footer>`),
		page("https://s.at/agb", ``), page("https://s.at/widerruf", ``), page("https://s.at/p/9", ``), page("https://s.at/datenschutz", ``),
	}}
	if l := FindLinked(in, lexicon.Terms); !l.Loaded || l.Page.URL != "https://s.at/agb" {
		t.Errorf("terms: got %+v", l)
	}
	if l := FindLinked(in, lexicon.Withdrawal); !l.Loaded || l.Page.URL != "https://s.at/widerruf" {
		t.Errorf("withdrawal: got %+v", l)
	}
	in.Pages[0] = page("https://s.at/", `<footer><a href="/p/9">Terms &amp; Conditions</a></footer>`)
	if l := FindLinked(in, lexicon.Terms); !l.Loaded || l.Page.URL != "https://s.at/p/9" {
		t.Errorf("english text: got %+v", l)
	}
	if l := FindLinked(in, lexicon.Privacy); !l.Found || !l.Loaded || l.Page.URL != "https://s.at/datenschutz" {
		t.Errorf("path only: got %+v", l)
	}
}

// Spec: R1-1 — corpus pages (file URLs) link to each other.
func TestFindLinkedFileURLs(t *testing.T) {
	in := rules.Input{Pages: []rules.Page{page("file:///corpus/index.html", `<footer><a href="impressum.html">Impressum</a></footer>`), page("file:///corpus/impressum.html", ``)}}
	if l := FindLinked(in, lexicon.LegalNotice); !l.Loaded {
		t.Errorf("got %+v", l)
	}
}
