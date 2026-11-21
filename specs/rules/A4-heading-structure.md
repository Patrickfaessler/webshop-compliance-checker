# A4 Heading structure

- **Status:** verified (2026-11-05): unit tests A4-1..A4-4 and corpus 3 defect + 3 clean, recall 3/3, 0 false alarms. Limitation: hidden headings are detected from markup only (hidden, aria-hidden, inline style), not stylesheets. Code: `backend/internal/rules/a4`. Fix templates implemented and tested (2026-11-21, A4-F; code in `backend/internal/fixes` and the rule). ·  **legal-checked:** n/a · **Area:** accessibility · **Severity:** warning
- **Basis:** WCAG 2.1 SC 1.3.1, 2.4.6 (EN 301 549 clause 9.1.3.1, 9.2.4.6). Custom rule, not only axe.
- **applies_when:** every HTML page
- **ai_fallback:** no

## Logic
1. Collect `h1`–`h6` and `role=heading[aria-level]` in document order.
2. Violation if: no `h1`; more than one `h1`; a level jumps down by more than one (e.g. `h2` → `h4`); a heading is empty.
3. Headings hidden from all users (`display:none`) are ignored.

## Result mapping
passed / failed. A page with no headings at all fails (no `h1`).

## Finding
URL, selector of the offending heading, snippet, which sub-rule (`no-h1`, `multi-h1`, `skipped-level`, `empty`).

## Explanation (German)
Überschriften gliedern die Seite wie ein Inhaltsverzeichnis. Screenreader-Nutzer springen damit durch die Seite; fehlende oder übersprungene Ebenen machen das schwer.

## Fix templates
No code; text only:
- `skipped-level`: `Ändere <h{{from}}> zu <h{{to}}>.` (`from` = found level, `to` = previous level + 1)
- `no-h1`: `Füge genau eine <h1> mit dem Seitentitel hinzu.`
- `multi-h1`: `Behalte nur eine <h1> und mache die anderen zu <h2>.`
- `empty`: `Gib der Überschrift einen Text oder entferne das leere Element.`

## Acceptance
- A4-1: h1,h2,h3 passes. A4-2: h1,h3 fails (`skipped-level`). A4-3: two h1 fail. A4-4: empty `<h2></h2>` fails.
- A4-F: every finding carries a fix (origin `template`, non-empty German text); page values in the code are HTML-escaped.
