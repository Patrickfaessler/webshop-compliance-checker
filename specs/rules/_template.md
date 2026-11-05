# <ID> <Name>

- **Status:** draft
- **legal-checked:** n/a | no | yes (by, date)
- **Area:** accessibility | legal
- **Severity:** critical | warning | notice
- **Basis:** (EU article / WCAG criterion)
- **applies_when:** (precondition; else `not_applicable`)
- **ai_fallback:** no | yes (for what)

## Input
What data the rule reads (pages, DOM, requests per browser state).

## Logic
Numbered, deterministic steps. Mark where confidence is computed.

## Result mapping
| Situation | State |
| --- | --- |
| … | passed / failed / not_applicable / not_assessable |

## Finding
Location format (URL, selector, snippet), evidence.

## Explanation (German, plain language)
`explanation_de` text shown to the user.

## Fix templates
Template(s) with placeholders. AI only if none fits (D-3).

## Acceptance (sub-requirements)
- `<ID>-1` …
- Corpus: ≥ 3 planted defects, ≥ 3 clean counter-examples in `testcorpus/cases/`.
