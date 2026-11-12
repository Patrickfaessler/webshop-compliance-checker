package api

import (
	"bytes"
	"context"
	"errors"
	"strings"
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
	rec = e.do("GET", "/api/scans/"+id+"/report.pdf", "")
	if rec.Code != 200 || !bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF-")) || rec.Body.String() == "%PDF-1.7 printed" {
		t.Fatalf("fallback pdf: %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/pdf" {
		t.Fatal(rec.Header().Get("Content-Type"))
	}
}
