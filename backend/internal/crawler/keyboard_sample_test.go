package crawler

import "testing"

// Spec: A5-5 — page types for the keyboard sample (rules/A5-keyboard.md, table "Modes and pages").
func TestClassifyPage(t *testing.T) {
	tests := []struct {
		url  string
		sig  pageSignals
		want PageType
	}{
		{"https://s.example/kasse.html", pageSignals{}, PageCheckout},
		{"https://s.example/checkout/", pageSignals{}, PageCheckout},
		{"https://s.example/cart/checkout", pageSignals{}, PageCheckout},
		{"https://s.example/warenkorb.html", pageSignals{}, PageCart},
		{"https://s.example/basket", pageSignals{}, PageCart},
		{"https://s.example/lampe", pageSignals{LDTypes: []string{"Product"}}, PageProduct},
		{"https://s.example/lampe", pageSignals{OGType: "product"}, PageProduct},
		{"https://s.example/produkt-lampe.html", pageSignals{}, PageProduct},
		{"https://s.example/kategorie-1.html", pageSignals{}, PageCategory},
		{"https://s.example/neu", pageSignals{LDTypes: []string{"ItemList"}}, PageCategory},
		{"https://s.example/angebote", pageSignals{ProductLinks: 4}, PageCategory},
		{"https://s.example/angebote", pageSignals{ProductLinks: 3}, PageOther},
		{"https://s.example/impressum.html", pageSignals{}, PageOther},
		{"https://kasse.example/impressum?next=/kasse", pageSignals{}, PageOther}, // host and query do not count
	}
	for _, tt := range tests {
		if got := classifyPage(tt.url, tt.sig); got != tt.want {
			t.Errorf("classifyPage(%s, %+v) = %q, want %q", tt.url, tt.sig, got, tt.want)
		}
	}
}

// Spec: A5-5 — mode Stichprobe: start page first, then the first page of each type, at most 5.
func TestSamplerSample(t *testing.T) {
	k := newKeyboardSampler(KeyboardOptions{})
	calls := []struct {
		url      string
		sig      pageSignals
		wantType PageType
		want     bool
	}{
		{"https://s.example/", pageSignals{}, PageStart, true},
		{"https://s.example/produkt-a.html", pageSignals{}, PageProduct, true},
		{"https://s.example/produkt-b.html", pageSignals{}, PageProduct, false},
		{"https://s.example/kategorie-1.html", pageSignals{}, PageCategory, true},
		{"https://s.example/warenkorb.html", pageSignals{}, PageCart, true},
		{"https://s.example/kasse.html", pageSignals{}, PageCheckout, true},
		{"https://s.example/impressum.html", pageSignals{}, PageOther, false},
	}
	probed := 0
	for _, c := range calls {
		typ, ok := k.decide(c.url, c.sig)
		if typ != c.wantType || ok != c.want {
			t.Errorf("decide(%s) = %q %v, want %q %v", c.url, typ, ok, c.wantType, c.want)
		}
		if ok {
			probed++
		}
	}
	if probed != MaxKeyboardSample {
		t.Errorf("probed %d pages, want %d", probed, MaxKeyboardSample)
	}
}

// Spec: A5-6 — mode "alle Seiten" probes every page.
func TestSamplerAll(t *testing.T) {
	k := newKeyboardSampler(KeyboardOptions{AllPages: true})
	for i, u := range []string{"https://s.example/", "https://s.example/produkt-a.html", "https://s.example/produkt-b.html", "https://s.example/impressum.html"} {
		typ, ok := k.decide(u, pageSignals{})
		if !ok {
			t.Errorf("%s not probed", u)
		}
		if (i == 0) != (typ == PageStart) {
			t.Errorf("%s: type %q", u, typ)
		}
	}
}
