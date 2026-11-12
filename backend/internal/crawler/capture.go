package crawler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// Capture settings. Spec: specs/11-report.md RP-2.
const (
	CaptureViewportWidth  = 1280
	CaptureViewportHeight = 900
	CaptureMaxHeight      = 8000
	CaptureMaxWidth       = 2 * CaptureViewportWidth
	CaptureJPEGQuality    = 70
	minFrame              = 8 // px, smallest frame for elements with one zero side (RP-3)
)

// Position says where a finding's element is on the screenshot. Spec: RP-3.
type Position string

const (
	PosElement  Position = "element"
	PosPage     Position = "page"
	PosHidden   Position = "hidden"
	PosOutside  Position = "outside"
	PosNotFound Position = "not_found"
)

// Box is a rectangle in CSS px of the captured page (document coordinates).
type Box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Placement is the result for one selector.
type Placement struct {
	Position Position
	Box      *Box // nil for page and not_found
}

// Capture is one render of a page: a JPEG screenshot plus the placement of each requested selector, in the same order.
type Capture struct {
	Image         []byte
	Width         int
	Height        int
	ViewportWidth int
	Truncated     bool
	Placements    []Placement
}

// rawBox is what the measuring script returns per selector.
type rawBox struct {
	Found   bool    `json:"found"`
	Visible bool    `json:"visible"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
}

type measurement struct {
	Width  float64  `json:"width"`
	Height float64  `json:"height"`
	Boxes  []rawBox `json:"boxes"`
}

// Capture renders rawURL once more and returns a screenshot and the placement of every selector (RP-1, RP-2, RP-3).
// Every request of the tab is checked like during the crawl (C-SSRF, RP-4).
func (s *Session) Capture(ctx context.Context, rawURL string, selectors []string, timeout time.Duration) (Capture, error) {
	return s.capture(ctx, timeout, rawURL, selectors)
}

// CaptureHTML is Capture for an HTML string (no network needed). Used by tests.
func (s *Session) CaptureHTML(ctx context.Context, htmlSrc string, selectors []string, timeout time.Duration) (Capture, error) {
	dataURL := "data:text/html;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte(htmlSrc))
	return s.capture(ctx, timeout, dataURL, selectors)
}

func (s *Session) capture(ctx context.Context, timeout time.Duration, target string, selectors []string) (Capture, error) {
	tctx, tcancel := chromedp.NewContext(s.bctx)
	defer tcancel()
	if err := chromedp.Listen(tctx, func(ev any) {
		if e, ok := ev.(*fetch.EventRequestPaused); ok {
			go s.decide(tctx, e)
		}
	}); err != nil {
		return Capture{}, fmt.Errorf("crawler: opening capture tab: %w", err)
	}

	rctx, rcancel := context.WithTimeout(tctx, timeout)
	defer rcancel()
	stop := context.AfterFunc(ctx, rcancel)
	defer stop()

	if _, err := chromedp.Call(rctx, fetch.Enable, fetch.EnableParams{}); err != nil {
		return Capture{}, fmt.Errorf("crawler: enabling request interception: %w", err)
	}
	if err := chromedp.Do(rctx,
		chromedp.EmulateViewport(CaptureViewportWidth, CaptureViewportHeight),
		chromedp.Navigate(target),
		chromedp.Sleep(700*time.Millisecond),
	); err != nil {
		return Capture{}, fmt.Errorf("crawler: loading %s for capture: %w", shortURL(target), err)
	}

	m, err := chromedp.Run(rctx, chromedp.Evaluate[measurement](measureScript(selectors)))
	if err != nil {
		return Capture{}, fmt.Errorf("crawler: measuring elements: %w", err)
	}
	w, h, truncated := captureSize(m.Width, m.Height)

	q := int64(CaptureJPEGQuality)
	yes := true
	shot, err := chromedp.Call(rctx, page.CaptureScreenshot, page.CaptureScreenshotParams{
		Format:                page.CaptureScreenshotFormatJpeg,
		Quality:               &q,
		Clip:                  &page.Viewport{X: 0, Y: 0, Width: float64(w), Height: float64(h), Scale: 1},
		CaptureBeyondViewport: &yes,
		FromSurface:           &yes,
	})
	if err != nil {
		return Capture{}, fmt.Errorf("crawler: taking screenshot: %w", err)
	}

	c := Capture{Image: shot.Data, Width: w, Height: h, ViewportWidth: CaptureViewportWidth, Truncated: truncated}
	for i, sel := range selectors {
		var rb rawBox
		if i < len(m.Boxes) {
			rb = m.Boxes[i]
		}
		c.Placements = append(c.Placements, place(sel, rb, w, h))
	}
	return c, nil
}

// captureSize caps the page size to the screenshot limits (RP-2).
func captureSize(pageW, pageH float64) (w, h int, truncated bool) {
	w = int(math.Ceil(pageW))
	if w < CaptureViewportWidth {
		w = CaptureViewportWidth
	}
	if w > CaptureMaxWidth {
		w = CaptureMaxWidth
	}
	h = int(math.Ceil(pageH))
	if h < 1 {
		h = CaptureViewportHeight
	}
	if h > CaptureMaxHeight {
		h, truncated = CaptureMaxHeight, true
	}
	return w, h, truncated
}

// place decides the position of one element (RP-3).
func place(selector string, rb rawBox, shotW, shotH int) Placement {
	if selector == "html" || selector == "" {
		return Placement{Position: PosPage}
	}
	if !rb.Found {
		return Placement{Position: PosNotFound}
	}
	b := &Box{X: rb.X, Y: rb.Y, Width: rb.Width, Height: rb.Height}
	if !rb.Visible || (rb.Width <= 0 && rb.Height <= 0) {
		return Placement{Position: PosHidden, Box: b}
	}
	// an element with one zero side (e.g. an empty heading) is still marked, with a minimum frame
	b.Width, b.Height = math.Max(b.Width, minFrame), math.Max(b.Height, minFrame)
	if rb.Y >= float64(shotH) || rb.X >= float64(shotW) || rb.Y+rb.Height <= 0 || rb.X+rb.Width <= 0 {
		return Placement{Position: PosOutside, Box: b}
	}
	return Placement{Position: PosElement, Box: clip(b, shotW, shotH)}
}

// clip keeps a box inside the screenshot.
func clip(b *Box, w, h int) *Box {
	x0, y0 := math.Max(b.X, 0), math.Max(b.Y, 0)
	x1, y1 := math.Min(b.X+b.Width, float64(w)), math.Min(b.Y+b.Height, float64(h))
	return &Box{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

// measureScript returns page size and one box per selector in document coordinates.
func measureScript(selectors []string) string {
	sels, _ := json.Marshal(selectors)
	return `(() => {
  const sels = ` + string(sels) + `;
  const de = document.documentElement, b = document.body;
  const width = Math.max(de.scrollWidth, b ? b.scrollWidth : 0, de.clientWidth);
  const height = Math.max(de.scrollHeight, b ? b.scrollHeight : 0, de.clientHeight);
  const boxes = sels.map(sel => {
    const none = {found: false, visible: false, x: 0, y: 0, width: 0, height: 0};
    if (!sel || sel === "html") return none;
    let el = null;
    try { el = document.querySelector(sel); } catch (e) { return none; }
    if (!el) return none;
    const r = el.getBoundingClientRect();
    const cs = getComputedStyle(el);
    const visible = (r.width > 0 || r.height > 0) && cs.display !== "none" &&
      cs.visibility !== "hidden" && parseFloat(cs.opacity || "1") > 0;
    return {found: true, visible, x: r.left + window.scrollX, y: r.top + window.scrollY, width: r.width, height: r.height};
  });
  return {width, height, boxes};
})()`
}
