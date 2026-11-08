// Package storetest is a contract test every scan.Store implementation must pass, so the in-memory
// store and the PostgreSQL store behave the same (ADR-5).
package storetest

import (
	"testing"
	"time"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/scan"
)

// Factory returns an empty store and a function that sets the store's clock.
type Factory func(t *testing.T) (store scan.Store, setNow func(time.Time))

var t0 = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

func sample(id, host string, created time.Time) scan.Scan {
	return scan.Scan{
		ID: id, Host: host, StartURL: "https://" + host + "/", Areas: []string{"accessibility", "legal"},
		Status: scan.Queued, CreatedAt: created, ExpiresAt: created.Add(scan.Retention),
	}
}

// Run runs all contract cases.
func Run(t *testing.T, newStore Factory) {
	// Spec: F11 — a created scan is readable by id; unknown ids are not found.
	t.Run("create and get", func(t *testing.T) {
		st, setNow := newStore(t)
		setNow(t0)
		if err := st.Create(sample("s1", "shop.example", t0)); err != nil {
			t.Fatal(err)
		}
		got, ok := st.Get("s1")
		if !ok || got.Host != "shop.example" || got.Status != scan.Queued || len(got.Areas) != 2 || got.Score != nil {
			t.Fatalf("got %+v ok=%v", got, ok)
		}
		if !got.CreatedAt.Equal(t0) || !got.FinishedAt.IsZero() {
			t.Fatalf("times: created %v finished %v", got.CreatedAt, got.FinishedAt)
		}
		if _, ok := st.Get("nope"); ok {
			t.Fatal("unknown id found")
		}
	})

	// Spec: F7 — update stores status, score, rule results and findings in order.
	t.Run("update with results", func(t *testing.T) {
		st, setNow := newStore(t)
		setNow(t0)
		if err := st.Create(sample("s1", "shop.example", t0)); err != nil {
			t.Fatal(err)
		}
		if !st.Update("s1", func(s *scan.Scan) { s.Status = scan.Running; s.PagesDone, s.PagesTotal = 1, 4 }) {
			t.Fatal("update failed")
		}
		score := 60
		finished := t0.Add(time.Minute)
		ok := st.Update("s1", func(s *scan.Scan) {
			s.Status, s.Score, s.FinishedAt = scan.Done, &score, finished
			s.RuleResults = []scan.RuleResult{
				{RuleID: "A6", State: rules.Failed, DecidedBy: rules.DecidedStructural, PagesAffected: 2},
				{RuleID: "A1", State: rules.Passed, DecidedBy: rules.DecidedStructural},
			}
			s.Findings = []scan.Finding{
				{ID: "s1-A6-1", RuleID: "A6", Severity: rules.Notice, URL: "https://shop.example/", Selector: "html", Snippet: "<html>", SummaryDe: "Sprache fehlt", Sub: "x"},
				{ID: "s1-A6-2", RuleID: "A6", Severity: rules.Notice, URL: "https://shop.example/a", Selector: "html"},
			}
		})
		if !ok {
			t.Fatal("update failed")
		}
		got, _ := st.Get("s1")
		if got.Status != scan.Done || got.Score == nil || *got.Score != 60 || got.PagesTotal != 4 || !got.FinishedAt.Equal(finished) {
			t.Fatalf("scan: %+v", got)
		}
		if len(got.RuleResults) != 2 || got.RuleResults[0].RuleID != "A6" || got.RuleResults[0].PagesAffected != 2 || got.RuleResults[1].State != rules.Passed {
			t.Fatalf("rule results: %+v", got.RuleResults)
		}
		if len(got.Findings) != 2 || got.Findings[0].SummaryDe != "Sprache fehlt" || got.Findings[0].Sub != "x" || got.Findings[1].URL != "https://shop.example/a" {
			t.Fatalf("findings: %+v", got.Findings)
		}
		// a later progress-only update keeps the results
		st.Update("s1", func(s *scan.Scan) { s.PagesDone = 4 })
		got, _ = st.Get("s1")
		if len(got.Findings) != 2 || got.PagesDone != 4 {
			t.Fatalf("results lost on later update: %+v", got)
		}
		if st.Update("nope", func(*scan.Scan) {}) {
			t.Fatal("update of unknown id succeeded")
		}
	})

	// Spec: F11, N4 — expired scans are not readable.
	t.Run("expiry", func(t *testing.T) {
		st, setNow := newStore(t)
		setNow(t0)
		if err := st.Create(sample("s1", "shop.example", t0)); err != nil {
			t.Fatal(err)
		}
		setNow(t0.Add(scan.Retention - time.Second))
		if _, ok := st.Get("s1"); !ok {
			t.Fatal("scan must still be readable just before expiry")
		}
		setNow(t0.Add(scan.Retention))
		if _, ok := st.Get("s1"); ok {
			t.Fatal("expired scan readable")
		}
	})

	// Spec: C-RL — latest non-failed scan of a host since a time.
	t.Run("latest for host", func(t *testing.T) {
		st, setNow := newStore(t)
		setNow(t0)
		for _, s := range []scan.Scan{
			sample("old", "shop.example", t0.Add(-20*time.Minute)),
			sample("new", "shop.example", t0.Add(-2*time.Minute)),
			sample("other", "other.example", t0.Add(-1*time.Minute)),
		} {
			if err := st.Create(s); err != nil {
				t.Fatal(err)
			}
		}
		failed := sample("failed", "shop.example", t0.Add(-1*time.Minute))
		failed.Status = scan.Failed
		if err := st.Create(failed); err != nil {
			t.Fatal(err)
		}
		got, ok := st.LatestForHost("shop.example", t0.Add(-10*time.Minute))
		if !ok || got.ID != "new" {
			t.Fatalf("got %q ok=%v, want new", got.ID, ok)
		}
		if _, ok := st.LatestForHost("shop.example", t0.Add(-time.Second)); ok {
			t.Fatal("nothing should be that recent")
		}
		if _, ok := st.LatestForHost("unknown.example", t0.Add(-time.Hour)); ok {
			t.Fatal("unknown host found")
		}
	})
}
