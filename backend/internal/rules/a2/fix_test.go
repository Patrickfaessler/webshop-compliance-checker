package a2

import (
	"testing"

	"compliancechecker/internal/rules"
)

func axeImg(src string) *rules.AxeReport {
	return &rules.AxeReport{Violations: []rules.AxeResult{{ID: "image-alt", Nodes: []rules.AxeNode{{Selector: "img", HTML: `<img src="` + src + `">`}}}}}
}

// Spec: A2-F — a product image gets the product name from its card as alt text; page values are escaped.
func TestA2FixProductName(t *testing.T) {
	res := run(t, `<html><body><div class="card"><img src="a.jpg"><h3>Lampe "Alpin" <b>X</b></h3></div></body></html>`, axeImg("a.jpg"))
	if len(res.Findings) != 1 || res.Findings[0].Fix == nil {
		t.Fatalf("findings = %+v", res.Findings)
	}
	f := res.Findings[0].Fix
	if f.TextDe != "Beschreibe das Bild in wenigen Worten, z. B. mit dem Produktnamen." || f.Code != `alt="Lampe &#34;Alpin&#34; X"` || f.NoteDe != "" {
		t.Errorf("fix = %+v", f)
	}
}

// Spec: A2-F — without a product name the generic alt text and a note are suggested; the page heading is not used.
func TestA2FixGeneric(t *testing.T) {
	res := run(t, `<html><body><h1>Teehaus</h1><header><img src="logo.png"></header></body></html>`, axeImg("logo.png"))
	f := res.Findings[0].Fix
	if f == nil || f.Code != `alt="Beschreibung des Bildes"` || f.NoteDe != `Beschreibung bitte anpassen. Rein dekorative Bilder bekommen alt="".` {
		t.Errorf("fix = %+v", f)
	}
}

// Spec: A2-F — file-name alt texts get a fix too.
func TestA2FixFilenameAlt(t *testing.T) {
	res := run(t, `<html><body><div><img src="b.jpg" alt="IMG_4711.jpg"><h2>Tasche</h2></div></body></html>`, nil)
	if len(res.Findings) != 1 || res.Findings[0].Fix == nil || res.Findings[0].Fix.Code != `alt="Tasche"` {
		t.Fatalf("findings = %+v", res.Findings)
	}
}

// Spec: A2-F — images sharing one src (lazy-load placeholder) get no product name.
func TestA2FixSharedSrc(t *testing.T) {
	page := `<html><body><div class="grid"><div class="card"><img src="ph.gif"><h3>Lampe</h3></div><div class="card"><img src="ph.gif"><h3>Tasche</h3></div></div></body></html>`
	res := run(t, page, &rules.AxeReport{Violations: []rules.AxeResult{{ID: "image-alt", Nodes: []rules.AxeNode{
		{Selector: ".card:nth-child(1) > img", HTML: `<img src="ph.gif">`}, {Selector: ".card:nth-child(2) > img", HTML: `<img src="ph.gif">`}}}}})
	for _, f := range res.Findings {
		if f.Fix == nil || f.Fix.Code != `alt="Beschreibung des Bildes"` {
			t.Errorf("%s: fix = %+v", f.Selector, f.Fix)
		}
	}
}

// Spec: A2-F — a section heading before the card is not the product name; a grid is not one card.
func TestA2FixNotSectionHeading(t *testing.T) {
	for _, page := range []string{
		`<html><body><section><h2>Unsere Bestseller</h2><div class="row"><div class="card"><img src="a.jpg"><span class="name">Lampe</span></div></div></section></body></html>`,
		`<html><body><div class="grid"><div class="card"><img src="b.jpg"><h3>Tasche</h3></div><div class="card"><img src="a.jpg"><span>Lampe</span></div></div></body></html>`,
	} {
		res := run(t, page, axeImg("a.jpg"))
		if f := res.Findings[0].Fix; f == nil || f.Code != `alt="Beschreibung des Bildes"` {
			t.Errorf("fix = %+v for %s", f, page)
		}
	}
	// a title before the image inside the image's own card still counts
	res := run(t, `<html><body><div class="card"><h3>Lampe</h3><img src="a.jpg"></div></body></html>`, axeImg("a.jpg"))
	if f := res.Findings[0].Fix; f == nil || f.Code != `alt="Lampe"` {
		t.Errorf("own card title: fix = %+v", f)
	}
}

// Spec: A2-F — an SVG graphic needs aria-label, not alt.
func TestA2FixSVG(t *testing.T) {
	res := run(t, `<html><body><svg role="img"></svg></body></html>`, &rules.AxeReport{Violations: []rules.AxeResult{{ID: "svg-img-alt",
		Nodes: []rules.AxeNode{{Selector: "svg", HTML: `<svg role="img">`}}}}})
	if f := res.Findings[0].Fix; f == nil || f.TextDe != "Gib der Grafik einen kurzen Namen." || f.Code != `aria-label="Beschreibung der Grafik"` || f.NoteDe != "Beschreibung bitte anpassen." {
		t.Errorf("fix = %+v", f)
	}
}
