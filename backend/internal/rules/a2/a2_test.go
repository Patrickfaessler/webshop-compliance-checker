package a2

import (
	"context"
	"testing"

	"compliancechecker/internal/rules"
)

func run(t *testing.T, htmlSrc string, axe *rules.AxeReport) rules.Result {
	t.Helper()
	res, err := New().Check(context.Background(), rules.Input{Pages: []rules.Page{{URL: "https://shop.at/", HTML: htmlSrc, Axe: axe}}})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// Spec: A2-3 — alt text that is a file name or camera name fails.
func TestFileNameAlt(t *testing.T) {
	clean := &rules.AxeReport{}
	for _, alt := range []string{"IMG_0012.jpg", "foto.png", "DSC1234", "image_7", "produkt-bild.webp", "Bild 12"} {
		res := run(t, `<html><body><img src="x" alt="`+alt+`"></body></html>`, clean)
		if res.State != rules.Failed || len(res.Findings) != 1 || res.Findings[0].Sub != "filename-alt" {
			t.Errorf("alt=%q: state=%s findings=%+v", alt, res.State, res.Findings)
		}
	}
	for _, alt := range []string{"Rotes T-Shirt", "Startseite", "Foto der Werkstatt in Salzburg", ""} {
		res := run(t, `<html><body><img src="x" alt="`+alt+`"></body></html>`, clean)
		if res.State == rules.Failed {
			t.Errorf("alt=%q must not fail: %+v", alt, res.Findings)
		}
	}
}

// Spec: A2-1 — axe violations (missing alt) fail the rule; selector and snippet come from axe.
func TestAxeViolation(t *testing.T) {
	rep := &rules.AxeReport{Violations: []rules.AxeResult{{ID: "image-alt", Nodes: []rules.AxeNode{{Selector: "img", HTML: `<img src="a.jpg">`}}}}}
	res := run(t, `<html><body><img src="a.jpg"></body></html>`, rep)
	if res.State != rules.Failed || res.Findings[0].Selector != "img" {
		t.Fatalf("%s %+v", res.State, res.Findings)
	}
}

// Spec: A2-2 — decorative images pass; no images is not applicable; axe missing is not assessable.
func TestStates(t *testing.T) {
	if s := run(t, `<html><body><img src="a" alt="" role="presentation"></body></html>`, &rules.AxeReport{}).State; s != rules.Passed {
		t.Errorf("decorative: %s", s)
	}
	if s := run(t, `<html><body></body></html>`, &rules.AxeReport{Inapplicable: axeIDs}).State; s != rules.NotApplicable {
		t.Errorf("no images: %s", s)
	}
	if s := run(t, `<html><body><img src="a" alt="Logo"></body></html>`, nil).State; s != rules.NotAssessable {
		t.Errorf("no axe: %s", s)
	}
	// a DOM finding is decisive even without axe
	if s := run(t, `<html><body><img src="a" alt="IMG_1.jpg"></body></html>`, nil).State; s != rules.Failed {
		t.Errorf("filename alt without axe: %s", s)
	}
}

func TestMeta(t *testing.T) {
	if m := New().Meta(); m.ID != "A2" || m.Severity != rules.Warning || m.AIFallback {
		t.Fatalf("%+v", m)
	}
}
