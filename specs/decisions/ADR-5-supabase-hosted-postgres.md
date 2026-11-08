# ADR-5 Supabase as hosted PostgreSQL

Status: accepted (2026-11-08)

**Decision.** The database is a hosted PostgreSQL on **Supabase** (EU region, e.g. Frankfurt) instead of a local
PostgreSQL container. Supabase is used **only as a Postgres database** (and later possibly Storage for screenshots).
The Go backend is its only client.

**Why.** Both team members share one database without installing Docker/WSL; the Supabase dashboard makes scans and
results easy to inspect; the free tier is enough for the Praktikum. It stays plain PostgreSQL, so `pgx`, `sqlc`, River
and the data model in `07-architecture.md` are unchanged and we can move back to self-hosted Postgres at any time.

**Rules.**
- **Only the Go backend talks to Supabase.** The Flutter app never gets a Supabase key and never calls Supabase directly;
  otherwise SSRF checks, rate limits and rule logic would be bypassed.
- **No Supabase Auth**, no user tables (ADR-1). Row Level Security stays enabled on every table with no policies, so the
  public `anon` key can read nothing; the backend connects as the database owner through `DATABASE_URL`.
- **Connection:** the backend uses the **session pooler** connection string (IPv4; the free direct connection is IPv6-only).
  River's job queue needs session mode, not the transaction pooler (port 6543).
- **Secrets:** `DATABASE_URL` lives in `deploy/.env` (git-ignored) or the environment, never in the repository or chat.
- **Region:** an EU region (N4, GDPR). No personal data is stored anyway (ADR-1, N4).
- **Schema changes** are versioned SQL migrations in `backend/migrations/`, applied by the backend tooling, not by
  editing tables in the dashboard.

**Consequences.**
- Docker is no longer needed for the database. It is still needed for the WooCommerce test shop (M1) and later for the
  egress proxy and the production image.
- Unit tests keep using the in-memory store. Database tests run only when `TEST_DATABASE_URL` is set (a separate
  Supabase project, a branch, or a local Postgres), so CI and offline work do not depend on Supabase.
- Free projects pause after a period of inactivity (about a week, per Supabase's terms as we know them); before a demo,
  open the dashboard and resume the project.
- Local Docker Postgres remains optional for offline development (`deploy/docker-compose.yml`, profile `localdb`).
