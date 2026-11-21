package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"compliancechecker/internal/crawler"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/a6"
	"compliancechecker/internal/scan"
)

type fakeSource struct {
	pages      []rules.Page
	incomplete bool
	err        error
	total      int                      // reported through progress when > 0
	gotKB      *crawler.KeyboardOptions // records the keyboard options it was called with
	delay      time.Duration            // waits this long (or until ctx ends)
}

func (f fakeSource) Pages(ctx context.Context, _ string, kb crawler.KeyboardOptions, progress func(done, total int)) ([]rules.Page, bool, error) {
	if f.gotKB != nil {
		*f.gotKB = kb
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return nil, true, ctx.Err()
		}
	}
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

// noteRule passes with a German note.
type noteRule struct{}

func (noteRule) Meta() rules.Meta {
	return rules.Meta{ID: "A5", Area: "accessibility", Severity: rules.Critical}
}
func (noteRule) Check(context.Context, rules.Input) (rules.Result, error) {
	return rules.Result{State: rules.Passed, DecidedBy: rules.DecidedStructural, NoteDe: "Hinweis X"}, nil
}

// Spec: A5-7 — the note of a rule result is stored with the scan.
func TestRunCopiesRuleNote(t *testing.T) {
	var released []string
	r, st := newRunner(fakeSource{pages: []rules.Page{page(`<html lang="de"><body></body></html>`)}}, &released)
	r.Rules = []rules.Rule{noteRule{}}
	seed(t, st, "accessibility")
	r.Run(context.Background(), Job{ScanID: "s1", ClientKey: "k"})
	s, _ := st.Get("s1")
	if len(s.RuleResults) != 1 || s.RuleResults[0].NoteDe != "Hinweis X" {
		t.Fatalf("rule results = %+v", s.RuleResults)
	}
}

// Spec: A5-8, A5-6 — the worker passes the scan's keyboard options to the source and records the probed pages.
func TestRunPassesKeyboardOptions(t *testing.T) {
	var released []string
	var got crawler.KeyboardOptions
	pages := []rules.Page{
		{URL: "https://shop.example/", HTML: `<html lang="de"></html>`, Keyboard: &rules.KeyboardReport{Completed: true}},
		{URL: "https://shop.example/agb", HTML: `<html lang="de"></html>`},
	}
	r, st := newRunner(fakeSource{pages: pages, gotKB: &got}, &released)
	if err := st.Create(scan.Scan{ID: "s1", StartURL: "https://shop.example/", Areas: []string{"accessibility"}, Status: scan.Queued,
		KeyboardAllPages: true, KeyboardMaxPresses: 500}); err != nil {
		t.Fatal(err)
	}
	r.Run(context.Background(), Job{ScanID: "s1"})
	if got != (crawler.KeyboardOptions{AllPages: true, MaxPresses: 500}) {
		t.Errorf("source got %+v", got)
	}
	s, _ := st.Get("s1")
	if len(s.KeyboardPages) != 1 || s.KeyboardPages[0] != "https://shop.example/" {
		t.Errorf("keyboard pages = %v", s.KeyboardPages)
	}
}

// Spec: C-1 — mode "alle Seiten" gets the longer time limit.
func TestRunAllPagesTimeout(t *testing.T) {
	for _, all := range []bool{false, true} {
		var released []string
		r, st := newRunner(fakeSource{pages: []rules.Page{page(`<html lang="de"></html>`)}, delay: 100 * time.Millisecond}, &released)
		r.Timeout, r.TimeoutAllPages = 50*time.Millisecond, 400*time.Millisecond
		if err := st.Create(scan.Scan{ID: "s1", StartURL: "https://shop.example/", Areas: []string{"accessibility"}, Status: scan.Queued, KeyboardAllPages: all}); err != nil {
			t.Fatal(err)
		}
		r.Run(context.Background(), Job{ScanID: "s1"})
		s, _ := st.Get("s1")
		if want := map[bool]scan.Status{false: scan.Failed, true: scan.Done}[all]; s.Status != want {
			t.Errorf("all=%v: status %s, want %s", all, s.Status, want)
		}
	}
}

// fixRule fails with one finding that carries a fix.
type fixRule struct{}

func (fixRule) Meta() rules.Meta {
	return rules.Meta{ID: "A3", Area: "accessibility", Severity: rules.Critical, ExplanationDe: "e"}
}
func (fixRule) Check(context.Context, rules.Input) (rules.Result, error) {
	return rules.Result{State: rules.Failed, DecidedBy: rules.DecidedStructural,
		Findings: []rules.Finding{{PageURL: "https://s.at/", Selector: "#x", Fix: &rules.Fix{TextDe: "t"}}}}, nil
}

// Spec: F8 — the fix of a finding and the rule's explanation are stored with the scan.
func TestRunCopiesFixAndExplanation(t *testing.T) {
	var released []string
	r, st := newRunner(fakeSource{pages: []rules.Page{page(`<html lang="de"><body></body></html>`)}}, &released)
	r.Rules = []rules.Rule{fixRule{}}
	seed(t, st, "accessibility")
	r.Run(context.Background(), Job{ScanID: "s1", ClientKey: "k"})
	s, _ := st.Get("s1")
	if len(s.Findings) != 1 || s.Findings[0].Fix == nil || s.Findings[0].Fix.TextDe != "t" || s.Findings[0].ExplanationDe != "e" {
		t.Fatalf("findings = %+v", s.Findings)
	}
}
