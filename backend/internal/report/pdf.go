package report

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // Europe/Vienna on machines without a zone database (Windows)

	"github.com/go-pdf/fpdf"
)

// vienna is the time zone used for dates in the report (the course and the first users are in Austria).
var vienna = func() *time.Location {
	if loc, err := time.LoadLocation("Europe/Vienna"); err == nil {
		return loc
	}
	return time.UTC
}()

func stateDe(s string) string {
	switch s {
	case "passed":
		return "Bestanden"
	case "failed":
		return "Nicht bestanden"
	case "not_applicable":
		return "Nicht anwendbar"
	case "not_assessable":
		return "Nicht automatisch beurteilbar"
	}
	return s
}

func severityDe(s string) string {
	switch s {
	case "critical":
		return "Kritisch"
	case "warning":
		return "Warnung"
	case "notice":
		return "Hinweis"
	}
	return s
}

func areaDe(a string) string {
	switch a {
	case "accessibility":
		return "Barrierefreiheit"
	case "legal":
		return "Rechtliche Pflichtangaben"
	}
	return a
}

func dateTimeDe(t time.Time) string { return t.In(vienna).Format("02.01.2006, 15:04") + " Uhr" }

// PDFOptions are for tests: uncompressed output makes the text searchable.
type PDFOptions struct{ Uncompressed bool }

// PDF renders the German report (F10). The disclaimer is on the first page (N3).
func (r Report) PDF(opt PDFOptions) ([]byte, error) {
	const margin, width = 15.0, 180.0
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(margin, margin, margin)
	pdf.SetAutoPageBreak(true, 18)
	pdf.SetCompression(!opt.Uncompressed)
	pdf.SetTitle("Prüfbericht Webshop-Check: "+r.Scan.Host, true)
	pdf.SetCreator("Webshop-Check (Software-Praktikum Gruppe 9)", true)
	pdf.AliasNbPages("{nb}")
	tr := pdf.UnicodeTranslatorFromDescriptor("") // UTF-8 -> cp1252 for the core fonts (umlauts, „“, –)

	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(width/2, 5, tr("Webshop-Check · Prüfung "+r.Scan.ID), "", 0, "L", false, 0, "")
		pdf.CellFormat(width/2, 5, tr(fmt.Sprintf("Seite %d von {nb}", pdf.PageNo())), "", 0, "R", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
	})
	pdf.AddPage()

	heading := func(text string) {
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "B", 13)
		pdf.MultiCell(width, 7, tr(text), "", "L", false)
		pdf.Ln(1)
	}
	line := func(label, value string) {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.CellFormat(38, 6, tr(label), "", 0, "L", false, 0, "")
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(width-38, 6, tr(value), "", "L", false)
	}

	// title and scan data
	pdf.SetFont("Helvetica", "B", 18)
	pdf.MultiCell(width, 9, tr("Prüfbericht Webshop-Check"), "", "L", false)
	pdf.Ln(2)
	line("Geprüfte Adresse:", r.Scan.StartURL)
	line("Prüfung vom:", dateTimeDe(r.Scan.CreatedAt))
	status := "abgeschlossen"
	if r.Scan.Status == "incomplete" {
		status = "unvollständig (Zeitlimit erreicht)"
	}
	line("Status:", status)
	line("Geprüfte Seiten:", fmt.Sprint(r.Scan.PagesScanned))
	var areas []string
	for _, a := range r.Scan.Areas {
		areas = append(areas, areaDe(a))
	}
	line("Prüfbereiche:", strings.Join(areas, ", "))
	line("Bericht erstellt:", dateTimeDe(r.GeneratedAt))

	// disclaimer box (N3)
	pdf.Ln(3)
	pdf.SetFillColor(240, 240, 240)
	pdf.SetFont("Helvetica", "", 9)
	pdf.MultiCell(width, 5, tr("Wichtiger Hinweis: "+r.DisclaimerDe), "1", "L", true)

	// score (S-2, S-3)
	heading("Ergebnis")
	pdf.SetFont("Helvetica", "B", 11)
	score := "nicht berechenbar"
	if r.Scan.Score != nil {
		score = fmt.Sprintf("%d %%", *r.Scan.Score)
	}
	pdf.MultiCell(width, 6, tr("Anteil bestandener automatischer Prüfungen: "+score), "", "L", false)
	pdf.SetFont("Helvetica", "", 10)
	pdf.MultiCell(width, 5, tr("Das ist keine Aussage über vollständige Konformität."), "", "L", false)
	counts := map[string]int{}
	for _, rr := range r.RuleResults {
		counts[rr.State]++
	}
	pdf.MultiCell(width, 5, tr(fmt.Sprintf("Bestanden: %d · Nicht bestanden: %d · Nicht anwendbar: %d · Nicht automatisch beurteilbar: %d",
		counts["passed"], counts["failed"], counts["not_applicable"], counts["not_assessable"])), "", "L", false)

	// rule results table
	heading("Prüfregeln")
	cols := []struct {
		title string
		w     float64
	}{{"Regel", 15}, {"Bezeichnung", 72}, {"Schweregrad", 25}, {"Ergebnis", 50}, {"Seiten", 18}}
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(225, 225, 225)
	for _, c := range cols {
		pdf.CellFormat(c.w, 7, tr(c.title), "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont("Helvetica", "", 9)
	if len(r.RuleResults) == 0 {
		pdf.CellFormat(width, 7, tr("Für die gewählten Bereiche wurden keine Regeln ausgeführt."), "1", 1, "L", false, 0, "")
	}
	for _, rr := range r.RuleResults {
		pages := ""
		if rr.State == "failed" {
			pages = fmt.Sprint(rr.PagesAffected)
		}
		vals := []string{rr.RuleID, rr.NameDe, severityDe(rr.Severity), stateDe(rr.State), pages}
		for i, c := range cols {
			pdf.CellFormat(c.w, 7, tr(vals[i]), "1", 0, "L", false, 0, "")
		}
		pdf.Ln(-1)
	}

	// findings
	heading(fmt.Sprintf("Gefundene Probleme (%d)", len(r.Findings)))
	if len(r.Findings) == 0 {
		pdf.SetFont("Helvetica", "", 10)
		pdf.MultiCell(width, 5, tr("Es wurden keine Probleme gefunden."), "", "L", false)
	}
	names := map[string]string{}
	for _, rr := range r.RuleResults {
		names[rr.RuleID] = rr.NameDe
	}
	for i, f := range r.Findings {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.MultiCell(width, 6, tr(fmt.Sprintf("%d. %s · %s %s", i+1, severityDe(f.Severity), f.RuleID, names[f.RuleID])), "", "L", false)
		pdf.SetFont("Helvetica", "", 10)
		if f.SummaryDe != "" {
			pdf.MultiCell(width, 5, tr(f.SummaryDe), "", "L", false)
		}
		if f.URL != "" {
			pdf.MultiCell(width, 5, tr("Seite: "+f.URL), "", "L", false)
		}
		if f.Selector != "" {
			pdf.MultiCell(width, 5, tr("Stelle: "+f.Selector), "", "L", false)
		}
		if f.Snippet != "" {
			pdf.SetFont("Courier", "", 8)
			pdf.MultiCell(width, 4, tr(truncate(f.Snippet, 300)), "", "L", false)
		}
		pdf.Ln(2)
	}

	// disclaimer again at the end
	pdf.Ln(3)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.MultiCell(width, 4, tr(r.DisclaimerDe), "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("report: rendering PDF: %w", err)
	}
	return buf.Bytes(), nil
}

func truncate(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= n {
		return string(r)
	}
	return string(r[:n]) + " …"
}
