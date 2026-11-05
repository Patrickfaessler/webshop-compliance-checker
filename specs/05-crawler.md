# 05 Crawler, browser states, safety

Status: draft · implemented 2026-11-05: C-SSRF filter (`internal/ssrf`), SSRF-safe fetcher, rate limits, browser crawler (`internal/crawler`, chromedp, see ADR-4) with request interception, robots.txt, 50-page / depth-3 limits, legal links first, axe-core injection.
Not yet: egress proxy (DNS-rebinding window remains for the browser), parallel tabs (pages are loaded one at a time), cookie states C-4, checkout C-3, product/category page selection by shop selectors (C-2 uses link keywords only).
Read "Playwright" in this file as "headless Chromium via chromedp".

## Limits (C-1)

| Parameter | MVP value |
| --- | --- |
| Max pages per scan | 50 |
| Domain | exact host of the start URL plus its `www` variant; no other subdomains |
| Link depth | 3 from start page |
| Page load timeout | 15 s load + wait for network idle (max 2 s) |
| Total scan time | 5 min, then report is marked `incomplete` |
| Parallel tabs per scan | 4 |
| Politeness | obey `robots.txt` (a disallowed page is skipped and listed), max 2 requests/s per host, own User-Agent with contact URL |

## Page selection (C-2)

1. Start page.
2. Links found by role: legal notice, privacy, terms, withdrawal (footer first, then any page).
3. Up to 5 category pages and 10 product pages (found by shop-system selectors, then by structured data `Product`).
4. Cart page.
5. Checkout: **only in our own test shop**. For other shops, checkout is read only if reachable without input (C-3).

## Checkout (C-3)

- Test shop: add a test product, fill test data, advance to the last step, record the order button label, **stop before submit**.
- Third-party shop: never fill or submit forms; if the cart/checkout needs input, rule R6 is `not_assessable`.

## Browser states for cookie checks (C-4)

Each state uses a fresh browser profile with empty storage and no cache.

1. `no_interaction`: load page, do nothing, wait for network idle, log all requests and cookies.
2. `rejected`: click reject, reload, log requests and cookies.
3. `accepted`: click accept, log requests and cookies (comparison only).

Banner not found → R3/R4 `not_assessable` unless trackers load in `no_interaction`, which is a violation even without a banner.

## SSRF protection (C-SSRF)

- Only `http` and `https`, ports 80 and 443.
- Resolve the host; reject loopback, private, link-local, unique-local and cloud-metadata ranges
  (`10/8, 172.16/12, 192.168/16, 127/8, 169.254/16, 100.64/10, ::1, fc00::/7, fe80::/10`).
- Re-check after every redirect and for **every** sub-request the browser makes (egress proxy).
- Browser runs in its own container with no route to internal networks; only the egress proxy has outbound access.
- Pin the resolved IP for the connection (defence against DNS rebinding).

## Abuse limits without accounts (F12 / C-RL)

- Per client IP: max 3 scans per hour, max 1 running scan.
- Per target host: max 1 scan per 10 minutes (the cached result is returned instead).
- Global cap on running scans; excess scans are queued and show the queue position.
- Optional proof-of-work or CAPTCHA token on `POST /scans` if abuse appears (decision deferred).
