package crawler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// Spec: RP-3 — position rules for one element.
func TestPlace(t *testing.T) {
	const w, h = 1280, 2000
	tests := []struct {
		name string
		sel  string
		rb   rawBox
		want Position
	}{
		{"whole page", "html", rawBox{Found: true, Visible: true, Width: 10, Height: 10}, PosPage},
		{"not found", "#x", rawBox{}, PosNotFound},
		{"hidden", "#x", rawBox{Found: true, Visible: false, Width: 10, Height: 10}, PosHidden},
		{"zero size", "#x", rawBox{Found: true, Visible: false, Width: 0, Height: 0}, PosHidden},
		{"empty heading (zero height)", "#x", rawBox{Found: true, Visible: true, X: 5, Y: 5, Width: 600, Height: 0}, PosElement},
		{"below screenshot", "#x", rawBox{Found: true, Visible: true, Y: 2500, Width: 10, Height: 10}, PosOutside},
		{"visible", "#x", rawBox{Found: true, Visible: true, X: 100, Y: 200, Width: 50, Height: 20}, PosElement},
	}
	for _, tt := range tests {
		got := place(tt.sel, tt.rb, w, h)
		if got.Position != tt.want {
			t.Errorf("%s: position = %s, want %s", tt.name, got.Position, tt.want)
		}
		if (tt.want == PosPage || tt.want == PosNotFound) != (got.Box == nil) {
			t.Errorf("%s: box = %v", tt.name, got.Box)
		}
	}
	// an element with zero height gets a minimum frame
	if got := place("#x", rawBox{Found: true, Visible: true, X: 5, Y: 5, Width: 600}, w, h); got.Box.Height != minFrame {
		t.Errorf("empty heading box = %+v", got.Box)
	}
	// an element reaching over the bottom edge is clipped to the screenshot
	got := place("#x", rawBox{Found: true, Visible: true, X: 10, Y: 1990, Width: 20, Height: 40}, w, h)
	if got.Position != PosElement || got.Box.Y+got.Box.Height != h {
		t.Errorf("clipped box = %+v", got.Box)
	}
}

// Spec: RP-2 — screenshot size limits.
func TestCaptureSize(t *testing.T) {
	if w, h, tr := captureSize(800, 500); w != CaptureViewportWidth || h != 500 || tr {
		t.Errorf("small page: %d×%d %v", w, h, tr)
	}
	if w, h, tr := captureSize(9000, 20000); w != CaptureMaxWidth || h != CaptureMaxHeight || !tr {
		t.Errorf("huge page: %d×%d %v", w, h, tr)
	}
}

const capturePage = `<!DOCTYPE html><html lang="de"><head><style>body{margin:0;font:16px sans-serif}</style></head><body>
<h1 id="title" style="position:absolute;left:100px;top:50px;width:300px;height:40px;margin:0">Titel</h1>
<p id="gone" style="display:none">versteckt</p>
<div style="height:9000px"></div>
<p id="deep">ganz unten</p>
</body></html>`

// Spec: RP-A1 — one render gives a JPEG screenshot and one placement per selector.
func TestCaptureHTML(t *testing.T) {
	s := openOrSkip(t, &Crawler{Validate: allowLoopback})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := s.CaptureHTML(ctx, capturePage, []string{"#title", "html", "#missing", "#gone", "#deep", "p:::bad"}, 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(c.Image, []byte{0xFF, 0xD8}) {
		t.Fatal("screenshot is not a JPEG")
	}
	if c.Width != CaptureViewportWidth || c.Height != CaptureMaxHeight || !c.Truncated || c.ViewportWidth != CaptureViewportWidth {
		t.Errorf("size = %d×%d truncated=%v", c.Width, c.Height, c.Truncated)
	}
	want := []Position{PosElement, PosPage, PosNotFound, PosHidden, PosOutside, PosNotFound}
	if len(c.Placements) != len(want) {
		t.Fatalf("placements = %+v", c.Placements)
	}
	for i, w := range want {
		if c.Placements[i].Position != w {
			t.Errorf("placement %d = %s, want %s", i, c.Placements[i].Position, w)
		}
	}
	b := c.Placements[0].Box
	if b == nil || b.X != 100 || b.Y != 50 || b.Width != 300 || b.Height != 40 {
		t.Errorf("box of #title = %+v", b)
	}
}

// Spec: RP-10 — the print tab makes no network requests and returns a PDF.
func TestPrintPDFBlocksNetwork(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()
	s := openOrSkip(t, &Crawler{Validate: allowLoopback})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	doc := `<!DOCTYPE html><html lang="de"><head><link rel="stylesheet" href="` + srv.URL + `/a.css"></head>
<body><h1>Prüfbericht ≥ 4,5:1</h1><img alt="x" src="` + srv.URL + `/b.png"></body></html>`
	pdf, err := s.PrintPDF(ctx, []byte(doc), 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("not a PDF: %q", pdf[:min(len(pdf), 20)])
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("print tab made %d network requests", n)
	}
}
