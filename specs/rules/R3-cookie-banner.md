# R3 Cookie banner with a fair reject option

- **Status:** draft · **legal-checked:** no · **Area:** legal · **Severity:** critical
- **Basis:** Directive 2002/58/EC Art. 5(3); GDPR Art. 4(11), 7; EDPB Guidelines 05/2020
- **applies_when:** the page sets or reads non-essential cookies/storage or loads third-party trackers (from R4 data); otherwise `not_applicable`
- **ai_fallback:** yes (unknown banners)

## Input
Browser state `no_interaction` DOM (C-4).

## Logic
1. Detect banner: known CMP signatures (Cookiebot, OneTrust, Usercentrics, Complianz, Borlabs, CookieYes, …) then DOM heuristic (fixed/overlay element with cookie vocabulary and ≥ 2 buttons).
2. Classify buttons by label lexicon: accept, reject, settings. `reject` counts only if it rejects **all** non-essential categories.
3. Fair reject check: reject reachable on the **first layer**; its size and contrast not clearly weaker than accept (area ratio ≥ 0.5, same element type: button vs. button); no pre-ticked optional categories.
4. Unknown banner structure → Claude fallback for classification, evidence quote required.

## Result mapping
| Situation | State |
| --- | --- |
| banner with first-layer reject and no pre-ticked boxes | passed |
| non-essential trackers present but no banner | failed (`no-banner`) |
| no reject on first layer / pre-ticked / reject visually hidden | failed (sub-rule named) |
| banner not recognisable and no tracker evidence | not_assessable |

## Finding
URL, banner selector, button labels found (quoted), screenshot.

## Explanation (German)
Bevor Tracking-Cookies gesetzt werden, müssen Besucher zustimmen können, und Ablehnen muss genauso einfach sein wie Zustimmen.

## Fix templates
Guidance per CMP detected (setting name to change in that plugin) or generic text; for custom banners an HTML snippet with two equal buttons.

## Acceptance
- R3-1: banner with equal Accept/Reject passes. R3-2: only "Accept" + "Settings" fails. R3-3: pre-ticked analytics fails. R3-4: trackers without banner fail. R3-5: shop with no non-essential cookies is `not_applicable`.
