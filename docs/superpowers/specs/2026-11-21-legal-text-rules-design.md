# Legal text rules R1, R2, R5: design

Date: 2026-11-21. Milestone M4 (`specs/10-roadmap.md`), requirements F4–F6, pipeline D-1 (`specs/06-detection-and-ai.md`).
The rule specs `specs/rules/R1-provider-identification.md`, `R2-privacy-notice.md` and `R5-terms-and-withdrawal-info.md`
are the source. This design settles what they leave open. Where it changes them, the spec is updated first.

## Intent

A shop owner should learn whether the basic legal pages of the shop are there and contain the most basic items:
- **R1 Impressum:** name, address, e-mail;
- **R2 Datenschutzerklärung:** the six GDPR Art. 13 sections;
- **R5 AGB and Widerrufsbelehrung:** with the 14-day period.

These are the first legal rules. They are the cheapest ones because they only read text. R3/R4 (cookies, trackers)
and R6/R7 (buttons) follow in separate projects; the user chose this order.

## Goals

- R1, R2 and R5 run in the area "legal" (already in the form, the API and the scoring weights). Each gives a result
  with findings, a German note and fix templates, like A1–A6.
- A shared German and English lexicon and shared validators that R3–R7 can reuse later.
- Corpus cases per rule (T-1: at least 3 with a defect and 3 clean) and test-shop expectations for R1, R2 and R5.

## Non-goals

- **No Claude fallback** (the user decided: later). Wherever the spec says "Claude fallback", the rule returns
  `not_assessable` with a German note saying what to check by hand. `ai_fallback: yes` stays in the specs for later.
- No judgement of whether the legal text is *correct* (R2 and R5 check presence only, as the specs say).
- No R3, R4, R6, R7. No languages beyond German and English (other languages → `not_assessable`).
- No extra page fetches (approach 1). No national law beyond the EU scope (ADR-2). No rule is marked `verified`;
  `legal-checked` stays `no` (CLAUDE.md rule 5, open question "who reviews").

## Anti-goals

- **A false "fehlt" is worse than "nicht prüfbar".** Telling a shop owner that their Impressum lacks an address when it
  has one destroys trust and may cost them a lawyer's hour. The rules report a *missing* item only when they are
  confident. Every doubtful case is `not_assessable` with a note.
- Never "konform", "rechtssicher" or similar (S-3, CLAUDE.md rule 7). A `passed` legal result always says that only
  the presence was checked.
- Page text is untrusted. Quotes from the page are shown as text only, escaped in the HTML report.

## Small crawler additions *(my call, after the builder check)*

Approach 1 stays: rules read only crawled pages, with no extra fetches and no pages beyond the limit. Two small
additions make "the link is there but the page was not loaded" rarer and keep it honest:
- **Priority by link text:** `priorityWords` today matches only the link URL. Links whose *text* matches (e.g.
  "Impressum" → `/p/123`) are queued first too. The crawler collects the anchor text alongside `href`.
- **Redirects:** `rules.Page` gains `RequestedURL` (the URL the crawler asked for, before redirects). `FindLinked`
  matches a link against `URL` or `RequestedURL`, ignoring a trailing slash and the fragment.

## Constraints

- Spec → test → code; every test cites `// Spec: R1-x` etc. New behaviour gets new acceptance IDs in the rule spec first.
- User-facing text German, code English. Rules are pure functions of `rules.Input` (no network), like A1–A6.
- The crawler already visits legal links first (`crawler/crawl.go` `priorityWords`, max 50 pages, depth 3).
- Register new rules in `cmd/api/main.go`, `internal/corpus/corpus_test.go` `allRules()` and the e2e lists in
  `internal/e2e` (CLAUDE.md).

## Approaches (picked by the user: 1)

1. **Rules read only the pages the crawler already loaded (chosen).** A rule finds the link by lexicon on any crawled
   page, resolves it and evaluates the target page from `Input.Pages`. If the link exists but the page was not loaded
   (robots.txt, error, page limit), the result is `not_assessable`, not "fehlt". Rules stay pure and are easy to test
   with corpus cases.
2. **Rules fetch the legal pages themselves** through the SSRF-safe fetcher. Rejected: double loading, rules need
   network access and SSRF handling, tests get heavier.
3. **The crawler guarantees the legal pages** beyond the page limit. Rejected: special logic for little gain, because
   the crawler already loads these links first.

## Design

### Packages

- `backend/internal/lexicon`: one file per language (`de.go`, `en.go`), plain Go data. Contents:
  - link words per page kind: legal notice, privacy, terms, withdrawal;
  - R2 section markers (six sections);
  - withdrawal vocabulary and period phrases (`14 Tage`, `vierzehn Tage`, `14 days`, `fourteen days`);
  - exempt-goods keywords (R5 note);
  - field labels for R1 (`E-Mail`, `Anschrift`, `Inhaber`, `Geschäftsführer`, `UID`, `USt-IdNr.`, `Firmenbuch`,
    `Handelsregister`, `VAT`, `Registered office` …).
  - `Language(page) (lang string, ok bool)` picks the list to use, in this order *(my call, after the builder check:
    Impressum pages are terse lists and often score below the A6 detector's "sure" threshold)*:
    1. `fixes.DetectLang` on the page text when it is sure;
    2. else `<html lang>` of the page when it is de or en;
    3. else the language whose lexicon (field labels, section markers, link words) has more hits on the page, with at
       least 2 hits.
    `ok = false` (→ `not_assessable`) only if all three fail or the result is not de/en.
- `backend/internal/legal`: shared helpers.
  - `FindLinked(in rules.Input, kind lexicon.Kind) Link`: scans `<a href>` on all pages (footer and header first, then
    the rest). It matches link text or URL path against the lexicon, resolves relative URLs, strips the fragment and
    picks the target in `Input.Pages`. It returns the link URL, whether the target page was loaded, and that page.
    When the link is missing, a crawled page whose URL path matches the lexicon also counts (e.g. `/impressum`
    reached from a sitemap). *(My call.)*
  - `Text(page) string`: the page's **content text** *(my call, after the builder check)*. This is visible text
    (`domutil.VisibleText`) without `<header>`, `<nav>`, `<footer>`, `<aside>` and `<form>` subtrees, whitespace
    collapsed. If that leaves nothing, it falls back to the whole visible text. This keeps footer words (newsletter
    "E-Mail", footer links "Widerrufsbelehrung") out of R1 fields, R2 counts, R5 withdrawal search and the length
    thresholds. Link discovery (`FindLinked`) still uses the whole page.
  - Validators: `Emails(text, doc)`, `VATNumbers(text)`, `PostalAddress(text)`, `LegalFormName(text)`,
    `RegisterNumber(text)`. Each returns found / not found / unsure plus the quote.
- `backend/internal/rules/r1`, `r2`, `r5`: one package per rule, as for A1–A6.

### R1 Impressum

1. Find the legal-notice page (`FindLinked`, kind `legal_notice`).
   - No link and no matching page: `failed`, sub `no-legal-notice-page`, finding on the start page.
   - Link but page not loaded: `not_assessable`, note `Das Impressum ist verlinkt, konnte aber nicht geladen werden.`
2. Language not de/en, or fewer than 200 characters of text, gives `not_assessable` with the note
   `Impressum bitte manuell prüfen (Sprache oder Aufbau nicht erkannt).` *(My call: 200.)*
3. Fields:
   - **E-Mail:** a `mailto:` link, or a syntactically valid address in the text. Common obfuscations (`info (at) shop.at`,
     `info[at]shop.at`, `info @ shop . at`) also count as found. *(My call: the address is reachable for a human.)*
     - No address, but an image or a "E-Mail" label with no address after it in the content text: unsure (the
       address may be an image).
     - Otherwise missing.
   - **Anschrift** *(my call, after the builder check: EU postal codes vary, and a false "fehlt" is the top anti-goal)*:
     - found: a street line with a house number (`Hauptstraße 12`, `12 High Street`, `Gasse 3/4`) **and** a postal-code
       token. Postal codes are matched from a table of EU formats (4–5 digits with optional `A-`/`D-`/`AT-` prefix;
       `00-950` PL; `1000-001` PT; `114 55` SE/CZ/SK/GR; `1012 AB` NL; Irish Eircode; Maltese `VLT 1117`; UK-style
       for English pages);
     - unsure: only one of the two, a PO box alone (`Postfach`, `PO Box`, not a geographic address), or the label
       `Anschrift`/`Adresse`/`Address` with text that matches neither pattern;
     - missing: **only** when there is neither a postal-code token, nor a street line with a number, nor an address
       label.
   - **Name:** a legal form suffix (GmbH, AG, KG, OG, e.U., e.K., UG, GmbH & Co. KG, Ltd, LLC, SARL, B.V., S.L. …), or
     a label (`Inhaber`, `Betreiber`, `Firmenname`, `Medieninhaber`, `Owner`, `Company name`) followed by text.
     - Otherwise **unsure, never missing.** A sole trader's plain name cannot be told from other text.
4. Conditional fields (note only, never `failed`):
   - **VAT/UID:** valid formats for all EU member states plus XI (e.g. `ATU12345678`, `DE123456789`), with spaces
     allowed. A malformed number is not counted (R1-4).
   - **Register:** `FN 123456a`, `HRB 12345`, `HRA …`, `Company number`.
   - When either is absent, the note says `UID-Nummer bzw. Firmenbuchnummer prüfen, ob zutreffend.`
5. Result:
   - any mandatory field missing → `failed`, one finding per missing field (sub `missing-email`, `missing-address`,
     `missing-name`);
   - else any unsure → `not_assessable`, the note names the unsure fields;
   - else `passed`, note `Nur geprüft, ob die Angaben vorhanden sind.` plus the conditional-field note.

### R2 Datenschutzerklärung

1. Find the privacy page (kind `privacy`).
   - No link: `failed`, sub `no-privacy-page`.
   - Link but not loaded: `not_assessable`.
   - Page text ≤ 500 characters: `failed`, sub `incomplete-sections` with all six missing (spec: "page reachable,
     > 500 characters").
2. Language not de/en gives `not_assessable`.
3. Six markers are searched in headings first, then in the text:
   - controller (`Verantwortlicher`, `Verantwortliche Stelle`, `controller`);
   - purposes and legal basis (`Zweck`, `Rechtsgrundlage`, `Art. 6`, `purpose`, `legal basis`);
   - recipients (`Empfänger`, `Weitergabe`, `Auftragsverarbeiter`, `recipients`, `third parties`);
   - retention (`Speicherdauer`, `gespeichert … bis`, `Löschung`, `retention`, `how long`);
   - rights (`Auskunft`, `Berichtigung`, `Löschung`, `Widerspruch`, `Datenübertragbarkeit`, `your rights`, `access`,
     `erasure`);
   - complaint (`Beschwerde`, `Aufsichtsbehörde`, `Datenschutzbehörde`, `supervisory authority`, `lodge a complaint`).
   A marker needs one heading hit or two text hits. *(My call; tuned on the corpus.)*
4. Result:
   - ≥ 5 of 6 → `passed`, note `Nur geprüft, ob die Abschnitte vorhanden sind, nicht ob der Inhalt stimmt.`;
   - fewer → `failed`, sub `incomplete-sections`, one finding listing the missing sections in German.

### R5 AGB und Widerrufsbelehrung

1. Find the terms page (kind `terms`) and the withdrawal page (kind `withdrawal`).
   - No terms link: `failed`, sub `no-terms`.
   - A link that was not loaded: `not_assessable`.
2. Withdrawal information is searched on the withdrawal page, or else inside the terms. A withdrawal word (`Widerruf`,
   `widerrufen`, `Rücktrittsrecht`, `withdrawal`, `cancel the contract`) must appear within 300 characters of a period
   phrase. The quote is kept as evidence.
   *(My call: Austria calls it `Rücktrittsrecht` in FAGG wording, so it is in the lexicon.)*
3. Result:
   - terms present and withdrawal info with the period → `passed`;
   - withdrawal vocabulary present but no period → `not_assessable`, note `Widerrufsfrist nicht gefunden – bitte
     manuell prüfen.`;
   - no withdrawal vocabulary anywhere (withdrawal page and terms) → `failed`, sub `no-withdrawal-info`.
   - Both failures can occur together; each gets its own finding.
4. Notes:
   - Always add `Annahme: Der Shop verkauft an Verbraucher.` (spec applies_when, B2C assumption).
   - Exempt-goods keywords (`Download`, `digitale Inhalte`, `Maßanfertigung`, `versiegelt`, `digital content`) add
     `Ausnahmen vom Widerrufsrecht prüfen.`
   - The Annex I model form is a hint only and never required.

### Findings, position and fixes

- Findings are page-level. The finding URL is the legal page, or the start page when the page is missing. There is no
  selector, and `Position` is `page` so the report says "Betrifft die ganze Seite". The quote, if any, goes to
  `Finding.Evidence` (`scan.Finding` already has it). It is **not** shown in the report, API or app in this round
  *(my call, after the builder check)*. Findings are for missing items, which have nothing to quote. Showing quotes
  comes with the AI fallback, where D-5 requires them. An empty selector already becomes position `page`
  (`crawler/capture.go`).
- Fix templates (in the specs first, German). `Fix.Code` is a plain-text block, never filled with page values, so no
  escaping question arises:
  - **R1:** text `Ergänze die fehlenden Angaben im Impressum.` The code block holds the lines for the missing fields
    (`Firmenname: {{Firmenname}}`, `Anschrift: {{Straße Nr., PLZ Ort}}`, `E-Mail: {{E-Mail}}`). The note says
    `Je nach Land und Rechtsform können weitere Angaben nötig sein (z. B. UID, Firmenbuch).`
    For `no-legal-notice-page`, the text is `Lege eine Seite „Impressum“ an und verlinke sie im Footer jeder Seite.`
  - **R2:** text `Ergänze die fehlenden Abschnitte der Datenschutzerklärung.` The code is a checklist of the missing
    sections. The note says `Ein Datenschutz-Generator oder eine Fachperson kann helfen.` (no external link).
  - **R5:** for `no-terms`, `Lege AGB an und verlinke sie im Footer und im Bestellprozess.`. For
    `no-withdrawal-info`, the text is `Ergänze eine Widerrufsbelehrung mit der 14-tägigen Frist.` with a code skeleton
    following CRD Annex I(A) headings (`Widerrufsrecht`, `Widerrufsfrist: 14 Tage ab …`, `Folgen des Widerrufs`) with
    placeholders and the link text suggestion `Widerrufsbelehrung`.
- `Meta.ExplanationDe` comes from each spec's "Explanation (German)".

### Spec updates (before code)

- R1, R2, R5 Logic: the confident-missing vs unsure split, link-not-loaded → `not_assessable`, the language limit,
  "fallback → not_assessable with note until the AI fallback exists".
- New acceptance IDs:
  - R1-6: obfuscated e-mail found;
  - R1-7: e-mail only as an image → `not_assessable`;
  - R1-8: name without a legal form → not `failed`;
  - R1-9: link not loaded → `not_assessable`;
  - R2-5: ≤ 500 chars fails;
  - R5-5: no period → `not_assessable`;
  - R5-6: Austrian `Rücktrittsrecht` wording passes;
  - `R1-F`/`R2-F`/`R5-F`: every finding carries a fix.
- R1-5 (fake LLM) is marked "deferred with the AI fallback".
- `specs/06-detection-and-ai.md`: a note that the fallback is not built yet and returns `not_assessable`.
- Status lines updated after implementation, `legal-checked: no` unchanged.

### Tests

- **Unit tests** per acceptance ID. Validator tables:
  - VAT for every EU country, valid and malformed;
  - e-mail obfuscations;
  - AT/DE/CH addresses with and without a street, PO box;
  - legal forms.
- **Corpus** (`testcorpus/cases/r1-*`, `r2-*`, `r5-*`): multi-page cases (`index.html` with a footer link plus the legal
  page). At least 3 defect and 3 clean cases per rule, including an English shop and an Austrian Impressum. The corpus
  test checks every finding has a fix.
- **Test shops:** `expected.json` gains R1, R2 and R5.
  - Teehaus: all `passed`.
  - Radwerk: R1 `failed` (e-mail), R2 `failed`, R5 `failed` (no withdrawal).
  - Seifen: all `passed`.
  - The expected scores are recomputed. The generator (`testshops/gen`) is the source; the builder checks that
    Radwerk's `agb.html` really lacks a 14-day withdrawal period, otherwise the planted defect is adjusted in the
    generator.
- Report e2e (`internal/e2e/report_test.go`, RP-A4) stays on the accessibility area *(my call)*. Legal findings are
  page-level with no frame, and R3/R6/R7 defects have no rule yet. A separate e2e check covers the R1/R2/R5 states of
  the test shops. Report self-scan stays green. The app needs no change: legal results flow through the existing result and findings
  views. The builder checks that the R names render (`ruleNameDe`).

## Left to the builder

- Exact lexicon lists within the examples above, and the exact German wording within the specs.
- The 200-character and two-hit thresholds, as long as the corpus is met (recall on defects, 0 false "failed" on clean
  cases).
- How `Link` and the validators are shaped inside `internal/legal`.
- Whether R2/R5 reuse one generic "find section by markers" helper.
