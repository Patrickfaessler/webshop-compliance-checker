// Package report builds the downloadable report of a finished scan (ADR-6): JSON (F13) and PDF (F10).
package report

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/scan"
)

// Format and Version identify the JSON layout; bump Version on incompatible changes.
const (
	Format  = "webshop-check-report"
	Version = 1
)

// ErrNotFinished is returned for scans that are still queued or running, or that failed.
var ErrNotFinished = errors.New("report: scan is not finished")

type Report struct {
	Format       string       `json:"format"`
	Version      int          `json:"version"`
	GeneratedAt  time.Time    `json:"generatedAt"`
	DisclaimerDe string       `json:"disclaimerDe"`
	Scan         ScanInfo     `json:"scan"`
	RuleResults  []RuleResult `json:"ruleResults"`
	Findings     []Finding    `json:"findings"`
	Pages        []Page       `json:"pages,omitempty"` // captured pages (specs/11-report.md RP-1)
}

// Page is a captured page. The screenshot is only used for the HTML/PDF report, not written to JSON (RP-1, RP-2).
type Page struct {
	URL           string `json:"url"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	ViewportWidth int    `json:"viewportWidth"`
	Truncated     bool   `json:"truncated"`
	Image         []byte `json:"-"` // JPEG
}

// Box is an element rectangle in CSS px of the page screenshot (RP-3).
type Box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type ScanInfo struct {
	ID           string     `json:"id"`
	StartURL     string     `json:"startUrl"`
	Host         string     `json:"host"`
	Areas        []string   `json:"areas"`
	Status       string     `json:"status"`
	Score        *int       `json:"score"`
	PagesScanned int        `json:"pagesScanned"`
	CreatedAt    time.Time  `json:"createdAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
}

type RuleResult struct {
	RuleID        string `json:"ruleId"`
	NameDe        string `json:"nameDe"`
	Severity      string `json:"severity"`
	State         string `json:"state"`
	DecidedBy     string `json:"decidedBy"`
	PagesAffected int    `json:"pagesAffected"`
}

type Finding struct {
	ID        string `json:"id"`
	RuleID    string `json:"ruleId"`
	Severity  string `json:"severity"`
	URL       string `json:"url"`
	Selector  string `json:"selector"`
	Snippet   string `json:"snippet"`
	Sub       string `json:"sub,omitempty"`
	SummaryDe string `json:"summaryDe"`
	Position  string `json:"position,omitempty"` // RP-3: element, page, hidden, outside, not_found
	Box       *Box   `json:"box,omitempty"`
}

// Build turns a finished scan into a report. metas supplies German names and severities per rule id.
func Build(sc scan.Scan, metas map[string]rules.Meta, now time.Time) (Report, error) {
	if sc.Status != scan.Done && sc.Status != scan.Incomplete {
		return Report{}, ErrNotFinished
	}
	r := Report{
		Format: Format, Version: Version, GeneratedAt: now.UTC(), DisclaimerDe: scan.DisclaimerDe,
		Scan: ScanInfo{
			ID: sc.ID, StartURL: sc.StartURL, Host: sc.Host, Areas: append([]string{}, sc.Areas...),
			Status: string(sc.Status), Score: sc.Score, PagesScanned: sc.PagesDone, CreatedAt: sc.CreatedAt.UTC(),
		},
		RuleResults: []RuleResult{},
		Findings:    []Finding{},
	}
	if !sc.FinishedAt.IsZero() {
		f := sc.FinishedAt.UTC()
		r.Scan.FinishedAt = &f
	}
	for _, rr := range sc.RuleResults {
		m := metas[rr.RuleID]
		name := m.NameDe
		if name == "" {
			name = rr.RuleID
		}
		r.RuleResults = append(r.RuleResults, RuleResult{
			RuleID: rr.RuleID, NameDe: name, Severity: string(m.Severity), State: string(rr.State),
			DecidedBy: string(rr.DecidedBy), PagesAffected: rr.PagesAffected,
		})
	}
	for _, f := range sc.Findings {
		rf := Finding{
			ID: f.ID, RuleID: f.RuleID, Severity: string(f.Severity), URL: f.URL, Selector: f.Selector,
			Snippet: f.Snippet, Sub: f.Sub, SummaryDe: f.SummaryDe, Position: f.Position,
		}
		if f.Box != nil {
			rf.Box = &Box{X: f.Box.X, Y: f.Box.Y, Width: f.Box.Width, Height: f.Box.Height}
		}
		r.Findings = append(r.Findings, rf)
	}
	for _, sh := range sc.Shots {
		r.Pages = append(r.Pages, Page{
			URL: sh.URL, Width: sh.Width, Height: sh.Height, ViewportWidth: sh.ViewportWidth,
			Truncated: sh.Truncated, Image: sh.Image,
		})
	}
	return r, nil
}

// JSON returns the indented JSON document (F13).
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

var unsafeChars = regexp.MustCompile(`[^a-z0-9.-]+`)

// FileName returns e.g. "webshop-check_shop.example_2026-11-08.pdf" for the given extension.
func (r Report) FileName(ext string) string {
	host := unsafeChars.ReplaceAllString(strings.ToLower(r.Scan.Host), "-")
	if host == "" {
		host = "shop"
	}
	return "webshop-check_" + host + "_" + r.Scan.CreatedAt.In(vienna).Format("2006-01-02") + "." + ext
}
