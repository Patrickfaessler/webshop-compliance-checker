# 07 Architecture

Status: draft · Flutter (web) frontend and Go backend are **mandatory**.

## Components

```
Flutter Web  ──HTTPS/JSON──▶  Go API  ──SQL──▶  PostgreSQL (scans, results, job queue)
                                                   ▲
                                          Go Worker (rule engine)
                                   ┌───────────────┼────────────────┐
                           Headless Chromium     Claude API     (fallback only)
                           (playwright-go, axe-core)
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
| PostgreSQL | scans, pages, findings, suggestions, rate-limit counters, job queue | `deploy/` |

## Technology choices

| Layer | Choice | Note |
| --- | --- | --- |
| Frontend | Flutter (web), Dart | mandatory |
| State / routing / HTTP | StatefulWidgets, go_router, `http` | in use; Riverpod/dio dropped for now (small app), revisit if state grows |
| Backend | Go ≥ 1.23, `net/http`, chi | mandatory language; libs proposal |
| Queue | River on PostgreSQL | no extra infrastructure |
| DB access | sqlc + pgx | typed SQL |
| Browser | playwright-go + Chromium; axe-core injected | |
| PDF | server side (Go library, decide in M4) | |
| Ops | Docker Compose: postgres, api, worker, chromium, proxy, testshop | |

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

Local: `docker compose -f deploy/docker-compose.yml up`. The same compose file is the reference environment for N1.
