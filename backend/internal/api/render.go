package api

import (
	"bytes"
	"context"
	"log"
	"sync"
	"time"

	"compliancechecker/internal/report"
	"compliancechecker/internal/scan"
)

// Render limits. Spec: specs/11-report.md RP-12.
const (
	maxCachedReports = 32
	maxPrints        = 2
	defaultPrintWait = 20 * time.Second
	printTimeout     = 60 * time.Second
)

// rendered holds the HTML and printed PDF of one finished scan (RP-12).
type rendered struct {
	lock    chan struct{} // one render per scan at a time; a channel so waiting can be cancelled
	added   time.Time
	expires time.Time
	html    []byte
	pdf     []byte            // nil until printed
	crops   map[string][]byte // finding id → JPEG crop (F8); nil until first requested
}

// renderCache keeps rendered reports for at most the scan retention and at most maxCachedReports scans.
type renderCache struct {
	mu sync.Mutex
	m  map[string]*rendered
}

func (c *renderCache) entry(sc scan.Scan, now time.Time) *rendered {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = map[string]*rendered{}
	}
	for id, e := range c.m {
		if !now.Before(e.expires) {
			delete(c.m, id) // N4, ADR-6: never keep a report longer than its scan
		}
	}
	if e, ok := c.m[sc.ID]; ok {
		return e
	}
	if len(c.m) >= maxCachedReports {
		oldest := ""
		for id, e := range c.m {
			if oldest == "" || e.added.Before(c.m[oldest].added) {
				oldest = id
			}
		}
		delete(c.m, oldest)
	}
	exp := sc.ExpiresAt
	if exp.IsZero() {
		exp = now.Add(scan.Retention)
	}
	e := &rendered{lock: make(chan struct{}, 1), added: now, expires: exp}
	c.m[sc.ID] = e
	return e
}

func (e *rendered) acquire(ctx context.Context) bool {
	select {
	case e.lock <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

func (e *rendered) release() { <-e.lock }

// htmlLocked returns the cached HTML, rendering it on first use. The caller holds e.lock.
func (e *rendered) htmlLocked(rep report.Report) ([]byte, error) {
	if e.html == nil {
		b, err := rep.HTML()
		if err != nil {
			return nil, err
		}
		e.html = b
	}
	return e.html, nil
}

// cachedHTML returns the HTML report of a finished scan, rendered once (RP-12).
func (s *Server) cachedHTML(ctx context.Context, sc scan.Scan, rep report.Report) ([]byte, error) {
	e := s.renders.entry(sc, s.now())
	if !e.acquire(ctx) {
		return nil, ctx.Err()
	}
	defer e.release()
	return e.htmlLocked(rep)
}

// printedPDF returns the PDF printed from the HTML report (RP-10), or nil if it cannot be printed now (RP-11, RP-12).
func (s *Server) printedPDF(ctx context.Context, sc scan.Scan, rep report.Report) []byte {
	e := s.renders.entry(sc, s.now())
	if !e.acquire(ctx) {
		return nil
	}
	defer e.release()
	if e.pdf != nil {
		return e.pdf
	}
	doc, err := e.htmlLocked(rep)
	if err != nil {
		log.Printf("report: building HTML for the PDF failed, serving text PDF: %v", err)
		return nil
	}

	wait := s.printWait
	if wait <= 0 {
		wait = defaultPrintWait
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	slots := s.printSlots()
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-t.C:
		log.Printf("report: %d PDFs already printing, serving text PDF", maxPrints)
		return nil
	case <-ctx.Done():
		return nil
	}

	b, err := s.PDF.PrintPDF(ctx, doc, printTimeout)
	if err != nil {
		log.Printf("report: printing PDF failed, serving text PDF: %v", err)
		return nil
	}
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		log.Printf("report: printer returned no PDF (%d bytes), serving text PDF", len(b))
		return nil
	}
	e.pdf = b
	return b
}

func (s *Server) printSlots() chan struct{} {
	s.printOnce.Do(func() { s.prints = make(chan struct{}, maxPrints) })
	return s.prints
}

// cachedCrop returns the crop of one finding; all crops of the scan are cut once (F8, RP-12).
func (s *Server) cachedCrop(ctx context.Context, sc scan.Scan, findingID string) ([]byte, bool) {
	if sc.Status != scan.Done && sc.Status != scan.Incomplete { // not final yet: do not cache
		b, ok := report.FindingCrops(sc)[findingID]
		return b, ok
	}
	e := s.renders.entry(sc, s.now())
	if !e.acquire(ctx) {
		return nil, false
	}
	defer e.release()
	if e.crops == nil {
		e.crops = report.FindingCrops(sc)
	}
	b, ok := e.crops[findingID]
	return b, ok
}
