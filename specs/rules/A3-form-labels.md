# A3 Form labels

- **Status:** verified (2026-11-05): corpus 3 defect + 3 clean, recall 3/3, 0 false alarms; axe label rules plus DOM check for placeholder-only fields (A3-2), because axe itself accepts a placeholder as a name. Code: `backend/internal/rules/a3`. Fix templates not yet implemented. ·  **legal-checked:** n/a · **Area:** accessibility · **Severity:** critical
- **Basis:** WCAG 2.1 SC 1.3.1, 3.3.2, 4.1.2 (EN 301 549 clause 9.1.3.1, 9.3.3.2, 9.4.1.2)
- **applies_when:** page has `input`, `select`, or `textarea` (excluding `hidden`, `submit`, `button`)
- **ai_fallback:** no

## Logic
1. axe-core `label`, `select-name`, `aria-input-field-name`.
2. Accessible name from `<label for>`, wrapping label, `aria-label` or `aria-labelledby`. `placeholder` alone is not enough.

## Result mapping
passed / failed / not_applicable.

## Finding
URL, selector, snippet (input tag), `type`, `name`.

## Explanation (German)
Dieses Eingabefeld hat keine Beschriftung, die ein Screenreader vorlesen kann. Nutzer wissen nicht, was sie eintragen sollen.

## Fix templates
`<label for="{{id}}">{{labelText}}</label>` placed before the field; `labelText` derived from `placeholder` or `name` when available, else a placeholder with note. Adds `id` if missing.

## Acceptance
- A3-1: input without label fails; with `<label for>` passes.
- A3-2: placeholder-only fails.
- A3-3: `aria-label` passes.
