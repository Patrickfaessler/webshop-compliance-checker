package crawler

import "testing"

// Spec: C-1 (robots.txt)
func TestRobots(t *testing.T) {
	const txt = `
# comment
User-agent: *
Disallow: /admin
Disallow: /cart$
Allow: /admin/public
Disallow: /*.pdf$

User-agent: ComplianceChecker
Disallow: /private
`
	generic := ParseRobots(txt, "SomeOtherBot")
	for path, want := range map[string]bool{
		"/":                  true,
		"/shop/item":         true,
		"/admin":             false,
		"/admin/x":           false,
		"/admin/public/page": true, // longer Allow wins
		"/cart":              false,
		"/cart/extra":        true, // $ anchors
		"/files/a.pdf":       false,
		"/files/a.pdf?x=1":   true, // $ anchors, query follows
		"":                   true,
	} {
		if got := generic.Allowed(path); got != want {
			t.Errorf("generic %q: got %v want %v", path, got, want)
		}
	}

	// our own group replaces "*"
	ours := ParseRobots(txt, "ComplianceCheckerBot")
	if !ours.Allowed("/admin") {
		t.Error("own group has no /admin rule, so it must be allowed")
	}
	if ours.Allowed("/private/x") {
		t.Error("/private must be disallowed for us")
	}
}

func TestRobotsEmptyAndNil(t *testing.T) {
	if !ParseRobots("", "x").Allowed("/anything") {
		t.Error("empty robots allows everything")
	}
	if !ParseRobots("User-agent: *\nDisallow:", "x").Allowed("/anything") {
		t.Error("empty Disallow allows everything")
	}
	var r *Robots
	if !r.Allowed("/x") {
		t.Error("nil robots allows everything")
	}
	if ParseRobots("User-agent: *\nDisallow: /", "x").Allowed("/x") {
		t.Error("Disallow: / blocks everything")
	}
}
