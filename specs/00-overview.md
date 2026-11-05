# 00 Overview

Status: draft · Working name: none yet (see `open-questions.md`)

## One sentence

Paste a webshop URL, get a plain-language report on accessibility (WCAG 2.1 AA / European Accessibility Act) and EU e-commerce
legal obligations, with a concrete fix for every problem. **No account, no login**, in the spirit of "Have I Been Pwned".

## Users

- Primary: owners of small webshops (WooCommerce, Shopify, …) without developers.
- Secondary: web agencies checking client shops.

## Principles

1. **Algorithms first, AI as fallback.** Every check and every fix is deterministic where possible; Claude is called only
   when the algorithm reports `unsure` (see `06-detection-and-ai.md`, ADR-3).
2. **Honest.** Automatic checks find only part of the problems and are not legal advice. Every report says so.
3. **No accounts.** A scan is reachable through an unguessable link; nothing is tied to a person (ADR-1).
4. **EU level.** Rules are based on EU instruments, not on one member state's law (ADR-2).
5. **Public pages only.** Never submit forms or complete orders on a third-party shop.
6. **Extensible.** A rule is one Go module implementing one interface (N5).

## MVP scope

- Shop type: WooCommerce (self-hostable test shop, see `09-testing.md`). Other shop systems are scanned best-effort but not guaranteed.
- Legal space: EU-level e-commerce rules (`01-legal-scope.md`). National add-ons are out of scope.
- Rules: A1–A6, R1–R7.
- Max 50 pages per scan.
- UI language: German. Detection dictionaries: German and English first, structured to add more EU languages.

## Non-goals

- Legal advice or any claim of full compliance.
- Automatically changing a shop.
- Manual WCAG checks (screen reader tests, content quality).
- User accounts, billing, scheduled scans, e-mail notifications (possible later, would need ADR).
- Placing real orders on shops we do not control.
