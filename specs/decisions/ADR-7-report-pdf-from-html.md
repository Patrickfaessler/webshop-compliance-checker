# ADR-7 Visual report: HTML first, PDF printed from it

Status: accepted (2026-11-12, agreed with Fässler)

**Decision.** The report gets a visual HTML version (screenshots with marked findings, `specs/11-report.md`). The
PDF is printed from that HTML by headless Chromium (`Page.printToPDF`) instead of being drawn with `fpdf`. The `fpdf`
PDF stays as fallback when no browser is available.

**Why.**
- One layout for HTML and PDF: they cannot drift apart, and every change is made once.
- The browser is already part of the backend (crawler, ADR-4) and of the server image.
- `fpdf` core fonts only cover cp1252 (no "≥", no non-Latin text from shops) and cannot produce a tagged
  (accessible) PDF; Chromium can (N7).
- Images, frames and cropped details are plain HTML/CSS instead of manual PDF drawing code.

**Consequences.**
- Element positions need a second render of each affected page after the rules (RP-1); capped at 15 pages.
- PDF generation needs the browser; without it the text PDF is served (RP-11).
- The print tab blocks all network requests; the report HTML contains no script (RP-5, RP-10).
- Screenshots live in memory with the scan (ADR-6) and are not persisted by the PostgreSQL store.
