# 01 Legal scope (EU e-commerce)

Status: draft · **legal-checked: no**

Rules are based on EU-level instruments. Member states transpose directives differently; the tool checks the common EU
minimum and says so in every report ("EU-Ebene, nationale Zusatzpflichten nicht geprüft").

> Citations below are from the team's current knowledge and have **not** been verified by a legal reviewer.
> Before a rule moves to `verified`, its row must be checked against EUR-Lex and the reviewer's name/date recorded here.

## Instruments

| Instrument | Used for | Rules |
| --- | --- | --- |
| Directive 2000/31/EC (E-Commerce Directive), Art. 5 | general provider information (name, address, e-mail, register, VAT no.) | R1 |
| Directive 2011/83/EU (Consumer Rights Directive, CRD), Art. 5, 6 | pre-contract information incl. identity, address, withdrawal right | R1, R5 |
| CRD Art. 9 and Annex I | 14-day withdrawal period, model withdrawal information | R5 |
| CRD Art. 8(2) | order button must state the obligation to pay | R6 |
| CRD Art. 11a (inserted by Directive (EU) 2023/2673, applicable from 19 June 2026) | "withdrawal function" for contracts concluded via an online interface | R7 |
| Regulation (EU) 2016/679 (GDPR), Art. 4(11), 7, 13 | privacy notice, valid consent | R2, R3 |
| Directive 2002/58/EC (ePrivacy), Art. 5(3) | consent before storing/reading non-essential data on the device | R3, R4 |
| EDPB Guidelines 05/2020 on consent | reject option as easy as accept; no pre-ticked boxes | R3 |
| Directive (EU) 2019/882 (European Accessibility Act), applicable since 28 June 2025 | accessibility of e-commerce services | A1–A6 |
| EN 301 549 (harmonised standard), clause 9 → WCAG 2.1 AA | technical criteria | A1–A6 |

## Applicability conditions

Each rule file has an `applies_when` field. Conditions the tool **cannot** decide automatically are reported as
`not_assessable` with an explanation, never silently assumed:

- **EAA micro-enterprise exemption:** service providers that are micro-enterprises (fewer than 10 staff and turnover or balance sheet
  ≤ EUR 2 million, to be verified) are exempt. The tool cannot know this, so the report states it once at the top.
- **B2C only:** CRD rules (R5, R6, R7) apply to consumer contracts. A shop selling only to businesses is out of scope; the report states the assumption.
- **Trade register / VAT number:** required only if the provider has one; R1 reports `not_assessable` for these two fields when absent
  and flags them as "prüfen, ob zutreffend".

## Required wording in every report

> Automatische Prüfungen erfassen nur einen Teil möglicher Probleme und ersetzen keine Rechtsberatung. Geprüft wurde auf EU-Ebene; nationale Zusatzpflichten sind nicht enthalten.

## Review log

| Rule | Reviewer | Date | Result |
| --- | --- | --- | --- |
| (none yet) | | | |
