# A1 Text contrast

- **Status:** verified (2026-11-05): corpus 3 defect + 3 clean, recall 3/3, 0 false alarms with axe-core 4.10.2 in Edge. Code: `backend/internal/rules/a1`. Fix templates implemented and tested (2026-11-21, A1-F; code in `backend/internal/fixes` and the rule). ·  **legal-checked:** n/a · **Area:** accessibility · **Severity:** warning
- **Basis:** WCAG 2.1 SC 1.4.3 (EN 301 549 clause 9.1.4.3)
- **applies_when:** page contains visible text
- **ai_fallback:** no

## Input
Rendered DOM with computed styles.

## Logic
1. Run axe-core rule `color-contrast` (version pinned in repo).
2. Violation if ratio < 4.5:1 (normal text) or < 3:1 (large text: ≥ 24 px, or ≥ 18.66 px bold).
3. axe "incomplete" results (background image, overlap) → element-level `not_assessable`, not a violation.

## Result mapping
| Situation | State |
| --- | --- |
| no violations | passed |
| ≥ 1 violation | failed |
| only incomplete results | not_assessable |
| no text | not_applicable |

## Finding
URL, selector, snippet, measured ratio, foreground/background colours.

## Explanation (German)
Text und Hintergrund sind zu ähnlich in der Farbe. Menschen mit Sehschwäche können den Text schlecht lesen.

## Fix templates
- Default: text `Ändere die Textfarbe von {{fg}} auf {{suggestedFg}} (Kontrast {{ratio}}:1).`, code `color: {{suggestedFg}};`. `suggestedFg` is the nearest colour (same hue and saturation, lightness changed) reaching the required ratio (`expectedContrastRatio`, default 4.5:1), computed algorithmically; `ratio` is the suggestion's own ratio with a German decimal comma and one decimal (e.g. `4,6`).
- Background unknown (no opaque `bgColor`, e.g. image or gradient): text `Wähle eine Textfarbe mit ausreichend Kontrast zum Hintergrund.`, note `Hintergrund nicht eindeutig (z. B. Bild) – Kontrast bitte selbst prüfen.`, no code.

## Acceptance
- A1-1: #777 on white fails (ratio ≈ 4.48), #767676 passes.
- A1-2: large text threshold 3:1 applies.
- A1-3: suggested colour always reaches the required ratio (property test).
- A1-F: every finding carries a fix (origin `template`, non-empty German text); page values in the code are HTML-escaped.
