// Package corpus runs every case in testcorpus/cases against the rules and compares the
// result with expected.json. Spec: specs/09-testing.md (T-1, T-2), testcorpus/README.md.
//
// Pages are rendered in a real headless browser (so axe-core runs) when one is installed;
// cases whose rules need axe are skipped otherwise.
package corpus

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"compliancechecker/internal/crawler"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/a1"
	"compliancechecker/internal/rules/a2"
	"compliancechecker/internal/rules/a3"
	"compliancechecker/internal/rules/a4"
	"compliancechecker/internal/rules/a6"
)

const casesDir = "../../../testcorpus/cases"

type expected struct {
	Spec   []string          `json:"spec"`
	Kind   string            `json:"kind"` // "defect" or "clean"
	Expect map[string]string `json:"expect"`
}

// allRules lists the rules the corpus can run. Add each new rule here.
func allRules() map[string]rules.Rule {
	list := []rules.Rule{a1.New(), a2.New(), a3.New(), a4.New(), a6.New()}
	m := make(map[string]rules.Rule, len(list))
	for _, r := range list {
		m[r.Meta().ID] = r
	}
	return m
}

// needsAxe lists rules that cannot be judged without an axe-core run.
var needsAxe = map[string]bool{"A1": true, "A2": true, "A3": true}

type stats struct{ planted, found, clean, falseAlarms int }

func TestCorpus(t *testing.T) {
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		t.Fatalf("cannot read corpus: %v", err)
	}
	rs := allRules()
	perRule := map[string]*stats{}

	var sess *crawler.Session
	if _, err := crawler.FindBrowser(""); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if s, err := (&crawler.Crawler{}).Open(ctx); err == nil {
			sess = s
			defer s.Close()
		} else {
			t.Logf("browser cannot start, axe-based cases are skipped: %v", err)
		}
	} else {
		t.Log("no browser installed, axe-based cases are skipped")
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(casesDir, e.Name())
		t.Run(e.Name(), func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, "expected.json"))
			if err != nil {
				t.Fatalf("missing expected.json: %v", err)
			}
			var exp expected
			if err := json.Unmarshal(raw, &exp); err != nil {
				t.Fatalf("bad expected.json: %v", err)
			}
			for id := range exp.Expect {
				if needsAxe[id] && sess == nil {
					t.Skip("needs a browser for axe-core")
				}
			}
			in := loadPages(t, dir, sess)

			for id, want := range exp.Expect {
				r, ok := rs[id]
				if !ok {
					t.Errorf("rule %s is expected by the case but not registered in the corpus runner", id)
					continue
				}
				res, err := r.Check(context.Background(), in)
				if err != nil {
					t.Errorf("%s: %v", id, err)
					continue
				}
				st := perRule[id]
				if st == nil {
					st = &stats{}
					perRule[id] = st
				}
				switch exp.Kind {
				case "defect":
					st.planted++
					if res.State == rules.Failed {
						st.found++
					}
				case "clean":
					st.clean++
					if res.State == rules.Failed {
						st.falseAlarms++
					}
				}
				if string(res.State) != want {
					t.Errorf("%s: state = %s, want %s (spec %v, findings %+v)", id, res.State, want, exp.Spec, res.Findings)
				}
			}
		})
	}

	ids := make([]string, 0, len(perRule))
	for id := range perRule {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		s := perRule[id]
		t.Logf("%s: recall %d/%d, false alarms %d/%d", id, s.found, s.planted, s.falseAlarms, s.clean)
	}
}

func loadPages(t *testing.T, dir string, sess *crawler.Session) rules.Input {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.html"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	var in rules.Input
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		url := "file://" + filepath.Base(f)
		if sess == nil {
			in.Pages = append(in.Pages, rules.Page{URL: url, HTML: string(b)})
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		page, err := sess.AnalyzeHTML(ctx, url, string(b), 20*time.Second)
		cancel()
		if err != nil {
			t.Fatalf("rendering %s: %v", f, err)
		}
		in.Pages = append(in.Pages, page)
	}
	return in
}
