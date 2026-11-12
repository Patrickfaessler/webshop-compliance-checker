package worker

import (
	"context"
	"log"
	"time"

	"compliancechecker/internal/crawler"
	"compliancechecker/internal/scan"
)

// MaxShotPages is the number of pages captured per scan (specs/11-report.md RP-4).
const MaxShotPages = 15

// CrawlAnnotator renders each page with findings once more in the browser and takes a screenshot (RP-1 … RP-4).
type CrawlAnnotator struct {
	Crawler     *crawler.Crawler
	PageTimeout time.Duration // per page; 0 = 20 s
}

func (a CrawlAnnotator) Annotate(ctx context.Context, findings []scan.Finding, pageOrder []string) ([]scan.Shot, []scan.Finding) {
	plan := shotPlan(findings, pageOrder, MaxShotPages)
	if len(plan) == 0 {
		return nil, findings
	}
	sess, err := a.Crawler.Open(ctx)
	if err != nil {
		log.Printf("annotate: browser: %v", err)
		return nil, findings
	}
	defer sess.Close()

	timeout := a.PageTimeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	out := append([]scan.Finding(nil), findings...)
	var shots []scan.Shot
	for _, p := range plan {
		if ctx.Err() != nil {
			break
		}
		sels := make([]string, len(p.idx))
		for i, fi := range p.idx {
			sels[i] = out[fi].Selector
		}
		c, err := sess.Capture(ctx, p.url, sels, timeout)
		if err != nil {
			log.Printf("annotate: %v", err) // RP-4: the report is shown without marks for this page
			continue
		}
		shots = append(shots, scan.Shot{
			URL: p.url, Image: c.Image, Width: c.Width, Height: c.Height,
			ViewportWidth: c.ViewportWidth, Truncated: c.Truncated,
		})
		applyPlacements(out, p.idx, c.Placements)
	}
	return shots, out
}

type pagePlan struct {
	url string
	idx []int // indexes into findings
}

// shotPlan groups findings by page in crawl order and keeps at most max pages (RP-1, RP-4).
func shotPlan(findings []scan.Finding, pageOrder []string, max int) []pagePlan {
	byURL := map[string][]int{}
	for i, f := range findings {
		byURL[f.URL] = append(byURL[f.URL], i)
	}
	var plan []pagePlan
	seen := map[string]bool{}
	add := func(u string) {
		if seen[u] || len(byURL[u]) == 0 || len(plan) >= max {
			return
		}
		seen[u] = true
		plan = append(plan, pagePlan{url: u, idx: byURL[u]})
	}
	for _, u := range pageOrder {
		add(u)
	}
	for _, f := range findings { // pages not in the crawl order (should not happen)
		add(f.URL)
	}
	return plan
}

// applyPlacements copies the capture result into the findings (RP-3).
func applyPlacements(findings []scan.Finding, idx []int, pl []crawler.Placement) {
	for i, fi := range idx {
		if i >= len(pl) {
			break
		}
		findings[fi].Position = string(pl[i].Position)
		if b := pl[i].Box; b != nil {
			findings[fi].Box = &scan.Box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}
		} else {
			findings[fi].Box = nil
		}
	}
}
