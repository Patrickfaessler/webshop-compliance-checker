package crawler

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

// PrintPDF starts a browser, prints the HTML document to an A4 PDF and stops the browser (RP-10, ADR-7).
func (c *Crawler) PrintPDF(ctx context.Context, htmlDoc []byte, timeout time.Duration) ([]byte, error) {
	s, err := c.Open(ctx)
	if err != nil {
		return nil, err
	}
	defer s.Close()
	return s.PrintPDF(ctx, htmlDoc, timeout)
}

// PrintPDF prints the HTML document in a tab that blocks every network request. The HTML is set directly,
// not loaded from a URL, so large embedded images work (RP-10).
func (s *Session) PrintPDF(ctx context.Context, htmlDoc []byte, timeout time.Duration) ([]byte, error) {
	tctx, tcancel := chromedp.NewContext(s.bctx)
	defer tcancel()
	if err := chromedp.Listen(tctx, func(ev any) {
		if e, ok := ev.(*fetch.EventRequestPaused); ok {
			go func() {
				_, _ = chromedp.Call(tctx, fetch.FailRequest, fetch.FailRequestParams{
					RequestID: e.RequestID, ErrorReason: network.ErrorReasonBlockedByClient,
				})
			}()
		}
	}); err != nil {
		return nil, fmt.Errorf("crawler: opening print tab: %w", err)
	}

	rctx, rcancel := context.WithTimeout(tctx, timeout)
	defer rcancel()
	stop := context.AfterFunc(ctx, rcancel)
	defer stop()

	if _, err := chromedp.Call(rctx, fetch.Enable, fetch.EnableParams{}); err != nil {
		return nil, fmt.Errorf("crawler: enabling request interception: %w", err)
	}
	if err := chromedp.Do(rctx, chromedp.Navigate("about:blank")); err != nil {
		return nil, fmt.Errorf("crawler: opening blank page: %w", err)
	}
	tree, err := chromedp.Call(rctx, page.GetFrameTree, cdp.Empty{})
	if err != nil || tree.FrameTree == nil || tree.FrameTree.Frame == nil {
		return nil, fmt.Errorf("crawler: reading frame tree: %v", err)
	}
	if _, err := chromedp.Call(rctx, page.SetDocumentContent, page.SetDocumentContentParams{
		FrameID: tree.FrameTree.Frame.ID, HTML: string(htmlDoc),
	}); err != nil {
		return nil, fmt.Errorf("crawler: setting report HTML: %w", err)
	}
	// wait until the embedded screenshots are decoded, otherwise they may be missing in the PDF
	if _, err := chromedp.Run(rctx, chromedp.Evaluate[chromedp.Void](
		`Promise.all(Array.from(document.images).map(i => i.decode().catch(() => null))).then(() => true)`,
		chromedp.EvalAwaitPromise,
	)); err != nil {
		return nil, fmt.Errorf("crawler: waiting for images: %w", err)
	}

	yes := true
	res, err := chromedp.Call(rctx, page.PrintToPDF, page.PrintToPDFParams{
		PrintBackground:         &yes,
		PreferCSSPageSize:       &yes,
		GenerateTaggedPDF:       &yes,
		GenerateDocumentOutline: &yes,
	})
	if err != nil {
		return nil, fmt.Errorf("crawler: printing PDF: %w", err)
	}
	return res.Data, nil
}
