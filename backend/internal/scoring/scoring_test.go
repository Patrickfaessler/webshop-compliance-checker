package scoring

import (
	"testing"

	"compliancechecker/internal/rules"
)

// Spec: S-1, S-4
func TestScore(t *testing.T) {
	tests := []struct {
		name   string
		in     []Outcome
		want   int
		wantOK bool
	}{
		{"empty", nil, 0, false},
		{"all passed", []Outcome{{rules.Critical, rules.Passed}, {rules.Notice, rules.Passed}}, 100, true},
		{"all failed", []Outcome{{rules.Critical, rules.Failed}}, 0, true},
		{
			"weighted: critical passed (3), warning failed (2)",
			[]Outcome{{rules.Critical, rules.Passed}, {rules.Warning, rules.Failed}},
			60, true,
		},
		{
			"not_applicable and not_assessable are ignored",
			[]Outcome{
				{rules.Critical, rules.Passed},
				{rules.Critical, rules.NotApplicable},
				{rules.Warning, rules.NotAssessable},
			},
			100, true,
		},
		{
			"only ignored states gives no score",
			[]Outcome{{rules.Critical, rules.NotApplicable}, {rules.Warning, rules.NotAssessable}},
			0, false,
		},
		{"rounding: 1 of 3 notices = 33", []Outcome{
			{rules.Notice, rules.Passed}, {rules.Notice, rules.Failed}, {rules.Notice, rules.Failed},
		}, 33, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Score(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("Score() = (%d, %v), want (%d, %v)", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
