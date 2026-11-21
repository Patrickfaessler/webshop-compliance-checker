# A5 Keyboard operation

- **Status:** verified (2026-11-21): unit tests (rule mapping A5-7, page types A5-5/A5-6, probe fixtures A5-1…A5-4, A5-9, cap), corpus 3 defect + 1 clean (recall 3/3, 0 false alarms), test shops (Radwerk planted defects found, Teehaus/Seifen pass). Limitations: delegated click handlers on ancestors and broken arrow keys inside radio groups are not detected. Code: `backend/internal/crawler/keyboard*.go`, `backend/internal/rules/a5`. Fix templates implemented and tested (2026-11-21, A5-F; code in `backend/internal/fixes` and the rule). · **legal-checked:** n/a · **Area:** accessibility · **Severity:** critical
- **Basis:** WCAG 2.1 SC 2.1.1, 2.1.2, 2.4.7 (EN 301 549 clause 9.2.1.1, 9.2.1.2, 9.2.4.7). Custom rule: a keyboard
  probe in the crawl tab with chromedp and real CDP key events (`Input.dispatchKeyEvent`, ADR-4).
- **applies_when:** page has interactive elements
- **ai_fallback:** no
- **Design:** `docs/superpowers/specs/2026-11-21-a5-keyboard-design.md`

## Modes and pages

The probe runs in the crawl tab **after** the DOM, the links and axe-core were read, so it never changes what the
other rules see. Only the **Tab** key is pressed: never Enter or Space, never a click (no link is followed, no form
submitted, `05-crawler.md`).

- **Stichprobe (default):** at most **5** pages, the first page of each type:

  | Type | Detected by (URL words on the lower-cased path only) |
  | --- | --- |
  | start | the start page (always, first) |
  | checkout | URL word `kasse`, `checkout` (wins over cart) |
  | cart | URL word `warenkorb`, `cart`, `basket` |
  | product | JSON-LD `@type` `Product`, `og:type` = `product`, or URL word `produkt`, `product`, `/p/` |
  | category | JSON-LD `@type` `ItemList` or `CollectionPage`, URL word `kategorie`, `category`, `collection`, `/c/`, or ≥ 4 distinct links to product-like URLs |

  Pages of another type, or of a type already probed, are not probed.
- **Alle Seiten (form option):** every crawled page is probed; the scan time limit is 10 min (C-1).
- **Tab limit N:** default **200** presses per page, settable in the form, allowed **50–1000**.
- The probe has its own time budget of **30 s** per page. A probe error or timeout leaves the page without a keyboard
  result (it still counts for all other rules) and never fails the scan.

## Logic

1. Collect candidates: visible, enabled interactive elements: `a[href]`, `button`, `input:not([type=hidden])`,
   `select`, `textarea`, `[tabindex]` except `tabindex="-1"`, `[role=button|link|menuitem]`. Not candidates: elements
   that are `disabled`, `display:none`, `visibility:hidden`, without area, inside `[inert]`, inside an
   `aria-hidden="true"` ancestor, or inside a closed `<details>` (initial state only, see 5). For each candidate and
   its parent, record the unfocused style: outline, box-shadow, border, background-color, text-decoration-line.
2. Start at the top of the document (the focus a page set itself, e.g. `autofocus`, is removed first). Press Tab up
   to N times; after each press read the focused element.
3. Violations (sub-rules):
   - `no-focus-style`: the style of the focused element **and** of its parent is equal to the unfocused style
     (so a `:focus-within` frame on a wrapper counts as visible).
   - `keyboard-trap`: focus stays on one element for ≥ 3 presses (≥ 15 for elements with inner Tab stops the probe
     cannot see: `input` of type date, time, datetime-local, month, week, and custom elements with a closed shadow
     root), or an element is focused a second time before the
     focus left the document. The probe stops there.
   - `unreachable`: a candidate that was never focused, judged **only when the loop completed** (focus left the
     document after the last element before the limit).
   - `not-focusable`: a `div`/`span` with `cursor:pointer` and an inline `onclick` or a `click`, `mousedown` or
     `pointerdown` listener (`DOMDebugger.getEventListeners`, at most 100 checked per page), without a role and
     without `tabindex`.
4. At most **20 findings per sub-rule and page**.
5. Menus and modals that need hover or click to open are tested only in their initial state.
6. A modal dialog that is open on load (visible `[aria-modal="true"]` or modal `<dialog open>`, e.g. a cookie
   consent) may hold the focus (WCAG 2.1.2 allows this while it has a way out). The candidates are then only the
   elements inside the dialog; the page behind it is not judged for `unreachable`.
7. Pages that come back with an error status (≥ 400) or end on another host are not probed and take no sample slot.

## Result mapping

- any finding on a probed page → `failed`;
- no page probed (no browser, or the probe failed everywhere) → `not_assessable`;
- every probed page's loop completed → `passed`;
- otherwise (limit reached on a page, no finding) → `passed` with the note
  `Erreichbarkeit nur für die ersten {N} Bedienelemente geprüft (Limit erreicht auf {k} von {n} geprüften Seiten).`

## Finding
URL, selector, sub-rule, snippet (opening tag).

## Explanation (German)
Nicht jeder kann eine Maus benutzen. Alle Bedienelemente müssen mit der Tab-Taste erreichbar sein und man muss sehen, wo man gerade ist.

## Fix templates
- `no-focus-style`: text `Mach sichtbar, welches Element den Fokus hat.`, code `{{selector}}:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }`
- `not-focusable`: text `Verwende ein <button>-Element. Alternativ role="button" und tabindex="0" ergänzen und Enter/Leertaste per Skript bedienen.`, code `<button type="button">…</button>`
- `unreachable`: text `Mach das Element mit der Tab-Taste erreichbar: entferne tabindex="-1" oder Skripte, die den Fokus wegnehmen.`, no code
- `keyboard-trap`: text `Sorge dafür, dass man das Element mit der Tab-Taste wieder verlassen kann.`, no code

## Acceptance
- A5-1: link and button reachable → passes.
- A5-2: `outline:none` without replacement fails.
- A5-3: `div onclick` fails.
- A5-4: trap fixture fails.
- A5-5: mode Stichprobe probes the start page and the first page of each other type, at most 5 pages.
- A5-6: mode "alle Seiten" probes every crawled page.
- A5-7: limit reached on a page, no finding → `passed` with the note above; `unreachable` is not judged there.
- A5-8: the Tab limit from the form is used; values outside 50–1000 are rejected with a German message.
- A5-9: a probe error keeps the page (HTML, axe result) without a keyboard result. When the probe's time
- A5-F: every finding carries a fix (origin `template`, non-empty German text); page values in the code are HTML-escaped.
  (30 s) runs out during the Tab loop, the presses so far count like a reached limit (A5-7, the note names the
  presses actually made).
