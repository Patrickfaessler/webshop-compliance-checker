package a5

import (
	"testing"

	"compliancechecker/internal/rules"
)

// Spec: A5-F — each A5 sub-rule gets its fix; the selector in the code is escaped.
func TestA5Fix(t *testing.T) {
	k := &rules.KeyboardReport{MaxPresses: 200, Presses: 10, Completed: true,
		NoFocusStyle: []rules.KeyboardNode{{Selector: "#b"}, {Selector: `a[title="x"]`}},
		NotFocusable: []rules.KeyboardNode{{Selector: "#d"}},
		Unreachable:  []rules.KeyboardNode{{Selector: "#u"}},
		Traps:        []rules.KeyboardNode{{Selector: "#t"}},
	}
	r := check(t, rules.Page{URL: "https://s.at/", Keyboard: k})
	got := map[string]*rules.Fix{}
	for _, f := range r.Findings {
		if f.Fix == nil || f.Fix.TextDe == "" {
			t.Fatalf("%s %s: no fix", f.Sub, f.Selector)
		}
		got[f.Selector] = f.Fix
	}
	if c := got["#b"].Code; c != "#b:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }" {
		t.Errorf("no-focus-style code = %q", c)
	}
	if c := got[`a[title="x"]`].Code; c != "a[title=&#34;x&#34;]:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }" {
		t.Errorf("escaped selector code = %q", c)
	}
	if c := got["#d"].Code; c != `<button type="button">…</button>` {
		t.Errorf("not-focusable code = %q", c)
	}
	if got["#u"].Code != "" || got["#t"].Code != "" {
		t.Errorf("unreachable/trap have code: %+v %+v", got["#u"], got["#t"])
	}
	if got["#t"].TextDe != "Sorge dafür, dass man das Element mit der Tab-Taste wieder verlassen kann." {
		t.Errorf("trap text = %q", got["#t"].TextDe)
	}
}
