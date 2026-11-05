// Package a2 implements rule A2 "Alternative text for images". Spec: specs/rules/A2-alt-text.md
package a2

import (
	"context"
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/rules/axecheck"
)

var axeIDs = []string{"image-alt", "input-image-alt", "area-alt", "svg-img-alt"}

// A2-3: alt text that is just a file name or a camera-style name.
var (
	fileNameAlt = regexp.MustCompile(`(?i)^[\w\-. ()]*\.(jpe?g|png|gif|webp|svg|avif|bmp|tiff?)$`)
	cameraAlt   = regexp.MustCompile(`(?i)^(img|dsc|dscn|image|photo|pic|bild|foto)[_\- ]?\d+$`)
)

type Rule struct{}

func New() Rule { return Rule{} }

func (Rule) Meta() rules.Meta {
	return rules.Meta{
		ID: "A2", Name: "Alternative text for images", Area: "accessibility", Severity: rules.Warning,
		SummaryDe: "Ein Bild hat keinen brauchbaren Alternativtext.",
	}
}

// Check combines axe-core (missing alt) with a DOM check for file-name alt texts (A2-3).
func (Rule) Check(_ context.Context, in rules.Input) (rules.Result, error) {
	res := axecheck.Check(axeIDs, in)
	for _, p := range in.Pages {
		res.Findings = append(res.Findings, fileNameFindings(p)...)
	}
	if len(res.Findings) > 0 {
		res.State = rules.Failed
	}
	return res, nil
}

func fileNameFindings(p rules.Page) []rules.Finding {
	doc := domutil.Parse(p.HTML)
	var out []rules.Finding
	domutil.Walk(doc, func(n *html.Node) bool {
		if !domutil.IsElement(n, "img") {
			return true
		}
		alt, ok := domutil.Attr(n, "alt")
		alt = strings.TrimSpace(alt)
		if ok && alt != "" && (fileNameAlt.MatchString(alt) || cameraAlt.MatchString(alt)) {
			out = append(out, rules.Finding{
				PageURL: p.URL, Selector: domutil.Selector(n), Snippet: domutil.OpeningTag(n), Sub: "filename-alt",
			})
		}
		return true
	})
	return out
}
