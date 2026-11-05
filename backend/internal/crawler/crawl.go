package crawler

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"compliancechecker/internal/fetch"
	"compliancechecker/internal/rules"
)

// Getter fetches plain documents such as robots.txt; *fetch.Client implements it (SSRF-safe).
type Getter interface {
	Get(ctx context.Context, rawURL string) (fetch.Page, error)
}

// Options are the crawl limits. Spec: specs/05-crawler.md (C-1).
type Options struct {
	MaxPages    int
	MaxDepth    int
	PageTimeout time.Duration
	MinInterval time.Duration // minimum time between page loads (politeness, 2 req/s)
}

// DefaultOptions returns the MVP limits.
func DefaultOptions() Options {
	return Options{MaxPages: 50, MaxDepth: 3, PageTimeout: 20 * time.Second, MinInterval: 500 * time.Millisecond}
}

// Result is the outcome of a crawl.
type Result struct {
	Pages      []rules.Page
	Incomplete bool     // stopped by the time limit
	Skipped    []string // URLs skipped because of robots.txt
}

// priorityWords make a link important for the legal rules (C-2): they are visited first.
var priorityWords = []string{
	"impressum", "imprint", "legal", "datenschutz", "privacy", "agb", "terms", "widerruf", "withdraw",
	"checkout", "kasse", "cart", "warenkorb", "kontakt", "contact",
}

func priority(u string) int {
	l := strings.ToLower(u)
	for _, w := range priorityWords {
		if strings.Contains(l, w) {
			return 0
		}
	}
	return 1
}

// Crawl visits up to opt.MaxPages pages of the start URL's site, breadth first, legal links first.
// progress (optional) is called after each page with pages done and the current estimate of the total.
// If ctx ends, the pages collected so far are returned with Incomplete set.
func (c *Crawler) Crawl(ctx context.Context, startURL string, opt Options, robotsClient Getter, progress func(done, total int)) (Result, error) {
	if opt.MaxPages <= 0 {
		opt = DefaultOptions()
	}
	su, err := url.Parse(startURL)
	if err != nil {
		return Result{}, err
	}
	if err := c.validate(ctx, startURL); err != nil {
		return Result{}, err
	}
	same := func(host string) bool {
		h, s := strings.ToLower(host), strings.ToLower(su.Hostname())
		return h == s || h == "www."+s || "www."+h == s
	}

	var robots *Robots
	if robotsClient != nil {
		if p, err := robotsClient.Get(ctx, su.Scheme+"://"+su.Host+"/robots.txt"); err == nil && p.Status == 200 {
			robots = ParseRobots(string(p.Body), "ComplianceCheckerBot")
		}
	}

	sess, err := c.Open(ctx)
	if err != nil {
		return Result{}, err
	}
	defer sess.Close()

	type item struct {
		url   string
		depth int
	}
	var hi, lo []item
	seen := map[string]bool{startURL: true}
	hi = append(hi, item{startURL, 0})

	var res Result
	var last time.Time
	for (len(hi) > 0 || len(lo) > 0) && len(res.Pages) < opt.MaxPages {
		if ctx.Err() != nil {
			res.Incomplete = true
			break
		}
		var it item
		if len(hi) > 0 {
			it, hi = hi[0], hi[1:]
		} else {
			it, lo = lo[0], lo[1:]
		}
		if pu, err := url.Parse(it.url); err == nil && robots != nil && !robots.Allowed(pu.RequestURI()) {
			res.Skipped = append(res.Skipped, it.url)
			continue
		}
		if wait := opt.MinInterval - time.Since(last); wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
			}
		}
		last = time.Now()

		r, err := sess.Render(ctx, it.url, opt.PageTimeout)
		if err != nil {
			if ctx.Err() != nil {
				res.Incomplete = true
				break
			}
			continue // one broken page does not stop the scan
		}
		if fu, err := url.Parse(r.FinalURL); err != nil || !same(fu.Hostname()) || r.Status >= 400 {
			continue // redirected off-site or error page
		}
		res.Pages = append(res.Pages, r.Page)

		if it.depth < opt.MaxDepth {
			for _, href := range r.Links {
				n, ok := normalizeLink(r.FinalURL, href)
				if !ok || seen[n] {
					continue
				}
				nu, err := url.Parse(n)
				if err != nil || !same(nu.Hostname()) {
					continue
				}
				seen[n] = true
				next := item{n, it.depth + 1}
				if priority(n) == 0 {
					hi = append(hi, next)
				} else {
					lo = append(lo, next)
				}
			}
		}
		if progress != nil {
			total := len(res.Pages) + len(hi) + len(lo)
			if total > opt.MaxPages {
				total = opt.MaxPages
			}
			progress(len(res.Pages), total)
		}
	}
	if len(res.Pages) == 0 && !res.Incomplete {
		return res, errors.New("crawler: no page could be loaded")
	}
	return res, nil
}
