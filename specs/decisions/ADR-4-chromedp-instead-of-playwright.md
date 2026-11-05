# ADR-4 chromedp instead of Playwright

Status: accepted (2026-11-05)

**Decision.** The headless browser is driven with `chromedp` (Chrome DevTools Protocol, pure Go) instead of `playwright-go`.
axe-core is injected into the page as before.

**Why.** Playwright's driver download failed (404 on `playwright.azureedge.net` and no working alternative host for the
version pinned by `playwright-go`), and it needs a Node-based driver plus a managed browser download. chromedp needs only a
Chromium-family binary: Microsoft Edge is already on Windows 11, and the Docker image just installs Chromium.

**Consequences.**
- `specs/07-architecture.md` and `05-crawler.md` mention "Playwright"; read that as "headless Chromium via chromedp".
- Request interception uses the CDP `Fetch` domain. Every browser sub-request is validated with `ssrf.Validate`
  (C-SSRF). Chromium resolves DNS itself, so a DNS-rebinding window remains until the egress proxy exists; the proxy stays required for production.
- Browser path comes from `CHROME_PATH` or is auto-detected (Edge, Chrome, Chromium).
- Tab-order checks for A5 use CDP key events and DOM focus queries instead of Playwright APIs.
- If Playwright becomes installable later, only `internal/crawler` changes.
