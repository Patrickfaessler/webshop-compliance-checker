# Specs: source of truth

This project is built **spec-driven**: the files in `specs/` define behaviour, the code implements them, the tests prove it.
If code and spec disagree, one of them is a bug. Decide which, and fix it in the same change.

## Workflow (every change)

1. **Spec first.** Change or add the spec file (`specs/…`). New behaviour without a spec ID is not allowed.
2. **Test second.** Write a failing test (or a `testcorpus/` case) that cites the spec ID, e.g. `// Spec: R6-3`.
3. **Code third.** Implement until the test passes.
4. **Trace.** Update the status column in `specs/02-requirements.md` (or the rule file) and mention the spec IDs in the commit message.

## ID scheme

| Prefix | Meaning | Where |
| --- | --- | --- |
| `F` | functional requirement | `02-requirements.md` |
| `N` | non-functional requirement | `02-requirements.md` |
| `A1…` | accessibility rule | `rules/A*.md` |
| `R1…` | legal rule | `rules/R*.md` |
| `S` | scoring | `04-scoring.md` |
| `C` | crawler | `05-crawler.md` |
| `D` | detection and AI fallback | `06-detection-and-ai.md` |
| `ADR-n` | decision record | `decisions/` |

Sub-requirements are numbered inside their file (`R6-1`, `R6-2`, …). IDs are never reused.

## Status values

`draft` → `reviewed` (team agrees) → `implemented` (code exists) → `verified` (tests/corpus pass).
Legal rules additionally need `legal-checked: yes/no` (see `01-legal-scope.md`).

## Language

Specs, code and comments are in English. The **product UI and all explanations shown to users are German**
(requirement N2). German strings live in the frontend `l10n` files and in the rule `explanation_de` fields.

## Index

- `00-overview.md` product, users, non-goals
- `01-legal-scope.md` EU instruments the rules are based on
- `02-requirements.md` F/N requirements with acceptance criteria
- `rules/` one file per rule (A1–A6, R1–R7), template in `rules/_template.md`
- `04-scoring.md`, `05-crawler.md`, `06-detection-and-ai.md`, `07-architecture.md`
- `08-api.yaml` OpenAPI contract (frontend and backend both build against it)
- `09-testing.md` test corpus, metrics, CI gates
- `10-roadmap.md` milestones
- `open-questions.md` unresolved items and name ideas
- `decisions/` ADRs
