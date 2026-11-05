package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/a6"
	"compliancechecker/internal/scan"
)

type fakeSource struct {
	pages      []rules.Page
	incomplete bool
	err        error
	total      int // reported through progress when > 0
}

func (f fakeSource) Pages(_ context.Context, _ string, progress func(done, total int)) ([]rules.Page, bool, error) {
	if progress != nil && f.total > 0 {
		progress(len(f.pages), f.total)
	}
	return f.pages, f.incomplete, f.err
}

func page(html string) rules.Page { return rules.Page{URL: "https://shop.example/", HTML: html} }

func newRunner(src PageSource, released *[]string) (*Runner, *scan.MemoryStore) {
	st := scan.NewMemoryStore()
	r := &Runner{
		Store: st, Source: src, Rules: []rules.Rule{a6.New()},
		Release: func(k string) { *released = append(*released, k) },
		Now:     func() time.Time { return time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC) },
	}
	return r, st
}

func seed(t *testing.T, st *scan.MemoryStore, areas ...string) {
	t.Helper()
	if err := st.Create(scan.Scan{ID: "s1", Host: "shop.example", StartURL: "https://shop.example/", Areas: areas, Status: scan.Queued}); err != nil {
		t.Fatal(err)
	}
}

// Spec: F7, A6-2, S-1 — a failing rule gives a finding and score 0.
func TestRunFailingRule(t *testing.T) {
	var released []string
	r, st := newRunner(fakeSource{pages: []rules.Page{page("<html><body></body></html>")}}, &released)
	seed(t, st, "accessibility")
	r.Run(context.Background(), Job{ScanID: "s1", ClientKey: "k"})

	s, _ := st.Get("s1")
	if s.Status != scan.Done {
		t.Fatalf("status = %s", s.Status)
	}
	if s.Score == nil || *s.Score != 0 {
		t.Fatalf("score = %v, want 0", s.Score)
	}
	if len(s.Findings) != 1 || s.Findings[0].RuleID != "A6" || s.Findings[0].Selector != "html" {
		t.Fatalf("findings = %+v", s.Findings)
	}
	if len(s.RuleResults) != 1 || s.RuleResults[0].State != rules.Failed || s.RuleResults[0].PagesAffected != 1 {
		t.Fatalf("rule results = %+v", s.RuleResults)
	}
	if len(released) != 1 || released[0] != "k" {
		t.Fatalf("limiter slot not released: %v", released)
	}
}

// Spec: F7, S-1 — a passing page scores 100.
func TestRunPassing(t *testing.T) {
	var released []string
	r, st := newRunner(fakeSource{pages: []rules.Page{page(`<html lang="de"></html>`)}}, &released)
	seed(t, st, "accessibility")
	r.Run(context.Background(), Job{ScanID: "s1"})
	s, _ := st.Get("s1")
	if s.Score == nil || *s.Score != 100 || len(s.Findings) != 0 {
		t.Fatalf("score=%v findings=%d", s.Score, len(s.Findings))
	}
}

// Spec: F2, F7 — several pages: findings are per page and pagesAffected counts pages.
func TestRunMultiplePages(t *testing.T) {
	var released []string
	pages := []rules.Page{
		{URL: "https://shop.example/", HTML: `<html lang="de"></html>`},
		{URL: "https://shop.example/a", HTML: `<html></html>`},
		{URL: "https://shop.example/b", HTML: `<html></html>`},
	}
	r, st := newRunner(fakeSource{pages: pages, total: 3}, &released)
	seed(t, st, "accessibility")
	r.Run(context.Background(), Job{ScanID: "s1"})
	s, _ := st.Get("s1")
	if s.RuleResults[0].PagesAffected != 2 || len(s.Findings) != 2 || s.PagesDone != 3 || s.PagesTotal != 3 {
		t.Fatalf("affected=%d findings=%d done=%d total=%d", s.RuleResults[0].PagesAffected, len(s.Findings), s.PagesDone, s.PagesTotal)
	}
}

// Spec: C-1 — a crawl stopped by the time limit gives status incomplete but keeps results.
func TestRunIncomplete(t *testing.T) {
	var released []string
	r, st := newRunner(fakeSource{pages: []rules.Page{page(`<html lang="de"></html>`)}, incomplete: true}, &released)
	seed(t, st, "accessibility")
	r.Run(context.Background(), Job{ScanID: "s1"})
	s, _ := st.Get("s1")
	if s.Status != scan.Incomplete || s.Score == nil {
		t.Fatalf("status=%s score=%v", s.Status, s.Score)
	}
}

// Spec: F1 — only requested areas run; no score if no rule ran.
func TestRunSkipsOtherAreas(t *testing.T) {
	var released []string
	r, st := newRunner(fakeSource{pages: []rules.Page{page("<html></html>")}}, &released)
	seed(t, st, "legal")
	r.Run(context.Background(), Job{ScanID: "s1"})
	s, _ := st.Get("s1")
	if len(s.RuleResults) != 0 || s.Score != nil || s.Status != scan.Done {
		t.Fatalf("unexpected: %+v", s)
	}
}

// Spec: F1/F2 — no pages fails with a German message and still releases the slot.
func TestRunSourceFailure(t *testing.T) {
	for name, src := range map[string]fakeSource{
		"error":    {err: errors.New("boom")},
		"no pages": {},
	} {
		t.Run(name, func(t *testing.T) {
			var released []string
			r, st := newRunner(src, &released)
			seed(t, st, "accessibility")
			r.Run(context.Background(), Job{ScanID: "s1", ClientKey: "k"})
			s, _ := st.Get("s1")
			if s.Status != scan.Failed || s.ErrorDe == "" || len(released) != 1 {
				t.Fatalf("status=%s err=%q released=%v", s.Status, s.ErrorDe, released)
			}
		})
	}
}
