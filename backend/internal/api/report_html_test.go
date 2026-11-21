package api

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/a6"
)

// Spec: F14, RP-5 — the HTML report is shown inline with a strict CSP, or downloaded with ?download=1.
func TestReportHTML(t *testing.T) {
	e := newEnv(3, 3)
	e.srv.RuleMeta = map[string]rules.Meta{"A6": a6.New().Meta()}
	id := runScan(t, e)

	rec := e.do("GET", "/api/scans/"+id+"/report.html", "")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Fatalf("html: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Content-Disposition") != "" {
		t.Fatal("inline view must not be an attachment")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") || strings.Contains(csp, "script-src") {
		t.Fatalf("csp = %q", csp)
	}
	if !strings.Contains(rec.Body.String(), "Prüfbericht Webshop-Check") || !strings.Contains(rec.Body.String(), "keinen Screenshot") {
		t.Fatal("unexpected HTML body")
	}

	rec = e.do("GET", "/api/scans/"+id+"/report.html?download=1", "")
	if cd := rec.Header().Get("Content-Disposition"); rec.Code != 200 || !strings.HasSuffix(cd, `.html"`) || !strings.HasPrefix(cd, "attachment;") {
		t.Fatalf("download: %d %q", rec.Code, cd)
	}

	if rec := e.do("GET", "/api/scans/nope/report.html", ""); rec.Code != 404 {
		t.Errorf("unknown scan: %d", rec.Code)
	}
}

type fakePrinter struct {
	out   []byte
	err   error
	calls int
	got   []byte
}

func (f *fakePrinter) PrintPDF(_ context.Context, doc []byte, _ time.Duration) ([]byte, error) {
	f.calls++
	f.got = doc
	return f.out, f.err
}

// Spec: RP-10, RP-11 — the PDF is printed from the HTML report; if printing fails the text PDF is served.
func TestReportPDFPrintedFromHTML(t *testing.T) {
	e := newEnv(3, 3)
	e.srv.RuleMeta = map[string]rules.Meta{"A6": a6.New().Meta()}
	id := runScan(t, e)

	p := &fakePrinter{out: []byte("%PDF-1.7 printed")}
	e.srv.PDF = p
	rec := e.do("GET", "/api/scans/"+id+"/report.pdf", "")
	if rec.Code != 200 || rec.Body.String() != "%PDF-1.7 printed" || p.calls != 1 || !bytes.Contains(p.got, []byte("<!DOCTYPE html>")) {
		t.Fatalf("printed pdf: %d %q calls=%d", rec.Code, rec.Body.String(), p.calls)
	}

	e.srv.PDF = &fakePrinter{err: errors.New("no browser")}
	e.srv.renders = renderCache{} // the printed PDF is cached (RP-12); start over for the fallback case
	rec = e.do("GET", "/api/scans/"+id+"/report.pdf", "")
	if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) || rec.Body.String() == "%PDF-1.7 printed" {
		t.Fatalf("fallback pdf: %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatal(rec.Header().Get("Content-Type"))
	}
}

// Spec: RP-12, RP-A7 — HTML and PDF are rendered once per scan; repeated requests get the cached bytes.
func TestReportRenderedOnce(t *testing.T) {
	e := newEnv(3, 3)
	e.srv.RuleMeta = map[string]rules.Meta{"A6": a6.New().Meta()}
	id := runScan(t, e)
	p := &fakePrinter{out: []byte("%PDF-1.7 printed")}
	e.srv.PDF = p

	clock := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	e.srv.Now = func() time.Time { return clock }
	first := e.do("GET", "/api/scans/"+id+"/report.html", "").Body.String()
	clock = clock.Add(5 * time.Minute) // a new render would show a new "erstellt am" time
	if again := e.do("GET", "/api/scans/"+id+"/report.html", "").Body.String(); again != first {
		t.Error("second HTML request rendered the report again")
	}
	for range 3 {
		if rec := e.do("GET", "/api/scans/"+id+"/report.pdf", ""); rec.Body.String() != "%PDF-1.7 printed" {
			t.Fatalf("pdf = %q", rec.Body.String())
		}
	}
	if p.calls != 1 {
		t.Errorf("printer called %d times, want 1", p.calls)
	}
	if !bytes.Equal(p.got, []byte(first)) {
		t.Error("the PDF must be printed from the cached HTML")
	}
}

// blockingPrinter holds every print until release is closed.
type blockingPrinter struct {
	started atomic.Int32
	release chan struct{}
}

func (b *blockingPrinter) PrintPDF(ctx context.Context, _ []byte, _ time.Duration) ([]byte, error) {
	b.started.Add(1)
	select {
	case <-b.release:
		return []byte("%PDF-1.7 printed"), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Spec: RP-12, RP-A7 — at most 2 prints at the same time; a request that cannot start in time gets the text PDF.
func TestReportPDFConcurrencyLimit(t *testing.T) {
	e := newEnv(3, 3)
	e.srv.RuleMeta = map[string]rules.Meta{"A6": a6.New().Meta()}
	id := runScan(t, e)
	sc, _ := e.st.Get(id)
	for _, other := range []string{"scan-2", "scan-3"} {
		c := sc
		c.ID = other
		if err := e.st.Create(c); err != nil {
			t.Fatal(err)
		}
	}
	bp := &blockingPrinter{release: make(chan struct{})}
	e.srv.PDF = bp
	e.srv.printWait = 50 * time.Millisecond

	var wg sync.WaitGroup
	for _, sid := range []string{id, "scan-2"} {
		wg.Go(func() {
			if rec := e.do("GET", "/api/scans/"+sid+"/report.pdf", ""); rec.Body.String() != "%PDF-1.7 printed" {
				t.Errorf("%s: blocked print did not finish", sid)
			}
		})
	}
	for bp.started.Load() < 2 {
		time.Sleep(time.Millisecond)
	}
	rec := e.do("GET", "/api/scans/scan-3/report.pdf", "")
	if !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) || rec.Body.String() == "%PDF-1.7 printed" {
		t.Errorf("third request: want the text PDF, got %q", rec.Body.String()[:min(rec.Body.Len(), 20)])
	}
	if n := bp.started.Load(); n != 2 {
		t.Errorf("prints started = %d, want 2", n)
	}
	close(bp.release)
	wg.Wait()
}
