// Command gen writes the static test shops (specs/09-testing.md, T-3) into testshops/site.
//
//	cd testshops/gen && go run . ../site
//
// The generated files are committed: they are what GitHub Pages serves and what the e2e test crawls.
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	out := "../site"
	if len(os.Args) > 1 {
		out = os.Args[1]
	}
	if err := os.RemoveAll(out); err != nil {
		log.Fatal(err)
	}
	all := shops()
	for _, s := range all {
		if err := writeShop(filepath.Join(out, s.Slug), s); err != nil {
			log.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(out, "index.html"), []byte(overview(all)), 0o644))
	must(os.WriteFile(filepath.Join(out, ".nojekyll"), nil, 0o644)) // serve files as they are
	log.Printf("wrote %d shops to %s", len(all), out)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

var e = html.EscapeString

// ---------- one shop ----------

type page struct{ key, file, title, body string }

func writeShop(dir string, s Shop) error {
	if err := os.MkdirAll(filepath.Join(dir, "img"), 0o755); err != nil {
		return err
	}
	pages := []page{
		{"index", "index.html", "Start", indexBody(s)},
		{"kategorie-1", "kategorie-1.html", s.Categories[0], categoryBody(s, 0)},
		{"kategorie-2", "kategorie-2.html", s.Categories[1], categoryBody(s, 1)},
	}
	for _, p := range s.Products {
		pages = append(pages, page{"produkt-" + p.Slug, "produkt-" + p.Slug + ".html", p.Name, productBody(s, p)})
	}
	pages = append(pages,
		page{"warenkorb", "warenkorb.html", "Warenkorb", cartBody(s)},
		page{"kasse", "kasse.html", "Kasse", checkoutBody(s)},
		page{"impressum", "impressum.html", "Impressum", impressumBody(s)},
		page{"datenschutz", "datenschutz.html", "Datenschutz", privacyBody(s)},
		page{"agb", "agb.html", "AGB", termsBody(s)},
	)
	if !s.Defects.NoWithdrawalPage {
		pages = append(pages, page{"widerruf", "widerruf.html", "Widerruf", withdrawalBody(s)})
	}
	for _, p := range pages {
		if err := os.WriteFile(filepath.Join(dir, p.file), []byte(layout(s, p)), 0o644); err != nil {
			return err
		}
	}
	for _, p := range s.Products {
		if err := os.WriteFile(filepath.Join(dir, "img", p.Slug+".svg"), []byte(productSVG(p)), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "style.css"), []byte(css(s)), 0o644); err != nil {
		return err
	}
	exp, err := json.MarshalIndent(expected(s, pages), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "expected.json"), append(exp, '\n'), 0o644)
}

func layout(s Shop, p page) string {
	d := s.Defects
	lang := ` lang="de"`
	if d.NoLangOn[p.key] {
		lang = "" // A6
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<!DOCTYPE html>
<html%s>
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex, nofollow">
  <title>%s – %s</title>
  <link rel="stylesheet" href="style.css">
</head>
<body>
<p class="testnote" role="note">Fiktiver Testshop für ein Software-Praktikum: keine echten Produkte, keine Bestellung möglich, alle Firmendaten sind erfunden.</p>
`, lang, e(p.title), e(s.Name))

	// cookie banner in the normal flow (a fixed overlay would make axe's contrast check "incomplete")
	b.WriteString(`<section id="cookie" class="cookie" aria-label="Cookie-Hinweis">
  <p><strong>Cookies</strong></p>
  <p>Wir verwenden technisch notwendige Cookies. Optionale Statistik-Cookies setzen wir nur mit Ihrer Zustimmung.</p>
  <p class="cookie-buttons">
    <button type="button" data-cookie="accept">Alle akzeptieren</button>
`)
	if !d.CookieNoReject {
		b.WriteString(`    <button type="button" data-cookie="reject">Nur notwendige</button>` + "\n") // R3
	}
	b.WriteString(`    <button type="button" data-cookie="settings" class="secondary">Einstellungen</button>
  </p>
</section>
`)

	fmt.Fprintf(&b, `<header class="site">
  <a class="brand" href="index.html">%s</a>
  <p class="tagline">%s</p>
  <nav aria-label="Hauptnavigation">
    <a href="index.html">Start</a>
    <a href="kategorie-1.html">%s</a>
    <a href="kategorie-2.html">%s</a>
    <a href="warenkorb.html">Warenkorb</a>
  </nav>
  <form class="search" role="search" action="#" onsubmit="return false">
`, e(s.Name), e(s.Tagline), e(s.Categories[0]), e(s.Categories[1]))
	if d.SearchNoLabel {
		b.WriteString(`    <input type="search" name="q">` + "\n") // A3: no label, no name at all
	} else {
		b.WriteString(`    <label for="q" class="sr-only">Suche im Shop</label>
    <input id="q" type="search" name="q">` + "\n")
	}
	b.WriteString(`    <button type="submit">Suchen</button>
  </form>
</header>
<main>
`)
	b.WriteString(p.body)
	b.WriteString("</main>\n<footer class=\"site\">\n")

	b.WriteString(`  <form class="newsletter" action="#" onsubmit="return false">` + "\n")
	if d.NewsletterPlaceholder {
		b.WriteString(`    <input type="email" name="nl" placeholder="Ihre E-Mail für den Newsletter">` + "\n") // A3: placeholder only
	} else {
		b.WriteString(`    <label for="nl">E-Mail für den Newsletter</label>
    <input id="nl" type="email" name="nl">` + "\n")
	}
	b.WriteString(`    <button type="submit">Anmelden</button>
  </form>
  <nav aria-label="Rechtliches">
    <a href="impressum.html">Impressum</a>
    <a href="datenschutz.html">Datenschutz</a>
    <a href="agb.html">AGB</a>
`)
	if !d.NoWithdrawalPage {
		b.WriteString(`    <a href="widerruf.html">Widerrufsbelehrung</a>` + "\n")
	}
	if d.WithdrawalFunction {
		b.WriteString(`    <a href="widerruf.html#formular" class="withdraw">Vertrag widerrufen</a>` + "\n") // R7
	}
	fmt.Fprintf(&b, `  </nav>
  <p>© 2026 %s</p>
</footer>
<script>
  document.querySelectorAll('[data-cookie]').forEach(function (b) {
    b.addEventListener('click', function () { document.getElementById('cookie').hidden = true; });
  });
</script>
</body>
</html>
`, e(s.Company.Name))
	return b.String()
}

func card(s Shop, p Product, headingTag string, withAlt bool) string {
	alt := fmt.Sprintf(` alt="%s, Produktfoto"`, e(p.Name))
	if !withAlt {
		alt = "" // A2
	}
	hint := "hint"
	if s.Defects.LowContrastHints {
		hint = "hint low" // A1
	}
	return fmt.Sprintf(`  <article class="card">
    <img src="img/%s.svg"%s width="160" height="120">
    <%s><a href="produkt-%s.html">%s</a></%s>
    <p>%s</p>
    <p class="price">%s</p>
    <p class="%s">inkl. 20 %% USt., zzgl. Versand</p>
  </article>
`, p.Slug, alt, headingTag, p.Slug, e(p.Name), headingTag, e(p.Desc), e(p.Price), hint)
}

func indexBody(s Shop) string {
	section := "h2"
	if s.Defects.SkipLevelOn["index"] {
		section = "h3" // A4: h1 -> h3
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>Willkommen bei %s</h1>\n<p>%s. Stöbern Sie in unseren Lieblingsprodukten.</p>\n<%s>Beliebte Produkte</%s>\n<div class=\"grid\">\n",
		e(s.Name), e(s.Tagline), section, section)
	for _, p := range s.Products {
		b.WriteString(card(s, p, "h3", true))
	}
	b.WriteString("</div>\n")
	return b.String()
}

func categoryBody(s Shop, i int) string {
	var prods []Product
	if i == 0 {
		prods = s.Products[:2]
	} else {
		prods = s.Products[2:]
	}
	cardTag := "h3"
	if s.Defects.SkipH2ToH4OnCategory {
		cardTag = "h4" // A4: h2 -> h4
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>%s</h1>\n<p>Alle Produkte der Kategorie %s.</p>\n<h2>Unsere Auswahl</h2>\n<div class=\"grid\">\n", e(s.Categories[i]), e(s.Categories[i]))
	for _, p := range prods {
		b.WriteString(card(s, p, cardTag, !s.Defects.ProductImgNoAlt))
	}
	b.WriteString("</div>\n")
	return b.String()
}

func productBody(s Shop, p Product) string {
	alt := e(p.Name) + ", Produktfoto"
	if s.Defects.FilenameAltProduct == p.Slug {
		if s.Slug == "radwerk" {
			alt = "IMG_4711.jpg" // A2: camera file name
		} else {
			alt = p.Slug + "-seife.jpg" // A2: file name
		}
	}
	hint := "hint"
	if s.Defects.LowContrastHints {
		hint = "hint low"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<h1>%s</h1>
<div class="product">
  <img src="img/%s.svg" alt="%s" width="320" height="240">
  <div>
    <p>%s</p>
    <p class="price">%s</p>
    <p class="%s">inkl. 20 %% USt., zzgl. Versand</p>
    <form action="#" onsubmit="return false">
      <label for="menge">Menge</label>
      <input id="menge" type="number" name="menge" value="1" min="1">
      <button type="submit">In den Warenkorb</button>
    </form>
  </div>
</div>
`, e(p.Name), p.Slug, alt, e(p.Desc), e(p.Price), hint)
	if s.Defects.DoubleH1OnProducts {
		b.WriteString("<h1>Produktdetails</h1>\n") // A4: second h1
	} else {
		b.WriteString("<h2>Produktdetails</h2>\n")
	}
	b.WriteString("<p>Sorgfältig ausgewählt und schnell verschickt. Lieferzeit 2–4 Werktage innerhalb Österreichs.</p>\n<h2>Versand und Rückgabe</h2>\n<p>Versand ab 4,90 €. Details finden Sie in den AGB.</p>\n")
	return b.String()
}

func cartBody(s Shop) string {
	p := s.Products[0]
	second := "<h2>Zusammenfassung</h2>"
	if s.Defects.EmptyHeadingOnCart {
		second = "<h2></h2>" // A4: empty heading
	}
	return fmt.Sprintf(`<h1>Warenkorb</h1>
<table class="cart">
  <caption>Ihr Warenkorb (Beispielinhalt)</caption>
  <thead><tr><th scope="col">Produkt</th><th scope="col">Menge</th><th scope="col">Preis</th></tr></thead>
  <tbody><tr><td>%s</td><td>1</td><td>%s</td></tr></tbody>
</table>
%s
<p>Zwischensumme: %s, zzgl. 4,90 € Versand.</p>
<p><a class="button" href="kasse.html">Zur Kasse</a></p>
`, e(p.Name), e(p.Price), second, e(p.Price))
}

func checkoutBody(s Shop) string {
	return fmt.Sprintf(`<h1>Kasse</h1>
<p>Dies ist ein Testshop: Beim Absenden passiert nichts, es wird nichts bestellt.</p>
<form class="checkout" action="#" onsubmit="return false">
  <h2>Lieferadresse</h2>
  <label for="name">Vor- und Nachname</label>
  <input id="name" name="name" autocomplete="name">
  <label for="street">Straße und Hausnummer</label>
  <input id="street" name="street" autocomplete="street-address">
  <label for="zip">Postleitzahl und Ort</label>
  <input id="zip" name="zip" autocomplete="postal-code">
  <label for="email">E-Mail-Adresse</label>
  <input id="email" type="email" name="email" autocomplete="email">
  <h2>Zahlungsart</h2>
  <fieldset>
    <legend>Bitte wählen</legend>
    <input id="pay-invoice" type="radio" name="pay" value="invoice" checked> <label for="pay-invoice">Rechnung</label>
    <input id="pay-card" type="radio" name="pay" value="card"> <label for="pay-card">Kreditkarte</label>
  </fieldset>
  <h2>Übersicht</h2>
  <p>%s, 1 Stück: %s, Versand 4,90 €.</p>
  <input id="terms" type="checkbox" name="terms"> <label for="terms">Ich habe die AGB gelesen.</label>
  <p><button type="submit" id="place_order">%s</button></p>
</form>
`, e(s.Products[0].Name), e(s.Products[0].Price), e(s.Defects.OrderButton))
}

func impressumBody(s Shop) string {
	c := s.Company
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>Impressum</h1>\n<p>Alle Angaben sind erfunden; dieser Shop dient nur zum Testen.</p>\n<p>%s<br>Inhaber/Geschäftsführung: %s<br>%s<br>%s</p>\n<p>",
		e(c.Name), e(c.Owner), e(c.Street), e(c.City))
	var lines []string
	if !s.Defects.ImpressumNoEmail {
		lines = append(lines, "E-Mail: "+e(c.Email)) // R1
	}
	lines = append(lines, "Telefon: "+e(c.Phone))
	if !s.Defects.ImpressumNoVAT {
		lines = append(lines, "UID-Nummer: "+e(c.VAT))
	}
	lines = append(lines, "Firmenbuch: "+e(c.Register))
	b.WriteString(strings.Join(lines, "<br>"))
	b.WriteString("</p>\n")
	return b.String()
}

func privacyBody(s Shop) string {
	if s.Defects.PrivacyShort {
		return "<h1>Datenschutzerklärung</h1>\n<p>Wir nehmen Datenschutz ernst.</p>\n" // R2
	}
	return fmt.Sprintf(`<h1>Datenschutzerklärung</h1>
<p>Fiktiver Beispieltext für den Testshop.</p>
<h2>Verantwortlicher</h2>
<p>%s, %s, %s, %s.</p>
<h2>Zwecke und Rechtsgrundlagen</h2>
<p>Wir verarbeiten Ihre Daten zur Abwicklung von Bestellungen (Art. 6 Abs. 1 lit. b DSGVO).</p>
<h2>Empfänger</h2>
<p>Versanddienstleister und Zahlungsanbieter, soweit für die Bestellung nötig.</p>
<h2>Speicherdauer</h2>
<p>Bis zum Ablauf der gesetzlichen Aufbewahrungsfristen.</p>
<h2>Ihre Rechte</h2>
<p>Auskunft, Berichtigung, Löschung, Einschränkung, Datenübertragbarkeit und Widerspruch.</p>
<h2>Beschwerderecht</h2>
<p>Sie können sich bei der Datenschutzbehörde beschweren.</p>
`, e(s.Company.Name), e(s.Company.Street), e(s.Company.City), e(s.Company.Email))
}

func termsBody(s Shop) string {
	note := ""
	if s.Defects.LowContrastOn["agb"] {
		note = `<p class="note-low">Stand: 1. November 2026 (fiktiver Text)</p>` + "\n" // A1
	}
	return "<h1>Allgemeine Geschäftsbedingungen</h1>\n" + note + `<p>Fiktive Beispiel-AGB für den Testshop.</p>
<h2>Geltungsbereich</h2>
<p>Diese AGB gelten für alle Bestellungen über diesen Shop.</p>
<h2>Vertragsschluss</h2>
<p>Der Vertrag kommt mit unserer Auftragsbestätigung zustande.</p>
<h2>Preise und Versand</h2>
<p>Alle Preise inkl. 20 % USt. Versand innerhalb Österreichs 4,90 €.</p>
`
}

func withdrawalBody(s Shop) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<h1>Widerrufsbelehrung</h1>
<h2>Widerrufsrecht</h2>
<p>Sie haben das Recht, binnen 14 Tagen ohne Angabe von Gründen diesen Vertrag zu widerrufen. Die Frist beginnt mit dem Tag, an dem Sie die Ware erhalten haben.</p>
<p>Um Ihr Widerrufsrecht auszuüben, informieren Sie uns (%s, %s, %s) mittels einer eindeutigen Erklärung.</p>
`, e(s.Company.Name), e(s.Company.Street), e(s.Company.City))
	if s.Defects.WithdrawalFunction {
		b.WriteString(`<h2 id="formular">Vertrag widerrufen</h2>
<form class="checkout" action="#" onsubmit="return false">
  <label for="order">Bestellnummer</label>
  <input id="order" name="order">
  <label for="w-email">E-Mail-Adresse</label>
  <input id="w-email" type="email" name="email">
  <p><button type="submit">Vertrag widerrufen</button></p>
</form>
`)
	}
	return b.String()
}

func productSVG(p Product) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="320" height="240" viewBox="0 0 320 240">
  <rect width="320" height="240" fill="%s"/>
  <text x="160" y="128" font-family="sans-serif" font-size="20" fill="#ffffff" text-anchor="middle">%s</text>
</svg>
`, p.Color, e(p.Name))
}

func css(s Shop) string {
	return fmt.Sprintf(`/* Test shop stylesheet. All text colours have at least 4.5:1 contrast, except the planted .low / .note-low. */
* { box-sizing: border-box; }
body { margin: 0; font-family: system-ui, -apple-system, "Segoe UI", sans-serif; color: #1a1a1a; background: #ffffff; line-height: 1.5; }
main, header.site, footer.site, .cookie, .testnote { max-width: 960px; margin: 0 auto; padding: 0 16px; }
a { color: %[1]s; }
a:focus-visible, button:focus-visible, input:focus-visible { outline: 3px solid %[1]s; outline-offset: 2px; }
.testnote { background: #fff3cd; color: #1a1a1a; padding: 8px 16px; margin: 0 auto; }
.cookie { background: #eef1f5; color: #1a1a1a; padding: 8px 16px; margin: 8px auto; border: 1px solid #c9d1db; }
.cookie p { margin: 4px 0; }
button, .button { background: %[1]s; color: #ffffff; border: 0; padding: 8px 14px; font: inherit; cursor: pointer; text-decoration: none; display: inline-block; }
button.secondary { background: #ffffff; color: %[1]s; border: 2px solid %[1]s; }
header.site { padding-top: 16px; padding-bottom: 16px; border-bottom: 1px solid #d0d0d0; }
.brand { font-size: 1.6rem; font-weight: 700; text-decoration: none; color: %[1]s; }
.tagline { margin: 0 0 8px; color: #444444; }
nav a { margin-right: 16px; }
.search { margin-top: 8px; }
.search input, .newsletter input, .checkout input, .product input { padding: 6px; border: 1px solid #767676; font: inherit; }
.grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 16px; }
.card { border: 1px solid #d0d0d0; padding: 12px; }
.card img, .product img { max-width: 100%%; height: auto; display: block; }
.price { font-weight: 700; color: %[2]s; }
.hint { color: #555555; font-size: 0.9rem; }
.hint.low { color: #aaaaaa; }
.note-low { color: #9a9a9a; }
.product { display: flex; flex-wrap: wrap; gap: 24px; }
.checkout label { display: block; margin-top: 8px; }
.checkout fieldset label, .checkout input[type=checkbox] + label { display: inline; }
.cart { border-collapse: collapse; }
.cart th, .cart td { border: 1px solid #d0d0d0; padding: 6px 10px; text-align: left; }
footer.site { margin-top: 32px; padding-top: 16px; padding-bottom: 24px; border-top: 1px solid #d0d0d0; }
.newsletter label { display: block; }
.sr-only { position: absolute; width: 1px; height: 1px; padding: 0; margin: -1px; overflow: hidden; clip: rect(0, 0, 0, 0); white-space: nowrap; border: 0; }
`, s.Primary, s.Accent)
}

// ---------- expected results ----------

type Expected struct {
	Shop          string            `json:"shop"`
	StartPath     string            `json:"startPath"`
	Kind          string            `json:"kind"`
	Expected      map[string]string `json:"expected"`
	ExpectedScore int               `json:"expectedScore"`
	Pages         []string          `json:"pages"`
	Planted       []Planted         `json:"planted"`
	Note          string            `json:"note"`
}

func expected(s Shop, pages []page) Expected {
	d := s.Defects
	var pl []Planted
	add := func(page, rule, what string) { pl = append(pl, Planted{page, rule, what}) }
	keys := func(m map[string]bool) []string {
		var k []string
		for x := range m {
			k = append(k, x)
		}
		sort.Strings(k)
		return k
	}
	for _, k := range keys(d.NoLangOn) {
		add(k+".html", "A6", "<html> ohne lang-Attribut")
	}
	if d.LowContrastHints {
		add("index.html, kategorie-*.html, produkt-*.html", "A1", "Preis-Hinweis in #aaaaaa auf Weiß (2,3:1)")
	}
	for _, k := range keys(d.LowContrastOn) {
		add(k+".html", "A1", "Stand-Hinweis in #9a9a9a auf Weiß (2,8:1)")
	}
	if d.ProductImgNoAlt {
		add("kategorie-*.html", "A2", "Produktbilder ohne alt-Attribut")
	}
	if d.FilenameAltProduct != "" {
		add("produkt-"+d.FilenameAltProduct+".html", "A2", "Alternativtext ist ein Dateiname")
	}
	if d.SearchNoLabel {
		add("alle Seiten (Kopfbereich)", "A3", "Suchfeld ohne Beschriftung")
	}
	if d.NewsletterPlaceholder {
		add("alle Seiten (Fußbereich)", "A3", "Newsletter-Feld nur mit Platzhalter")
	}
	for _, k := range keys(d.SkipLevelOn) {
		add(k+".html", "A4", "h1 direkt gefolgt von h3")
	}
	if d.DoubleH1OnProducts {
		add("produkt-*.html", "A4", "zwei h1-Überschriften")
	}
	if d.EmptyHeadingOnCart {
		add("warenkorb.html", "A4", "leere h2-Überschrift")
	}
	if d.SkipH2ToH4OnCategory {
		add("kategorie-*.html", "A4", "h2 direkt gefolgt von h4")
	}
	if d.ImpressumNoEmail {
		add("impressum.html", "R1", "E-Mail-Adresse fehlt")
	}
	if d.ImpressumNoVAT {
		add("impressum.html", "R1", "UID-Nummer fehlt (bedingte Angabe)")
	}
	if d.PrivacyShort {
		add("datenschutz.html", "R2", "Pflichtabschnitte fehlen")
	}
	if d.CookieNoReject {
		add("alle Seiten", "R3", "Cookie-Banner ohne Ablehnen-Möglichkeit auf erster Ebene")
	}
	if d.NoWithdrawalPage {
		add("—", "R5", "keine Widerrufsbelehrung und kein Link dazu")
	}
	switch d.OrderButton {
	case "Zahlungspflichtig bestellen":
	default:
		add("kasse.html", "R6", fmt.Sprintf("Bestellbutton heißt %q statt „zahlungspflichtig bestellen“", d.OrderButton))
	}
	if !d.WithdrawalFunction {
		add("—", "R7", "keine Funktion „Vertrag widerrufen“")
	}
	var files []string
	for _, p := range pages {
		files = append(files, p.file)
	}
	if pl == nil {
		pl = []Planted{}
	}
	return Expected{
		Shop: s.Name, StartPath: s.Slug + "/", Kind: s.Kind, Expected: s.Expected, ExpectedScore: s.ExpectedScore,
		Pages: files, Planted: pl,
		Note: "expected/expectedScore cover the implemented rules (A1-A4, A6). R-rule defects are planted and listed for later; the scanner does not check them yet.",
	}
}

// ---------- overview page ----------

func overview(all []Shop) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="robots" content="noindex, nofollow">
  <title>Testshops – Webshop-Check</title>
  <style>
    body { font-family: system-ui, sans-serif; color: #1a1a1a; max-width: 860px; margin: 32px auto; padding: 0 16px; line-height: 1.5; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid #c8c8c8; padding: 6px 10px; text-align: left; vertical-align: top; }
    a { color: #0b4a7a; }
  </style>
</head>
<body>
<h1>Testshops für den Webshop-Check</h1>
<p>Drei <strong>fiktive</strong> Webshops mit absichtlich eingebauten Fehlern, zum Testen des Webshop-Checks (Software-Praktikum WS 2026/27, Gruppe 9). Es gibt keine echten Produkte, nichts kann bestellt werden, alle Firmendaten sind erfunden.</p>
<p>Zum Testen die Adresse eines Shops kopieren und im Webshop-Check prüfen lassen. Die erwarteten Ergebnisse stehen in der Tabelle und in der Datei <code>expected.json</code> jedes Shops.</p>
<table>
  <thead><tr><th scope="col">Shop</th><th scope="col">Art</th><th scope="col">Erwartet (A1, A2, A3, A4, A6)</th><th scope="col">Erwarteter Score</th></tr></thead>
  <tbody>
`)
	for _, s := range all {
		var parts []string
		for _, r := range []string{"A1", "A2", "A3", "A4", "A6"} {
			state := "bestanden"
			if s.Expected[r] == "failed" {
				state = "nicht bestanden"
			}
			parts = append(parts, r+": "+state)
		}
		fmt.Fprintf(&b, "    <tr><td><a href=\"%s/\">%s</a></td><td>%s</td><td>%s</td><td>%d %%</td></tr>\n",
			s.Slug, e(s.Name), e(kindDe(s.Kind)), e(strings.Join(parts, ", ")), s.ExpectedScore)
	}
	b.WriteString(`  </tbody>
</table>
<p>Quelltext und Erklärung: <code>testshops/</code> im Projekt-Repository.</p>
</body>
</html>
`)
	return b.String()
}

func kindDe(k string) string {
	switch k {
	case "exemplary":
		return "vorbildlich, keine eingebauten Barrierefreiheitsfehler"
	case "many defects":
		return "viele Fehler auf vielen Seiten"
	case "mixed":
		return "gemischt, einzelne versteckte Fehler"
	}
	return k
}
