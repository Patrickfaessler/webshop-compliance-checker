# A6 Page language declared

- **Status:** verified (2026-11-05): unit tests A6-1..A6-4 and corpus (3 defect + 3 clean cases, recall 3/3, 0 false alarms) pass. Code: `backend/internal/rules/a6`. Wired to the crawler and API; live-tested.
- **legal-checked:** n/a
- **Area:** accessibility
- **Severity:** notice
- **Basis:** WCAG 2.1 SC 3.1.1 Language of Page (via EN 301 549 clause 9.3.1.1, Directive (EU) 2019/882)
- **applies_when:** every HTML page
- **ai_fallback:** no

This is the **first rule to implement end to end** (M2): it is simple and fully deterministic.

## Input
Rendered DOM of each crawled page.

## Logic
1. Read `lang` attribute of the `<html>` element.
2. Valid if it is a well-formed BCP 47 tag (primary subtag 2–3 letters, e.g. `de`, `de-AT`, `en`).
3. Missing, empty or malformed → violation.

## Result mapping
| Situation | State |
| --- | --- |
| all pages valid | passed |
| at least one page missing/invalid | failed |
| no HTML page reachable | not_assessable |

## Finding
URL, selector `html`, snippet = opening `<html …>` tag.

## Explanation (German)
Die Seite sagt nicht, in welcher Sprache sie geschrieben ist. Screenreader wissen dann nicht, wie sie den Text aussprechen sollen.

## Fix templates
- `Setze das Attribut lang im html-Element: <html lang="{{lang}}">`. `{{lang}}` is guessed from the visible text by a deterministic detector; if unsure, the template shows `de` with the note "bitte anpassen".

## Acceptance
- A6-1: `<html lang="de">` passes.
- A6-2: `<html>` without lang fails with selector `html`.
- A6-3: `lang=""` and `lang="deutsch"` fail.
- A6-4: `lang="de-AT"` passes.
- Corpus cases: `testcorpus/cases/a6-has-lang`, `a6-missing-lang` (more to add).
