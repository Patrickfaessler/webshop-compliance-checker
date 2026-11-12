package worker

import (
	"context"
	"testing"

	"compliancechecker/internal/crawler"
	"compliancechecker/internal/rules"
	"compliancechecker/internal/scan"
)

// Spec: RP-1, RP-4 — pages with findings in crawl order, at most max pages.
func TestShotPlan(t *testing.T) {
	fs := []scan.Finding{{URL: "/b"}, {URL: "/a"}, {URL: "/b"}, {URL: "/c"}, {URL: "/x"}}
	plan := shotPlan(fs, []string{"/a", "/b", "/c", "/empty"}, 2)
	if len(plan) != 2 || plan[0].url != "/a" || plan[1].url != "/b" {
		t.Fatalf("plan = %+v", plan)
	}
	if len(plan[1].idx) != 2 || plan[1].idx[0] != 0 || plan[1].idx[1] != 2 {
		t.Fatalf("findings of /b = %v", plan[1].idx)
	}
	if got := shotPlan(fs, []string{"/a"}, 10); len(got) != 4 || got[3].url != "/x" {
		t.Fatalf("pages missing from the crawl order are appended: %+v", got)
	}
	if shotPlan(nil, []string{"/a"}, 10) != nil {
		t.Fatal("no findings, no shots")
	}
}

// Spec: RP-3 — placements are copied onto the findings.
func TestApplyPlacements(t *testing.T) {
	fs := []scan.Finding{{Selector: "#a"}, {Selector: "html"}, {Selector: "#b"}}
	applyPlacements(fs, []int{0, 2}, []crawler.Placement{
		{Position: crawler.PosElement, Box: &crawler.Box{X: 1, Y: 2, Width: 3, Height: 4}},
		{Position: crawler.PosNotFound},
	})
	if fs[0].Position != "element" || fs[0].Box == nil || fs[0].Box.Height != 4 {
		t.Errorf("finding 0 = %+v", fs[0])
	}
	if fs[1].Position != "" || fs[2].Position != "not_found" || fs[2].Box != nil {
		t.Errorf("findings = %+v", fs)
	}
}

type fakeAnnotator struct{ calls int }

func (f *fakeAnnotator) Annotate(_ context.Context, fs []scan.Finding, order []string) ([]scan.Shot, []scan.Finding) {
	f.calls++
	out := append([]scan.Finding(nil), fs...)
	for i := range out {
		out[i].Position = "page"
	}
	return []scan.Shot{{URL: order[0], Image: []byte{0xFF, 0xD8}, Width: 1280, Height: 900}}, out
}

// Spec: RP-1 — the runner annotates after the rules and stores shots and positions with the scan.
func TestRunAnnotates(t *testing.T) {
	var released []string
	r, st := newRunner(fakeSource{pages: []rules.Page{page("<html><body></body></html>")}}, &released)
	ann := &fakeAnnotator{}
	r.Annotator = ann
	seed(t, st, "accessibility")
	r.Run(context.Background(), Job{ScanID: "s1", ClientKey: "k"})
	s, _ := st.Get("s1")
	if ann.calls != 1 || len(s.Shots) != 1 || s.Shots[0].URL != "https://shop.example/" {
		t.Fatalf("calls=%d shots=%+v", ann.calls, s.Shots)
	}
	if len(s.Findings) != 1 || s.Findings[0].Position != "page" {
		t.Fatalf("findings = %+v", s.Findings)
	}

	// no findings: no capture at all
	r2, st2 := newRunner(fakeSource{pages: []rules.Page{page(`<html lang="de"></html>`)}}, &released)
	ann2 := &fakeAnnotator{}
	r2.Annotator = ann2
	seed(t, st2, "accessibility")
	r2.Run(context.Background(), Job{ScanID: "s1", ClientKey: "k"})
	if ann2.calls != 0 {
		t.Fatal("annotator called without findings")
	}
}
