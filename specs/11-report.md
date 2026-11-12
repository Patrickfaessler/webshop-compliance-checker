# 11 Visual report (HTML + PDF)

Status: draft · Owner: Kahraman · Requirements: F8 (location, screenshot), F10 (PDF), F14 (HTML report), N3, N7

The report shows **where** each problem is: a screenshot of every affected page with numbered marks on the faulty
elements, next to the list of findings. It exists as a self-contained HTML file (view and download) and as a PDF
printed from the same HTML (ADR-7). It builds on the JSON report (`internal/report`, F13).

## Capture (RP-1 … RP-4)

- **RP-1 Separate step after the rules.** The crawler closes each tab before the rules run, so element positions are
  unknown while crawling. After all rules ran, the worker opens every page **that has findings** once more and takes
  one capture per page. Pages without findings are not captured.
- **RP-2 One render per page.** Screenshot and element positions come from the **same** render, so the marks match.
  Viewport 1280 × 900 CSS px, device scale 1. The screenshot covers the full page height, capped at **8 000 px**
  (`truncated` is set). Width is the page's scroll width, capped at 2 × viewport. JPEG, quality 70.
- **RP-3 Positions per finding.** For each finding's selector the browser returns the element's box in document
  coordinates (`getBoundingClientRect` + scroll offset). Each finding gets one `position`:

  | position | Meaning | Shown as |
  | --- | --- | --- |
  | `element` | element found, visible, inside the screenshot | numbered frame on the screenshot + cropped detail |
  | `page` | selector is `html` (rule concerns the whole page, e.g. A6, A4 `no-h1`) | banner "betrifft die ganze Seite" |
  | `hidden` | element found but without area (width and height 0), `display:none`, `visibility:hidden` or `opacity:0` | listed, "Element ist nicht sichtbar" |
  | `outside` | element lies below the capped screenshot | listed, "außerhalb des Screenshots" |
  | `not_found` | selector matches nothing in the new render (page changed, invalid selector) | listed, "Position nicht ermittelbar" |
  | *(empty)* | no capture for this page (no browser, limit, error) | listed without marks |

  An element with only one zero side (e.g. an empty `<h2></h2>`, A4 `empty-heading`) is `element` with a frame of
  at least 8 px, so the user still sees where it is.
- **RP-4 Limits and safety.** At most **15 pages** are captured per scan (crawl order). Every request of the capture
  tab goes through the same SSRF check as the crawler (C-SSRF). A failed capture never fails the scan; the report is
  then shown without marks for that page. Captures stay in memory with the scan and are deleted with it (ADR-6, N4);
  the PostgreSQL store does not persist them.

## HTML report (RP-5 … RP-9)

- **RP-5 Self-contained file.** One HTML document, German (`lang="de"`), all CSS inline, images as `data:` URIs,
  **no JavaScript, no external requests**. It works offline and can be downloaded and passed on.
- **RP-6 Content.** Header: shop address, date (Vienna time), status, pages scanned, score with the S-3 wording
  ("Anteil bestandener automatischer Prüfungen", never "konform"), counts per state, the disclaimer (N3) at the top
  and at the end. Rule table (name, severity, state, pages affected). Then one section per affected page.
- **RP-7 Page section.** Left: screenshot with numbered frames (positions in % of the image, so it scales). Right:
  finding cards with number, severity (text, not colour only), rule ID and German name, explanation (`summaryDe`),
  location (URL, selector, snippet), position note (RP-3), and for `element` a cropped detail of the area.
  Findings are numbered 1…n across the whole report.
- **RP-8 Linking without script.** A frame links to its card (`#f-<n>`) and the card links back to the frame
  (`#m-<n>`); the target is highlighted with CSS `:target`.
- **RP-9 Accessible report (N7).** Real headings in order, contrast ≥ 4.5:1, every frame has a text label
  ("Fundstelle 3: …"), visible keyboard focus, screenshots have alt text naming the page and the number of marks.
  The report must pass our own rules A1–A6 (self-scan test).

## PDF (RP-10 … RP-11)

- **RP-10 Printed from the HTML (ADR-7).** The PDF is the HTML report printed by headless Chromium (A4, backgrounds
  on, **tagged PDF** for screen readers, document outline). The print tab blocks every network request; the HTML is
  set with `Page.setDocumentContent`, not via a URL. Print CSS: one column, page breaks between page sections,
  marks and numbers stay visible.
- **RP-11 Fallback.** Without a browser (or if printing fails) the existing text PDF (`fpdf`) is served, so the
  download never breaks.

## API

- `GET /api/scans/{id}/report.html` shows the HTML report inline; with `?download=1` it is sent as attachment
  `webshop-check_<host>_<date>.html`. Same 404/409 rules as the PDF/JSON downloads.
- `GET /api/scans/{id}/report.pdf` unchanged URL; content per RP-10/RP-11.
- JSON report (F13): findings gain `position` and `box` (`x, y, width, height` in CSS px of the screenshot), the
  report gains `pages` (`url, width, height, viewportWidth, truncated`). Screenshots themselves are **not** in the
  JSON. The changes only add fields, so `version` stays 1.

## Acceptance

- RP-A1: the capture of a page returns one position per selector; `html` → `page`; a missing selector → `not_found`;
  a `display:none` element → `hidden`; boxes lie inside the image for `element`.
- RP-A2: the HTML report of a scan with findings on two pages has two page sections, one frame per `element`
  finding, numbers 1…n without gaps, a link from every frame to its card and back, and the disclaimer twice.
- RP-A3: the HTML contains no `<script>`, no `http://`/`https://` resource URLs (only text) and no event handler
  attributes; user-controlled text (snippet, selector, URL) is escaped.
- RP-A4: test shop "Radwerk Alpin": every planted defect of A1–A4 that has an element appears as a frame on the
  right page.
- RP-A5: the printed PDF starts with `%PDF`, contains the disclaimer text and is tagged; without a browser the
  `fpdf` PDF is returned.
