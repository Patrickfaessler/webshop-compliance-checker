# CLAUDE.md

Webshop compliance checker (working title). Flutter web frontend + Go backend (both mandatory). Spec-driven.

## Rules for working in this repo

1. **`specs/` is the source of truth.** Read `specs/README.md` and the relevant spec file before changing code.
   If a request contradicts a spec, say so and update the spec first (with the user's agreement).
2. **Spec → test → code.** Every test cites a spec ID in a comment (`// Spec: R6-2`). No behaviour without a spec ID.
3. **Algorithms first, Claude as fallback** (ADR-3, `specs/06-detection-and-ai.md`). Never add an LLM call where a deterministic check can work.
   The LLM may never turn `failed` into `passed`; it must quote evidence.
4. **No user accounts** (ADR-1). Do not add login, user tables or per-person storage.
   Database: local Postgres by default, Supabase optional (ADR-5); only the Go backend connects, no Supabase Auth, no DB access from Flutter.
5. **EU-level legal scope** (ADR-2). Legal rules cite EU articles in `specs/01-legal-scope.md`. Do not mark a legal rule `verified` without the `legal-checked` entry.
6. **Never submit forms or place orders on third-party shops** (`specs/05-crawler.md`). SSRF rules are mandatory for any outbound request.
7. **Honesty:** never describe a score or result as "compliant". Use the disclaimer from `specs/01-legal-scope.md`.
8. **Language:** code, comments, specs in English; all user-facing text in German.
9. Secrets only via environment (`deploy/.env`, git-ignored). Never commit API keys.

## Layout

`specs/` · `backend/` (Go module `compliancechecker`) · `frontend/` (Flutter) · `testcorpus/` · `deploy/`

## Commands (once toolchains are installed)

- Backend: `cd backend; go vet ./...; go test ./...`
- Frontend: `cd frontend; flutter analyze; flutter test`
- DB: not used by default (ADR-6). Optional local PostgreSQL 17 (Windows service `postgresql-17`, localhost:5432) via `DATABASE_URL` in `deploy/.env` (ADR-5). Supabase optional. psql: `C:\Program Files\PostgreSQL\17\bin\psql.exe`

## Status

- Installed: Go 1.27, Git, GitHub CLI (logged in as Patrickfaessler), Flutter 3.47.9 (`C:\src\flutter`, on the user PATH since 2026-11-08; old shells still need the PATH line below).
  Also installed 2026-11-08: Google Chrome, Docker Desktop. WSL 2 NOT yet (needs an admin shell: `wsl --install --no-distribution`, then reboot); Docker only works after that.
  Repo: https://github.com/Patrickfaessler/webshop-compliance-checker (public, MIT; axe-core MPL-2.0, see THIRD_PARTY_NOTICES.md). Line endings LF via `.gitattributes`.
- Backend verified: `go vet`, `go test ./...` green. Done: scoring (S-1), rule A6 (verified against the corpus), SSRF address filter (`backend/internal/ssrf`).
  `backend/internal/corpus` runs `testcorpus/cases`; register each new rule in `allRules()` there.
- Browser: `internal/crawler` drives Edge/Chrome/Chromium with **chromedp** (not Playwright, ADR-4; chromedp v0.22 has a generic API: `chromedp.Run/Do/Call/Listen/Evaluate[T]`).
  Browser path from `CHROME_PATH` or auto-detected (Edge is installed on this machine). axe-core 4.10.2 is embedded (`internal/crawler/axe/axe.min.js`).
  Browser tests skip themselves when no browser exists. Without a browser the API falls back to start-page-only scanning.
- Server deploy: `deploy/docker-compose.server.yml` (Traefik + Let's Encrypt; only `DOMAIN`/`ACME_EMAIL` in `deploy/.env`; check./teehaus./radwerk./seifen.<DOMAIN>, DEP-1). Live since 2026-11-08 on vps1-ubuntu (37.27.218.167, Hetzner DNS): https://check.patrick-faessler.dev, shops on teehaus./radwerk./seifen.patrick-faessler.dev; repo in ~/webshop-compliance-checker (branch main), RATE_LIMIT_PER_HOUR=20 in its deploy/.env for testing. ufw needs 80/443 allowed (hairpin for the scanner). API reads `TRUSTED_PROXIES` (C-RL-PROXY).
- Test shops: `testshops/` (generator `gen/`, output `site/`, local server `serve/` on 127.0.0.1:5500). GitHub Pages publish (branch `gh-pages`, `git subtree push --prefix testshops/site origin gh-pages`) is NOT done yet. `internal/e2e` crawls them locally and checks `expected.json`. Scanning them through the local app needs `ALLOW_LOOPBACK_TARGETS=1` + `API_ADDR=127.0.0.1:8080` (C-SSRF-DEV, dev only) and `RATE_LIMIT_PER_HOUR=100`.
- Rules done and verified on the corpus: A1, A2, A3, A4, A6 (register new ones in `cmd/api/main.go` AND `internal/corpus/corpus_test.go`). Missing: A5 and all R rules.
- **Scans are not stored (ADR-6):** in memory for 1 hour (`scan.Retention`), purged every minute; users download `GET /api/scans/{id}/report.pdf` / `report.json` (`internal/report`, fpdf). `STORE=postgres` switches the PostgreSQL store back on.
- PostgreSQL store (off by default): `internal/pgstore` (pgx, migrations in `backend/migrations`, applied on start-up) when `DATABASE_URL` is set, else in-memory. `cmd/api` loads `deploy/.env` (local: role `checker`, DBs `checker` and `checker_test`). Store contract test: `internal/scan/storetest`.
- Windows Smart App Control sometimes blocks a freshly built test binary in %TEMP% ("Application Control policy has blocked this file"). Not a code error: rerun with `GOTMPDIR` set to another folder, e.g. `$env:GOTMPDIR="$env:LOCALAPPDATAgotest-tmp"`.
- API (interim, worker in the same process): `POST /api/scans`, `GET /api/scans/{id}`, `GET /api/scans/{id}/findings`
  with SSRF check, rate limits (3/h, 1 running per hashed IP), host cooldown, 30-day expiry. Run: `cd backend; go run ./cmd/api` (env `API_ADDR`, `CORS_ORIGIN`).
  Live-tested against https://example.com (browser crawl + axe-core through the API). Packages: `api`, `crawler`, `domutil`, `fetch`, `ratelimit`, `scan`, `ssrf`, `worker`, `scoring`, `rules/*`.
- Frontend verified: `flutter analyze`, `flutter test` (20 tests) green. Result page has a download card (PDF, JSON, copy link) via `url_launcher`; `App(onDownload:)` is injectable for tests. Screens: start page (URL + areas, F1), result page at `/r/:id`
  (progress polling every 2 s, score, rule results, findings, disclaimer, copy link; F7, F11, N3). Uses `http` + `go_router`, plain StatefulWidgets (no Riverpod/dio).
  API base via `--dart-define=API_BASE=http://localhost:8080`. Not yet viewed in a real browser. Missing: finding detail, suggestions, re-scan (postponed, ADR-6).
  To try it: backend `$env:CORS_ORIGIN='http://localhost:5000'; go run ./cmd/api`, frontend `flutter run -d chrome --web-port 5000`.
- In a fresh PowerShell, set PATH first: `$env:Path = 'C:\src\flutter\bin;' + [Environment]::GetEnvironmentVariable('Path','Machine') + ';' + [Environment]::GetEnvironmentVariable('Path','User')`.
- Next: A5 (keyboard, CDP key events); fix templates and `GET /findings/{id}` detail view; legal rules R1–R7 with the lexicon/Claude fallback; egress proxy; PostgreSQL store + River queue on the local Postgres.
