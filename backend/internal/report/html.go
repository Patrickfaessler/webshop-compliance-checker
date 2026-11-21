package report

import (
	"bytes"
	_ "embed"
	"encoding/base64"
	"fmt"
	"html/template"
	"image"
	"image/jpeg"
	"math"
	"strings"

	"compliancechecker/internal/scan"
)

// The visual report (specs/11-report.md RP-5 … RP-9): one self-contained HTML file, no script, images as data URIs.
//
//go:embed report.html.tmpl
var reportTemplate string

var tmpl = template.Must(template.New("report").Parse(reportTemplate))

// maxCrops limits the cropped details per report to keep the file small (RP-7).
const maxCrops = 60

type htmlView struct {
	Host, StartURL, CreatedDe, GeneratedDe, StatusDe, AreasDe string
	PagesScanned                                              int
	KeyboardDe                                                string   // A5 mode line (RP-6)
	KeyboardPages                                             []string // probed pages
	ScoreText                                                 string
	Counts                                                    []countView
	Disclaimer                                                string
	Rules                                                     []ruleView
	Sections                                                  []sectionView
	FindingCount                                              int
	CropsOmitted                                              bool
}

type countView struct {
	LabelDe string
	N       int
}

type ruleView struct {
	ID, NameDe, SeverityDe, SevClass, StateDe, StateClass string
	Pages                                                 string
	NoteDe                                                string // A5-7
}

type sectionView struct {
	N         int
	URL       string
	HasShot   bool
	ShotSrc   template.URL
	ShotAlt   string
	Truncated bool
	Marks     []markView
	Cards     []cardView
}

type markView struct {
	N        int
	Style    template.CSS // left/top/width/height in percent of the image
	Label    string
	SevClass string
}

type cardView struct {
	N                                        int
	HasMark                                  bool
	RuleID, NameDe, SeverityDe, SevClass     string
	Summary, URL, Selector, Snippet, PosNote string
	HasCrop                                  bool
	CropSrc                                  template.URL
	CropMark                                 markView
	CropAlt                                  string
	Fix                                      *Fix
}

// HTML renders the visual report (F14). The result contains no script and no external resources (RP-5).
func (r Report) HTML() ([]byte, error) {
	v := htmlView{
		Host: r.Scan.Host, StartURL: r.Scan.StartURL,
		CreatedDe: dateTimeDe(r.Scan.CreatedAt), GeneratedDe: dateTimeDe(r.GeneratedAt),
		PagesScanned: r.Scan.PagesScanned, Disclaimer: r.DisclaimerDe, FindingCount: len(r.Findings),
		KeyboardDe: r.KeyboardLineDe(), KeyboardPages: r.Scan.KeyboardPages,
	}
	v.StatusDe = "abgeschlossen"
	if r.Scan.Status == "incomplete" {
		v.StatusDe = "unvollständig (Zeitlimit erreicht)"
	}
	var areas []string
	for _, a := range r.Scan.Areas {
		areas = append(areas, areaDe(a))
	}
	v.AreasDe = strings.Join(areas, ", ")
	v.ScoreText = "nicht berechenbar"
	if r.Scan.Score != nil {
		v.ScoreText = fmt.Sprintf("%d %%", *r.Scan.Score)
	}

	counts := map[string]int{}
	names := map[string]string{}
	for _, rr := range r.RuleResults {
		counts[rr.State]++
		names[rr.RuleID] = rr.NameDe
		pages := ""
		if rr.State == "failed" {
			pages = fmt.Sprint(rr.PagesAffected)
		}
		v.Rules = append(v.Rules, ruleView{
			ID: rr.RuleID, NameDe: rr.NameDe, SeverityDe: severityDe(rr.Severity), SevClass: sevClass(rr.Severity),
			StateDe: stateDe(rr.State), StateClass: "st-" + strings.ReplaceAll(rr.State, "_", "-"), Pages: pages,
			NoteDe: rr.NoteDe,
		})
	}
	for _, s := range []string{"passed", "failed", "not_applicable", "not_assessable"} {
		v.Counts = append(v.Counts, countView{LabelDe: stateDe(s), N: counts[s]})
	}

	v.Sections, v.CropsOmitted = r.sections(names)

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, v); err != nil {
		return nil, fmt.Errorf("report: rendering HTML: %w", err)
	}
	return buf.Bytes(), nil
}

// sections groups the findings by page: captured pages first (crawl order), then pages without a screenshot.
// Findings are numbered 1…n across the report (RP-7).
func (r Report) sections(names map[string]string) ([]sectionView, bool) {
	shots := map[string]Page{}
	var order []string
	for _, p := range r.Pages {
		if _, dup := shots[p.URL]; !dup {
			shots[p.URL] = p
			order = append(order, p.URL)
		}
	}
	byURL := map[string][]Finding{}
	for _, f := range r.Findings {
		if len(byURL[f.URL]) == 0 {
			if _, ok := shots[f.URL]; !ok {
				order = append(order, f.URL)
			}
		}
		byURL[f.URL] = append(byURL[f.URL], f)
	}

	var (
		out     []sectionView
		n       int
		crops   int
		omitted bool
	)
	for _, u := range order {
		fs := byURL[u]
		if len(fs) == 0 {
			continue
		}
		sec := sectionView{N: len(out) + 1, URL: u}
		p, hasShot := shots[u]
		var img image.Image
		if hasShot && len(p.Image) > 0 && p.Width > 0 && p.Height > 0 {
			sec.HasShot = true
			sec.Truncated = p.Truncated
			sec.ShotSrc = dataURI(p.Image)
			if decoded, err := jpeg.Decode(bytes.NewReader(p.Image)); err == nil {
				img = decoded
			}
		}
		marks := 0
		for _, f := range fs {
			n++
			c := cardView{
				N: n, RuleID: f.RuleID, NameDe: names[f.RuleID], SeverityDe: severityDe(f.Severity), SevClass: sevClass(f.Severity),
				Summary: f.SummaryDe, URL: f.URL, Selector: f.Selector, Snippet: truncate(f.Snippet, 400), Fix: f.Fix,
				PosNote: positionNoteDe(f.Position, sec.HasShot),
			}
			if sec.HasShot && f.Position == "element" && f.Box != nil {
				c.HasMark = true
				marks++
				m := markIn(*f.Box, float64(p.Width), float64(p.Height))
				m.N, m.SevClass = n, c.SevClass
				m.Label = fmt.Sprintf("Fundstelle %d: %s %s (%s)", n, f.RuleID, names[f.RuleID], c.SeverityDe)
				sec.Marks = append(sec.Marks, m)
				if img != nil {
					if crops < maxCrops {
						if src, cm, ok := crop(img, *f.Box, float64(p.Width)); ok {
							c.HasCrop, c.CropSrc, c.CropMark = true, src, cm
							c.CropMark.N, c.CropMark.SevClass = n, c.SevClass
							c.CropAlt = fmt.Sprintf("Ausschnitt um Fundstelle %d", n)
							crops++
						}
					} else {
						omitted = true
					}
				}
			}
			sec.Cards = append(sec.Cards, c)
		}
		if sec.HasShot {
			sec.ShotAlt = fmt.Sprintf("Screenshot der Seite %s mit %d markierten Fundstellen", u, marks)
		}
		out = append(out, sec)
	}
	return out, omitted
}

func positionNoteDe(pos string, hasShot bool) string {
	switch pos {
	case "element":
		return ""
	case "page":
		return "Betrifft die ganze Seite, deshalb gibt es keine Markierung im Screenshot."
	case "hidden":
		return "Das Element ist auf der Seite nicht sichtbar, deshalb gibt es keine Markierung."
	case "outside":
		return "Das Element liegt außerhalb des Screenshot-Ausschnitts."
	case "not_found":
		return "Die Position des Elements konnte nicht ermittelt werden."
	}
	if hasShot {
		return "Die Position des Elements konnte nicht ermittelt werden."
	}
	return "Für diese Seite gibt es keinen Screenshot."
}

func sevClass(s string) string {
	switch s {
	case "critical", "warning", "notice":
		return "sev-" + s
	}
	return "sev-notice"
}

func dataURI(jpg []byte) template.URL {
	// safe: the content is our own base64-encoded JPEG
	return template.URL("data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(jpg))
}

func pct(v, total float64) string {
	if total <= 0 {
		return "0%"
	}
	return fmt.Sprintf("%.3f%%", math.Max(0, math.Min(100, v/total*100)))
}

// markIn places a frame on the screenshot in percent of its size, so it scales with the image (RP-7).
func markIn(b Box, w, h float64) markView {
	// safe: only numbers formatted by pct
	return markView{Style: template.CSS(fmt.Sprintf("left:%s;top:%s;width:%s;height:%s",
		pct(b.X, w), pct(b.Y, h), pct(b.Width, w), pct(b.Height, h)))}
}

// crop cuts the area around a box out of the screenshot (with some margin) and returns it with the box position
// inside the crop. pageW is the screenshot width in CSS px; the image may have more pixels (device scale).
func crop(img image.Image, b Box, pageW float64) (template.URL, markView, bool) {
	return cropTo(img, b, pageW, nil)
}

// CropJPEG returns the same crop as the HTML report shows, as JPEG bytes (F8, app detail page).
func CropJPEG(img image.Image, b Box, pageW float64) ([]byte, bool) {
	var out []byte
	_, _, ok := cropTo(img, b, pageW, &out)
	return out, ok
}

// FindingCrop crops the screenshot of a finding's page around its element; ok is false when the finding is
// unknown, has no box, or its page has no screenshot.
func FindingCrop(sc scan.Scan, findingID string) ([]byte, bool) {
	for _, f := range sc.Findings {
		if f.ID != findingID {
			continue
		}
		if f.Box == nil || f.Position != "element" {
			return nil, false
		}
		for _, sh := range sc.Shots {
			if sh.URL != f.URL || len(sh.Image) == 0 || sh.Width <= 0 {
				continue
			}
			img, err := jpeg.Decode(bytes.NewReader(sh.Image))
			if err != nil {
				return nil, false
			}
			return CropJPEG(img, Box{X: f.Box.X, Y: f.Box.Y, Width: f.Box.Width, Height: f.Box.Height}, float64(sh.Width))
		}
		return nil, false
	}
	return nil, false
}

// HasCrop says whether FindingCrop can produce an image for f, without decoding.
func HasCrop(sc scan.Scan, f scan.Finding) bool {
	if f.Box == nil || f.Position != "element" {
		return false
	}
	for _, sh := range sc.Shots {
		if sh.URL == f.URL && len(sh.Image) > 0 && sh.Width > 0 {
			return true
		}
	}
	return false
}

func cropTo(img image.Image, b Box, pageW float64, cropOut *[]byte) (template.URL, markView, bool) {
	bounds := img.Bounds()
	if pageW <= 0 || bounds.Dx() == 0 {
		return "", markView{}, false
	}
	scale := float64(bounds.Dx()) / pageW
	const pad, minW, minH, maxW, maxH = 32.0, 360.0, 140.0, 1000.0, 420.0
	cx, cy := b.X+b.Width/2, b.Y+b.Height/2
	w := math.Min(math.Max(b.Width+2*pad, minW), maxW)
	h := math.Min(math.Max(b.Height+2*pad, minH), maxH)
	x0, y0 := cx-w/2, cy-h/2
	if b.Width+2*pad > maxW {
		x0 = b.X - pad
	}
	if b.Height+2*pad > maxH {
		y0 = b.Y - pad
	}
	r := image.Rect(int(x0*scale), int(y0*scale), int((x0+w)*scale), int((y0+h)*scale)).Intersect(bounds)
	if r.Dx() < 8 || r.Dy() < 8 {
		return "", markView{}, false
	}
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return "", markView{}, false
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, sub.SubImage(r), &jpeg.Options{Quality: 80}); err != nil {
		return "", markView{}, false
	}
	if cropOut != nil {
		*cropOut = buf.Bytes()
	}
	// box relative to the crop, in CSS px
	cx0, cy0 := float64(r.Min.X)/scale, float64(r.Min.Y)/scale
	cw, ch := float64(r.Dx())/scale, float64(r.Dy())/scale
	inner := Box{X: b.X - cx0, Y: b.Y - cy0, Width: b.Width, Height: b.Height}
	return dataURI(buf.Bytes()), markIn(inner, cw, ch), true
}
