package axecheck

import (
	"testing"

	"compliancechecker/internal/rules"
)

func page(url string, rep *rules.AxeReport) rules.Page { return rules.Page{URL: url, Axe: rep} }

// Spec: A1/A2/A3 result mapping.
func TestCheck(t *testing.T) {
	ids := []string{"color-contrast"}
	violation := &rules.AxeReport{Violations: []rules.AxeResult{{ID: "color-contrast", Nodes: []rules.AxeNode{{Selector: "p", HTML: "<p>"}}}}}
	otherViolation := &rules.AxeReport{Violations: []rules.AxeResult{{ID: "image-alt", Nodes: []rules.AxeNode{{Selector: "img"}}}}}
	incomplete := &rules.AxeReport{Incomplete: []rules.AxeResult{{ID: "color-contrast", Nodes: []rules.AxeNode{{Selector: "p"}}}}}
	inapplicable := &rules.AxeReport{Inapplicable: []string{"color-contrast"}}
	clean := &rules.AxeReport{}

	tests := []struct {
		name  string
		pages []rules.Page
		want  rules.State
	}{
		{"no pages", nil, rules.NotAssessable},
		{"axe did not run", []rules.Page{page("u", nil)}, rules.NotAssessable},
		{"violation", []rules.Page{page("u", violation)}, rules.Failed},
		{"violation of another rule is ignored", []rules.Page{page("u", otherViolation)}, rules.Passed},
		{"only incomplete", []rules.Page{page("u", incomplete)}, rules.NotAssessable},
		{"violation beats incomplete", []rules.Page{page("a", incomplete), page("b", violation)}, rules.Failed},
		{"inapplicable everywhere", []rules.Page{page("a", inapplicable), page("b", inapplicable)}, rules.NotApplicable},
		{"inapplicable on one page only", []rules.Page{page("a", inapplicable), page("b", clean)}, rules.Passed},
		{"clean", []rules.Page{page("u", clean)}, rules.Passed},
		{"page without axe next to clean page", []rules.Page{page("a", nil), page("b", clean)}, rules.Passed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Check(ids, rules.Input{Pages: tt.pages}).State; got != tt.want {
				t.Fatalf("state = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestFindingsCarryLocation(t *testing.T) {
	rep := &rules.AxeReport{Violations: []rules.AxeResult{{ID: "label", Nodes: []rules.AxeNode{
		{Selector: "input#q", HTML: `<input id="q">`}, {Selector: "input#r", HTML: `<input id="r">`},
	}}}}
	res := Check([]string{"label"}, rules.Input{Pages: []rules.Page{page("https://shop.at/", rep)}})
	if len(res.Findings) != 2 {
		t.Fatalf("findings = %d", len(res.Findings))
	}
	f := res.Findings[0]
	if f.PageURL != "https://shop.at/" || f.Selector != "input#q" || f.Snippet != `<input id="q">` || f.Sub != "label" {
		t.Fatalf("finding = %+v", f)
	}
	if res.DecidedBy != rules.DecidedStructural {
		t.Fatalf("decided_by = %s", res.DecidedBy)
	}
}
