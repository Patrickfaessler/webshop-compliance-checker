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
type probePlan struct {
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
	Left         bool   `json:"left"`  // focus is on body / outside the document
	ID           int    `json:"id"`    // stable number of the focused element
	First        bool   `json:"first"` // this is the first element focused in this probe
	Seen         bool   `json:"seen"`  // focused before
	Frame        bool   `json:"frame"` // iframe or shadow host without own focus info: repeats do not count
	Selector     string `json:"sel"`
	Snippet      string `json:"snippet"`
	Candidate    bool   `json:"candidate"`
	StyleChanged bool   `json:"styleChanged"`
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

	lastID, repeats := -1, 0
	for rep.Presses < maxPresses {
		if err := pressTab(ctx); err != nil {
			return nil, err
		}
		rep.Presses++
		st, err := chromedp.Run(ctx, chromedp.Evaluate[keyboardStep]("window.__a5.step()"))
		if err != nil {
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
			if repeats++; repeats >= 3 {
				rep.Traps = append(rep.Traps, rules.KeyboardNode{Selector: st.Selector, Snippet: st.Snippet})
				break
			}
			continue
		}
		lastID, repeats = st.ID, 1
		if st.Seen {
			if st.First {
				rep.Completed = true // focus wrapped around to the first element
			} else {
				rep.Traps = append(rep.Traps, rules.KeyboardNode{Selector: st.Selector, Snippet: st.Snippet})
			}
			break
		}
		if st.Candidate && !st.StyleChanged && len(rep.NoFocusStyle) < maxKeyboardFindings {
			// a CSS transition may not have finished yet: look once more
			if err := chromedp.Do(ctx, chromedp.Sleep(styleRecheckDelay)); err != nil {
				return nil, err
			}
			changed, err := chromedp.Run(ctx, chromedp.Evaluate[bool]("window.__a5.styleChanged()"))
			if err != nil {
				return nil, err
			}
			if !changed {
				rep.NoFocusStyle = append(rep.NoFocusStyle, rules.KeyboardNode{Selector: st.Selector, Snippet: st.Snippet})
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
  const cands = Array.from(document.querySelectorAll(SEL)).filter(el =>
    el.getAttribute("tabindex") !== "-1" && !el.matches(":disabled") && !hidden(el));
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
  document.body && document.body.prepend(start); start.focus({preventScroll: true}); window.scrollTo(0, 0);
  window.__a5 = {
    ptr, node,
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
      const b = base.get(el);
      return {left: false, id, first: id === firstId, seen, frame, sel: selOf(el), snippet: snippet(el),
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
