# R5 Terms and withdrawal information present

- **Status:** draft · **legal-checked:** no · **Area:** legal · **Severity:** warning
- **Basis:** Directive 2011/83/EU Art. 5, 6(1)(h), 9 and Annex I(A)
- **applies_when:** shop sells to consumers (assumption stated in report; B2B-only shops out of scope)
- **ai_fallback:** yes (locate withdrawal information inside long terms)

Presence only; correctness of the text is not judged.

## Logic
1. Find links to terms (`agb`, `terms and conditions`, `conditions générales`, …) and to withdrawal information (`widerruf`, `withdrawal`, `rétractation`, …), in footer and on checkout.
2. Fetch pages. Withdrawal information may be part of the terms: search headings and text for the 14-day period (`14 Tage`, `14 days`, `quatorze jours`) near withdrawal vocabulary.
3. Check for the model withdrawal form/instructions marker (Annex I) as a hint only, not a requirement.
4. Unclear → Claude fallback with evidence quote.

## Result mapping
| Situation | State |
| --- | --- |
| terms found and withdrawal info with 14-day period found | passed |
| terms missing | failed (`no-terms`) |
| withdrawal info missing | failed (`no-withdrawal-info`) |
| period not found / unclear | not_assessable |
| digital-only or exempt goods suspected (keywords) | note "Ausnahmen vom Widerrufsrecht prüfen" |

## Finding
URLs of terms and withdrawal page (or start page if missing), quote of the period.

## Explanation (German)
Kunden müssen vor dem Kauf lesen können, unter welchen Bedingungen sie kaufen und wie sie innerhalb von 14 Tagen widerrufen können.

## Fix templates
Text block "Widerrufsbelehrung" with placeholders from Annex I(A) of the CRD and a link text suggestion for the footer.

## Acceptance
- R5-1: both present passes. R5-2: no terms link fails. R5-3: withdrawal inside terms is found. R5-4: English fixture passes.
