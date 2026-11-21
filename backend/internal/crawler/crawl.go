package crawler

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/fetch"
	"compliancechecker/internal/lexicon"
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
	MinInterval time.Duration   // minimum time between page loads (politeness, 2 req/s)
	Keyboard    KeyboardOptions // A5 probe mode and Tab limit
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

// priority is 0 for a link whose URL or anchor text (lowercase) contains a priority word, else 1.
// legalWords are the legal-page link words of the lexicon (R1/R2/R5), so the crawler and the rules agree.
var legalWords = lexicon.PriorityWords()

func priority(u, text string) int {
	l := strings.ToLower(u)
	if d, err := url.PathUnescape(l); err == nil {
		l = d
	}
	for _, w := range append(priorityWords, legalWords...) {
		if strings.Contains(l, w) || strings.Contains(text, w) {
			return 0
		}
	}
	return 1
}

// linkTexts maps each normalised link of a page to its anchor text, lowercased and whitespace-collapsed (C-2).
func linkTexts(baseURL, htmlSrc string) map[string]string {
	out := map[string]string{}
	domutil.Walk(domutil.Parse(htmlSrc), func(n *html.Node) bool {
		if !domutil.IsElement(n, "a") {
			return true
		}
		href, ok := domutil.Attr(n, "href")
		if !ok {
			return true
		}
		if l, ok := normalizeLink(baseURL, href); ok {
			t := strings.ToLower(strings.Join(strings.Fields(domutil.Text(n)), " "))
			if out[l] == "" {
				out[l] = t
			}
		}
		return false
	})
	return out
}

// Crawl visits up to opt.MaxPages pages of the start URL's site, breadth first, legal links first.
// progress (optional) is called after each page with pages done and the current estimate of the total.
// If ctx ends, the pages collected so far are returned with Incomplete set.
func (c *Crawler) Crawl(ctx context.Context, startURL string, opt Options, robotsClient Getter, progress func(done, total int)) (Result, error) {
	if opt.MaxPages <= 0 {
		kb := opt.Keyboard
		opt = DefaultOptions()
		opt.Keyboard = kb
	}
	// A5: one sampler per crawl decides which pages get the keyboard probe (A5-5, A5-6)
	sampler := newKeyboardSampler(opt.Keyboard)
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
	plan := &probePlan{decide: sampler.decide, presses: opt.Keyboard.Presses(),
		accept: func(finalURL string, status int) bool { // same check as below: dropped pages are not probed
			fu, err := url.Parse(finalURL)
			return err == nil && same(fu.Hostname()) && status < 400
		}}

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
	seen := map[string]bool{pageKey(startURL): true}
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

		r, err := sess.run(ctx, opt.PageTimeout, it.url, "", plan)
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
		r.Page.RequestedURL = it.url
		res.Pages = append(res.Pages, r.Page)

		if it.depth < opt.MaxDepth {
			texts := linkTexts(r.FinalURL, r.Page.HTML)
			for _, href := range r.Links {
				n, ok := normalizeLink(r.FinalURL, href)
				if !ok || seen[pageKey(n)] {
					continue
				}
				nu, err := url.Parse(n)
				if err != nil || !same(nu.Hostname()) {
					continue
				}
				seen[pageKey(n)] = true
				next := item{n, it.depth + 1}
				if priority(n, texts[n]) == 0 {
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

// pageKey identifies a page for the visited set: "/dir/" and "/dir/index.html" (or index.htm, index.php)
// are the same page and must be crawled only once (C-1), otherwise findings would be counted twice.
func pageKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	for _, idx := range []string{"index.html", "index.htm", "index.php"} {
		if strings.HasSuffix(u.Path, "/"+idx) {
			u.Path = strings.TrimSuffix(u.Path, idx)
			break
		}
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Fragment = ""
	return u.String()
}
