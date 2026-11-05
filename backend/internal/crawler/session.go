// Package crawler renders pages in headless Chromium (via chromedp, ADR-4), runs axe-core and
// collects links. Spec: specs/05-crawler.md.
package crawler

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	appfetch "compliancechecker/internal/fetch"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/ssrf"
)

// axe-core 4.10.2 (MPL-2.0), sha256 B511CD9DEC01C76F4B2AD1723B66B6DB37D4C2EB4ED199076E1829D9EE7B75E3.
//
//go:embed axe/axe.min.js
var axeSource string

// axeRules are the axe rule ids the MVP rules A1-A3 use (plus none else, to keep runs fast).
var axeRules = []string{
	"color-contrast",
	"image-alt", "input-image-alt", "area-alt", "svg-img-alt",
	"label", "select-name", "aria-input-field-name",
}

// ErrNoBrowser means no Chromium-family binary was found.
var ErrNoBrowser = errors.New("crawler: no Chromium/Edge/Chrome binary found (set CHROME_PATH)")

// Crawler opens browser sessions.
type Crawler struct {
	// ExecPath is the browser binary. Empty = CHROME_PATH or auto-detect.
	ExecPath string
	// NoSandbox disables the Chromium sandbox; needed inside some containers only.
	NoSandbox bool
	// Validate decides whether the browser may request a URL. Default: ssrf.Validate (C-SSRF).
	Validate func(ctx context.Context, rawURL string) error
}

func (c *Crawler) validate(ctx context.Context, rawURL string) error {
	if c.Validate != nil {
		return c.Validate(ctx, rawURL)
	}
	_, _, err := ssrf.Validate(ctx, rawURL, nil)
	return err
}

// FindBrowser returns a usable browser binary path.
func FindBrowser(preferred string) (string, error) {
	cands := []string{preferred, os.Getenv("CHROME_PATH")}
	cands = append(cands,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		"/usr/bin/chromium", "/usr/bin/chromium-browser", "/usr/bin/google-chrome",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
	)
	for _, p := range cands {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	for _, name := range []string{"chromium", "chromium-browser", "google-chrome", "msedge"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", ErrNoBrowser
}

// Session is one running browser. Close it when done.
type Session struct {
	c      *Crawler
	bctx   context.Context
	cancel context.CancelFunc
}

// Open starts the browser.
func (c *Crawler) Open(ctx context.Context) (*Session, error) {
	path, err := FindBrowser(c.ExecPath)
	if err != nil {
		return nil, err
	}
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.ExecPath(path),
		chromedp.UserAgent(appfetch.UserAgent),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("mute-audio", true),
	)
	if c.NoSandbox {
		opts = append(opts, chromedp.NoSandbox)
	}
	actx, acancel := chromedp.NewExecAllocator(ctx, opts...)
	bctx, bcancel := chromedp.NewContext(actx)
	if err := chromedp.Do(bctx); err != nil { // no steps: just starts the browser
		bcancel()
		acancel()
		return nil, fmt.Errorf("crawler: starting browser %s: %w", path, err)
	}
	return &Session{c: c, bctx: bctx, cancel: func() { bcancel(); acancel() }}, nil
}

// Close stops the browser.
func (s *Session) Close() { s.cancel() }

// Rendered is one rendered page plus what the crawler needs to continue.
type Rendered struct {
	Page     rules.Page
	Links    []string
	Status   int
	FinalURL string
}

// Render loads rawURL, waits for it to settle, runs axe-core and returns the result.
// Every request the page makes is validated first (C-SSRF).
func (s *Session) Render(ctx context.Context, rawURL string, timeout time.Duration) (Rendered, error) {
	return s.run(ctx, timeout, rawURL, "")
}

// AnalyzeHTML renders an HTML string (no network needed) and runs axe-core on it. Used by the corpus.
func (s *Session) AnalyzeHTML(ctx context.Context, pageURL, htmlSrc string, timeout time.Duration) (rules.Page, error) {
	dataURL := "data:text/html;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte(htmlSrc))
	r, err := s.run(ctx, timeout, dataURL, pageURL)
	return r.Page, err
}

func (s *Session) run(ctx context.Context, timeout time.Duration, target, displayURL string) (Rendered, error) {
	// One tab per page. The listener is registered first: that also opens the tab. A timeout must not
	// be set before the tab exists, so the time limit is applied to a child context afterwards.
	tctx, tcancel := chromedp.NewContext(s.bctx)
	defer tcancel()

	var (
		mu     sync.Mutex
		status int
	)
	// The listener runs inside the tab's message loop: it must be quick and must not run commands itself.
	if err := chromedp.Listen(tctx, func(ev any) {
		switch e := ev.(type) {
		case *fetch.EventRequestPaused:
			go s.decide(tctx, e)
		case *network.EventResponseReceived:
			if e.Type == network.ResourceTypeDocument && e.Response != nil {
				mu.Lock()
				if status == 0 {
					status = int(e.Response.Status)
				}
				mu.Unlock()
			}
		}
	}); err != nil {
		return Rendered{}, fmt.Errorf("crawler: opening tab: %w", err)
	}

	rctx, rcancel := context.WithTimeout(tctx, timeout)
	defer rcancel()
	stop := context.AfterFunc(ctx, rcancel) // the caller's context ends the page load too
	defer stop()

	if _, err := chromedp.Call(rctx, network.Enable, network.EnableParams{}); err != nil {
		return Rendered{}, fmt.Errorf("crawler: enabling network events: %w", err)
	}
	if _, err := chromedp.Call(rctx, fetch.Enable, fetch.EnableParams{}); err != nil {
		return Rendered{}, fmt.Errorf("crawler: enabling request interception: %w", err)
	}
	if err := chromedp.Do(rctx, chromedp.Navigate(target)); err != nil {
		return Rendered{}, fmt.Errorf("crawler: navigating to %s: %w", shortURL(target), err)
	}
	// let scripts settle; cookie banners and similar usually render within this time
	if err := chromedp.Do(rctx, chromedp.Sleep(700*time.Millisecond)); err != nil {
		return Rendered{}, err
	}

	finalURL, err := chromedp.Run(rctx, chromedp.Location())
	if err != nil {
		return Rendered{}, fmt.Errorf("crawler: reading location: %w", err)
	}
	docHTML, err := chromedp.Run(rctx, chromedp.Evaluate[string](`"<!DOCTYPE html>" + document.documentElement.outerHTML`))
	if err != nil {
		return Rendered{}, fmt.Errorf("crawler: reading DOM: %w", err)
	}
	links, err := chromedp.Run(rctx, chromedp.Evaluate[[]string](`Array.from(document.querySelectorAll('a[href]')).map(a => a.href)`))
	if err != nil {
		return Rendered{}, fmt.Errorf("crawler: reading links: %w", err)
	}
	if _, err := chromedp.Run(rctx, chromedp.Evaluate[chromedp.Void](axeSource)); err != nil {
		return Rendered{}, fmt.Errorf("crawler: injecting axe-core: %w", err)
	}
	axeOut, err := chromedp.Run(rctx, chromedp.Evaluate[string](axeRunScript(), chromedp.EvalAwaitPromise))
	if err != nil {
		return Rendered{}, fmt.Errorf("crawler: running axe-core: %w", err)
	}

	report, err := parseAxe(axeOut)
	if err != nil {
		return Rendered{}, err
	}
	pageURL := finalURL
	if displayURL != "" {
		pageURL = displayURL
	}
	mu.Lock()
	st := status
	mu.Unlock()
	return Rendered{
		Page:     rules.Page{URL: pageURL, HTML: docHTML, Axe: report},
		Links:    links,
		Status:   st,
		FinalURL: finalURL,
	}, nil
}

// decide continues or blocks one browser request after the SSRF check.
func (s *Session) decide(tctx context.Context, e *fetch.EventRequestPaused) {
	u := e.Request.URL
	allowed := false
	switch {
	case strings.HasPrefix(u, "data:"), strings.HasPrefix(u, "blob:"), strings.HasPrefix(u, "about:"):
		allowed = true // no network access
	default:
		allowed = s.c.validate(tctx, u) == nil
	}
	// errors are ignored on purpose: the tab may already be closed
	if allowed {
		_, _ = chromedp.Call(tctx, fetch.ContinueRequest, fetch.ContinueRequestParams{RequestID: e.RequestID})
	} else {
		_, _ = chromedp.Call(tctx, fetch.FailRequest, fetch.FailRequestParams{
			RequestID: e.RequestID, ErrorReason: network.ErrorReasonBlockedByClient,
		})
	}
}

func shortURL(u string) string {
	if strings.HasPrefix(u, "data:") {
		return "data: URL"
	}
	return u
}

func axeRunScript() string {
	ids, _ := json.Marshal(axeRules)
	return `(async () => {
  const r = await axe.run(document, {
    runOnly: { type: 'rule', values: ` + string(ids) + ` },
    resultTypes: ['violations', 'incomplete', 'inapplicable']
  });
  const sel = n => Array.isArray(n.target[0]) ? n.target[0].join(' ') : String(n.target[0]);
  const map = list => list.map(x => ({ id: x.id, nodes: (x.nodes || []).map(n => ({
    selector: sel(n), html: String(n.html || '').slice(0, 300),
    data: (n.any && n.any[0] && n.any[0].data) || null })) }));
  return JSON.stringify({ violations: map(r.violations), incomplete: map(r.incomplete),
                          inapplicable: r.inapplicable.map(x => x.id) });
})()`
}

type axeJSON struct {
	Violations   []axeResultJSON `json:"violations"`
	Incomplete   []axeResultJSON `json:"incomplete"`
	Inapplicable []string        `json:"inapplicable"`
}

type axeResultJSON struct {
	ID    string `json:"id"`
	Nodes []struct {
		Selector string         `json:"selector"`
		HTML     string         `json:"html"`
		Data     map[string]any `json:"data"`
	} `json:"nodes"`
}

func parseAxe(raw string) (*rules.AxeReport, error) {
	var j axeJSON
	if err := json.Unmarshal([]byte(raw), &j); err != nil {
		return nil, fmt.Errorf("crawler: bad axe output: %w", err)
	}
	conv := func(in []axeResultJSON) []rules.AxeResult {
		out := make([]rules.AxeResult, 0, len(in))
		for _, r := range in {
			ar := rules.AxeResult{ID: r.ID}
			for _, n := range r.Nodes {
				ar.Nodes = append(ar.Nodes, rules.AxeNode{Selector: n.Selector, HTML: n.HTML, Data: n.Data})
			}
			out = append(out, ar)
		}
		return out
	}
	return &rules.AxeReport{Violations: conv(j.Violations), Incomplete: conv(j.Incomplete), Inapplicable: j.Inapplicable}, nil
}

// normalizeLink resolves href against base, drops fragments and rejects non-page links.
// ok is false for other schemes and obvious non-HTML files. Spec: C-1, C-2.
func normalizeLink(base, href string) (string, bool) {
	b, err := url.Parse(base)
	if err != nil {
		return "", false
	}
	u, err := b.Parse(strings.TrimSpace(href))
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	u.Fragment = ""
	switch ext := strings.ToLower(pathExt(u.Path)); ext {
	case ".pdf", ".zip", ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".mp4", ".mp3", ".css", ".js", ".xml", ".ico", ".woff", ".woff2":
		return "", false
	}
	return u.String(), true
}

func pathExt(p string) string {
	i := strings.LastIndex(p, ".")
	if i < 0 || strings.Contains(p[i:], "/") {
		return ""
	}
	return p[i:]
}
