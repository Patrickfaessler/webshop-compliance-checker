# A5 Keyboard operation — design

Date: 2026-11-21 · Rule spec: `specs/rules/A5-keyboard.md` · Milestone M3 (last open A rule)

## Intent

Shop visitors who cannot use a mouse must reach every control with Tab and see where the focus is (WCAG 2.1
SC 2.1.1, 2.1.2, 2.4.7). A5 is the last accessibility rule of M3. A5 needs a live page (key presses, focus,
computed styles), unlike A1–A4/A6, which work on the HTML and the axe-core result after the tab is closed.

## Goals

- Sub-rules from the spec: `unreachable`, `no-focus-style`, `keyboard-trap`, `not-focusable`.
- **Default mode "Stichprobe" (sample):** A5 runs on at most 5 pages: one per page type (start, category, product,
  cart, checkout). Shops build pages from a few templates, so one page per template finds most keyboard defects.
  Scan time grows by about 20–30 s.
- **Optional mode "alle Seiten" (all pages):** chosen by the user in the scan form. A5 runs on every crawled page.
  The scan time limit (C-1) is **10 min** instead of 5 min for this mode.
- The Tab limit per page (default 200) can be set in the form (50–1000).
- Users can always see which mode and limit ran (status, result page, HTML/PDF/JSON report), so a sample is never mistaken
  for a check of all pages.

## Non-goals

- No deeper menu or modal states: A5 tests the initial page state only (spec Logic 4). Elements inside closed
  menus are hidden and therefore not expected to be reachable.
- No mouse simulation, no Enter/Space: **only Tab is pressed**, so no link is
  followed, no form submitted, no order placed (CLAUDE.md rule 6, `specs/05-crawler.md`).
- No LLM (`ai_fallback: no`).

## Anti-goals

- A5 must not change what the other rules see: the probe runs **after** DOM, links and axe-core are read.
- Mode B must not change the rate limits (3/h, 1 running per client).
- Never call a page "tastaturbedienbar"/"konform". Wording follows S-3.

## Constraints

- Browser automation is chromedp (ADR-4), not Playwright. The spec text is updated accordingly.
- Key presses are real CDP input events (`Input.dispatchKeyEvent`, key `Tab`), not JS `focus()`, so the browser's own
  focus order and `:focus-visible` apply.
- Every request the page makes during the probe still passes the SSRF check (same tab, same listener).
- Spec → test → code; every test cites a spec ID (`A5-x`, `C-1`, …).

## Approaches considered

1. **Probe inside the crawl tab (chosen).** After axe-core, the same tab runs the keyboard probe if the page is in
   the sample (mode A) or always (mode B). No extra page load; A5 sees the same page state as the other rules.
   Trade-off: the sample is picked while crawling, so a page type found late is probed late (or, if the crawl
   stops at the limit, not at all).
2. **Separate pass after the crawl**, like the screenshot capture (RP-1). Picks the sample knowing all pages, but
   loads every probed page a second time (+1–2 s per page, up to 50 more loads in mode B) and the page may differ
   from the crawled one (the RP-3a problem). Rejected.

## Design

### Spec changes (first)

- `specs/rules/A5-keyboard.md`: chromedp instead of Playwright; the two modes and the page-type sample; the exact
  checks below; acceptance A5-1…A5-4 plus A5-5 (sample picks one page per type, max 5), A5-6 (mode B probes every
  page), A5-7 (loop cap reached → `unreachable` not judged, rule `passed` with note), A5-8 (custom cap is used and
  validated).
- `specs/05-crawler.md` C-1: 10 min for mode "alle Seiten".
- `specs/08-api.yaml`: `ScanRequest.keyboardAllPages: boolean, default false`, `keyboardMaxPresses: integer
  50–1000, default 200`; echoed in `ScanStatus` and in the
  JSON report (additive, `version` stays 1).
- `specs/02-requirements.md`: trace A5 / form option (F1).

### Page types for the sample (mode A)

Decided in the tab from the rendered DOM and URL; first page of each type wins, max 5 slots:

| Type | Detected by |
| --- | --- |
| start | the start URL (always probed, first) |
| product | JSON-LD `@type: Product` or `og:type` = `product` |
| category | JSON-LD `ItemList`/`CollectionPage`, or URL word (`kategorie`, `category`, `collection`, `/c/`), or ≥ 4 product-like tiles (builder's call on the exact heuristic, documented in the spec) |
| cart | URL word `warenkorb`, `cart`, `basket` |
| checkout | URL word `kasse`, `checkout` |

A page that fits no free slot is not probed. Empty slots at the end are fine. The report lists the probed pages.

### Probe (per probed page)

1. Collect candidates: visible, enabled interactive elements (`a[href]`, `button`, `input:not([type=hidden])`,
   `select`, `textarea`, `[tabindex]:not([tabindex="-1"])`, `[role=button|link|menuitem]`), each with its
   computed focus-relevant style while **unfocused** (outline, box-shadow, border, background-color,
   text-decoration).
2. Press Tab up to **N** times (default **200**, user-settable, see below). After each press read `document.activeElement` (selector, snippet, style).
3. Checks:
   - `no-focus-style`: the focused element's style equals its unfocused style (no outline/shadow/border/background/
     underline change). One finding per element.
   - `keyboard-trap`: focus stays on one element for ≥ 3 presses, or cycles inside a set of elements while other
     candidates were never reached and focus never returns to the document start. One finding per trap.
   - `unreachable`: a candidate never focused **when the loop completed** (focus went past the last element /
     back to `body`). Not judged if the 200-press cap ended the loop (A5-7).
   - `not-focusable`: `div`/`span` with an inline `onclick` or a click listener (CDP
     `DOMDebugger.getEventListeners`, checked on `cursor:pointer` candidates, max 100) and without a
     role/tabindex.
4. Result goes on `rules.Page` as `Keyboard *KeyboardReport` (nil = not probed); probe errors → nil + log, never
   fail the page or the scan.

### Result mapping (rule A5)

- any finding on a probed page → `failed`;
- else, no page probed (no browser, nothing in the sample) → `not_assessable`;
- else, every probed page's loop completed → `passed`;
- else (cap reached on at least one page, no finding anywhere) → `passed` **with a note** (decided by the user):
  "Erreichbarkeit nur für die ersten N Bedienelemente geprüft (Limit erreicht auf n Seiten)." The note needs a new
  field `rules.Result.NoteDe`, shown on the result page, in the HTML/PDF report and in the JSON (`noteDe`, additive).

Calls made while reviewing (builder check): only Tab is pressed, no Shift+Tab; a trap is ≥ 3 presses on one
element or a repeating cycle.

### API, worker, UI

- `POST /api/scans` accepts `keyboardAllPages` (default false) and `keyboardMaxPresses` (default 200, allowed
  **50–1000**, otherwise 400 with a German message; my call on the bounds: below 50 is useless, above 1000 one page
  could eat a large part of the time limit). Both stored on `scan.Scan` and echoed in status and reports; stored on `scan.Scan`; the worker passes it to the
  crawler options and sets the job timeout to 10 min for mode B.
- Flutter start page: below the "Barrierefreiheit" checkbox, an indented checkbox (only enabled when that area is
  on):

  ```
  [x] Barrierefreiheit
      [ ] Gründliche Tastaturprüfung auf allen Seiten (dauert bis zu 10 Minuten)
      Tab-Drücke pro Seite: [ 200 ]   (50–1000; mehr = gründlicher, aber langsamer)
  [x] Rechtliche Pflichtangaben
  ```

- Result page and reports: a line "Tastaturprüfung: Stichprobe (n Seiten)" or "… alle Seiten (n Seiten)".
- German explanation and fix templates as in the rule spec.

### Tests

- Corpus cases `a5-*` for A5-1…A5-4 (link+button reachable, `outline:none`, `div onclick`, trap) run through
  `AnalyzeHTML`, which always probes.
- Unit tests: page-type detection, sample slots, result mapping, cap handling.
- Test shops: plant an A5 defect in Radwerk (e.g. `outline:none` on buttons) and add `A5` to the `expected.json`
  files (Teehaus stays clean); e2e covers both modes.
- API + widget tests for the new field and checkbox.

## Left to the builder

Exact category heuristic, trap-cycle detection details, style-comparison tolerance, selector/snippet reuse
(`domutil`/RP-3b escaping), wording of log lines.
