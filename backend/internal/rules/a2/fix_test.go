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
