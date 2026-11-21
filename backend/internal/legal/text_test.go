package legal

import (
	"strings"
	"testing"

	"compliancechecker/internal/rules"
)

// Spec: R1-10 — content text leaves out header, nav, footer, aside and forms; full text keeps them.
func TestContentTextDropsFooter(t *testing.T) {
	p := rules.Page{HTML: `<html><body><header>Shop</header><nav>Menü</nav><main>Hauptstraße 1</main><aside>Tipp</aside><form><label>Suche</label></form><footer>Newsletter E-Mail</footer></body></html>`}
	c := ContentText(p)
	if !strings.Contains(c, "Hauptstraße 1") {
		t.Errorf("content = %q", c)
	}
	for _, w := range []string{"Shop", "Menü", "Tipp", "Suche", "Newsletter"} {
		if strings.Contains(c, w) {
			t.Errorf("content contains %q: %q", w, c)
		}
	}
	f := FullText(p)
	if !strings.Contains(f, "Hauptstraße 1") || !strings.Contains(f, "Newsletter E-Mail") {
		t.Errorf("full = %q", f)
	}
	if got := ContentText(rules.Page{HTML: `<body><footer>x  y</footer></body>`}); got != "x y" {
		t.Errorf("fallback = %q, want %q", got, "x y")
	}
}

const terse = `<h1>Impressum</h1><p>Radwerk Alpin GmbH</p><p>Inhaber: Max Muster</p><p>Anschrift: Hauptstraße 1, 6020 Innsbruck</p><p>Firmenbuch: FN 123456a</p><p>UID: ATU12345678</p>`

// Spec: R1-1 — a terse German Impressum is read as German even without a usable lang attribute.
func TestLanguageTerseImpressum(t *testing.T) {
	if l, ok := Language(rules.Page{HTML: `<html><body>` + terse + `</body></html>`}); !ok || l != "de" {
		t.Errorf("no lang: got (%q,%v), want (de,true)", l, ok)
	}
	if l, ok := Language(rules.Page{HTML: `<html lang="fr"><body>` + terse + `</body></html>`}); !ok || l != "de" {
		t.Errorf("mis-declared fr: got (%q,%v), want (de,true)", l, ok)
	}
	if l, ok := Language(rules.Page{HTML: `<html lang="en-GB"><body><p>Hello</p></body></html>`}); !ok || l != "en" {
		t.Errorf("lang en-GB: got (%q,%v), want (en,true)", l, ok)
	}
	if _, ok := Language(rules.Page{HTML: `<html lang="fr"><body><h1>Mentions légales</h1><p>Société Exemple, rue de la Paix</p></body></html>`}); ok {
		t.Error("French page should not be assessable")
	}
}
