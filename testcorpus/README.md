# Test corpus

Spec: `specs/09-testing.md` (T-1, T-2).

Each case is a folder under `cases/` named `<rule>-<what>`:

```
cases/a6-missing-lang/
  index.html       the page(s) to scan
  expected.json    expected state per rule
```

## expected.json

```json
{
  "spec": ["A6-2"],
  "kind": "defect",
  "expect": { "A6": "failed" }
}
```

- `kind`: `defect` (planted defect) or `clean` (counter-example, must not be reported).
- `expect`: rule ID → `passed | failed | not_applicable | not_assessable`. Rules not listed are not checked for this case.
- Optional `llm`: fixture responses the fake LLM returns (for fallback tests).

## Rules of the corpus

- Per rule ≥ 3 `defect` and ≥ 3 `clean` cases before the rule can be `verified`.
- A case documents **one** planted defect. Keep cases small.
- Also planned: WooCommerce test shop in Docker (M1) for checkout and cookie-state cases.
