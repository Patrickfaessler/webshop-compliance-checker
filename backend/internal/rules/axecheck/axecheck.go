// Package axecheck turns axe-core results into rule results. Used by A1, A2, A3.
// Spec: specs/rules/A1-contrast.md, A2-alt-text.md, A3-form-labels.md.
package axecheck

import (
	"slices"

	"compliancechecker/internal/rules"
)

// Check evaluates the axe rule ids over all pages:
//   - no page has axe data                         -> not_assessable (axe did not run)
//   - any violation of the ids                     -> failed (one finding per flagged element)
//   - else any "incomplete" result of the ids      -> not_assessable (axe could not decide)
//   - else every page reports all ids inapplicable -> not_applicable
//   - else                                          -> passed
func Check(ids []string, in rules.Input) rules.Result {
	res := rules.Result{DecidedBy: rules.DecidedStructural}

	withAxe, incomplete := 0, false
	allInapplicable := true
	for _, p := range in.Pages {
		if p.Axe == nil {
			continue
		}
		withAxe++
		for _, v := range p.Axe.Violations {
			if !slices.Contains(ids, v.ID) {
				continue
			}
			for _, n := range v.Nodes {
				res.Findings = append(res.Findings, rules.Finding{
					PageURL: p.URL, Selector: n.Selector, Snippet: n.HTML, Sub: v.ID,
				})
			}
		}
		for _, v := range p.Axe.Incomplete {
			if slices.Contains(ids, v.ID) && len(v.Nodes) > 0 {
				incomplete = true
			}
		}
		for _, id := range ids {
			if !slices.Contains(p.Axe.Inapplicable, id) {
				allInapplicable = false
			}
		}
	}

	switch {
	case withAxe == 0:
		res.State = rules.NotAssessable
	case len(res.Findings) > 0:
		res.State = rules.Failed
	case incomplete:
		res.State = rules.NotAssessable
	case allInapplicable:
		res.State = rules.NotApplicable
	default:
		res.State = rules.Passed
	}
	return res
}
