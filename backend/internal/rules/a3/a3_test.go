package a3

import (
	"context"
	"testing"

	"compliancechecker/internal/rules"
)

func run(t *testing.T, body string, axe *rules.AxeReport) rules.Result {
	t.Helper()
	res, err := New().Check(context.Background(), rules.Input{Pages: []rules.Page{{
		URL: "https://shop.at/", HTML: `<html><body>` + body + `</body></html>`, Axe: axe,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// Spec: A3-2 — placeholder alone is not an accessible name.
func TestPlaceholderOnly(t *testing.T) {
	clean := &rules.AxeReport{}
	for name, body := range map[string]string{
		"input":       `<input type="text" placeholder="Suche">`,
		"search":      `<input type="search" placeholder="Suche">`,
		"textarea":    `<textarea placeholder="Nachricht"></textarea>`,
		"empty label": `<label for="q"> </label><input id="q" placeholder="Suche">`,
	} {
		res := run(t, body, clean)
		if res.State != rules.Failed || len(res.Findings) != 1 || res.Findings[0].Sub != "placeholder-only" {
			t.Errorf("%s: state=%s findings=%+v", name, res.State, res.Findings)
		}
	}
}

// Spec: A3-1, A3-3 — a real label, aria-label, title or wrapping label counts.
func TestNamedFieldsPass(t *testing.T) {
	clean := &rules.AxeReport{}
	for name, body := range map[string]string{
		"label for":        `<label for="q">Suche</label><input id="q" placeholder="z. B. Schuhe">`,
		"aria-label":       `<input aria-label="Suche" placeholder="Suche">`,
		"labelledby":       `<span id="l">Suche</span><input aria-labelledby="l" placeholder="Suche">`,
		"title":            `<input title="Suche" placeholder="Suche">`,
		"wrapping":         `<label>Suche <input placeholder="Suche"></label>`,
		"hidden":           `<input type="hidden" placeholder="x">`,
		"button":           `<input type="submit" placeholder="x" value="Los">`,
		"hidden by markup": `<div hidden><input placeholder="Suche"></div>`,
		"no placeholder":   `<input type="text">`, // plain missing labels are axe's job
	} {
		if res := run(t, body, clean); res.State != rules.Passed {
			t.Errorf("%s: state=%s findings=%+v", name, res.State, res.Findings)
		}
	}
}

// Spec: A3-1 — axe violations fail; no axe is not assessable.
func TestAxeAndStates(t *testing.T) {
	rep := &rules.AxeReport{Violations: []rules.AxeResult{{ID: "label", Nodes: []rules.AxeNode{{Selector: "input", HTML: `<input name="q">`}}}}}
	if res := run(t, `<input name="q">`, rep); res.State != rules.Failed || res.Findings[0].Selector != "input" {
		t.Fatalf("%s %+v", res.State, res.Findings)
	}
	if s := run(t, `<input aria-label="x">`, nil).State; s != rules.NotAssessable {
		t.Errorf("no axe: %s", s)
	}
	if s := run(t, `<p>kein Formular</p>`, &rules.AxeReport{Inapplicable: axeIDs}).State; s != rules.NotApplicable {
		t.Errorf("no forms: %s", s)
	}
}

func TestMeta(t *testing.T) {
	if m := New().Meta(); m.ID != "A3" || m.Severity != rules.Critical {
		t.Fatalf("%+v", m)
	}
}
