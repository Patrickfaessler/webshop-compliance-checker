package a4

import (
	"context"
	"testing"

	"compliancechecker/internal/rules"
)

func run(t *testing.T, body string) rules.Result {
	t.Helper()
	res, err := New().Check(context.Background(), rules.Input{Pages: []rules.Page{{
		URL: "https://shop.at/", HTML: `<!DOCTYPE html><html lang="de"><head><title>t</title></head><body>` + body + `</body></html>`,
	}}})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func subs(res rules.Result) []string {
	var s []string
	for _, f := range res.Findings {
		s = append(s, f.Sub)
	}
	return s
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Spec: A4-1..A4-4
func TestCheck(t *testing.T) {
	tests := []struct {
		spec  string
		name  string
		body  string
		state rules.State
		subs  []string
	}{
		{"A4-1", "h1 h2 h3 passes", `<h1>A</h1><h2>B</h2><h3>C</h3>`, rules.Passed, nil},
		{"A4-1", "going back up is fine", `<h1>A</h1><h2>B</h2><h3>C</h3><h2>D</h2>`, rules.Passed, nil},
		{"A4-2", "h1 then h3 skips a level", `<h1>A</h1><h3>C</h3>`, rules.Failed, []string{"skipped-level"}},
		{"A4-3", "two h1", `<h1>A</h1><h2>B</h2><h1>C</h1>`, rules.Failed, []string{"multi-h1"}},
		{"A4-4", "empty h2", `<h1>A</h1><h2></h2>`, rules.Failed, []string{"empty"}},
		{"A4", "empty with whitespace and tags", `<h1>A</h1><h2> <span> </span> </h2>`, rules.Failed, []string{"empty"}},
		{"A4", "heading with only an image alt is not empty", `<h1><img src="x" alt="Shop"></h1>`, rules.Passed, nil},
		{"A4", "aria-label makes it non-empty", `<h1 aria-label="Shop"></h1>`, rules.Passed, nil},
		{"A4", "no headings at all", `<p>Text</p>`, rules.Failed, []string{"no-h1"}},
		{"A4", "starts with h2: no h1", `<h2>A</h2>`, rules.Failed, []string{"no-h1"}},
		{"A4", "role=heading with aria-level", `<div role="heading" aria-level="1">A</div><div role="heading" aria-level="2">B</div>`, rules.Passed, nil},
		{"A4", "role=heading default level is 2", `<h1>A</h1><div role="heading">B</div>`, rules.Passed, nil},
		{"A4", "hidden headings are ignored", `<h1>A</h1><h4 hidden>x</h4><h2>B</h2>`, rules.Passed, nil},
		{"A4", "aria-hidden subtree is ignored", `<h1>A</h1><div aria-hidden="true"><h5>x</h5></div>`, rules.Passed, nil},
		{"A4", "inline display:none is ignored", `<h1>A</h1><h3 style="display: none">x</h3>`, rules.Passed, nil},
		{"A4", "several problems at once", `<h2>A</h2><h4></h4>`, rules.Failed, []string{"skipped-level", "empty", "no-h1"}},
	}
	for _, tt := range tests {
		t.Run(tt.spec+" "+tt.name, func(t *testing.T) {
			res := run(t, tt.body)
			if res.State != tt.state || !same(subs(res), tt.subs) {
				t.Fatalf("state=%s subs=%v, want %s %v", res.State, subs(res), tt.state, tt.subs)
			}
			if res.DecidedBy != rules.DecidedStructural {
				t.Fatalf("decided_by = %s", res.DecidedBy)
			}
		})
	}
}

// Spec: A4 — finding names the offending heading with a selector and snippet.
func TestFindingLocation(t *testing.T) {
	res := run(t, `<main><h1>A</h1><h3 class="x">C</h3></main>`)
	if len(res.Findings) != 1 {
		t.Fatalf("%+v", res.Findings)
	}
	f := res.Findings[0]
	if f.Selector != "html > body > main > h3" || f.Snippet != `<h3 class="x">` || f.PageURL != "https://shop.at/" {
		t.Fatalf("finding = %+v", f)
	}
}

func TestNoPagesNotAssessable(t *testing.T) {
	res, _ := New().Check(context.Background(), rules.Input{})
	if res.State != rules.NotAssessable {
		t.Fatalf("state = %s", res.State)
	}
}
