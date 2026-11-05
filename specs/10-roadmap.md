# 10 Roadmap

Dates to be set with the course schedule. A milestone is done when its listed specs are `verified`.

| # | Milestone | Specs | Done when |
| --- | --- | --- | --- |
| M0 | Project scaffold, toolchains installed, CI green on empty tests | N8 | `go test` and `flutter test` run in CI |
| M1 | Docker Compose with Postgres + WooCommerce test shop; corpus runner skeleton | 09 | `docker compose up` gives a shop; runner reports 0 cases failing |
| M2 | Crawler + SSRF + rate limits + job queue; rule A6 end to end through API and Flutter. **Progress (2026-11-05):** SSRF filter, SSRF-safe fetcher, rate limits, host cooldown, in-memory store, in-process runner (start page only), `POST/GET /api/scans…` and A6 end to end through the API are done and tested. Flutter start and result screens (progress, score, rule results, findings, disclaimer) are done and widget-tested. The 50-page browser crawler (chromedp) is done too. Open: egress proxy, PostgreSQL + River queue, `GET /findings/{id}`, suggestions, PDF. | F1, F2, F11, F12, C-*, A6 | URL in the UI → result for A6 |
| M3 | Rules A1–A5 | F3 | corpus targets met. **Progress (2026-11-05):** A1, A2, A3, A4 done and verified on the corpus (recall 3/3, 0 false alarms each, real browser + axe-core). Open: A5 keyboard, fix templates. |
| M4 | Rules R1–R7 with dictionaries and Claude fallback | F4–F6, D-* | corpus targets met, algorithm share measured |
| M5 | Report, detail view, fix templates, AI suggestions | F7, F8 | UI complete for Must requirements |
| M6 | Should items (re-scan comparison, PDF), self-scan, field test, final presentation | F9, F10, N7 | field test report written |
