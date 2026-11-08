# ADR-6 No stored scans for now: show the result, offer a download

Status: accepted (2026-11-08)

**Decision.** Scan results are **not stored**. A scan lives in memory while it runs and for **1 hour after it was
created**, so the result page can poll it, be reloaded and offer downloads. Then it is deleted. The user keeps the result
by downloading it as **PDF** (people) and **JSON** (machine-readable). Nothing is written to a database or to disk.

**Why.** Simplest and most private option for now: no database to run, no data to protect or delete later, and the user
owns their copy. The PostgreSQL store from ADR-5 stays in the code, switched off, so storing can return later without
redesign.

**Consequences.**
- F11: the result link works for 1 hour only; the page says until when and asks to download. A backend restart also
  ends all results (acceptable: they were never promised beyond the download).
- F10 (PDF) becomes Must, F13 (JSON) is new. F9 (re-scan comparison) is postponed; it could later compare a new scan with
  an uploaded JSON report.
- N4: retention is 1 hour in memory instead of 30 days; expired scans are purged every minute.
- `cmd/api` uses the in-memory store. The PostgreSQL store is used only if `STORE=postgres` is set explicitly, so a
  `DATABASE_URL` left in `deploy/.env` does not start storing scans by accident.
- The host cooldown (C-RL, 10 minutes) still works, because the result stays in memory long enough.
- Several API instances would not share results; fine for the Praktikum, revisit before any real deployment.
- PDF is built in the Go backend with `github.com/go-pdf/fpdf` (pure Go, no external tools); umlauts via cp1252.
