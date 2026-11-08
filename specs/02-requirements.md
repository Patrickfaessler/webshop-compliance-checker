# 02 Requirements

Status column: draft / reviewed / implemented / verified (see `README.md`).

## Functional

| ID | Requirement | Prio | Acceptance criterion | Status |
| --- | --- | --- | --- | --- |
| F1 | Enter a shop URL, choose check areas (accessibility, legal), start a scan. No login. | Must | Invalid or internal URLs are rejected with a German message (C-SSRF); a valid request returns a scan ID and a result link | draft |
| F2 | Crawler visits up to 50 pages incl. product, cart, checkout (checkout only in test shop) | Must | Limits from `05-crawler.md` hold; visited pages are listed in the report | draft |
| F3 | Automatic WCAG checks A1–A6 | Must | Corpus: recall ≥ 90 %, false alarms ≤ 10 % per rule | draft |
| F4 | Provider identification check (R1) | Must | Each missing mandatory field named individually | draft |
| F5 | Privacy notice, cookie banner, trackers before consent (R2–R4) | Must | Tracker requests listed per browser state with domain | draft |
| F6 | Terms, withdrawal information, order button, withdrawal function (R5–R7) | Must | Button label quoted verbatim | draft |
| F7 | Report with score, severity, list of all findings | Must | Score reproducible from rule results (`04-scoring.md`) | draft |
| F8 | Finding detail: explanation, location (URL + selector), screenshot, suggested fix | Must | Fix is copyable; its origin (`template` or `ai`) and evidence are visible | draft |
| F9 | Re-scan from a result page with per-rule comparison | Later | **Postponed (ADR-6):** without stored scans there is nothing to compare; could later compare against an uploaded JSON report | draft |
| F10 | Download the finished report as **PDF** (German) | Must | PDF has URL, date, score with its wording (S-3), all rule results, all findings with location, and the disclaimer from `01-legal-scope.md` on the first page. Raised from Should: the download is the only lasting copy (ADR-6) | draft |
| F11 | Result link `/r/{id}` readable by anyone who has it, while the scan is in memory | Must | ID has ≥ 128 bits of randomness; not listed anywhere; **expires 1 hour after the scan was created (ADR-6)**; the page tells the user until when and to download the report | draft |
| F12 | Abuse protection without accounts | Must | Rate limits per IP and per target host as in `05-crawler.md` | draft |
| F13 | Download the finished report as **JSON** | Must | Versioned format (`format`, `version`) with scan data, rule results and findings incl. selector and snippet; same content as the PDF | draft |

Removed from V2: user accounts, scheduled scans, e-mail notifications (ADR-1). Not stored for now: scan results (ADR-6).

## Non-functional

| ID | Category | Requirement | Measurement | Status |
| --- | --- | --- | --- | --- |
| N1 | Performance | 50 pages in < 5 min **without** Claude calls | Reference env: 4 vCPU, 8 GB RAM, test shop local in Docker | draft |
| N2 | Comprehensibility | German explanations without jargon | Review by 3 non-technical people | draft |
| N3 | Honesty | Disclaimer on report, detail view and PDF | Visual check, widget test | draft |
| N4 | Privacy | Only public pages; no form input on third-party shops; no personal data stored; **scans are never written to disk and are deleted from memory 1 hour after creation (ADR-6)**; IPs only kept transiently (hashed) for rate limiting | Code review, purge test | draft |
| N5 | Extensibility | New rule = new Go package implementing `Rule`, no change to core | Example rule in repo | draft |
| N6 | Security | SSRF protection (`05-crawler.md`); HTTPS; no secrets in repo | Unit tests for address filter | draft |
| N7 | Accessibility of the tool | The Flutter UI passes A1–A6 | Self-scan in CI | draft |
| N8 | Quality | Backend coverage ≥ 70 %; CI runs all tests on each push | `go test -cover`, CI | draft |
| N9 | AI minimisation | Claude is called only on fallback (D-rules); share of findings needing Claude is reported | Metric in corpus run | draft |
