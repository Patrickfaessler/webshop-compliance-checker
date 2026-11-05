package a6

import (
	"context"
	"testing"

	"compliancechecker/internal/rules"
)

func check(t *testing.T, pages ...string) rules.Result {
	t.Helper()
	in := rules.Input{}
	for i, h := range pages {
		in.Pages = append(in.Pages, rules.Page{URL: "https://shop.example/p" + string(rune('0'+i)), HTML: h})
	}
	res, err := New().Check(context.Background(), in)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	return res
}

func TestCheck(t *testing.T) {
	tests := []struct {
		spec  string
		name  string
		pages []string
		want  rules.State
	}{
		{"A6-1", "lang=de passes", []string{`<!DOCTYPE html><html lang="de"><head></head><body></body></html>`}, rules.Passed},
		{"A6-2", "missing lang fails", []string{`<!DOCTYPE html><html><head></head><body></body></html>`}, rules.Failed},
		{"A6-3", "empty lang fails", []string{`<html lang=""><body></body></html>`}, rules.Failed},
		{"A6-3", "malformed lang fails", []string{`<html lang="deutsch"><body></body></html>`}, rules.Failed},
		{"A6-3", "whitespace only fails", []string{`<html lang="  "><body></body></html>`}, rules.Failed},
		{"A6-4", "region subtag passes", []string{`<html lang="de-AT"><body></body></html>`}, rules.Passed},
		{"A6", "one bad page among good ones fails", []string{`<html lang="de"></html>`, `<html></html>`}, rules.Failed},
		{"A6", "no pages is not assessable", nil, rules.NotAssessable},
	}
	for _, tt := range tests {
		t.Run(tt.spec+" "+tt.name, func(t *testing.T) { // Spec: see tt.spec
			if got := check(t, tt.pages...).State; got != tt.want {
				t.Fatalf("state = %s, want %s", got, tt.want)
			}
		})
	}
}

// Spec: A6-2 — finding carries selector `html` and the opening tag as snippet.
func TestFindingLocation(t *testing.T) {
	res := check(t, `<!DOCTYPE html><html class="no-js"><body></body></html>`)
	if len(res.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(res.Findings))
	}
	f := res.Findings[0]
	if f.Selector != "html" {
		t.Errorf("selector = %q, want html", f.Selector)
	}
	if f.Snippet != `<html class="no-js">` {
		t.Errorf("snippet = %q", f.Snippet)
	}
	if res.DecidedBy != rules.DecidedStructural {
		t.Errorf("decided_by = %s, want structural", res.DecidedBy)
	}
}

// Spec: A6 Meta — severity notice (weight 1), no AI fallback.
func TestMeta(t *testing.T) {
	m := New().Meta()
	if m.ID != "A6" || m.Severity != rules.Notice || m.AIFallback {
		t.Fatalf("unexpected meta: %+v", m)
	}
}
