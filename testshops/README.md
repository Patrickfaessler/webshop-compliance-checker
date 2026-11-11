# Test shops

Three **fictional** webshops with deliberately planted defects, for testing the whole flow: crawl, rules, score, report.
Spec: `specs/09-testing.md` (T-3).

| Shop | Folder | Kind | Expected (A1, A2, A3, A4, A6) | Score |
| --- | --- | --- | --- | --- |
| Teehaus Bergkristall | `site/teehaus/` | exemplary | all passed | 100 % |
| Radwerk Alpin | `site/radwerk/` | many defects | all failed | 0 % |
| Seifenmanufaktur Lavendel | `site/seifen/` | mixed | A1, A2, A4 failed; A3, A6 passed | 40 % |

Each shop's `expected.json` lists the expected state per implemented rule, the expected score, all pages, and every
planted defect (page, rule, what). Legal defects for R1–R7 are planted and listed already; the scanner does not check them yet.

## Online

Published with GitHub Pages from the `gh-pages` branch:

- Overview: https://patrickfaessler.github.io/webshop-compliance-checker/
- https://patrickfaessler.github.io/webshop-compliance-checker/teehaus/
- https://patrickfaessler.github.io/webshop-compliance-checker/radwerk/
- https://patrickfaessler.github.io/webshop-compliance-checker/seifen/

Paste one of the shop addresses into the Webshop-Check and compare the report with the table above.

## Local

Serve the shops and scan them through the local app (the switch `ALLOW_LOOPBACK_TARGETS=1` lets the scanner
reach `127.0.0.1`; it only works while the API listens on loopback, spec C-SSRF-DEV):

```powershell
cd testshops/serve; go run .            # shops on http://127.0.0.1:5500/
# second shell:
cd backend
$env:API_ADDR='127.0.0.1:8080'; $env:CORS_ORIGIN='http://localhost:5000'
$env:RATE_LIMIT_PER_HOUR='100'; $env:ALLOW_LOOPBACK_TARGETS='1'
go run ./cmd/api
# third shell: cd frontend; flutter run -d chrome --web-port 5000
```

Then scan `http://127.0.0.1:5500/teehaus/`, `.../radwerk/` or `.../seifen/` (type the `http://`).

## Change a shop

The HTML is generated; do not edit `site/` by hand.

1. Edit `gen/shops.go` (shop data and defect flags) or `gen/main.go` (page templates). Update `Expected`/`ExpectedScore`.
2. Regenerate: `cd testshops/gen; go run . ../site`
3. Check: `cd backend; go test ./internal/e2e/` crawls all shops in the browser and compares with `expected.json`.
4. Commit `gen/` and `site/` together, then publish: `git subtree push --prefix testshops/site origin gh-pages`
   (from the repository root, on a branch that contains the change).

## Rules for the shops

- Clearly fictional: a note on every page, invented company data (`.example` addresses), `noindex`, nothing can be ordered.
- No real third-party scripts or trackers.
- Shops never link to each other: all share one host on GitHub Pages, and the crawler stays on the host.
