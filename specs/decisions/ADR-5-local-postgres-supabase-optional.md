# ADR-5 Local PostgreSQL by default, Supabase optional

Status: accepted (2026-11-08, revised the same day: local first)

**Decision.** Development uses a **local PostgreSQL 17** on each machine (native Windows install on `localhost:5432`,
or the Docker `localdb` profile where virtualisation works). A **Supabase**-hosted PostgreSQL (EU region) is a
supported alternative when the team wants one shared database, e.g. for a demo. The backend only sees `DATABASE_URL`,
so switching is one line in `deploy/.env`. Without `DATABASE_URL` the backend uses the in-memory store.

**Why local first.** The app must run fully on one device, offline, with no account or cloud project. Docker is not an
option on every machine yet (virtualisation is off on one of them), so a native install is the default. Supabase is plain
PostgreSQL, so nothing in the code depends on which one is used.

**Rules (both variants).**
- Only the Go backend connects to the database. The Flutter app never gets a database string or Supabase key.
- No user tables, no Supabase Auth (ADR-1).
- Schema changes are versioned SQL migrations in `backend/migrations/`, applied by the backend on start-up.
- Secrets (`DATABASE_URL`, passwords) only in `deploy/.env` (git-ignored) or the environment.
- The backend uses its own role (`checker`), not the superuser.
- DB tests use `TEST_DATABASE_URL`, a separate database (`checker_test`), because they truncate tables.

**Supabase specifics (only when used).** EU region; connect via the **session pooler** (IPv4; the River queue needs
session mode, not the transaction pooler on port 6543); Row Level Security on every table with no policies so the public
`anon` key can read nothing; free projects pause after inactivity (about a week), so resume before a demo.

**Consequences.**
- Each developer has their own data; results are not shared unless Supabase is used.
- Docker is still needed later for the WooCommerce test shop (M1) and the egress proxy.
- Setup: `winget install PostgreSQL.PostgreSQL.17`, then create role and databases (see `README.md`).
