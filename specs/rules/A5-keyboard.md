# A5 Keyboard operation

- **Status:** draft · **legal-checked:** n/a · **Area:** accessibility · **Severity:** critical
- **Basis:** WCAG 2.1 SC 2.1.1, 2.1.2, 2.4.7 (EN 301 549 clause 9.2.1.1, 9.2.1.2, 9.2.4.7). Custom rule using Playwright.
- **applies_when:** page has interactive elements
- **ai_fallback:** no

## Logic
1. Collect interactive elements: links with `href`, buttons, inputs, `[tabindex]`, `[role=button|link|menuitem]`.
2. Press Tab repeatedly (max 200 presses); record each focused element and whether a visible focus indicator appears (outline/box-shadow/border changed vs. unfocused, or non-zero `:focus-visible` style).
3. Violations: interactive element never receives focus (`unreachable`); focused element has no visible indicator (`no-focus-style`); focus gets stuck on one element (`keyboard-trap`); `div`/`span` with click handler but no role/tabindex (`not-focusable`).
4. Menus and modals that need hover/click to open are tested only in their initial state; deeper states → element-level `not_assessable`.

## Result mapping
passed / failed / not_assessable (if the tab loop cannot be completed).

## Finding
URL, selector, sub-rule, snippet.

## Explanation (German)
Nicht jeder kann eine Maus benutzen. Alle Bedienelemente müssen mit der Tab-Taste erreichbar sein und man muss sehen, wo man gerade ist.

## Fix templates
`no-focus-style`: `{{selector}}:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }`; `not-focusable`: use `<button>` or add `role="button" tabindex="0"` plus key handler note.

## Acceptance
- A5-1: link and button reachable → passes. A5-2: `outline:none` without replacement fails. A5-3: `div onclick` fails. A5-4: trap fixture fails.
