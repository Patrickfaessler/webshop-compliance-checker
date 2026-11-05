# 09 Testing

Status: draft

## Test corpus (T-1)

`testcorpus/` holds small static HTML cases plus a WooCommerce test shop (Docker, added in M1).

- Per rule at least 3 cases with a deliberately planted defect and 3 clean counter-examples.
- Each case is a folder: `index.html` (and more pages if needed) and `expected.json` with the expected state per rule.
- Format and example: `testcorpus/README.md`.

## Metrics per rule (T-2)

| Metric | Definition | MVP target |
| --- | --- | --- |
| Recall | found defects / planted defects | ≥ 90 % |
| Precision | real defects / reported defects | ≥ 90 % |
| False alarms | reports on clean counter-examples | ≤ 10 % |
| Algorithm share | decisions without Claude | ≥ 85 % (R-rules, D-7) |
| Runtime | 50 pages, reference env | < 5 min (N1) |

## Levels

- Go unit tests per rule package; table-driven, each test names its spec ID in a comment (`// Spec: A6-1`).
- Corpus runner: `go test ./... -run Corpus` loads all cases, runs the engine with a **fake LLM**, compares with `expected.json`, prints the metrics table.
- Flutter widget tests for the report, detail view and disclaimer (N3).
- SSRF tests: table of internal addresses, redirect chains, DNS-rebinding stub.
- Fix check: apply the template fix to the planted case, re-run the rule, expect `passed`.
- Self-scan in CI: the built Flutter app is scanned with A1–A6 (N7).
- Field test: 10 real shops in the Salzburg region compared with a manual review; willingness-to-pay survey is separate from functional testing.

## CI gates

Every push: `go vet`, `go test ./... -cover` (≥ 70 %), `flutter analyze`, `flutter test`, corpus run. A drop of recall or precision below the targets fails the build.
