# R5 Terms and withdrawal information present

- **Status:** implemented (2026-11-21, deterministic; AI fallback not built → not_assessable): unit tests, corpus 3 defect + 3 clean (recall 3/3, 0 false alarms), test shops. Code: `backend/internal/rules/r5`, `backend/internal/legal`, `backend/internal/lexicon`. · **legal-checked:** no · **Area:** legal · **Severity:** warning
- **Basis:** Directive 2011/83/EU Art. 5, 6(1)(h), 9 and Annex I(A)
- **applies_when:** shop sells to consumers (assumption stated in report; B2B-only shops out of scope)
- **ai_fallback:** yes (locate withdrawal information inside long terms)

Presence only; correctness of the text is not judged.

## Logic
Deterministic version (2026-11-21); the AI fallback is not built yet (→ `not_assessable` with a note).

1. Find the terms page (`agb`, `allgemeine geschäftsbedingungen`, `terms`, `terms and conditions`, …) and the
   withdrawal page (`widerruf`, `widerrufsbelehrung`, `rücktritt`, `withdrawal`, `cancellation`, …) among the crawled
   pages like R1. A withdrawal-only link never counts as the terms link.
2. Withdrawal information is searched on the withdrawal page if crawled, else inside the terms: a withdrawal word
   (`widerruf`, `widerrufen`, `rücktrittsrecht`, `withdrawal`, `cancel the contract`, …) within 300 characters of a
   period phrase (`14 Tage(n)`, `vierzehn Tage(n)`, `14 days`, `fourteen days`) in the content text (R1 definition).
   Each page uses its own language (de/en; else `not_assessable`).
3. The Annex I model form is a hint only, not a requirement.

## Result mapping
| Situation | State |
| --- | --- |
| terms found and withdrawal info with 14-day period found | passed |
| no terms link | failed (`no-terms`) |
| no withdrawal word on the withdrawal page or in the terms | failed (`no-withdrawal-info`) |
| withdrawal words but no period | not_assessable, note `Widerrufsfrist nicht gefunden – bitte manuell prüfen.` |
| a linked page that is needed was not crawled | not_assessable, note `Die AGB bzw. die Widerrufsbelehrung sind verlinkt, konnten aber nicht geladen werden.` |
| language not de/en on the page that matters | not_assessable, note `Seite bitte manuell prüfen (Sprache oder Aufbau nicht erkannt).` |

Notes, joined in this order: always `Annahme: Der Shop verkauft an Verbraucher.`; the state note above; exempt-goods
keywords (`download`, `digitale inhalte`, `maßanfertigung`, `versiegelt`, `digital content`) → `Ausnahmen vom
Widerrufsrecht prüfen.` Both failures can occur together, each with its own finding.

## Finding
Page-level: URL of the terms or withdrawal page (or the start page if missing), sub-rule, fix. The quote of the period is
kept as evidence but not shown yet.

## Explanation (German)
Kunden müssen vor dem Kauf lesen können, unter welchen Bedingungen sie kaufen und wie sie innerhalb von 14 Tagen widerrufen können.

## Fix templates
- `no-terms`: text `Lege AGB an und verlinke sie im Footer und im Bestellprozess.`; no code.
- `no-withdrawal-info`: text `Ergänze eine Widerrufsbelehrung mit der 14-tägigen Frist.`; code:
  ```
  Widerrufsbelehrung
  Widerrufsrecht: Sie haben das Recht, binnen vierzehn Tagen ohne Angabe von Gründen diesen Vertrag zu widerrufen.
  Widerrufsfrist: 14 Tage ab dem Tag, an dem Sie oder ein von Ihnen benannter Dritter die Waren in Besitz genommen haben.
  Ausübung: Senden Sie uns ({{Name}}, {{Anschrift}}, {{E-Mail}}) eine eindeutige Erklärung.
  Folgen des Widerrufs: {{Rückzahlung und Rücksendung}}
  ```
  note `Link-Text im Footer: „Widerrufsbelehrung“. Den vollständigen Mustertext finden Sie in Anhang I der Richtlinie 2011/83/EU.`

## Acceptance
- R5-1: both present passes. R5-2: no terms link fails. R5-3: withdrawal inside terms is found. R5-4: English fixture passes.
- R5-5: withdrawal vocabulary without period → not_assessable.
- R5-6: Austrian "Rücktrittsrecht … 14 Tagen" passes.
- R5-7: English terms page on a German shop is read as English.
- R5-F: every finding carries a fix (origin template, non-empty German text).
