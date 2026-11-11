package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"compliancechecker/internal/ratelimit"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/a6"
	"compliancechecker/internal/scan"
	"compliancechecker/internal/worker"
)

type fakeSource struct{ html string }

func (f fakeSource) Pages(_ context.Context, u string, _ func(done, total int)) ([]rules.Page, bool, error) {
	return []rules.Page{{URL: u, HTML: f.html}}, false, nil
}

func lookup(_ context.Context, host string) ([]netip.Addr, error) {
	switch {
	case strings.HasSuffix(host, ".example"):
		return []netip.Addr{netip.MustParseAddr("93.184.216.34")}, nil
	case host == "internal.test":
		return []netip.Addr{netip.MustParseAddr("10.0.0.5")}, nil
	}
	return nil, context.DeadlineExceeded
}

type env struct {
	srv  *Server
	h    http.Handler
	jobs chan worker.Job
	st   *scan.MemoryStore
}

func newEnv(maxPerWindow, maxRunning int) env {
	st := scan.NewMemoryStore()
	jobs := make(chan worker.Job, 8)
	srv := &Server{
		Store: st, Limiter: ratelimit.New(maxPerWindow, time.Hour, maxRunning), Jobs: jobs,
		Lookup: lookup, Salt: []byte("test"), HostCooldown: 10 * time.Minute,
	}
	return env{srv, srv.Handler(), jobs, st}
}

func (e env) do(method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	req.RemoteAddr = "203.0.113.7:5555"
	rec := httptest.NewRecorder()
	e.h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("bad JSON %q: %v", rec.Body.String(), err)
	}
}

// Spec: F1 / C-SSRF — invalid and internal URLs are rejected with a German problem message.
func TestCreateScanRejectsBadURLs(t *testing.T) {
	e := newEnv(3, 3)
	for _, u := range []string{
		"ftp://shop.example/", "http://shop.example:8080/", "http://127.0.0.1/", "http://169.254.169.254/",
		"https://internal.test/", "https://unknown.test/", "not a url", "",
	} {
		rec := e.do("POST", "/api/scans", `{"url":"`+u+`"}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%q: status %d, want 400", u, rec.Code)
			continue
		}
		var p struct{ Title string }
		decode(t, rec, &p)
		if p.Title == "" || !strings.Contains(rec.Header().Get("Content-Type"), "problem+json") {
			t.Errorf("%q: missing German problem details", u)
		}
	}
	if rec := e.do("POST", "/api/scans", `{"url":"https://shop.example/","areas":["bogus"]}`); rec.Code != 400 {
		t.Errorf("bad area: status %d", rec.Code)
	}
	if rec := e.do("POST", "/api/scans", `not json`); rec.Code != 400 {
		t.Errorf("bad json: status %d", rec.Code)
	}
}

// Spec: F1, F11 — a valid request returns 202, a random scan id and queues a job; result is readable by id.
func TestCreateAndGetScan(t *testing.T) {
	e := newEnv(3, 3)
	rec := e.do("POST", "/api/scans", `{"url":"https://shop.example/"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var st scanStatus
	decode(t, rec, &st)
	if len(st.ID) < 22 || st.Status != "queued" || st.DisclaimerDe == "" {
		t.Fatalf("unexpected status: %+v", st)
	}
	if exp := time.Until(st.ExpiresAt); exp < 59*time.Minute || exp > 61*time.Minute { // ADR-6
		t.Fatalf("expiry not ~1 hour: %v", exp)
	}
	if len(e.jobs) != 1 {
		t.Fatal("job not queued")
	}

	get := e.do("GET", "/api/scans/"+st.ID, "")
	if get.Code != 200 {
		t.Fatalf("get: %d", get.Code)
	}
	if nf := e.do("GET", "/api/scans/doesnotexist", ""); nf.Code != 404 {
		t.Fatalf("unknown id: %d", nf.Code)
	}
}

// Spec: C-RL — same host within the cooldown returns the existing scan; no second job.
func TestHostCooldownReturnsExisting(t *testing.T) {
	e := newEnv(3, 3)
	var a, b scanStatus
	decode(t, e.do("POST", "/api/scans", `{"url":"https://shop.example/"}`), &a)
	rec := e.do("POST", "/api/scans", `{"url":"https://SHOP.example/other"}`)
	decode(t, rec, &b)
	if rec.Code != http.StatusOK || a.ID != b.ID || len(e.jobs) != 1 {
		t.Fatalf("code=%d ids %q/%q jobs=%d", rec.Code, a.ID, b.ID, len(e.jobs))
	}
}

// Spec: C-RL / F12 — 3 scans per hour per client, then 429 with Retry-After.
func TestRateLimit(t *testing.T) {
	e := newEnv(3, 10)
	for i, h := range []string{"a", "b", "c"} {
		if rec := e.do("POST", "/api/scans", `{"url":"https://`+h+`.example/"}`); rec.Code != 202 {
			t.Fatalf("scan %d: %d", i, rec.Code)
		}
	}
	rec := e.do("POST", "/api/scans", `{"url":"https://d.example/"}`)
	if rec.Code != 429 || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("code=%d retry-after=%q", rec.Code, rec.Header().Get("Retry-After"))
	}
}

// Spec: C-RL — only one running scan per client.
func TestOneRunningPerClient(t *testing.T) {
	e := newEnv(3, 1)
	if rec := e.do("POST", "/api/scans", `{"url":"https://a.example/"}`); rec.Code != 202 {
		t.Fatal(rec.Code)
	}
	if rec := e.do("POST", "/api/scans", `{"url":"https://b.example/"}`); rec.Code != 429 {
		t.Fatalf("second concurrent scan: %d", rec.Code)
	}
}

// Spec: F1 — full queue gives 503 and frees the limiter slot.
func TestQueueFull(t *testing.T) {
	e := newEnv(3, 3)
	e.srv.Jobs = make(chan worker.Job) // unbuffered, nobody reading
	rec := e.do("POST", "/api/scans", `{"url":"https://a.example/"}`)
	if rec.Code != 503 {
		t.Fatalf("code %d", rec.Code)
	}
}

// Spec: F7, A6-2, S-1 — end to end: create scan, run the worker, read score and findings.
func TestEndToEndA6(t *testing.T) {
	e := newEnv(3, 3)
	var st scanStatus
	decode(t, e.do("POST", "/api/scans", `{"url":"https://shop.example/","areas":["accessibility"]}`), &st)

	r := &worker.Runner{
		Store: e.st, Source: fakeSource{html: "<html><body></body></html>"},
		Rules: []rules.Rule{a6.New()}, Release: e.srv.Limiter.Release,
	}
	r.Run(context.Background(), <-e.jobs)

	var done scanStatus
	decode(t, e.do("GET", "/api/scans/"+st.ID, ""), &done)
	if done.Status != "done" || done.Score == nil || *done.Score != 0 {
		t.Fatalf("status: %+v", done)
	}

	var fr struct {
		RuleResults []ruleResultJSON `json:"ruleResults"`
		Findings    []findingJSON    `json:"findings"`
	}
	decode(t, e.do("GET", "/api/scans/"+st.ID+"/findings", ""), &fr)
	if len(fr.RuleResults) != 1 || fr.RuleResults[0].State != "failed" || len(fr.Findings) != 1 {
		t.Fatalf("findings: %+v", fr)
	}
	if fr.Findings[0].Severity != "notice" || fr.Findings[0].SummaryDe == "" {
		t.Fatalf("finding: %+v", fr.Findings[0])
	}

	// filters
	decode(t, e.do("GET", "/api/scans/"+st.ID+"/findings?severity=critical", ""), &fr)
	if len(fr.Findings) != 0 {
		t.Fatal("severity filter not applied")
	}
	if rec := e.do("GET", "/api/scans/"+st.ID+"/findings?severity=bogus", ""); rec.Code != 400 {
		t.Fatalf("bogus severity: %d", rec.Code)
	}

	// slot was released, so the same client may start another scan of a different host
	if rec := e.do("POST", "/api/scans", `{"url":"https://other.example/"}`); rec.Code != 202 {
		t.Fatalf("limiter slot not released: %d", rec.Code)
	}
}

func TestCORS(t *testing.T) {
	e := newEnv(3, 3)
	e.srv.AllowOrigin = "http://localhost:5000"
	h := e.srv.Handler()
	req := httptest.NewRequest("OPTIONS", "/api/scans", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 204 || rec.Header().Get("Access-Control-Allow-Origin") != "http://localhost:5000" {
		t.Fatalf("code=%d headers=%v", rec.Code, rec.Header())
	}
}

// Spec: N4 — the client key is a salted hash, never the IP.
func TestClientKeyIsHashed(t *testing.T) {
	e := newEnv(3, 3)
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.7:1234"
	k := e.srv.clientKey(req)
	if strings.Contains(k, "203") || len(k) != 16 {
		t.Fatalf("key = %q", k)
	}
	req.RemoteAddr = "203.0.113.7:9999" // same IP, other port
	if e.srv.clientKey(req) != k {
		t.Fatal("key must not depend on the port")
	}
}

// runScan creates a scan through the API and runs it with a page that fails A6.
func runScan(t *testing.T, e env) string {
	t.Helper()
	var st scanStatus
	decode(t, e.do("POST", "/api/scans", `{"url":"https://shop.example/","areas":["accessibility"]}`), &st)
	r := &worker.Runner{Store: e.st, Source: fakeSource{html: "<html><body></body></html>"}, Rules: []rules.Rule{a6.New()}, Release: e.srv.Limiter.Release}
	r.Run(context.Background(), <-e.jobs)
	return st.ID
}

// Spec: F10, F13, ADR-6 — finished scans download as JSON and PDF attachments.
func TestReportDownloads(t *testing.T) {
	e := newEnv(3, 3)
	e.srv.RuleMeta = map[string]rules.Meta{"A6": a6.New().Meta()}
	id := runScan(t, e)

	rec := e.do("GET", "/api/scans/"+id+"/report.json", "")
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("json: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.HasPrefix(cd, `attachment; filename="webshop-check_shop.example_`) || !strings.HasSuffix(cd, `.json"`) {
		t.Fatalf("content-disposition: %q", cd)
	}
	var rep struct {
		Format      string
		RuleResults []struct{ RuleID, NameDe, State string }
		Findings    []struct{ Selector string }
	}
	decode(t, rec, &rep)
	if rep.Format != "webshop-check-report" || rep.RuleResults[0].NameDe != "Seitensprache angegeben" || rep.RuleResults[0].State != "failed" || rep.Findings[0].Selector != "html" {
		t.Fatalf("report: %+v", rep)
	}

	rec = e.do("GET", "/api/scans/"+id+"/report.pdf", "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/pdf" || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) {
		t.Fatalf("pdf: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("reports must not be cached")
	}
}

// Spec: F10, F13 — a running scan gives 409, an unknown or expired one 404 with the one-hour hint.
func TestReportNotReady(t *testing.T) {
	e := newEnv(3, 3)
	var st scanStatus
	decode(t, e.do("POST", "/api/scans", `{"url":"https://shop.example/"}`), &st)
	for _, ext := range []string{"json", "pdf"} {
		if rec := e.do("GET", "/api/scans/"+st.ID+"/report."+ext, ""); rec.Code != http.StatusConflict {
			t.Errorf("%s queued: %d", ext, rec.Code)
		}
		rec := e.do("GET", "/api/scans/nope/report."+ext, "")
		if rec.Code != 404 || !strings.Contains(rec.Body.String(), "nach einer Stunde") {
			t.Errorf("%s unknown: %d %s", ext, rec.Code, rec.Body)
		}
	}
}
