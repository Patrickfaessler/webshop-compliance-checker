# ADR-2 EU-level legal scope

Status: accepted (2026-11-05)

**Decision.** Legal rules are based on EU instruments (E-Commerce Directive, Consumer Rights Directive, GDPR, ePrivacy, European Accessibility Act), not on one member state's national law. Replaces the Austria-only scope of V2 draft.

**Why.** Shops sell across the EU; the EU minimum is the common denominator and keeps rules reusable.

**Consequences.**
- Reports state that national additions are not checked.
- Each rule cites the EU article it derives from (`01-legal-scope.md`).
- Dictionaries need several languages over time (`06-detection-and-ai.md`).
- Some obligations depend on national transposition (e.g. exact legal-notice content); R1 checks only the EU-level common fields.
