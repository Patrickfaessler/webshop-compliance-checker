package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"regexp"
	"strings"
	"testing"
	"time"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/scan"
)

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 200, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// visualScan has findings on two captured pages and one page without screenshot.
func visualScan(t *testing.T) scan.Scan {
	sc := finished()
	sc.Areas = []string{"accessibility"}
	sc.RuleResults = append(sc.RuleResults, scan.RuleResult{RuleID: "A1", State: rules.Failed, PagesAffected: 2})
	shot := testJPEG(t, 1280, 1600)
	sc.Shots = []scan.Shot{
		{URL: "https://shop.example/", Image: shot, Width: 1280, Height: 1600, ViewportWidth: 1280},
		{URL: "https://shop.example/produkt", Image: shot, Width: 1280, Height: 1600, ViewportWidth: 1280, Truncated: true},
	}
	box := func(x, y float64) *scan.Box { return &scan.Box{X: x, Y: y, Width: 200, Height: 30} }
	sc.Findings = []scan.Finding{
		{ID: "1", RuleID: "A1", Severity: rules.Warning, URL: "https://shop.example/", Selector: "p.price", Snippet: `<p class="price">`, SummaryDe: "Zu wenig Kontrast.", Position: "element", Box: box(100, 400)},
		{ID: "2", RuleID: "A6", Severity: rules.Notice, URL: "https://shop.example/", Selector: "html", Snippet: "<html>", Position: "page"},
		{ID: "3", RuleID: "A1", Severity: rules.Warning, URL: "https://shop.example/produkt", Selector: "#x", Snippet: `<script>alert("x")</script>`, Position: "element", Box: box(0, 0)},
		{ID: "4", RuleID: "A4", Severity: rules.Warning, URL: "https://shop.example/produkt", Selector: "#gone", Position: "hidden", Box: box(0, 0)},
		{ID: "5", RuleID: "A4", Severity: rules.Warning, URL: "https://shop.example/agb", Selector: "h3", Snippet: "<h3>"},
	}
	return sc
}

// Spec: RP-A2 — one section per page, one frame per visible element, numbers 1…n, links both ways, disclaimer twice.
func TestHTMLReportStructure(t *testing.T) {
	r, err := Build(visualScan(t), metas, t0.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	b, err := r.HTML()
	if err != nil {
		t.Fatal(err)
	}
	h := string(b)
	if n := strings.Count(h, `class="page" id="p-`); n != 3 {
		t.Errorf("page sections = %d, want 3", n)
	}
	if n := strings.Count(h, `class="mark" id="m-`); n != 2 {
		t.Errorf("frames = %d, want 2", n)
	}
	for i := 1; i <= 5; i++ {
		if !strings.Contains(h, fmt.Sprintf(`id="f-%d"`, i)) {
			t.Errorf("card %d missing", i)
		}
	}
	if strings.Contains(h, `id="f-6"`) {
		t.Error("numbering has more cards than findings")
	}
	for _, n := range []int{1, 3} {
		if !strings.Contains(h, fmt.Sprintf(`id="m-%d" href="#f-%d"`, n, n)) || !strings.Contains(h, fmt.Sprintf(`href="#m-%d"`, n)) {
			t.Errorf("finding %d is not linked both ways", n)
		}
	}
	if strings.Count(h, scan.DisclaimerDe) != 2 {
		t.Error("disclaimer must appear at the top and at the end (N3)")
	}
	for _, want := range []string{"Betrifft die ganze Seite", "nicht sichtbar", "keinen Screenshot", "nur den oberen Teil",
		"Anteil bestandener automatischer Prüfungen", `lang="de"`, "Ausschnitt um Fundstelle 1"} {
		if !strings.Contains(h, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(strings.ToLower(h), "konform ") && !strings.Contains(h, "keine Aussage über vollständige Konformität") {
		t.Error("score must not be called compliant (S-3)")
	}
}

// Spec: RP-A3, RP-5 — no script, no external resources, user text escaped.
func TestHTMLReportIsSafe(t *testing.T) {
	r, _ := Build(visualScan(t), metas, t0)
	b, err := r.HTML()
	if err != nil {
		t.Fatal(err)
	}
	h := string(b)
	if strings.Contains(strings.ToLower(h), "<script") {
		t.Fatal("report contains a script element (the snippet must be escaped)")
	}
	if !strings.Contains(h, "&lt;script&gt;") {
		t.Fatal("snippet not shown escaped")
	}
	if regexp.MustCompile(`(?i)(src|href)="(https?:)?//`).MatchString(h) {
		t.Fatal("report loads or links an external resource")
	}
	if regexp.MustCompile(`(?i)\son[a-z]+=`).MatchString(h) {
		t.Fatal("report contains an event handler attribute")
	}
	if !strings.Contains(h, `src="data:image/jpeg;base64,`) {
		t.Fatal("screenshot is not embedded")
	}
}

// Spec: RP-7 — frames are placed in percent of the screenshot.
func TestMarkPosition(t *testing.T) {
	m := markIn(Box{X: 128, Y: 400, Width: 256, Height: 80}, 1280, 1600)
	if want := "left:10.000%;top:25.000%;width:20.000%;height:5.000%"; string(m.Style) != want {
		t.Errorf("style = %s, want %s", m.Style, want)
	}
}

// Spec: F13, RP-1 — JSON has positions and page sizes but no screenshot data.
func TestJSONHasPositionsNotImages(t *testing.T) {
	r, _ := Build(visualScan(t), metas, t0)
	b, err := r.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Version  int
		Findings []struct {
			Position string
			Box      *Box
		}
		Pages []map[string]any
	}
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != 1 || got.Findings[0].Position != "element" || got.Findings[0].Box == nil || got.Findings[0].Box.Y != 400 {
		t.Fatalf("findings: %+v", got.Findings)
	}
	if len(got.Pages) != 2 || got.Pages[1]["truncated"] != true {
		t.Fatalf("pages: %v", got.Pages)
	}
	if _, has := got.Pages[0]["image"]; has || bytes.Contains(b, []byte("base64")) || len(b) > 10000 {
		t.Fatal("screenshots must not be in the JSON")
	}
}

// Spec: F14 — a report without findings still renders.
func TestHTMLReportWithoutFindings(t *testing.T) {
	sc := finished()
	sc.Findings = nil
	r, _ := Build(sc, metas, t0)
	b, err := r.HTML()
	if err != nil || !bytes.Contains(b, []byte("Es wurden keine Probleme gefunden.")) {
		t.Fatalf("err=%v", err)
	}
}
