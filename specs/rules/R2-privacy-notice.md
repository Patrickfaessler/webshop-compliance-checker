# R2 Privacy notice present

- **Status:** implemented (2026-11-21, deterministic; AI fallback not built → not_assessable): unit tests, corpus 3 defect + 3 clean (recall 3/3, 0 false alarms), test shops. Code: `backend/internal/rules/r2`, `backend/internal/legal`, `backend/internal/lexicon`. · **legal-checked:** no · **Area:** legal · **Severity:** warning
- **Basis:** Regulation (EU) 2016/679 Art. 13 (information to data subjects)
- **applies_when:** every shop
- **ai_fallback:** yes (section detection only)

Presence only. Whether the content is legally correct is **not** judged; such cases are explicitly `not_assessable` with the note "inhaltliche Prüfung nicht möglich".

## Logic
Deterministic version (2026-11-21); the AI fallback is not built yet (→ `not_assessable` with a note).

1. Find the privacy page among the crawled pages like R1 (lexicon `datenschutz`, `datenschutzerklärung`, `privacy`,
   `privacy policy`, …). No link → `failed` (`no-privacy-page`). Link but page not crawled → `not_assessable`.
2. Language as in R1 (de/en only, else `not_assessable`); it is detected on the content text first, so a German menu
   around an English policy does not decide it. Content text as in R1 (without header/nav/footer/aside/forms).
3. Content text ≤ 500 characters → `failed` (`incomplete-sections`) with all six sections missing.
4. Six section markers, each with de/en words: (1) `Verantwortlicher` (verantwortlich, verantwortliche stelle,
   controller), (2) `Zwecke und Rechtsgrundlage` (zweck, rechtsgrundlage, art. 6, purpose, legal basis), (3) `Empfänger`
   (empfänger, weitergabe, auftragsverarbeiter, recipients, third parties), (4) `Speicherdauer` (speicherdauer, löschung,
   dauer der speicherung, retention, how long), (5) `Rechte der betroffenen Personen` (auskunft, berichtigung,
   widerspruch, datenübertragbarkeit, your rights, erasure), (6) `Beschwerderecht bei der Aufsichtsbehörde`
   (beschwerde, aufsichtsbehörde, datenschutzbehörde, supervisory authority, lodge a complaint).
   A marker counts with one heading hit (`h1`–`h6`, `dt`, a `p` whose only child is `strong`/`b`; a leading number,
   `§ n` or roman numeral is ignored; case-insensitive) or two hits in the content text.

## Result mapping
| Situation | State |
| --- | --- |
| page reachable and ≥ 5 of 6 section markers | passed, note `Nur geprüft, ob die Abschnitte vorhanden sind, nicht ob der Inhalt stimmt.` |
| no link | failed (`no-privacy-page`) |
| link found, page not crawled | not_assessable, note `Die Datenschutzerklärung ist verlinkt, konnte aber nicht geladen werden.` |
| content ≤ 500 characters, or fewer than 5 markers | failed (`incomplete-sections`), the fix lists the missing sections |
| language not de/en | not_assessable, note `Seite bitte manuell prüfen (Sprache oder Aufbau nicht erkannt).` |

## Finding
Page-level: URL of the privacy page (or the start page if missing), sub-rule, fix with the missing sections.

## Explanation (German)
Wer personenbezogene Daten verarbeitet, muss darüber informieren: wer verantwortlich ist, wozu, wie lange und welche Rechte Kunden haben.

## Fix templates
- `incomplete-sections`: text `Ergänze die fehlenden Abschnitte der Datenschutzerklärung.`; code one line `- <Abschnitt>`
  per missing section in the order above; note `Ein Datenschutz-Generator oder eine Fachperson kann helfen.` (no
  external link).
- `no-privacy-page`: text `Lege eine Datenschutzerklärung an und verlinke sie im Footer jeder Seite.`; code all six
  lines; same note.

## Acceptance
- R2-1: complete fixture passes. R2-2: no link fails. R2-3: page with only a heading fails with missing list. R2-4: English fixture passes via dictionary.
- R2-5: page with ≤ 500 characters fails with all six missing.
- R2-6: numbered/uppercase headings (`1. VERANTWORTLICHER`, `§ 3 Ihre Rechte`) count.
- R2-7: an English policy inside a German header/menu/footer passes.
- R2-8: a policy embedded in an iframe → not_assessable (note as R1).
- R2-9: an off-site policy link plus a "Datenschutz-Einstellungen" `#` link or a blog teaser "Datenschutz im Alltag" → not_assessable, not failed; a `<div class="footer">` link beats a teaser in the main text.
- R2-F: every finding carries a fix (origin template, non-empty German text).
