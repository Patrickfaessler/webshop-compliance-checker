# R4 No non-essential trackers before consent

- **Status:** draft · **legal-checked:** no · **Area:** legal · **Severity:** critical
- **Basis:** Directive 2002/58/EC Art. 5(3)
- **applies_when:** every shop
- **ai_fallback:** yes (classify unknown third-party domains, cached per domain)

## Input
Request and cookie logs from browser states `no_interaction` and `rejected` (C-4).

## Logic
1. List third-party requests and cookies set.
2. Match against the pinned tracker list (e.g. DuckDuckGo Tracker Radar, version in repo) and known cookie names (`_ga`, `_fbp`, …).
3. Known tracker in `no_interaction` or `rejected` state → violation.
4. Unknown third-party domain → Claude fallback (classify analytics/advertising vs. functional such as CDN or payment); if not decided → listed as "möglicher Tracker", **not** a violation.
5. Strictly necessary items (session cookie, cart, load balancer, CSRF) are allowed (dictionary of names and known payment providers).

## Result mapping
| Situation | State |
| --- | --- |
| no known tracker before consent | passed |
| ≥ 1 known tracker before consent or after reject | failed, one finding per tracker domain |
| only unknown domains left undecided | not_assessable (with the list) |

## Finding
Domain, first request URL, page where it loaded, state (`no_interaction`/`rejected`), cookie name if any.

## Explanation (German)
Diese Seite lädt schon beim Öffnen Dienste, die Besucher verfolgen (z. B. Analyse oder Werbung), bevor jemand zugestimmt hat.

## Fix templates
Per detected provider: how to load it only after consent (e.g. "Google Analytics über den Consent-Manager blockieren bis zur Zustimmung") plus a generic script-blocking snippet (`type="text/plain" data-category="analytics"`).

## Acceptance
- R4-1: page with Google Analytics firing on load fails naming `google-analytics.com`. R4-2: tracker only after accept passes. R4-3: first-party CDN passes. R4-4: an unknown third-party domain yields `not_assessable` when the fallback is disabled; with the fake LLM enabled the classification is used and cached per domain.
