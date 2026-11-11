# 07 Architecture

Status: draft · Flutter (web) frontend and Go backend are **mandatory**.

## Components

```
Flutter Web  ──HTTPS/JSON──▶  Go API  ──SQL──▶  PostgreSQL, local or Supabase (scans, results, job queue; ADR-5)
                                                   ▲
                                          Go Worker (rule engine)
                                   ┌───────────────┼────────────────┐
                           Headless Chromium     Claude API     (fallback only)
                           (chromedp, axe-core; ADR-4)
                                   │
                              Egress proxy ──▶ public internet (private ranges blocked)
```

| Component | Responsibility | Dir |
| --- | --- | --- |
| Flutter Web app | URL form, progress, report, finding detail, PDF download. No login. | `frontend/` |
| Go API | validates URL (C-SSRF), rate limits (C-RL), creates scan job, serves status/results/PDF | `backend/cmd/api` |
| Go worker | takes jobs, crawls, runs rules, computes score, stores results | `backend/cmd/worker` |
| Rule modules | one package per rule implementing `rules.Rule` | `backend/internal/rules/<id>` |
| Lexicon | per-language term lists | `backend/internal/lexicon` |
| Scoring | pure functions from `04-scoring.md` | `backend/internal/scoring` |
| LLM client | Claude fallback behind interface | `backend/internal/llm` |
| PostgreSQL 17 (ADR-5) | scans, pages, findings, suggestions, rate-limit counters, job queue. Local by default, Supabase optional; only the Go backend connects (`DATABASE_URL`) | `backend/migrations/` |

## Technology choices

| Layer | Choice | Note |
| --- | --- | --- |
| Frontend | Flutter (web), Dart | mandatory |
| State / routing / HTTP | StatefulWidgets, go_router, `http` | in use; Riverpod/dio dropped for now (small app), revisit if state grows |
| Backend | Go ≥ 1.23, `net/http`, chi | mandatory language; libs proposal |
| Database | Local PostgreSQL 17 (native or Docker), Supabase EU optional (ADR-5) | plain Postgres: no lock-in; in-memory store without `DATABASE_URL` |
| Queue | River on PostgreSQL | no extra infrastructure; needs the session pooler, not the transaction pooler |
| DB access | sqlc + pgx | typed SQL |
| Browser | chromedp + Edge/Chrome/Chromium; axe-core injected | ADR-4 |
| PDF | server side, `github.com/go-pdf/fpdf` (ADR-6) | pure Go; cp1252 for umlauts |
| Ops | Docker Compose: api, worker, chromium, proxy, testshop (+ optional `localdb` Postgres) | database: native local Postgres or Supabase |

## Rule interface (contract, N5)

```go
type Rule interface {
    ID() string                              // "A1", "R6"
    Meta() Meta                              // severity, areas, ai_fallback allowed
    Check(ctx context.Context, in Input) (Result, error)
}
```

`Input` carries rendered pages, requests per browser state and an optional `llm.Client`. `Result` carries state
(`passed|failed|not_applicable|not_assessable`), findings (URL, selector, snippet, evidence) and `decided_by`
(`structural|dictionary|pattern|ai`). Exact types: `backend/internal/rules/rule.go`.

## Data model (no user table, ADR-1)

> ADR-6: for now scans are **not stored**; they live in memory for 1 hour (`scan.MemoryStore`). The tables below are the
> PostgreSQL store (ADR-5), used only with `STORE=postgres`.

| Table | Fields |
| --- | --- |
| scans | id (random 128-bit, URL-safe), host, start_url, areas, status (`queued|running|done|incomplete|failed`), score, rule_set_version, previous_scan_id, created_at, finished_at, expires_at |
| pages | id, scan_id, url, http_status, screenshot_path |
| rule_results | scan_id, rule_id, state, decided_by, pages_affected |
| findings | id, scan_id, page_id, rule_id, severity, selector, snippet, evidence, fingerprint |
| suggestions | finding_id, origin (`template|ai`), text, model, prompt_version, created_at |
| rate_limits | key (hashed IP or host), window_start, count — expires quickly |

IP addresses are stored only hashed with a rotating salt and only for rate limiting (N4).

## Deployment

Database: `DATABASE_URL` in `deploy/.env` (see `deploy/.env.example`): local PostgreSQL by default, Supabase optional (ADR-5).
Local services: `docker compose -f deploy/docker-compose.yml up` (test shop, later proxy). Offline database: add `--profile localdb`.
The compose file plus a local PostgreSQL 17 is the reference environment for N1.

### Server deployment (DEP-1)

`deploy/docker-compose.server.yml` runs the whole stack on one server behind Traefik with automatic Let's Encrypt
certificates. The operator sets only `DOMAIN` (and `ACME_EMAIL`) in `deploy/.env`; everything else is derived:

| Host | Service |
| --- | --- |
| `check.<DOMAIN>` | Flutter web app; `check.<DOMAIN>/api/*` goes to the Go API (same origin, no CORS) |
| `teehaus.<DOMAIN>`, `radwerk.<DOMAIN>`, `seifen.<DOMAIN>` | the three test shops (T-3), one per subdomain |

- The API container includes Chromium (`CHROME_NO_SANDBOX=1`) and is not published on a host port; only Traefik is.
- DEP-2: the web app built with an empty `API_BASE` calls the API on its own origin.
- Scans stay in memory (ADR-6); no database container is needed. `ALLOW_LOOPBACK_TARGETS` is never set (C-SSRF-DEV).
- Rate limits use the real client IP from Traefik (C-RL-PROXY).
