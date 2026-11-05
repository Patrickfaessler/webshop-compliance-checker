# ADR-1 No user accounts

Status: accepted (2026-11-05)

**Decision.** There are no accounts, logins or passwords. A scan is addressed by an unguessable 128-bit ID (`/r/{id}`); anyone with the link can read it. Like "Have I Been Pwned": enter, look up, done.

**Why.** Less to build and secure, no personal data to protect (GDPR), lower barrier for small shop owners.

**Consequences.**
- Abuse protection relies on rate limits per IP and per target host (`05-crawler.md`).
- Results expire after 30 days (F11, N4).
- "Mark as done" (F9) lives in the browser (local storage); the server stores nothing per person.
- Re-scan comparison works through `previousScanId` from the result page.
- Dropped: scheduled scans, e-mail notifications, agency multi-shop dashboards. Would need a new ADR.
