# R1 Provider identification (legal notice)

- **Status:** implemented (2026-11-21, deterministic; AI fallback not built → not_assessable): unit tests, corpus 3 defect + 3 clean (recall 3/3, 0 false alarms), test shops. Code: `backend/internal/rules/r1`, `backend/internal/legal`, `backend/internal/lexicon`. · **legal-checked:** no · **Area:** legal · **Severity:** critical
- **Basis:** Directive 2000/31/EC Art. 5; Directive 2011/83/EU Art. 6(1)(b)(c)
- **applies_when:** every shop
- **ai_fallback:** yes (field extraction when stages 1–3 are below threshold)

## Input
Crawled pages; legal-notice page if found.

## Logic (D-1 pipeline)
Deterministic version (2026-11-21). The AI fallback (D-2) is not built yet: wherever it would run, the result is
`not_assessable` with a German "bitte manuell prüfen" note.

1. **Find the page** among the crawled pages (no extra fetch): a link whose text or URL path matches the lexicon
   (`impressum`, `anbieterkennzeichnung`, `imprint`, `legal notice`, …), footer/header/nav links first. The target is
   matched against the page URL and the URL requested before redirects (C-2), ignoring fragment and trailing slash.
   A crawled page whose path matches also counts. No link and no such page → `failed` (`no-legal-notice-page`).
   Link found but target not crawled (other domain, robots.txt, error, page limit) → `not_assessable`.
2. **Language** of the legal page: stopword detector when sure, else `<html lang>` if de/en, else the language whose
   lexicon has ≥ 2 hits on the page. Only de and en are supported; otherwise `not_assessable`.
3. **Text:** fields are searched in the content text (visible text without header, nav, footer, aside, forms). A field
   not found there is searched again in the full text of the same page (R1-10), so an e-mail in the page footer counts.
4. **Fields**, each found / unsure / missing:
   - E-mail: `mailto:` link or valid address; obfuscations `info (at) shop.at`, `info[at]shop.at`, `info @ shop . at`
     count as found. An e-mail label followed only by an image → unsure. Otherwise missing.
   - Address: street line with house number **and** a postal code from the EU format table (4–5 digits with optional
     `A-`/`D-`/`AT-` prefix, PL `00-950`, PT `1000-001`, SE/CZ/SK/GR `114 55`, NL `1012 AB`, IE Eircode, MT `VLT 1117`,
     UK-style on English pages) → found. Only one of the two, a PO box alone, or an address label with unmatched text →
     unsure. Missing only when there is no postal code, no street line and no address label.
   - Provider name: legal-form suffix (GmbH, AG, KG, OG, e.U., e.K., UG, GmbH & Co. KG, Ltd, LLC, SARL, B.V., S.L., …)
     or a label (`Inhaber`, `Betreiber`, `Firmenname`, `Medieninhaber`, `Owner`, `Company name`) → found. Otherwise
     unsure, **never missing** (a sole trader's plain name cannot be told from other text).
5. **Conditional:** VAT number (valid format for every EU member state and XI; malformed ones are not counted) and
   register number (`FN 123456a`, `HRB 12345`, `Company number …`). Absent → note only.

## Result mapping
| Situation | State |
| --- | --- |
| no link and no matching page | failed (`no-legal-notice-page`) |
| link found, page not crawled | not_assessable, note `Das Impressum ist verlinkt, konnte aber nicht geladen werden.` |
| language not de/en | not_assessable, note `Seite bitte manuell prüfen (Sprache oder Aufbau nicht erkannt).` |
| a mandatory field missing | failed, one finding per missing field (`missing-email`, `missing-address`, `missing-name`) |
| no field missing, a field unsure | not_assessable, note `Bitte manuell prüfen: {{Felder}}.` (Firmenname, Anschrift, E-Mail) |
| all three found | passed, note `Nur geprüft, ob die Angaben vorhanden sind.` |
| VAT or register absent (any state) | note `UID-Nummer bzw. Firmenbuchnummer prüfen, ob zutreffend.`, does not fail |

## Finding
Page-level (no selector): URL of the legal notice (or the start page if missing), sub-rule, fix. The quote of a found
field is kept as evidence but not shown yet (it is shown once the AI fallback exists, D-5).

## Explanation (German)
Online-Shops müssen zeigen, wer sie betreiben und wie man sie erreicht: Name, Anschrift und E-Mail-Adresse. Das gehört gut auffindbar auf eine eigene Seite.

## Fix templates
- Missing fields: text `Ergänze die fehlenden Angaben im Impressum.`; code (plain text, one line per missing field, in
  this order): `Firmenname: {{Firmenname}}`, `Anschrift: {{Straße Nr., PLZ Ort}}`, `E-Mail: {{E-Mail}}`; note
  `Je nach Land und Rechtsform können weitere Angaben nötig sein (z. B. UID, Firmenbuch).`
- `no-legal-notice-page`: text `Lege eine Seite „Impressum“ an und verlinke sie im Footer jeder Seite.`; code all three
  lines; same note.

## Acceptance
- R1-1: page with all three fields passes. R1-2: missing e-mail fails naming "E-Mail". R1-3: no legal link fails. R1-4: German VAT `DE123456789` recognised; malformed rejected. R1-5: free-form page in an unknown layout needs fallback with fake LLM and evidence quote — **deferred with the AI fallback**.
- R1-6: obfuscated e-mail (`info (at) shop.at`, `info[at]shop.at`) is found.
- R1-7: e-mail label with only an image → not_assessable.
- R1-8: name without a legal form → not failed.
- R1-9: legal link present but page not loaded → not_assessable.
- R1-10: e-mail only in the legal page's footer counts.
- R1-F: every finding carries a fix (origin template, non-empty German text).
