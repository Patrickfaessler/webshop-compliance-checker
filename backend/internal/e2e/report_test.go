package e2e

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"compliancechecker/internal/crawler"
	"compliancechecker/internal/report"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/a1"
	"compliancechecker/internal/rules/a2"
	"compliancechecker/internal/rules/a3"
	"compliancechecker/internal/rules/a4"
	"compliancechecker/internal/rules/a6"
	"compliancechecker/internal/scan"
	"compliancechecker/internal/scoring"
	"compliancechecker/internal/worker"
)

// scanShop crawls one test shop, runs the rules and the capture step like the worker does.
func scanShop(t *testing.T, ctx context.Context, c *crawler.Crawler, base, path string) (scan.Scan, map[string]rules.Meta) {
	t.Helper()
	opt := crawler.DefaultOptions()
	opt.MinInterval = 50 * time.Millisecond
	res, err := c.Crawl(ctx, base+"/"+path, opt, nil, nil)
	if err != nil {
		t.Fatalf("crawl: %v", err)
	}
	sc := scan.Scan{ID: "e2e", Host: "127.0.0.1", StartURL: base + "/" + path, Areas: []string{"accessibility"},
		Status: scan.Done, PagesDone: len(res.Pages), CreatedAt: time.Now(), FinishedAt: time.Now()}
	metas := map[string]rules.Meta{}
	var outcomes []scoring.Outcome
	for _, r := range []rules.Rule{a1.New(), a2.New(), a3.New(), a4.New(), a6.New()} {
		m := r.Meta()
		metas[m.ID] = m
		out, err := r.Check(ctx, rules.Input{Pages: res.Pages})
		if err != nil {
			t.Fatal(err)
		}
		pages := map[string]bool{}
		for i, f := range out.Findings {
			pages[f.PageURL] = true
			sc.Findings = append(sc.Findings, scan.Finding{ID: fmt.Sprintf("%s-%d", m.ID, i), RuleID: m.ID, Severity: m.Severity,
				URL: f.PageURL, Selector: f.Selector, Snippet: f.Snippet, Sub: f.Sub, SummaryDe: m.SummaryDe})
		}
		sc.RuleResults = append(sc.RuleResults, scan.RuleResult{RuleID: m.ID, State: out.State, DecidedBy: out.DecidedBy, PagesAffected: len(pages)})
		outcomes = append(outcomes, scoring.Outcome{Severity: m.Severity, State: out.State})
	}
	if s, ok := scoring.Score(outcomes); ok {
		sc.Score = &s
	}
	order := make([]string, len(res.Pages))
	for i, p := range res.Pages {
		order[i] = p.URL
	}
	sc.Shots, sc.Findings = worker.CrawlAnnotator{Crawler: c}.Annotate(ctx, sc.Findings, order)
	return sc, metas
}

// Spec: RP-A4 — every planted defect with an element is framed on its page; whole-page rules get the banner.
// Spec: RP-9, N7 — the report itself passes our rules A1–A6.
// With REPORT_OUT=<dir> the HTML and PDF report of each shop are written there for a visual check.
func TestShopReports(t *testing.T) {
	if _, err := crawler.FindBrowser(""); err != nil {
		t.Skip("no browser installed")
	}
	if _, err := os.Stat(siteDir); err != nil {
		t.Skip("testshops/site not generated")
	}
	srv := httptest.NewServer(http.FileServer(http.Dir(siteDir)))
	defer srv.Close()

	for _, shop := range []string{"radwerk/", "seifen/"} {
		t.Run(shop, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
			defer cancel()
			c := &crawler.Crawler{Validate: allowLoopback}
			sc, metas := scanShop(t, ctx, c, srv.URL, shop)
			if len(sc.Findings) == 0 {
				t.Fatal("shop has no findings")
			}

			withFindings := map[string]bool{}
			elements := 0
			for _, f := range sc.Findings {
				withFindings[f.URL] = true
				switch {
				case f.RuleID == "A6" && f.Position != "page":
					t.Errorf("A6 on %s: position %q, want page", f.URL, f.Position)
				case (f.RuleID == "A1" || f.RuleID == "A2" || f.RuleID == "A3") && f.Position != "element":
					t.Errorf("%s %s on %s: position %q, want element", f.RuleID, f.Selector, f.URL, f.Position)
				case f.RuleID == "A4" && f.Position != "element" && f.Position != "page":
					t.Errorf("A4 %s on %s: position %q", f.Selector, f.URL, f.Position)
				}
				if f.Position == "element" {
					elements++
				}
			}
			if want := min(len(withFindings), worker.MaxShotPages); len(sc.Shots) != want {
				t.Errorf("shots = %d, want %d", len(sc.Shots), want)
			}

			rep, err := report.Build(sc, metas, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			doc, err := rep.HTML()
			if err != nil {
				t.Fatal(err)
			}
			if n := strings.Count(string(doc), `class="mark" id="m-`); n != elements {
				t.Errorf("frames in report = %d, want %d", n, elements)
			}

			selfScan(t, ctx, c, doc)

			if dir := os.Getenv("REPORT_OUT"); dir != "" {
				name := strings.TrimSuffix(shop, "/")
				if err := os.WriteFile(filepath.Join(dir, name+".html"), doc, 0o644); err != nil {
					t.Fatal(err)
				}
				pdf, err := c.PrintPDF(ctx, doc, time.Minute)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, name+".pdf"), pdf, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// selfScan serves the report over loopback, renders it in the browser and runs A1–A6 on it (RP-9, N7).
func selfScan(t *testing.T, ctx context.Context, c *crawler.Crawler, doc []byte) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(doc)
	}))
	defer srv.Close()
	s, err := c.Open(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err := s.Render(ctx, srv.URL+"/bericht.html", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []rules.Rule{a1.New(), a2.New(), a3.New(), a4.New(), a6.New()} {
		out, err := rule.Check(ctx, rules.Input{Pages: []rules.Page{r.Page}})
		if err != nil {
			t.Fatal(err)
		}
		if out.State == rules.Failed {
			t.Errorf("report fails its own rule %s: %+v", rule.Meta().ID, out.Findings)
		}
	}
}
