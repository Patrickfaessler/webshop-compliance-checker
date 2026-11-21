package report

import (
	"bytes"
	"strings"
	"testing"

	"compliancechecker/internal/rules"
)

// Spec: RP-13 — the JSON report carries explanation and fix per finding.
func TestReportJSONFix(t *testing.T) {
	sc := finished()
	sc.Findings[0].ExplanationDe = "Erklärung"
	sc.Findings[0].Fix = &rules.Fix{TextDe: "Ändere die Farbe.", Code: "color: #595959;"}
	r, err := Build(sc, nil, t0)
	if err != nil {
		t.Fatal(err)
	}
	js, _ := r.JSON()
	for _, want := range []string{`"explanationDe": "Erklärung"`, `"origin": "template"`, `"textDe": "Ändere die Farbe."`, `"code": "color: #595959;"`} {
		if !bytes.Contains(js, []byte(want)) {
			t.Errorf("JSON lacks %s: %s", want, js)
		}
	}
}

// Spec: RP-13 — every finding card shows the fix block with text, code and note.
func TestHTMLFixBlock(t *testing.T) {
	sc := finished()
	sc.Findings[0].Fix = &rules.Fix{TextDe: "Ändere die Farbe.", Code: "color: #595959;", NoteDe: "bitte prüfen"}
	r, _ := Build(sc, nil, t0)
	doc, err := r.HTML()
	if err != nil {
		t.Fatal(err)
	}
	s := string(doc)
	for _, want := range []string{`class="fix"`, "So können Sie es beheben", "(Vorlage)", "<p>Ändere die Farbe.</p>", "<pre><code>color: #595959;</code></pre>", "bitte prüfen"} {
		if !strings.Contains(s, want) {
			t.Errorf("HTML lacks %q", want)
		}
	}
}

// Spec: RP-13 — code built from page values cannot inject markup into the report (Review Focus 1).
func TestHTMLFixEscaped(t *testing.T) {
	sc := finished()
	sc.Findings[0].Fix = &rules.Fix{TextDe: "t", Code: `<label for="x">&#34;&gt;&lt;script&gt;</label>`}
	r, _ := Build(sc, nil, t0)
	doc, _ := r.HTML()
	if strings.Contains(string(doc), "<script") || strings.Contains(string(doc), `<label for="x">`) {
		t.Errorf("fix code not escaped in HTML")
	}
}

// Spec: RP-13 — the text PDF fallback prints the fix.
func TestPDFFallbackFix(t *testing.T) {
	sc := finished()
	sc.Findings[0].Fix = &rules.Fix{TextDe: "Ändere die Farbe.", Code: "color: #595959;"}
	r, _ := Build(sc, nil, t0)
	b, err := r.PDF(PDFOptions{Uncompressed: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("Vorschlag: \xc4ndere die Farbe.")) || !bytes.Contains(b, []byte("color: #595959;")) {
		t.Error("fix missing in PDF")
	}
}
