package a5

import (
	"context"
	"testing"

	"compliancechecker/internal/rules"
)

func check(t *testing.T, pages ...rules.Page) rules.Result {
	t.Helper()
	r, err := New().Check(context.Background(), rules.Input{Pages: pages})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func done() *rules.KeyboardReport {
	return &rules.KeyboardReport{MaxPresses: 200, Presses: 10, Completed: true}
}

// Spec: A5 Result mapping — no probed page → not_assessable.
func TestNoProbedPage(t *testing.T) {
	if r := check(t, rules.Page{URL: "https://s.example/"}); r.State != rules.NotAssessable {
		t.Errorf("state = %s", r.State)
	}
}

// Spec: A5 Result mapping, A5-2 — a finding fails the rule with sub-rule, selector, snippet and page.
func TestFindingFails(t *testing.T) {
	k := done()
	k.NoFocusStyle = []rules.KeyboardNode{{Selector: "#b", Snippet: `<button id="b">`}}
	k.Traps = []rules.KeyboardNode{{Selector: "#t", Snippet: "<input id=\"t\">"}}
	r := check(t, rules.Page{URL: "https://s.example/p", Keyboard: k})
	if r.State != rules.Failed || r.DecidedBy != rules.DecidedStructural || len(r.Findings) != 2 {
		t.Fatalf("result = %+v", r)
	}
	f := r.Findings[0]
	if f.PageURL != "https://s.example/p" || f.Selector != "#b" || f.Snippet != `<button id="b">` || f.Sub != "no-focus-style" {
		t.Errorf("finding = %+v", f)
	}
	if r.Findings[1].Sub != "keyboard-trap" {
		t.Errorf("sub = %q", r.Findings[1].Sub)
	}
}

// Spec: A5 Result mapping, A5-1 — all loops completed, nothing found → passed without note.
func TestAllCompletedPasses(t *testing.T) {
	r := check(t, rules.Page{URL: "a", Keyboard: done()}, rules.Page{URL: "b"})
	if r.State != rules.Passed || r.NoteDe != "" || r.DecidedBy != rules.DecidedStructural {
		t.Errorf("result = %+v", r)
	}
}

// Spec: A5-7 — limit reached on one of three probed pages, no finding → passed with the note.
func TestLimitReachedPassesWithNote(t *testing.T) {
	capped := &rules.KeyboardReport{MaxPresses: 200, Presses: 200}
	r := check(t, rules.Page{URL: "a", Keyboard: done()}, rules.Page{URL: "b", Keyboard: capped}, rules.Page{URL: "c", Keyboard: done()})
	want := "Erreichbarkeit nur für die ersten 200 Bedienelemente geprüft (Limit erreicht auf 1 von 3 geprüften Seiten)."
	if r.State != rules.Passed || r.NoteDe != want {
		t.Errorf("result = %s %q, want passed %q", r.State, r.NoteDe, want)
	}
}

// Spec: A5 Logic 3 — every sub-rule name.
func TestSubRules(t *testing.T) {
	k := done()
	n := []rules.KeyboardNode{{Selector: "x"}}
	k.Unreachable, k.NoFocusStyle, k.Traps, k.NotFocusable = n, n, n, n
	r := check(t, rules.Page{URL: "a", Keyboard: k})
	got := map[string]bool{}
	for _, f := range r.Findings {
		got[f.Sub] = true
	}
	for _, s := range []string{"unreachable", "no-focus-style", "keyboard-trap", "not-focusable"} {
		if !got[s] {
			t.Errorf("sub-rule %s missing", s)
		}
	}
}
