# 04 Scoring and severity

Status: draft

## Result states (per rule, per scan)

| State | Meaning | In score |
| --- | --- | --- |
| `passed` | check ran completely, no violation | yes (counts as passed) |
| `failed` | violation with location (URL, selector, snippet) | yes (counts as failed) |
| `not_applicable` | precondition absent (e.g. no forms on page) | no |
| `not_assessable` | algorithm and fallback could not decide; manual check needed | no |

A rule is `failed` for the scan if at least one page violates it; the detail view shows how many pages are affected.

## Severity and weight

| Severity | Weight g | Rules |
| --- | --- | --- |
| critical | 3 | R1, R3, R4, R6, R7, A3, A5 |
| warning | 2 | R2, R5, A1, A2, A4 |
| notice | 1 | A6 |

## Formula (S-1)

```latex
\text{Score} = 100 \cdot \frac{\sum_{r \in P} g_r}{\sum_{r \in P \cup F} g_r}
```

`P` = passed rules, `F` = failed rules. If `P ∪ F` is empty the score is not shown ("nicht berechenbar").

## Rules for presenting the score

- S-2: Score and result counts (including `not_applicable`, `not_assessable`) are always shown together.
- S-3: The score is never labelled "compliant" or "konform". Wording: "Anteil bestandener automatischer Prüfungen".
- S-4: Score is a pure function of the rule results; the same results always give the same score (unit-tested).
