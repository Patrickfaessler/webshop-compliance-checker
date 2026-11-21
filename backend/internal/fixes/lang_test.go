package fixes

import "testing"

// Spec: A6-F — the language for the lang fix is guessed from the visible text; unsure texts say so.
func TestDetectLang(t *testing.T) {
	de := "Willkommen in unserem Shop. Hier finden Sie die besten Tees aus aller Welt, und wir liefern schnell und sicher zu Ihnen nach Hause. Die Versandkosten sind ab einem Bestellwert von 40 Euro frei, und Sie können jederzeit mit uns in Kontakt treten, wenn Sie eine Frage zu einem Produkt oder zu Ihrer Bestellung haben."
	en := "Welcome to our shop. Here you will find the best teas from all over the world, and we deliver them fast and safely to your home. Shipping is free for orders over 40 euros, and you can contact us at any time if you have a question about a product or about your order."
	mix := "Willkommen in unserem Shop und wir liefern schnell. Welcome to our shop and we deliver fast. Die Tees sind gut und frisch. The teas are good and fresh."
	tests := []struct {
		name, text, lang string
		sure             bool
	}{
		{"de", de, "de", true},
		{"en", en, "en", true},
		{"short", "Shop 123 €", "", false},
		{"mixed", mix, "", false},
	}
	for _, tt := range tests {
		lang, sure := DetectLang(tt.text)
		if sure != tt.sure || (tt.sure && lang != tt.lang) {
			t.Errorf("%s: DetectLang = %q, %v; want %q, %v", tt.name, lang, sure, tt.lang, tt.sure)
		}
	}
}
