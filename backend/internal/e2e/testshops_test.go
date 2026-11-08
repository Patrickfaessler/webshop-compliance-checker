// Package e2e crawls the test shops (testshops/site) end to end and compares the rule results with each
// shop's expected.json. Spec: specs/09-testing.md (T-3). Needs a Chromium-family browser; skipped otherwise.
package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"compliancechecker/internal/scoring"
)

const siteDir = "../../../testshops/site"

type expectation struct {
	Shop          string            `json:"shop"`
	StartPath     string            `json:"startPath"`
	Expected      map[string]string `json:"expected"`
	ExpectedScore int               `json:"expectedScore"`
	Pages         []string          `json:"pages"`
}

// allowLoopback lets the crawler reach the local test server; everything else stays blocked.
func allowLoopback(_ context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() != "127.0.0.1" {
		return errors.New("blocked in test")
	}
	return nil
}

// Spec: T-3 — every test shop gives the expected rule states and score, and the crawler finds all its pages.
func TestShops(t *testing.T) {
	if _, err := crawler.FindBrowser(""); err != nil {
		t.Skip("no browser installed")
	}
	if _, err := os.Stat(siteDir); err != nil {
		t.Skip("testshops/site not generated")
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(siteDir)))
	defer srv.Close()

	ruleList := []rules.Rule{a1.New(), a2.New(), a3.New(), a4.New(), a6.New()}
	dirs, _ := filepath.Glob(filepath.Join(siteDir, "*", "expected.json"))
	if len(dirs) == 0 {
		t.Fatal("no shops found")
	}
	for _, expPath := range dirs {
		raw, err := os.ReadFile(expPath)
		if err != nil {
			t.Fatal(err)
		}
		var exp expectation
		if err := json.Unmarshal(raw, &exp); err != nil {
			t.Fatal(err)
		}
		t.Run(exp.Shop, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			c := &crawler.Crawler{Validate: allowLoopback}
			opt := crawler.DefaultOptions()
			opt.MinInterval = 50 * time.Millisecond
			res, err := c.Crawl(ctx, srv.URL+"/"+exp.StartPath, opt, nil, nil)
			if err != nil {
				t.Fatalf("crawl: %v", err)
			}

			// all pages of the shop are found, and nothing outside it
			var got []string
			for _, p := range res.Pages {
				u, _ := url.Parse(p.URL)
				rel, _ := filepath.Rel("/"+exp.StartPath, u.Path)
				if rel == "." {
					rel = "index.html"
				}
				got = append(got, filepath.ToSlash(rel))
			}
			sort.Strings(got)
			want := append([]string(nil), exp.Pages...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Errorf("pages crawled = %v, want %v", got, want)
			}

			var outcomes []scoring.Outcome
			for _, r := range ruleList {
				m := r.Meta()
				result, err := r.Check(ctx, rules.Input{Pages: res.Pages})
				if err != nil {
					t.Fatal(err)
				}
				if want := exp.Expected[m.ID]; string(result.State) != want {
					t.Errorf("%s: state = %s, want %s; findings: %+v", m.ID, result.State, want, result.Findings)
				}
				outcomes = append(outcomes, scoring.Outcome{Severity: m.Severity, State: result.State})
			}
			if score, _ := scoring.Score(outcomes); score != exp.ExpectedScore {
				t.Errorf("score = %d, want %d", score, exp.ExpectedScore)
			}
		})
	}
}
