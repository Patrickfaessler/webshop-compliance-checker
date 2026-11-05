# R1 Provider identification (legal notice)

- **Status:** draft · **legal-checked:** no · **Area:** legal · **Severity:** critical
- **Basis:** Directive 2000/31/EC Art. 5; Directive 2011/83/EU Art. 6(1)(b)(c)
- **applies_when:** every shop
- **ai_fallback:** yes (field extraction when stages 1–3 are below threshold)

## Input
Crawled pages; legal-notice page if found.

## Logic (D-1 pipeline)
1. **Find the page:** footer/header link whose text or URL matches the lexicon (`impressum`, `legal notice`, `imprint`, `mentions légales`, `aviso legal`, …) → fetch. None found → state `failed` (`no-legal-notice-page`).
2. **Extract fields** by regex/validators: provider name (legal form suffix patterns such as GmbH, AG, e.U., Ltd, SARL, B.V.), geographic address (street + postal code + city heuristic), e-mail (syntax), trade register + number, VAT number (country-specific formats).
3. **Mandatory fields:** name, address, e-mail. **Conditional:** register number, VAT number (see `applies_when` below).
4. Confidence < 0.8 for a field → Claude fallback (D-2), must quote evidence (D-5). Still unsure → field `not_assessable`.

## Result mapping
| Situation | State |
| --- | --- |
| page found, name + address + e-mail found | passed (with note on conditional fields) |
| page missing or a mandatory field missing | failed, each missing field its own finding |
| mandatory fields uncertain after fallback | not_assessable |
| register / VAT absent | note "prüfen, ob zutreffend", does not fail |

## Finding
URL of legal notice (or start page if missing), field name, quote if found.

## Explanation (German)
Online-Shops müssen zeigen, wer sie betreiben und wie man sie erreicht: Name, Anschrift und E-Mail-Adresse. Das gehört gut auffindbar auf eine eigene Seite.

## Fix templates
Template text block with placeholders for the missing fields (`{{Firmenname}}`, `{{Anschrift}}`, `{{E-Mail}}`, optional `{{Handelsregisternummer}}`, `{{USt-IdNr.}}`) and a note that national law may require more.

## Acceptance
- R1-1: page with all three fields passes. R1-2: missing e-mail fails naming "E-Mail". R1-3: no legal link fails. R1-4: German VAT `DE123456789` recognised; malformed rejected. R1-5: free-form page in an unknown layout needs fallback with fake LLM and evidence quote.
