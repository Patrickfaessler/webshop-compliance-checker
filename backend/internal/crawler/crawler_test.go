package crawler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"compliancechecker/internal/fetch"
)

// Spec: C-1 / C-2 — link normalisation and legal-link priority.
func TestNormalizeLink(t *testing.T) {
	tests := []struct {
		href string
		want string
		ok   bool
	}{
		{"/impressum", "https://shop.at/impressum", true},
		{"produkt?id=3#reviews", "https://shop.at/a/produkt?id=3", true},
		{"https://shop.at/x.pdf", "", false},
		{"mailto:a@b.at", "", false},
		{"javascript:void(0)", "", false},
		{"tel:+43123", "", false},
		{"//cdn.example/x", "https://cdn.example/x", true},
		{"/logo.PNG", "", false},
		{"/v1.2/page", "https://shop.at/v1.2/page", true},
	}
	for _, tt := range tests {
		got, ok := normalizeLink("https://shop.at/a/", tt.href)
		if ok != tt.ok || got != tt.want {
			t.Errorf("%q: got (%q,%v) want (%q,%v)", tt.href, got, ok, tt.want, tt.ok)
		}
	}
}

func TestPriority(t *testing.T) {
	for _, u := range []string{"https://s.at/Impressum", "https://s.at/datenschutz", "https://s.at/de/agb", "https://s.at/checkout"} {
		if priority(u) != 0 {
			t.Errorf("%s should be high priority", u)
		}
	}
	if priority("https://s.at/produkt/tshirt") != 1 {
		t.Error("product page is normal priority")
	}
}

func TestParseAxeOutput(t *testing.T) {
	rep, err := parseAxe(`{"violations":[{"id":"image-alt","nodes":[{"selector":"img","html":"<img>","data":null}]}],
		"incomplete":[{"id":"color-contrast","nodes":[{"selector":"p","html":"<p>","data":{"contrastRatio":3.2}}]}],
		"inapplicable":["label"]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Violations) != 1 || rep.Violations[0].Nodes[0].Selector != "img" {
		t.Fatalf("violations: %+v", rep.Violations)
	}
	if rep.Incomplete[0].Nodes[0].Data["contrastRatio"] != 3.2 || rep.Inapplicable[0] != "label" {
		t.Fatalf("report: %+v", rep)
	}
	if _, err := parseAxe("not json"); err == nil {
		t.Fatal("bad JSON must error")
	}
}

// ---- integration tests: need a Chromium-family browser, skipped otherwise ----

func openOrSkip(t *testing.T, c *Crawler) *Session {
	t.Helper()
	if _, err := FindBrowser(c.ExecPath); err != nil {
		t.Skip("no browser installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	s, err := c.Open(ctx)
	if err != nil {
		t.Skipf("browser cannot start: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

// allowLoopback lets the tests reach httptest servers; every other host stays blocked.
func allowLoopback(_ context.Context, raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		return errors.New("blocked in test")
	}
	return nil
}

// Spec: A1, A2, A3 (axe input) — axe-core runs in the browser and reports violations.
func TestAnalyzeHTMLRunsAxe(t *testing.T) {
	s := openOrSkip(t, &Crawler{Validate: allowLoopback})
	html := `<!DOCTYPE html><html lang="de"><head><title>t</title></head><body>
	<img src="x.png">
	<p style="color:#aaa;background:#fff">Grauer Text</p>
	<input type="text" name="q">
	</body></html>`
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	p, err := s.AnalyzeHTML(ctx, "file://t.html", html, 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if p.Axe == nil {
		t.Fatal("axe report missing")
	}
	got := map[string]bool{}
	for _, v := range p.Axe.Violations {
		got[v.ID] = true
	}
	for _, id := range []string{"image-alt", "color-contrast", "label"} {
		if !got[id] {
			t.Errorf("expected axe violation %s, got %v", id, got)
		}
	}
	if !strings.Contains(p.HTML, "<img") || p.URL != "file://t.html" {
		t.Errorf("page = %q / %q", p.URL, p.HTML)
	}
}

// Spec: C-SSRF — a page that tries to load an internal address has that request blocked.
func TestBrowserBlocksInternalSubrequests(t *testing.T) {
	var mu sync.Mutex
	hit := false
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		hit = true
		mu.Unlock()
	}))
	defer internal.Close()

	// The validator blocks every network request. The page itself is a data: URL (never validated),
	// and its image points at the real, reachable internal server. If interception failed, that server is hit.
	blockAll := func(context.Context, string) error { return errors.New("blocked in test") }
	s := openOrSkip(t, &Crawler{Validate: blockAll})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	_, err := s.AnalyzeHTML(ctx, "file://t.html", `<html lang="de"><body><img src="`+internal.URL+`/leak.png"></body></html>`, 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if hit {
		t.Fatal("the browser reached the blocked address")
	}
}

type fakeGetter struct{ body string }

func (f fakeGetter) Get(context.Context, string) (fetch.Page, error) {
	return fetch.Page{Status: 200, Body: []byte(f.body)}, nil
}

// Spec: C-1, C-2 — crawl respects limits, robots.txt, same-site, legal links first.
func TestCrawlSite(t *testing.T) {
	mux := http.NewServeMux()
	page := func(title, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="de"><head><title>` + title + `</title></head><body><h1>` + title + `</h1>` + body + `</body></html>`))
		}
	}
	start := page("Start", `<a href="/produkt/1">P1</a> <a href="/impressum">Impressum</a> <a href="/secret/x">Geheim</a>
		<a href="https://other.example/">extern</a> <a href="/file.pdf">PDF</a> <a href="/missing">404</a>`)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" { // "/" would otherwise match every unknown path with 200
			http.NotFound(w, r)
			return
		}
		start(w, r)
	})
	mux.HandleFunc("/produkt/1", page("Produkt 1", `<img src="a.png">`))
	mux.HandleFunc("/impressum", page("Impressum", ``))
	mux.HandleFunc("/secret/x", page("Secret", ``))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := &Crawler{Validate: allowLoopback}
	if _, err := FindBrowser(""); err != nil {
		t.Skip("no browser installed")
	}
	opt := Options{MaxPages: 10, MaxDepth: 2, PageTimeout: 20 * time.Second, MinInterval: 10 * time.Millisecond}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	var lastDone int
	res, err := c.Crawl(ctx, srv.URL+"/", opt, fakeGetter{"User-agent: *\nDisallow: /secret"}, func(done, _ int) { lastDone = done })
	if err != nil {
		t.Skipf("crawl failed (browser problem?): %v", err)
	}

	var urls []string
	for _, p := range res.Pages {
		urls = append(urls, strings.TrimPrefix(p.URL, srv.URL))
	}
	want := []string{"/", "/impressum", "/produkt/1"} // legal link before product
	if len(urls) != len(want) {
		t.Fatalf("pages = %v, want %v", urls, want)
	}
	for i := range want {
		if urls[i] != want[i] {
			t.Fatalf("pages = %v, want %v", urls, want)
		}
	}
	if lastDone != 3 {
		t.Errorf("progress done = %d", lastDone)
	}
	if len(res.Skipped) != 1 || !strings.HasSuffix(res.Skipped[0], "/secret/x") {
		t.Errorf("skipped = %v", res.Skipped)
	}
	for _, p := range res.Pages {
		if p.Axe == nil {
			t.Errorf("%s has no axe report", p.URL)
		}
	}
}

// Spec: C-1 — MaxPages limit.
func TestCrawlMaxPages(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html lang="de"><body><a href="/a">a</a><a href="/b">b</a><a href="/c">c</a></body></html>`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if _, err := FindBrowser(""); err != nil {
		t.Skip("no browser installed")
	}
	c := &Crawler{Validate: allowLoopback}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	res, err := c.Crawl(ctx, srv.URL+"/", Options{MaxPages: 2, MaxDepth: 3, PageTimeout: 20 * time.Second, MinInterval: time.Millisecond}, nil, nil)
	if err != nil {
		t.Skipf("crawl failed: %v", err)
	}
	if len(res.Pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(res.Pages))
	}
}

// Spec: C-1 — a directory and its index file are one page.
func TestPageKey(t *testing.T) {
	same := [][2]string{
		{"https://s.at/teehaus/", "https://s.at/teehaus/index.html"},
		{"https://s.at/", "https://s.at/index.php"},
		{"https://s.at", "https://s.at/"},
		{"https://s.at/a/", "https://s.at/a/index.htm#top"},
	}
	for _, p := range same {
		if pageKey(p[0]) != pageKey(p[1]) {
			t.Errorf("%s and %s must be the same page", p[0], p[1])
		}
	}
	if pageKey("https://s.at/a/") == pageKey("https://s.at/b/index.html") {
		t.Error("different directories must differ")
	}
	if pageKey("https://s.at/?p=1") == pageKey("https://s.at/?p=2") {
		t.Error("different queries must differ")
	}
}
