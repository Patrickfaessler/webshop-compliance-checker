# R2 Privacy notice present

- **Status:** draft · **legal-checked:** no · **Area:** legal · **Severity:** warning
- **Basis:** Regulation (EU) 2016/679 Art. 13 (information to data subjects)
- **applies_when:** every shop
- **ai_fallback:** yes (section detection only)

Presence only. Whether the content is legally correct is **not** judged; such cases are explicitly `not_assessable` with the note "inhaltliche Prüfung nicht möglich".

## Logic
1. Find link by lexicon (`datenschutz`, `privacy policy`, `politique de confidentialité`, …) in footer/header and on checkout.
2. Fetch page (HTTP 200, > 500 characters of text).
3. Detect sections by heading/keyword dictionaries: controller identity, purposes and legal basis, recipients, retention, data-subject rights, right to complain to a supervisory authority. Count found sections.
4. Unknown layout → Claude fallback for section detection with evidence quotes.

## Result mapping
| Situation | State |
| --- | --- |
| page reachable and ≥ 5 of 6 section markers | passed (note: content not reviewed) |
| no link or page unreachable | failed |
| page reachable, fewer than 5 markers | failed (`incomplete-sections`), lists missing markers |
| uncertain after fallback | not_assessable |

## Finding
URL, missing section markers.

## Explanation (German)
Wer personenbezogene Daten verarbeitet, muss darüber informieren: wer verantwortlich ist, wozu, wie lange und welche Rechte Kunden haben.

## Fix templates
Checklist of the six sections with a link to a generator suggestion (no external link baked in; text only).

## Acceptance
- R2-1: complete fixture passes. R2-2: no link fails. R2-3: page with only a heading fails with missing list. R2-4: English fixture passes via dictionary.
