package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/scan"
)

var t0 = time.Date(2026, 11, 8, 13, 54, 0, 0, time.UTC) // 14:54 in Vienna

var metas = map[string]rules.Meta{
	"A6": {ID: "A6", NameDe: "Seitensprache angegeben", Severity: rules.Notice},
	"A4": {ID: "A4", NameDe: "Überschriftenstruktur", Severity: rules.Warning},
}

func finished() scan.Scan {
	score := 60
	return scan.Scan{
		ID: "abc123", Host: "Shop.Example", StartURL: "https://shop.example/", Areas: []string{"accessibility"},
		Status: scan.Done, Score: &score, PagesDone: 3, CreatedAt: t0, FinishedAt: t0.Add(time.Minute), ExpiresAt: t0.Add(scan.Retention),
		RuleResults: []scan.RuleResult{
			{RuleID: "A6", State: rules.Passed, DecidedBy: rules.DecidedStructural},
			{RuleID: "A4", State: rules.Failed, DecidedBy: rules.DecidedStructural, PagesAffected: 2},
		},
		Findings: []scan.Finding{{
			ID: "abc123-A4-1", RuleID: "A4", Severity: rules.Warning, URL: "https://shop.example/produkt",
			Selector: "html > body > h3", Snippet: `<h3 class="x">`, Sub: "skipped-level",
			SummaryDe: "Die Überschriften der Seite sind nicht sauber gegliedert.",
		}},
	}
}

// Spec: F13 — JSON report carries format, version, scan data, named rule results and full findings.
func TestBuildJSON(t *testing.T) {
	r, err := Build(finished(), metas, t0.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["format"] != Format || got["version"] != float64(1) || got["disclaimerDe"] != scan.DisclaimerDe {
		t.Fatalf("header: %v", got)
	}
	sc := got["scan"].(map[string]any)
	if sc["score"] != float64(60) || sc["pagesScanned"] != float64(3) || sc["startUrl"] != "https://shop.example/" {
		t.Fatalf("scan: %v", sc)
	}
	rr := got["ruleResults"].([]any)[1].(map[string]any)
	if rr["nameDe"] != "Überschriftenstruktur" || rr["severity"] != "warning" || rr["pagesAffected"] != float64(2) {
		t.Fatalf("rule result: %v", rr)
	}
	f := got["findings"].([]any)[0].(map[string]any)
	if f["selector"] != "html > body > h3" || f["snippet"] != `<h3 class="x">` || f["sub"] != "skipped-level" {
		t.Fatalf("finding: %v", f)
	}
}

// Spec: F10, F13 — only finished scans give a report.
func TestBuildRejectsUnfinished(t *testing.T) {
	for _, st := range []scan.Status{scan.Queued, scan.Running, scan.Failed} {
		sc := finished()
		sc.Status = st
		if _, err := Build(sc, metas, t0); !errors.Is(err, ErrNotFinished) {
			t.Errorf("%s: err = %v", st, err)
		}
	}
	sc := finished()
	sc.Status = scan.Incomplete
	if _, err := Build(sc, metas, t0); err != nil {
		t.Errorf("incomplete scans must give a report: %v", err)
	}
}

// Spec: F13 — empty lists are [] not null; a missing score is null; unknown rules fall back to their id.
func TestBuildEdgeCases(t *testing.T) {
	sc := finished()
	sc.Score, sc.Findings, sc.FinishedAt = nil, nil, time.Time{}
	sc.RuleResults = []scan.RuleResult{{RuleID: "R9", State: rules.NotAssessable}}
	r, _ := Build(sc, metas, t0)
	b, _ := r.JSON()
	s := string(b)
	for _, want := range []string{`"score": null`, `"findings": []`, `"finishedAt": null`, `"nameDe": "R9"`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
}

func TestFileName(t *testing.T) {
	r, _ := Build(finished(), metas, t0)
	if got := r.FileName("pdf"); got != "webshop-check_shop.example_2026-11-08.pdf" {
		t.Fatalf("file name = %q", got)
	}
	r.Scan.Host = "a/b c"
	if got := r.FileName("json"); got != "webshop-check_a-b-c_2026-11-08.json" {
		t.Fatalf("file name = %q", got)
	}
}

// Spec: F10, N3, S-3 — PDF contains URL, Vienna time, score wording, rules, findings and the disclaimer.
func TestPDF(t *testing.T) {
	r, _ := Build(finished(), metas, t0.Add(2*time.Minute))
	b, err := r.PDF(PDFOptions{Uncompressed: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		t.Fatal("not a PDF")
	}
	// core fonts store text as cp1252: ü = 0xfc
	for _, want := range []string{
		"Pr\xfcfbericht Webshop-Check",
		"https://shop.example/",
		"08.11.2026, 14:54 Uhr",
		"Anteil bestandener automatischer Pr\xfcfungen: 60 %",
		"keine Aussage \xfcber vollst\xe4ndige Konformit\xe4t",
		"\xdcberschriftenstruktur",
		"Nicht bestanden",
		"html > body > h3",
		"Automatische Pr\xfcfungen erfassen nur einen Teil",
	} {
		if !bytes.Contains(b, []byte(want)) {
			t.Errorf("PDF misses %q", want)
		}
	}
	if bytes.Contains(b, []byte("konform ")) || bytes.Contains(b, []byte("compliant")) {
		t.Error("PDF must not call the shop compliant (S-3)")
	}
}

// Spec: F10 — a report without findings or score still renders, and long content breaks across pages.
func TestPDFEdgeCases(t *testing.T) {
	sc := finished()
	sc.Score, sc.Findings = nil, nil
	r, _ := Build(sc, metas, t0)
	b, err := r.PDF(PDFOptions{Uncompressed: true})
	if err != nil || !bytes.Contains(b, []byte("nicht berechenbar")) || !bytes.Contains(b, []byte("keine Probleme gefunden")) {
		t.Fatalf("err=%v", err)
	}

	sc = finished()
	for i := 0; i < 80; i++ {
		f := sc.Findings[0]
		f.Snippet = strings.Repeat("<div class=\"very-long-class-name\">", 20)
		sc.Findings = append(sc.Findings, f)
	}
	r, _ = Build(sc, metas, t0)
	b, err = r.PDF(PDFOptions{})
	if err != nil || len(b) < 1000 {
		t.Fatalf("big report: err=%v size=%d", err, len(b))
	}
}
