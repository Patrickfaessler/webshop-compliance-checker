// Package r1 implements rule R1 "Provider identification (legal notice)" (specs/rules/R1-provider-identification.md).
// Deterministic version: the AI fallback is not built, uncertain fields give not_assessable with a note.
package r1

import (
	"context"
	"strings"

	"compliancechecker/internal/legal"
	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

// Rule is R1.
type Rule struct{}

// New returns R1.
func New() Rule { return Rule{} }

// Meta implements rules.Rule.
func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID:            "R1",
		Name:          "Provider identification (legal notice)",
		NameDe:        "Anbieterkennzeichnung (Impressum)",
		Area:          "legal",
		Severity:      rules.Critical,
		AIFallback:    true,
		SummaryDe:     "Im Impressum fehlen Pflichtangaben oder es gibt kein Impressum.",
		ExplanationDe: "Online-Shops müssen zeigen, wer sie betreiben und wie man sie erreicht: Name, Anschrift und E-Mail-Adresse. Das gehört gut auffindbar auf eine eigene Seite.",
	}
}

const (
	notePassed      = "Nur geprüft, ob die Angaben vorhanden sind."
	noteConditional = "UID-Nummer bzw. Firmenbuchnummer prüfen, ob zutreffend."
	noteNotLoaded   = "Das Impressum ist verlinkt, konnte aber nicht geladen werden."
	fixText         = "Ergänze die fehlenden Angaben im Impressum."
	fixTextNoPage   = "Lege eine Seite „Impressum“ an und verlinke sie im Footer jeder Seite."
	fixNote         = "Je nach Land und Rechtsform können weitere Angaben nötig sein (z. B. UID, Firmenbuch)."
)

// field is one mandatory R1 field, in report order.
type field struct {
	nameDe, sub, codeLine string
}

var fields = []field{
	{"Firmenname", "missing-name", "Firmenname: {{Firmenname}}"},
	{"Anschrift", "missing-address", "Anschrift: {{Straße Nr., PLZ Ort}}"},
	{"E-Mail", "missing-email", "E-Mail: {{E-Mail}}"},
}

// Check implements rules.Rule.
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	if len(in.Pages) == 0 {
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedStructural}, nil
	}
	link := legal.FindLinked(in, lexicon.LegalNotice)
	switch {
	case !link.Found:
		var code []string
		for _, f := range fields {
			code = append(code, f.codeLine)
		}
		return rules.Result{State: rules.Failed, DecidedBy: rules.DecidedStructural, Findings: []rules.Finding{{
			PageURL: in.Pages[0].URL, Sub: "no-legal-notice-page",
			Fix: &rules.Fix{TextDe: fixTextNoPage, Code: strings.Join(code, "\n"), NoteDe: fixNote},
		}}}, nil
	case !link.Loaded:
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedStructural, NoteDe: noteNotLoaded}, nil
	}
	p := *link.Page
	lang, ok := legal.Language(p)
	if !ok {
		return rules.Result{State: rules.NotAssessable, DecidedBy: rules.DecidedDictionary, NoteDe: legal.NoteLanguage}, nil
	}
	l, _ := lexicon.For(lang)
	content, full := legal.ContentText(p), legal.FullText(p)

	// content text first; a field not found there is searched again in the whole page (R1-10)
	best := func(a, b legal.Field) legal.Field {
		if b.State > a.State {
			return b
		}
		return a
	}
	got := []legal.Field{
		best(legal.ProviderName(content, l), legal.ProviderName(full, l)),
		best(legal.Address(content, l), legal.Address(full, l)),
		best(legal.Email(content, p, l, false), legal.Email(full, p, l, true)),
	}

	res := rules.Result{DecidedBy: rules.DecidedPattern}
	var missing []string
	var unsure []string
	for i, f := range fields {
		switch got[i].State {
		case legal.Missing:
			missing = append(missing, f.codeLine)
			res.Findings = append(res.Findings, rules.Finding{PageURL: p.URL, Sub: f.sub})
		case legal.Unsure:
			unsure = append(unsure, f.nameDe)
		}
	}
	for i := range res.Findings {
		res.Findings[i].Fix = &rules.Fix{TextDe: fixText, Code: strings.Join(missing, "\n"), NoteDe: fixNote}
	}
	var notes []string
	if len(res.Findings) > 0 && legal.Embedded(p) { // the text may sit in an iframe we cannot read (R1-11)
		res.Findings = nil
		unsure = nil
		notes = append(notes, legal.NoteEmbedded)
		res.State = rules.NotAssessable
	}
	switch {
	case res.State == rules.NotAssessable:
	case len(res.Findings) > 0:
		res.State = rules.Failed
	case len(unsure) > 0:
		res.State = rules.NotAssessable
		notes = append(notes, "Bitte manuell prüfen: "+strings.Join(unsure, ", ")+".")
	default:
		res.State = rules.Passed
		notes = append(notes, notePassed)
	}
	_, hasRegister := legal.RegisterNumber(full)
	if len(legal.VATNumbers(full)) == 0 || !hasRegister {
		notes = append(notes, noteConditional)
	}
	res.NoteDe = strings.Join(notes, " ")
	return res, nil
}
