# R7 Withdrawal function ("Widerrufsbutton")

- **Status:** draft · **legal-checked:** **no, verify first** · **Area:** legal · **Severity:** critical
- **Basis:** Directive 2011/83/EU Art. 11a, inserted by Directive (EU) 2023/2673, application date believed to be 19 June 2026. Text and date must be checked against EUR-Lex before this rule is `reviewed` (see `open-questions.md`).
- **applies_when:** consumer contracts concluded via an online interface; not_applicable for catalogue-only
- **ai_fallback:** yes

## Logic
1. Search all crawled pages (header, footer, account/order pages, confirmation page text if reachable) for a prominently placed function or link whose label matches the withdrawal lexicon (`Vertrag widerrufen`, `Widerruf hier`, `withdraw from contract`, `rétracter` …).
2. Check it is reachable without login from a normal page (no account system assumed, ADR-1) and visible (not `display:none`, not in a collapsed menu only).
3. The function must be labelled clearly as withdrawal; a plain "Widerrufsbelehrung" text link is **not** enough.
4. Unclear → Claude fallback with evidence.

## Result mapping
| Situation | State |
| --- | --- |
| clearly labelled withdrawal function visible | passed |
| only an information page, no function | failed (`info-only`) |
| nothing found | failed (`missing`) |
| ambiguous | not_assessable |

## Finding
URL of page, selector and label, or note that none found.

## Explanation (German)
Seit Juni 2026 sollen Kunden einen Vertrag über eine gut sichtbare Schaltfläche „Vertrag widerrufen“ online widerrufen können, nicht nur über eine Textseite.

## Fix templates
Link/button snippet in footer and order confirmation: `<a href="{{withdrawalFormUrl}}">Vertrag widerrufen</a>` plus a note on what the form must collect (to confirm with legal reviewer).

## Acceptance
- R7-1: "Vertrag widerrufen" button in footer passes. R7-2: only "Widerrufsbelehrung" link fails with `info-only`. R7-3: missing fails.
