// Package worker runs scan jobs: collect pages, run rules, score, store.
// Interim in-process runner; it moves to cmd/worker with a PostgreSQL job queue later (specs/07-architecture.md).
package worker

import (
	"context"
	"fmt"
	"slices"
	"time"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/scan"
	"compliancechecker/internal/scoring"
)

// PageSource delivers the pages of a scan. Implementations: CrawlSource (browser, up to 50 pages)
// and FetchSource (start page only, no browser). Tests use a fake.
type PageSource interface {
	// Pages returns the pages found. incomplete is true if the time limit stopped the crawl.
	// progress may be nil.
	Pages(ctx context.Context, startURL string, progress func(done, total int)) (pages []rules.Page, incomplete bool, err error)
}

// Job identifies a scan to run and the rate-limit key to release afterwards.
type Job struct {
	ScanID    string
	ClientKey string
}

// Annotator takes screenshots of the pages with findings and locates each finding on them, after the rules ran
// (specs/11-report.md RP-1). It returns the shots and the findings with Position/Box set. Errors are not fatal.
type Annotator interface {
	Annotate(ctx context.Context, findings []scan.Finding, pageOrder []string) ([]scan.Shot, []scan.Finding)
}

type Runner struct {
	Store     scan.Store
	Source    PageSource
	Annotator Annotator // nil = no screenshots (no browser)
	Rules     []rules.Rule
	Release   func(clientKey string) // may be nil
	Now       func() time.Time
	Timeout   time.Duration // total scan time, C-1 (5 min)
}

// Loop processes jobs until ctx is cancelled or jobs is closed.
func (r *Runner) Loop(ctx context.Context, jobs <-chan Job) {
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-jobs:
			if !ok {
				return
			}
			r.Run(ctx, j)
		}
	}
}

// Run executes one scan.
func (r *Runner) Run(ctx context.Context, job Job) {
	if r.Release != nil {
		defer r.Release(job.ClientKey)
	}
	now := r.Now
	if now == nil {
		now = time.Now
	}
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	s, ok := r.Store.Get(job.ScanID)
	if !ok {
		return
	}
	r.Store.Update(job.ScanID, func(s *scan.Scan) { s.Status = scan.Running })

	progress := func(done, total int) {
		r.Store.Update(job.ScanID, func(s *scan.Scan) { s.PagesDone, s.PagesTotal = done, total })
	}
	pages, incomplete, err := r.Source.Pages(ctx, s.StartURL, progress)
	if err != nil || len(pages) == 0 {
		r.fail(job.ScanID, now(), "Die Startseite konnte nicht geladen werden.")
		return
	}

	in := rules.Input{Pages: pages}
	var (
		results  []scan.RuleResult
		findings []scan.Finding
		outcomes []scoring.Outcome
	)
	for _, rule := range r.Rules {
		m := rule.Meta()
		if !slices.Contains(s.Areas, m.Area) {
			continue
		}
		res, err := rule.Check(ctx, in)
		if err != nil {
			res = rules.Result{State: rules.NotAssessable}
		}
		affected := map[string]struct{}{}
		for i, f := range res.Findings {
			affected[f.PageURL] = struct{}{}
			findings = append(findings, scan.Finding{
				ID:        fmt.Sprintf("%s-%s-%d", job.ScanID, m.ID, i+1),
				RuleID:    m.ID,
				Severity:  m.Severity,
				URL:       f.PageURL,
				Selector:  f.Selector,
				Snippet:   f.Snippet,
				Evidence:  f.Evidence,
				Sub:       f.Sub,
				SummaryDe: m.SummaryDe,
			})
		}
		results = append(results, scan.RuleResult{
			RuleID: m.ID, State: res.State, DecidedBy: res.DecidedBy, PagesAffected: len(affected),
		})
		outcomes = append(outcomes, scoring.Outcome{Severity: m.Severity, State: res.State})
	}

	var shots []scan.Shot
	if r.Annotator != nil && len(findings) > 0 && ctx.Err() == nil {
		order := make([]string, len(pages))
		for i, p := range pages {
			order[i] = p.URL
		}
		shots, findings = r.Annotator.Annotate(ctx, findings, order)
	}

	score, scoreOK := scoring.Score(outcomes)
	r.Store.Update(job.ScanID, func(s *scan.Scan) {
		s.RuleResults, s.Findings, s.Shots = results, findings, shots
		s.PagesDone = len(pages)
		if s.PagesTotal < len(pages) {
			s.PagesTotal = len(pages)
		}
		s.Score = nil
		if scoreOK {
			s.Score = &score
		}
		s.Status = scan.Done
		if incomplete {
			s.Status = scan.Incomplete
		}
		s.FinishedAt = now()
	})
}

func (r *Runner) fail(id string, at time.Time, msgDe string) {
	r.Store.Update(id, func(s *scan.Scan) {
		s.Status = scan.Failed
		s.ErrorDe = msgDe
		s.FinishedAt = at
	})
}
