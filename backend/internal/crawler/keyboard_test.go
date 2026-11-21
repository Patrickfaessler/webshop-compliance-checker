package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"compliancechecker/internal/rules"
)

func kbPage(body string) string {
	return `<!DOCTYPE html><html lang="de"><head><title>t</title><style>body{font:16px sans-serif}</style></head><body>` + body + `</body></html>`
}

func sels(ns []rules.KeyboardNode) []string {
	out := make([]string, len(ns))
	for i, n := range ns {
		out[i] = n.Selector
	}
	return out
}

func probeHTML(t *testing.T, s *Session, html string, kb KeyboardOptions) rules.Page {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	p, err := s.analyze(ctx, "https://shop.example/", html, 50*time.Second, kb)
	if err != nil {
		t.Fatal(err)
	}
	if p.Keyboard == nil {
		t.Fatal("no keyboard report")
	}
	return p
}

// Spec: A5-1 … A5-4; Review Focus 3–5 (autofocus, hidden menus, :focus-within).
func TestProbeKeyboard(t *testing.T) {
	s := openOrSkip(t, &Crawler{Validate: allowLoopback})
	tests := []struct {
		name                                         string
		body                                         string
		unreachable, noFocusStyle, traps, notFocusab []string
		trap                                         bool
	}{
		{name: "ok (A5-1)", body: `<a id="l" href="/x">Link</a> <button id="b">Kaufen</button>`},
		{name: "no-outline (A5-2)", body: `<style>#b:focus{outline:none}</style><a id="l" href="/x">Link</a> <button id="b">Kaufen</button>`,
			noFocusStyle: []string{"#b"}},
		{name: "div-onclick (A5-3)", body: `<div id="d" onclick="void 0" style="cursor:pointer">Mehr</div>
<span id="s" style="cursor:pointer">Filter</span><script>document.getElementById('s').addEventListener('click',()=>{})</script>
<a href="/x" style="cursor:pointer"><span id="inner">im Link</span></a>`,
			notFocusab: []string{"#d", "#s"}},
		{name: "trap (A5-4)", body: `<a id="a" href="/x">A</a> <input id="t" aria-label="Suche"> <a id="c" href="/y">C</a>
<script>document.getElementById('t').addEventListener('keydown',e=>{if(e.key==='Tab'){e.preventDefault();e.target.focus();}})</script>`,
			trap: true},
		{name: "unreachable", body: `<a id="l" href="/x">Link</a> <a id="m" href="/y" tabindex="-1">nicht im Tab</a> <span id="r" role="button">Weiter</span>`,
			unreachable: []string{"#r"}},
		{name: "autofocus", body: `<a id="l" href="/x">Link</a> <button id="b">Los</button> <input id="q" aria-label="Suche" autofocus>`},
		{name: "hidden-menu", body: `<a id="l" href="/x">Start</a>
<nav style="display:none"><a href="/h1">versteckt</a></nav>
<div inert><a href="/h2">inert</a></div>
<div aria-hidden="true"><a href="/h3" tabindex="-1">aria</a></div>
<div style="visibility:hidden"><a href="/h4">unsichtbar</a></div>
<details><summary id="sum">Mehr</summary><a href="/h5">zu</a></details>`},
		{name: "focus-within", body: `<style>#q:focus{outline:none} .w:focus-within{outline:2px solid #000}</style><a id="l" href="/x">Link</a> <span class="w"><input id="q" aria-label="Suche"></span>`},
		{name: "empty", body: `<p>Nur Text</p>`},
		// a radio group is one Tab stop; the other radios are reached with the arrow keys (WCAG 2.1.1)
		// inner Tab stops the probe cannot see are not a trap (Logic 3)
		{name: "date-time", body: `<a id="l" href="/x">Link</a> <input type="date" id="d" aria-label="Geburtsdatum"> <input type="time" id="tm" aria-label="Uhrzeit"> <button id="b">Weiter</button>`},
		{name: "closed-shadow", body: `<a id="l" href="/x">Link</a> <x-pay id="c"></x-pay> <button id="b">Weiter</button>
<script>customElements.define('x-pay', class extends HTMLElement { constructor() { super(); const r = this.attachShadow({mode: 'closed'}); r.innerHTML = '<button>A</button><button>B</button><button>C</button>'; } });</script>`},
		// a consent modal open on load may hold the focus; the page behind it is not unreachable (Logic 6)
		{name: "consent-modal", body: `<a id="l" href="/x">Start</a> <a href="/y">Shop</a>
<div id="dlg" role="dialog" aria-modal="true" aria-label="Cookies"><button id="ok">Akzeptieren</button> <button id="no">Ablehnen</button></div>
<script>const d=document.getElementById('dlg'); document.addEventListener('focusin', e => { if (!d.contains(e.target)) document.getElementById('ok').focus(); }); document.getElementById('ok').focus();</script>`},
		{name: "radio-group", body: `<form><input type="radio" id="r1" name="pay" value="a" aria-label="Karte"> <input type="radio" id="r2" name="pay" value="b" aria-label="Rechnung"> <button id="b">Weiter</button></form>`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := probeHTML(t, s, kbPage(tt.body), KeyboardOptions{}).Keyboard
			if got := sels(k.NoFocusStyle); !slices.Equal(got, tt.noFocusStyle) && !(len(got) == 0 && len(tt.noFocusStyle) == 0) {
				t.Errorf("NoFocusStyle = %v, want %v", got, tt.noFocusStyle)
			}
			if got := sels(k.NotFocusable); !slices.Equal(got, tt.notFocusab) && !(len(got) == 0 && len(tt.notFocusab) == 0) {
				t.Errorf("NotFocusable = %v, want %v", got, tt.notFocusab)
			}
			if tt.trap != (len(k.Traps) > 0) {
				t.Errorf("Traps = %v, want trap=%v", sels(k.Traps), tt.trap)
			}
			if tt.trap {
				if !slices.Contains(sels(k.Traps), "#t") {
					t.Errorf("trap element = %v, want #t", sels(k.Traps))
				}
				return
			}
			if !k.Completed {
				t.Errorf("loop not completed after %d presses", k.Presses)
			}
			if got := sels(k.Unreachable); !slices.Equal(got, tt.unreachable) && !(len(got) == 0 && len(tt.unreachable) == 0) {
				t.Errorf("Unreachable = %v, want %v", got, tt.unreachable)
			}
			if tt.name == "empty" && k.Presses > 1 {
				t.Errorf("empty page: %d presses", k.Presses)
			}
			for _, n := range append(k.NoFocusStyle, k.NotFocusable...) {
				if !strings.HasPrefix(n.Snippet, "<") {
					t.Errorf("snippet %q is not an opening tag", n.Snippet)
				}
			}
		})
	}
}

// Spec: A5-7 — the Tab limit stops the loop; unreachable is then not judged.
func TestProbeKeyboardCap(t *testing.T) {
	s := openOrSkip(t, &Crawler{Validate: allowLoopback})
	var b strings.Builder
	for i := range 80 {
		fmt.Fprintf(&b, `<a href="/p%d">Produkt %d</a> `, i, i)
	}
	b.WriteString(`<span role="button">nie erreicht</span>`)
	k := probeHTML(t, s, kbPage(b.String()), KeyboardOptions{MaxPresses: 50}).Keyboard
	if k.Completed || k.Presses != 50 || k.MaxPresses != 50 {
		t.Errorf("completed=%v presses=%d max=%d, want false 50 50", k.Completed, k.Presses, k.MaxPresses)
	}
	if len(k.Unreachable) != 0 {
		t.Errorf("unreachable judged although the limit was reached: %v", sels(k.Unreachable))
	}
}

// Spec: A5-9 — a probe that runs out of time leaves the page without keyboard result, but keeps HTML and axe.
func TestRenderKeepsPageWhenProbeTimesOut(t *testing.T) {
	s := openOrSkip(t, &Crawler{Validate: allowLoopback})
	old := probeTimeout
	probeTimeout = time.Millisecond
	defer func() { probeTimeout = old }()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	p, err := s.analyze(ctx, "https://shop.example/", kbPage(`<a href="/x">Link</a>`), 20*time.Second, KeyboardOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if p.Keyboard != nil || p.Axe == nil || !strings.Contains(p.HTML, "Link") {
		t.Errorf("keyboard=%v axe=%v html=%d bytes", p.Keyboard, p.Axe != nil, len(p.HTML))
	}
}

// Spec: A5-9, A5-7 — when the probe's time runs out during the Tab loop, the presses so far count as a reached limit.
func TestProbeKeyboardTimeoutKeepsPartialResult(t *testing.T) {
	s := openOrSkip(t, &Crawler{Validate: allowLoopback})
	old := probeTimeout
	probeTimeout = 3 * time.Second
	defer func() { probeTimeout = old }()
	var b strings.Builder
	for i := range 1200 {
		fmt.Fprintf(&b, `<a href="/p%d">Produkt %d</a> `, i, i)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	p, err := s.analyze(ctx, "https://shop.example/", kbPage(b.String()), 50*time.Second, KeyboardOptions{MaxPresses: 1000})
	if err != nil {
		t.Fatal(err)
	}
	k := p.Keyboard
	if k == nil {
		t.Fatal("no keyboard report after the probe ran out of time")
	}
	if k.Completed || k.Presses == 0 || k.Presses >= 1000 || len(k.Unreachable) != 0 {
		t.Errorf("completed=%v presses=%d unreachable=%d", k.Completed, k.Presses, len(k.Unreachable))
	}
}

// Spec: A5 Logic 7, A5-5 — an error page takes no sample slot.
func TestCrawlKeyboardSkipsErrorPages(t *testing.T) {
	if _, err := FindBrowser(""); err != nil {
		t.Skip("no browser installed")
	}
	mux := http.NewServeMux()
	product := kbPage(`<script type="application/ld+json">{"@type":"Product","name":"x"}</script><a href="/">Start</a>`)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		switch r.URL.Path {
		case "/":
			_, _ = w.Write([]byte(kbPage(`<a href="/produkt-a">A</a> <a href="/produkt-b">B</a>`)))
		case "/produkt-a":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(product))
		default:
			_, _ = w.Write([]byte(product))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	opt := Options{MaxPages: 10, MaxDepth: 2, PageTimeout: 20 * time.Second, MinInterval: time.Millisecond}
	res, err := (&Crawler{Validate: allowLoopback}).Crawl(ctx, srv.URL+"/", opt, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	probed := map[string]string{}
	for _, p := range res.Pages {
		if p.Keyboard != nil {
			probed[strings.TrimPrefix(p.URL, srv.URL)] = p.Keyboard.PageType
		}
	}
	if want := map[string]string{"/": "start", "/produkt-b": "product"}; fmt.Sprint(probed) != fmt.Sprint(want) {
		t.Errorf("probed %v, want %v", probed, want)
	}
}

// Spec: A5-5, A5-6 — the crawl probes the sample (start + first page per type) or, in mode "alle Seiten", every page.
func TestCrawlKeyboardSample(t *testing.T) {
	if _, err := FindBrowser(""); err != nil {
		t.Skip("no browser installed")
	}
	mux := http.NewServeMux()
	page := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(kbPage(body)))
		}
	}
	mux.HandleFunc("/", page(`<a href="/produkt-a">A</a> <a href="/produkt-b">B</a> <a href="/impressum">I</a>`))
	mux.HandleFunc("/produkt-a", page(`<a href="/">Start</a>`))
	mux.HandleFunc("/produkt-b", page(`<a href="/">Start</a>`))
	mux.HandleFunc("/impressum", page(`<a href="/">Start</a>`))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for _, all := range []bool{false, true} {
		c := &Crawler{Validate: allowLoopback}
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		opt := Options{MaxPages: 10, MaxDepth: 2, PageTimeout: 20 * time.Second, MinInterval: time.Millisecond, Keyboard: KeyboardOptions{AllPages: all}}
		res, err := c.Crawl(ctx, srv.URL+"/", opt, nil, nil)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		probed := map[string]string{}
		for _, p := range res.Pages {
			if p.Keyboard != nil {
				probed[strings.TrimPrefix(p.URL, srv.URL)] = p.Keyboard.PageType
			}
		}
		want := map[string]string{"/": "start", "/produkt-a": "product"}
		if all {
			want = map[string]string{"/": "start", "/produkt-a": "product", "/produkt-b": "product", "/impressum": ""}
		}
		if fmt.Sprint(probed) != fmt.Sprint(want) {
			t.Errorf("all=%v: probed %v, want %v", all, probed, want)
		}
	}
}
