package crawler

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/domdebugger"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	"compliancechecker/internal/rules"
)

// KeyboardProbeTimeout is the time budget of one keyboard probe (A5-9).
const KeyboardProbeTimeout = 30 * time.Second

// probeTimeout is KeyboardProbeTimeout; a variable so tests can shorten it.
var probeTimeout = KeyboardProbeTimeout

const (
	maxKeyboardFindings = 20  // per sub-rule and page (A5 Logic 4)
	maxPointerChecks    = 100 // div/span with cursor:pointer checked for click listeners (A5 Logic 3)
	styleRecheckDelay   = 150 * time.Millisecond
)

// probePlan tells run whether and how to probe a page. nil = never probe.
// innerTrapPresses is the trap threshold for elements whose inner Tab stops the probe cannot see (A5 Logic 3).
const innerTrapPresses = 15

// probeReserve is the time left at the end of the probe budget for the last calls; the loop stops before it (A5-9).
const probeReserve = time.Second

type probePlan struct {
	// accept says whether the page is kept at all (status, final host); rejected pages are not probed
	// and take no sample slot (A5 Logic 7). nil accepts every page.
	accept  func(finalURL string, status int) bool
	decide  func(pageURL string, sig pageSignals) (PageType, bool)
	presses int
}

// analyze is AnalyzeHTML with explicit keyboard options; the page is always probed.
func (s *Session) analyze(ctx context.Context, pageURL, htmlSrc string, timeout time.Duration, kb KeyboardOptions) (rules.Page, error) {
	dataURL := "data:text/html;charset=utf-8;base64," + base64.StdEncoding.EncodeToString([]byte(htmlSrc))
	plan := &probePlan{decide: func(string, pageSignals) (PageType, bool) { return PageOther, true }, presses: kb.Presses()}
	r, err := s.run(ctx, timeout, dataURL, pageURL, plan)
	return r.Page, err
}

// keyboardStep is what the page reports after one Tab press.
type keyboardStep struct {
	Left         bool `json:"left"`  // focus is on body / outside the document
	ID           int  `json:"id"`    // stable number of the focused element
	First        bool `json:"first"` // this is the first element focused in this probe
	Seen         bool `json:"seen"`  // focused before
	Frame        bool `json:"frame"` // iframe or shadow host without own focus info: repeats do not count
	Inner        bool `json:"inner"` // has inner Tab stops the probe cannot see (date/time input, closed shadow root)
	Candidate    bool `json:"candidate"`
	StyleChanged bool `json:"styleChanged"`
}

type keyboardPrep struct {
	Candidates   int                  `json:"candidates"`
	Pointers     int                  `json:"pointers"`     // entries in __a5.ptr to check for listeners
	NotFocusable []rules.KeyboardNode `json:"notFocusable"` // div/span with inline onclick
}

// probeKeyboard walks the page with real Tab key events (A5 Logic 1–4). Only Tab is pressed.
func (s *Session) probeKeyboard(ctx context.Context, maxPresses int) (*rules.KeyboardReport, error) {
	rep := &rules.KeyboardReport{MaxPresses: maxPresses}
	prep, err := chromedp.Run(ctx, chromedp.Evaluate[keyboardPrep](keyboardPrepareScript))
	if err != nil {
		return nil, fmt.Errorf("crawler: preparing keyboard probe: %w", err)
	}
	rep.NotFocusable = capNodes(prep.NotFocusable)

	// not-focusable: click listeners on pointer elements (DOMDebugger.getEventListeners)
	for i := 0; i < prep.Pointers && len(rep.NotFocusable) < maxKeyboardFindings; i++ {
		ev, err := chromedp.Call(ctx, runtime.Evaluate, runtime.EvaluateParams{Expression: fmt.Sprintf("window.__a5.ptr[%d]", i)})
		if err != nil || ev.Result == nil || ev.Result.ObjectID == "" {
			continue
		}
		ls, err := chromedp.Call(ctx, domdebugger.GetEventListeners, domdebugger.GetEventListenersParams{ObjectID: ev.Result.ObjectID})
		if err != nil {
			continue
		}
		for _, l := range ls.Listeners {
			if l.Type == "click" || l.Type == "mousedown" || l.Type == "pointerdown" {
				n, err := chromedp.Run(ctx, chromedp.Evaluate[rules.KeyboardNode](fmt.Sprintf("window.__a5.node(window.__a5.ptr[%d])", i)))
				if err == nil {
					rep.NotFocusable = append(rep.NotFocusable, n)
				}
				break
			}
		}
	}

	// When the probe's time runs out during the loop, the presses so far count like a reached limit (A5-9).
	deadline, hasDeadline := ctx.Deadline()
	outOfTime := func(err error) bool { return ctx.Err() != nil && rep.Presses > 0 && err != nil }
	current := func() rules.KeyboardNode {
		n, err := chromedp.Run(ctx, chromedp.Evaluate[rules.KeyboardNode]("window.__a5.node(window.__a5.current())"))
		if err != nil {
			return rules.KeyboardNode{}
		}
		return n
	}
	lastID, repeats := -1, 0
	for rep.Presses < maxPresses {
		if hasDeadline && time.Until(deadline) < probeReserve {
			return rep, nil
		}
		if err := pressTab(ctx); err != nil {
			if outOfTime(err) {
				return rep, nil
			}
			return nil, err
		}
		rep.Presses++
		st, err := chromedp.Run(ctx, chromedp.Evaluate[keyboardStep]("window.__a5.step()"))
		if err != nil {
			if outOfTime(err) {
				return rep, nil
			}
			return nil, fmt.Errorf("crawler: reading focus: %w", err)
		}
		if st.Left {
			rep.Completed = true
			break
		}
		if st.Frame { // focus moves inside an iframe or shadow root: no own focus information
			continue
		}
		if st.ID == lastID { // focus did not move
			limit := 3
			if st.Inner {
				limit = innerTrapPresses
			}
			if repeats++; repeats >= limit {
				rep.Traps = append(rep.Traps, current())
				break
			}
			continue
		}
		lastID, repeats = st.ID, 1
		if st.Seen {
			if st.First {
				rep.Completed = true // focus wrapped around to the first element
			} else {
				rep.Traps = append(rep.Traps, current())
			}
			break
		}
		if st.Candidate && !st.StyleChanged && len(rep.NoFocusStyle) < maxKeyboardFindings {
			// a CSS transition may not have finished yet: look once more
			if err := chromedp.Do(ctx, chromedp.Sleep(styleRecheckDelay)); err != nil {
				if outOfTime(err) {
					return rep, nil
				}
				return nil, err
			}
			changed, err := chromedp.Run(ctx, chromedp.Evaluate[bool]("window.__a5.styleChanged()"))
			if err != nil {
				if outOfTime(err) {
					return rep, nil
				}
				return nil, err
			}
			if !changed {
				rep.NoFocusStyle = append(rep.NoFocusStyle, current())
			}
		}
	}
	if rep.Completed && len(rep.Traps) == 0 {
		un, err := chromedp.Run(ctx, chromedp.Evaluate[[]rules.KeyboardNode]("window.__a5.unreached()"))
		if err != nil {
			return nil, err
		}
		rep.Unreachable = capNodes(un)
	}
	return rep, nil
}

func pressTab(ctx context.Context) error {
	for _, typ := range []input.DispatchKeyEventType{input.DispatchKeyEventTypeKeyDown, input.DispatchKeyEventTypeKeyUp} {
		if _, err := chromedp.Call(ctx, input.DispatchKeyEvent, input.DispatchKeyEventParams{
			Type: typ, Key: "Tab", Code: "Tab", WindowsVirtualKeyCode: 9, NativeVirtualKeyCode: 9,
		}); err != nil {
			return fmt.Errorf("crawler: pressing Tab: %w", err)
		}
	}
	return nil
}

func capNodes(ns []rules.KeyboardNode) []rules.KeyboardNode {
	if len(ns) > maxKeyboardFindings {
		return ns[:maxKeyboardFindings]
	}
	return ns
}

// pageSignalsScript reads the page-type signals from the DOM (A5-5).
const pageSignalsScript = `(() => {
  const ld = [];
  const add = t => { if (Array.isArray(t)) t.forEach(add); else if (typeof t === "string") ld.push(t); };
  const walk = o => { if (!o || typeof o !== "object") return; if (Array.isArray(o)) { o.forEach(walk); return; }
    add(o["@type"]); if (o["@graph"]) walk(o["@graph"]); };
  document.querySelectorAll('script[type="application/ld+json"]').forEach(s => { try { walk(JSON.parse(s.textContent)); } catch (e) {} });
  const og = document.querySelector('meta[property="og:type"]');
  const prod = new Set();
  document.querySelectorAll("a[href]").forEach(a => {
    try { const p = new URL(a.href, location.href).pathname.toLowerCase();
      if (/produkt|product|\/p\//.test(p)) prod.add(p); } catch (e) {}
  });
  return {ldTypes: ld, ogType: og ? (og.getAttribute("content") || "") : "", productLinks: prod.size};
})()`

// keyboardPrepareScript collects candidates and their unfocused styles and moves focus to the document start
// (A5 Logic 1–2). It keeps its state in window.__a5 for the step, styleChanged and unreached calls.
const keyboardPrepareScript = `(() => {
  const SEL = 'a[href],button,input:not([type=hidden]),select,textarea,[tabindex],[role=button],[role=link],[role=menuitem]';
  const esc = s => (window.CSS && CSS.escape) ? CSS.escape(s) : s;
  const selOf = el => {
    const parts = [];
    for (let e = el; e && e.nodeType === 1; e = e.parentElement) {
      if (e.id) { parts.unshift("#" + esc(e.id)); break; }
      let seg = e.localName;
      if (e.parentElement && !["html", "body", "head"].includes(seg)) {
        const same = Array.from(e.parentElement.children).filter(c => c.localName === e.localName);
        if (same.length > 1) seg += ":nth-of-type(" + (same.indexOf(e) + 1) + ")";
      }
      parts.unshift(seg);
    }
    return parts.join(" > ");
  };
  const snippet = el => { const h = el.outerHTML; const i = h.indexOf(">"); return (i >= 0 ? h.slice(0, i + 1) : h).slice(0, 300); };
  const node = el => ({Selector: selOf(el), Snippet: snippet(el)});
  const inClosedDetails = el => { for (let e = el.parentElement; e; e = e.parentElement) {
      if (e.localName === "details" && !e.open) { const sum = e.querySelector(":scope > summary"); if (!sum || !sum.contains(el)) return true; } }
    return false; };
  const hidden = el => {
    const r = el.getBoundingClientRect(); const cs = getComputedStyle(el);
    return (r.width === 0 && r.height === 0) || cs.visibility !== "visible" ||
      !!el.closest('[inert],[aria-hidden="true"]') || inClosedDetails(el);
  };
  const style = el => { if (!el || el.nodeType !== 1) return "";
    const c = getComputedStyle(el);
    const outline = (c.outlineStyle === "none" || c.outlineWidth === "0px") ? "none" : c.outlineStyle + " " + c.outlineWidth + " " + c.outlineColor;
    const border = ["Top", "Right", "Bottom", "Left"].map(s => c["border" + s + "Style"] === "none" ? "none" :
      c["border" + s + "Width"] + " " + c["border" + s + "Style"] + " " + c["border" + s + "Color"]).join(",");
    return [outline, c.boxShadow, border, c.backgroundColor, c.textDecorationLine].join("|");
  };
  if (document.activeElement && document.activeElement.blur) document.activeElement.blur();
  // a modal dialog open on load (e.g. cookie consent) may hold the focus: judge only inside it (Logic 6)
  let modal = null;
  try { modal = document.querySelector("dialog:modal"); } catch (e) {}
  if (!modal) modal = Array.from(document.querySelectorAll('[aria-modal="true"]')).find(el => !hidden(el)) || null;
  const cands = Array.from(document.querySelectorAll(SEL)).filter(el =>
    el.getAttribute("tabindex") !== "-1" && !el.matches(":disabled") && !hidden(el) && (!modal || modal.contains(el)));
  const base = new Map(cands.map(el => [el, [style(el), style(el.parentElement)]]));
  const nf = [], ptr = [];
  document.querySelectorAll("div,span").forEach(el => {
    if (nf.length + ptr.length >= ` + "100" + `) return;
    if (el.hasAttribute("role") || el.hasAttribute("tabindex") || hidden(el)) return;
    if (getComputedStyle(el).cursor !== "pointer") return;
    if (el.parentElement && el.parentElement.closest('a[href],button,label,summary,[role],[tabindex],[onclick]')) return;
    if (el.hasAttribute("onclick")) nf.push(node(el)); else ptr.push(el);
  });
  const ids = new Map(); let firstId = -1, current = null;
  const deep = () => { let a = document.activeElement;
    while (a && a.shadowRoot && a.shadowRoot.activeElement) a = a.shadowRoot.activeElement; return a; };
  const start = document.createElement("div"); start.tabIndex = -1; start.id = "__a5start";
  start.style.cssText = "position:absolute;top:0;left:0;width:1px;height:1px;overflow:hidden";
  const home = modal || document.body; // inside a modal, start at its top so its own focus handling keeps us there
  home && home.prepend(start); start.focus({preventScroll: true}); window.scrollTo(0, 0);
  const INNER = ["date", "time", "datetime-local", "month", "week"];
  window.__a5 = {
    ptr, node, current: () => current,
    step() {
      if (start.isConnected) start.remove();
      const el = deep();
      if (!el || el === document.body || el === document.documentElement || el === start) return {left: true};
      const seen = ids.has(el);
      if (!seen) ids.set(el, ids.size);
      const id = ids.get(el);
      if (firstId < 0) firstId = id;
      current = el;
      const frame = el.localName === "iframe" || (!!el.shadowRoot && !el.shadowRoot.activeElement);
      const inner = (el.localName === "input" && INNER.includes(el.type)) || (el.localName.includes("-") && el.shadowRoot === null);
      const b = base.get(el);
      return {left: false, id, first: id === firstId, seen, frame, inner,
        candidate: !!b, styleChanged: b ? (style(el) !== b[0] || style(el.parentElement) !== b[1]) : true};
    },
    styleChanged() { const b = base.get(current); return !b || style(current) !== b[0] || style(current.parentElement) !== b[1]; },
    unreached() {
      // a radio group is one Tab stop: one focused radio reaches the whole group (same name and form)
      const group = el => el.localName === "input" && el.type === "radio" && el.name ? (el.form ? "f" : "d") + "|" + el.name : "";
      const formOf = el => el.form || document;
      const reached = el => { const g = group(el); if (!g) return false;
        for (const r of ids.keys()) if (group(r) === g && formOf(r) === formOf(el)) return true; return false; };
      return cands.filter(el => !ids.has(el) && el.isConnected && !reached(el)).map(node);
    },
  };
  return {candidates: cands.length, pointers: ptr.length, notFocusable: nf};
})()`

// readSignals reads the page-type signals; errors give empty signals (the page is then "other").
func readSignals(ctx context.Context) pageSignals {
	sig, err := chromedp.Run(ctx, chromedp.Evaluate[pageSignals](pageSignalsScript))
	if err != nil {
		return pageSignals{}
	}
	return sig
}
