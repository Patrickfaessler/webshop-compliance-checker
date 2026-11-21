# Legal text rules R1, R2, R5 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the legal rules R1 (Impressum), R2 (Datenschutzerklärung) and R5 (AGB + Widerrufsbelehrung). They are
deterministic, run on the crawled pages, and have German/English lexicons, EU validators and fix templates.

**Architecture:**
- `internal/lexicon` holds per-language word lists.
- `internal/legal` holds shared helpers: link discovery over `rules.Input`, content text, language choice and validators.
- Three rule packages `internal/rules/r1|r2|r5` follow the A1–A6 pattern (`New()`, `Meta()`, `Check()`).
- Two small crawler additions: link-text priority and `Page.RequestedURL`.
- No Claude: where the spec says fallback, the result is `not_assessable` with a German note.

**Tech Stack:** Go 1.27, golang.org/x/net/html, existing `domutil`, `fixes`, `crawler`, corpus and e2e harnesses.

**Spec:** `docs/superpowers/specs/2026-11-21-legal-text-rules-design.md`. Binding rule specs:
`specs/rules/R1-provider-identification.md`, `R2-privacy-notice.md`, `R5-terms-and-withdrawal-info.md`.

## Global Constraints

- Spec → test → code; every test has `// Spec: <ID>` (e.g. `// Spec: R1-2`).
- User-facing text German, code/comments English. Never "konform", "rechtssicher", "compliant" in any German text.
- Rules are pure functions of `rules.Input`; no network.
- `legal-checked` stays `no`; no rule `verified`.
- A false "fehlt" is worse than "nicht prüfbar". Report *missing* only when confident; doubt gives `not_assessable` plus a note.
- Languages: de and en only. Other languages give `not_assessable` with the note
  `Seite bitte manuell prüfen (Sprache oder Aufbau nicht erkannt).`
- Every finding has `Fix` with non-empty `TextDe`. `Fix.Code` is plain text and is never filled with page values.
- Findings are page-level: no selector. `PageURL` is the legal page, or the start page (`Input.Pages[0].URL`) when the
  legal page is missing.
- Severity from `specs/04-scoring.md`: R1 critical, R2 warning, R5 warning. Area `legal`. `AIFallback: true`
  (spec allows it; not built).
- Browser tests: `CHROME_PATH=/usr/bin/chromium`.

## Review Focus

1. **E-mail only in the Impressum page's own footer** (common): R1 must not say "E-Mail fehlt". Content text
   excludes the footer, so missing fields are re-searched in the full text of the legal page. Test
   `TestR1FooterEmailCounts` (Task 6).
2. **Legal link to another domain** (hosted legal-text services) or blocked by robots.txt: the page is never crawled.
   Expect `not_assessable` ("verlinkt, aber nicht geladen"), never `failed`. Test `TestFindLinkedOffsite` (Task 4)
   and `TestR1LinkNotLoaded` (Task 6).
3. **Same-page anchor** `href="#impressum"`, or the Impressum on the start page itself: the target is the start page
   and must be evaluated. Test `TestFindLinkedFragment` (Task 4).
4. **Numbered or uppercase headings** in R2 (`1. VERANTWORTLICHER`, `§ 3 Ihre Rechte`): they still count as heading
   hits. Test `TestR2NumberedHeadings` (Task 7).
5. **German shop with an English Impressum/terms page** (or the reverse): language is chosen per page, not per
   shop. Test `TestR5EnglishTermsOnGermanShop` (Task 8).

---

### Task 1: Spec updates

**Files:**
- Modify: `specs/rules/R1-provider-identification.md`, `specs/rules/R2-privacy-notice.md`,
  `specs/rules/R5-terms-and-withdrawal-info.md`, `specs/06-detection-and-ai.md`, `specs/05-crawler.md`

- [ ] **Step 1: R1 spec.**
  - Rewrite "Logic" and "Result mapping" from the design's R1 section: field states found/unsure/missing, name
    never missing, address table of EU formats, obfuscated e-mail, link-not-loaded → `not_assessable`, language
    rule, footer re-search.
  - Add under Acceptance:
    - `R1-6: obfuscated e-mail (info (at) shop.at, info[at]shop.at) is found.`
    - `R1-7: e-mail label with only an image → not_assessable.`
    - `R1-8: name without a legal form → not failed.`
    - `R1-9: legal link present but page not loaded → not_assessable.`
    - `R1-10: e-mail only in the legal page's footer counts.`
    - `R1-F: every finding carries a fix (origin template, non-empty German text).`
  - Mark R1-5 `deferred with the AI fallback`.
  - "Fix templates" gets the exact copy:
    - text `Ergänze die fehlenden Angaben im Impressum.`
    - code lines: `Firmenname: {{Firmenname}}`, `Anschrift: {{Straße Nr., PLZ Ort}}`, `E-Mail: {{E-Mail}}` (only
      the missing ones);
    - note `Je nach Land und Rechtsform können weitere Angaben nötig sein (z. B. UID, Firmenbuch).`
    - `no-legal-notice-page`: text `Lege eine Seite „Impressum“ an und verlinke sie im Footer jeder Seite.`, the
      code is all three lines, same note.
  - Notes (exact):
    - passed: `Nur geprüft, ob die Angaben vorhanden sind.`
    - conditional: `UID-Nummer bzw. Firmenbuchnummer prüfen, ob zutreffend.`
    - not loaded: `Das Impressum ist verlinkt, konnte aber nicht geladen werden.`
    - unsure: `Bitte manuell prüfen: {{Felder}}.`, with the fields named `Firmenname`, `Anschrift`, `E-Mail`.
- [ ] **Step 2: R2 spec.**
  - Logic from the design: six markers with their example words; heading hit = 1, or two text hits; ≤ 500 chars
    fails; language rule; link-not-loaded → `not_assessable`.
  - Add:
    - `R2-5: page with ≤ 500 characters fails with all six missing.`
    - `R2-6: numbered/uppercase headings count.`
    - `R2-F: every finding carries a fix.`
  - German section names (used in findings and fix):
    1. `Verantwortlicher`
    2. `Zwecke und Rechtsgrundlage`
    3. `Empfänger`
    4. `Speicherdauer`
    5. `Rechte der betroffenen Personen`
    6. `Beschwerderecht bei der Aufsichtsbehörde`
  - Fix copy:
    - text `Ergänze die fehlenden Abschnitte der Datenschutzerklärung.`
    - code: one line `- <section>` per missing section;
    - note `Ein Datenschutz-Generator oder eine Fachperson kann helfen.`
    - `no-privacy-page`: text `Lege eine Datenschutzerklärung an und verlinke sie im Footer jeder Seite.`, the code
      lists all six.
  - Notes:
    - passed: `Nur geprüft, ob die Abschnitte vorhanden sind, nicht ob der Inhalt stimmt.`
    - not loaded: `Die Datenschutzerklärung ist verlinkt, konnte aber nicht geladen werden.`
- [ ] **Step 3: R5 spec.**
  - Logic from the design: the withdrawal word within 300 chars of a period phrase, the withdrawal page else inside
    the terms, `Rücktrittsrecht`, the B2C note, exempt-goods note.
  - Add:
    - `R5-5: withdrawal vocabulary without period → not_assessable.`
    - `R5-6: Austrian "Rücktrittsrecht … 14 Tagen" passes.`
    - `R5-7: English terms page on a German shop is read as English.`
    - `R5-F: every finding carries a fix.`
  - Fix copy:
    - `no-terms`: text `Lege AGB an und verlinke sie im Footer und im Bestellprozess.`, no code.
    - `no-withdrawal-info`: text `Ergänze eine Widerrufsbelehrung mit der 14-tägigen Frist.`, code:
      ```
      Widerrufsbelehrung
      Widerrufsrecht: Sie haben das Recht, binnen vierzehn Tagen ohne Angabe von Gründen diesen Vertrag zu widerrufen.
      Widerrufsfrist: 14 Tage ab dem Tag, an dem Sie oder ein von Ihnen benannter Dritter die Waren in Besitz genommen haben.
      Ausübung: Senden Sie uns ({{Name}}, {{Anschrift}}, {{E-Mail}}) eine eindeutige Erklärung.
      Folgen des Widerrufs: {{Rückzahlung und Rücksendung}}
      ```
    - note `Link-Text im Footer: „Widerrufsbelehrung“. Den vollständigen Mustertext finden Sie in Anhang I der Richtlinie 2011/83/EU.`
  - Notes (joined with a space, in this order):
    - always `Annahme: Der Shop verkauft an Verbraucher.`
    - period missing: `Widerrufsfrist nicht gefunden – bitte manuell prüfen.`
    - exempt goods: `Ausnahmen vom Widerrufsrecht prüfen.`
    - not loaded: `Die AGB bzw. die Widerrufsbelehrung sind verlinkt, konnten aber nicht geladen werden.`
- [ ] **Step 4: Other specs.**
  - `06-detection-and-ai.md` D-1 step 5: add `Not built yet (2026-11-21): rules return not_assessable with a German "bitte manuell prüfen" note.`
  - `05-crawler.md` C-2: `Links whose URL or link text matches a priority word are visited first; each page records the requested URL before redirects (RequestedURL).`
- [ ] **Step 5: Commit** `docs(specs): R1, R2, R5 logic, acceptance and fix copy for the deterministic version`.

### Task 2: Crawler link-text priority, RequestedURL, corpus URLs

**Files:**
- Modify: `backend/internal/rules/rule.go` (Page), `backend/internal/crawler/crawl.go`,
  `backend/internal/worker/sources.go:47`, `backend/internal/corpus/corpus_test.go` (`loadPages`)
- Test: `backend/internal/crawler/crawl_test.go` (or the existing priority test file)

**Interfaces:**
- Produces: `rules.Page.RequestedURL string` (URL asked for before redirects; equals `URL` when there was no redirect).
- Produces: `crawler.linkTexts(baseURL, htmlSrc string) map[string]string` (normalised link → anchor text, lowercased)
  and `priority(link, text string) int`.
- Corpus page URLs become `file:///corpus/<file>` so relative links (`impressum.html`) resolve to a loaded page.

- [ ] **Step 1: Failing tests.**
  - `// Spec: C-2` `TestPriorityByLinkText`: `priority("https://s.at/p/123", "impressum") == 0`,
    `priority("https://s.at/p/123", "lampe") == 1`, `priority("https://s.at/agb", "") == 0`.
  - `TestLinkTexts`: `linkTexts("https://s.at/", `<a href="/p/1"> Impressum </a><a href="x.html#top">AGB</a>`)` gives
    `{"https://s.at/p/1": "impressum", "https://s.at/x.html": "agb"}` (keys as `normalizeLink` produces them).
  - `// Spec: C-2` `TestCrawlRecordsRequestedURL` (browser test, next to the existing crawl tests): a httptest server
    redirects `/impressum` → `/impressum/`. The crawled page has `URL` ending `/impressum/` and `RequestedURL` ending
    `/impressum`.
- [ ] **Step 2:** Run `cd backend && CHROME_PATH=/usr/bin/chromium go test ./internal/crawler -run 'Priority|LinkTexts|RequestedURL'`. Expected: FAIL (undefined / wrong signature).
- [ ] **Step 3: Implement.**
  - `linkTexts` parses with `domutil.Parse`; anchors with `href`; text `strings.ToLower(strings.Join(strings.Fields(domutil.Text(a)), " "))`.
  - In `Crawl`, build the map once per page from `r.Page.HTML` and call `priority(n, texts[n])`.
  - Set `r.Page.RequestedURL = it.url` before appending.
  - `sources.go` sets `RequestedURL` to the requested start URL.
  - `loadPages` uses `"file:///corpus/" + filepath.Base(f)`.
- [ ] **Step 4:** Run `cd backend && go vet ./... && CHROME_PATH=/usr/bin/chromium go test ./internal/crawler ./internal/corpus ./internal/worker`. Expected: PASS. The corpus A1–A6 stats are unchanged (recall 3/3, 0 false alarms in the log).
- [ ] **Step 5: Commit** `feat(crawler): prioritise legal links by text, record requested URL`.

### Task 3: Lexicon, content text, language choice

**Files:**
- Create: `backend/internal/lexicon/lexicon.go`, `de.go`, `en.go`, `lexicon_test.go`
- Create: `backend/internal/legal/text.go`, `text_test.go`

**Interfaces:**
- Produces:
  ```go
  package lexicon
  type Kind string // "legal_notice", "privacy", "terms", "withdrawal"
  type Section struct{ NameDe string; Words []string } // R2 marker
  type List struct {
      Links          map[Kind][]string // link words matched against link text and URL path (lowercase)
      Sections       []Section         // exactly 6, in the order of Task 1 Step 2
      Withdrawal     []string          // "widerruf", "widerrufen", "rücktrittsrecht", "withdrawal", "cancel the contract" …
      Periods        []string          // "14 tage", "14 tagen", "vierzehn tage", "vierzehn tagen", "14 days", "fourteen days"
      Exempt         []string          // "download", "digitale inhalte", "maßanfertigung", "versiegelt", "digital content" …
      EmailLabels    []string          // "e-mail", "email", "mail"
      AddressLabels  []string          // "anschrift", "adresse", "sitz", "address", "registered office"
      NameLabels     []string          // "inhaber", "betreiber", "firmenname", "medieninhaber", "owner", "company name", "operator"
  }
  func For(lang string) (List, bool) // "de", "en"
  func Hits(l List, text string) int  // count of label/section/link words in lowercased text
  ```
  ```go
  package legal
  func ContentText(p rules.Page) string // visible text minus header/nav/footer/aside/form; falls back to full; whitespace collapsed
  func FullText(p rules.Page) string    // domutil.VisibleText of the whole page, whitespace collapsed
  func Language(p rules.Page) (lang string, ok bool) // design order: DetectLang sure → <html lang> → lexicon hits ≥ 2
  ```
- Link words at least:
  - de: impressum, anbieterkennzeichnung, datenschutz, datenschutzerklärung, agb, allgemeine geschäftsbedingungen,
    widerruf, widerrufsbelehrung, rücktritt;
  - en: imprint, legal notice, legal, privacy, privacy policy, terms, terms and conditions, withdrawal, cancellation,
    right of withdrawal.
  - URL path words: the same without spaces plus `legal-notice`, `privacy-policy`, `terms-of-service`.

- [ ] **Step 1: Failing tests.**
  - `// Spec: R2-1` `TestListsComplete`: `For("de")` and `For("en")` are ok, each with 6 sections and non-empty
    `Links` for all four kinds. `For("fr")` gives `!ok`.
  - `// Spec: R1-10` `TestContentTextDropsFooter`: page
    `<body><header>Shop</header><main>Hauptstraße 1</main><footer>Newsletter E-Mail</footer></body>` → ContentText
    contains `Hauptstraße 1` and not `Newsletter`. FullText contains both. A page with only `<footer>x</footer>`
    falls back to `x`.
  - `// Spec: R1-1` `TestLanguageTerseImpressum`: a 6-line German Impressum (`Impressum`, `Radwerk Alpin GmbH`,
    `Inhaber: Max Muster`, `Anschrift: Hauptstraße 1, 6020 Innsbruck`, `Firmenbuch: FN 123456a`, `UID: ATU12345678`)
    with no `lang` → `("de", true)`. The same text with `<html lang="fr">` but German stopwords below threshold →
    `("de", true)` via lexicon hits. A French page `<html lang="fr"><p>Mentions légales … société</p>` → `ok == false`.
- [ ] **Step 2:** Run `cd backend && go test ./internal/lexicon ./internal/legal`. Expected: FAIL (packages missing).
- [ ] **Step 3: Implement.** `Language`: when `<html lang>` is a non-de/en language and the lexicon hits stay < 2,
  return `!ok`. When lang says fr but the de hits are ≥ 2, the hits win (the shop mis-declared).
- [ ] **Step 4:** Run the same command. Expected: PASS.
- [ ] **Step 5: Commit** `feat(legal): lexicon de/en, content text, language choice`.

### Task 4: Link discovery

**Files:**
- Create: `backend/internal/legal/links.go`, `links_test.go`

**Interfaces:**
- Consumes: `lexicon.Kind`, `lexicon.For`, `rules.Page.RequestedURL`.
- Produces:
  ```go
  type Link struct {
      Found  bool        // a link (or a crawled page whose path matches) exists
      Href   string      // absolute link target without fragment; "" if !Found
      Loaded bool        // target is one of Input.Pages
      Page   *rules.Page // the loaded target; nil if !Loaded
  }
  func FindLinked(in rules.Input, kind lexicon.Kind) Link
  ```
- Matching:
  - Link text (lowercased, whitespace collapsed) **contains** a de or en link word, or the URL path contains a path
    word.
  - Anchors inside `<footer>`/`<header>`/`<nav>` are tried before others.
  - The target is matched against `Page.URL` and `Page.RequestedURL`, ignoring the fragment and a trailing slash.
  - The first loaded match wins; if none is loaded, the first match is returned with `Loaded=false`.
  - Without any anchor, a page whose URL path matches a path word counts as `Found` and `Loaded`.
  - The `terms` kind must not match withdrawal-only links: "Widerrufsbelehrung" is `withdrawal`, not `terms`.

- [ ] **Step 1: Failing tests** (pages built inline, URLs `https://s.at/...`).
  - `// Spec: R1-3` `TestFindLinkedNone`: no matching anchor and no matching path gives `Found=false`.
  - `// Spec: R1-1` `TestFindLinkedFooter`: `<footer><a href="/impressum">Impressum</a></footer>` plus a loaded
    `https://s.at/impressum` gives `Found, Loaded`, `Page.URL == "https://s.at/impressum"`.
  - `// Spec: C-2` `TestFindLinkedRedirect`: anchor `/impressum`; the page has `URL` `https://s.at/de/impressum/` and
    `RequestedURL` `https://s.at/impressum` → `Loaded`.
  - `// Spec: R1-9` `TestFindLinkedOffsite`: anchor `https://legal-texts.example/s.at/impressum` with text `Impressum`
    → `Found, !Loaded`.
  - `// Spec: R1-1` `TestFindLinkedFragment`: anchor `href="#impressum"` on the start page → `Loaded`,
    `Page == &in.Pages[0]`.
  - `// Spec: R5-1` `TestFindLinkedKinds`: footer `AGB` → `/agb` and `Widerrufsbelehrung` → `/widerruf`. `terms`
    gives `/agb` and `withdrawal` gives `/widerruf`. Link text `Text in English: Terms & Conditions` → terms.
- [ ] **Step 2:** `cd backend && go test ./internal/legal -run FindLinked`. Expected: FAIL.
- [ ] **Step 3: Implement** `FindLinked` in `links.go`.
- [ ] **Step 4:** Same command. Expected: PASS.
- [ ] **Step 5: Commit** `feat(legal): find linked legal pages among crawled pages`.

### Task 5: Validators

**Files:**
- Create: `backend/internal/legal/validate.go`, `validate_test.go`, `vat.go`, `postal.go`

**Interfaces:**
- Produces:
  ```go
  type FieldState int
  const ( Missing FieldState = iota; Unsure; Found )
  type Field struct { State FieldState; Quote string } // Quote: matched text, ≤ 120 runes
  func Email(text string, page rules.Page, l lexicon.List) Field   // mailto in page DOM, address, obfuscations; label+img → Unsure
  func Address(text string, l lexicon.List) Field                  // street+postcode → Found; one of them / PO box / label only → Unsure; none → Missing
  func ProviderName(text string, l lexicon.List) Field             // legal form or label → Found; else Unsure (never Missing)
  func VATNumbers(text string) []string                            // valid-format EU VAT ids, normalised (no spaces)
  func RegisterNumber(text string) (string, bool)                  // FN 123456a, HRB/HRA 12345, Company number 1234567
  ```
- VAT formats (country prefix + body):
  - AT `U\d{8}`, BE `[01]\d{9}`, BG `\d{9,10}`, CY `\d{8}[A-Z]`, CZ `\d{8,10}`, DE `\d{9}`, DK `\d{8}`, EE `\d{9}`,
    EL `\d{9}`, ES `[A-Z0-9]\d{7}[A-Z0-9]`, FI `\d{8}`, FR `[A-HJ-NP-Z0-9]{2}\d{9}`, HR `\d{11}`, HU `\d{8}`,
    IE `\d{7}[A-W][A-I]?|\d[A-Z+*]\d{5}[A-W]`, IT `\d{11}`, LT `\d{9}|\d{12}`, LU `\d{8}`, LV `\d{11}`,
    MT `\d{8}`, NL `\d{9}B\d{2}`, PL `\d{10}`, PT `\d{9}`, RO `\d{2,10}`, SE `\d{12}`, SI `\d{8}`, SK `\d{10}`,
    XI `\d{9}|\d{12}|GD\d{3}|HA\d{3}`.
  - Spaces and dots inside are allowed and stripped. Matches need word boundaries.
- Postal tokens:
  - `(?:[A-Z]{1,2}-)?\d{4,5}` followed by a capitalised word;
  - PL `\d{2}-\d{3}`; PT `\d{4}-\d{3}`; SE/CZ/SK/GR `\d{3} \d{2}`; NL `\d{4} ?[A-Z]{2}`;
  - IE Eircode `[A-Z]\d[\dW] ?[A-Z\d]{4}`; MT `[A-Z]{3} ?\d{4}`;
  - UK-style (en) `[A-Z]{1,2}\d[A-Z\d]? ?\d[A-Z]{2}`.
- Street lines:
  - word ending in a street suffix (`straße|strasse|str\.|gasse|weg|platz|allee|ring|ufer|markt|street|road|avenue|lane`)
    or a capitalised word, followed by a house number `\d+[a-z]?(/\d+)?`;
  - en order `\d+ [A-Z][a-z]+ (Street|Road|Avenue|Lane)`.
- Legal forms: GmbH, AG, KG, OG, OHG, e.U., e.K., UG (haftungsbeschränkt), GmbH & Co. KG, SE, Ltd, LLC, LLP, PLC,
  SARL, SAS, SA, B.V., N.V., S.L., S.r.l., S.p.A., Sp. z o.o., s.r.o., ApS, AB, Oy.

- [ ] **Step 1: Failing table tests.**
  - `// Spec: R1-4` `TestVATNumbers`: valid `DE123456789`, `ATU12345678`, `ATU 1234 5678`, `NL123456789B01`,
    `FRXX123456789`, `IE1234567T`; invalid `DE12345678`, `ATU1234567`, `XX123456789`, `DE1234567890` → none.
  - `// Spec: R1-1` `TestAddress`: Found for `Hauptstraße 12, 6020 Innsbruck`, `Musterweg 3/4 A-1010 Wien`,
    `ul. Marszałkowska 10, 00-950 Warszawa`, `Rua Augusta 5, 1100-053 Lisboa`, `Damrak 1, 1012 AB Amsterdam`,
    `12 High Street, London SW1A 1AA`. Unsure for `Postfach 12, 1010 Wien`, `6020 Innsbruck` alone, and
    `Anschrift: siehe unten`. Missing for `Wir sind ein Familienbetrieb.`.
  - `// Spec: R1-6, R1-7` `TestEmail`: Found for `info@shop.at`, `info (at) shop.at`, `info[at]shop.at`,
    `info @ shop . at`, and `<a href="mailto:x@y.at">Schreiben Sie uns</a>`. Unsure for
    `E-Mail: <img src="mail.png">`. Missing for `Telefon: 0512 123`.
  - `// Spec: R1-8` `TestProviderName`: Found for `Radwerk Alpin GmbH`, `Inhaber: Max Muster`, `Seifen Lavendel e.U.`,
    `Shop Ltd`. Unsure (not Missing) for `Max Muster`.
  - `TestRegisterNumber`: `FN 123456a` ok, `HRB 12345` ok, `Firmenbuchgericht Innsbruck` alone not ok.
- [ ] **Step 2:** `cd backend && go test ./internal/legal -run 'VAT|Address|Email|ProviderName|Register'`. Expected: FAIL.
- [ ] **Step 3: Implement** with precompiled regexps; quotes trimmed to 120 runes.
- [ ] **Step 4:** Same command. Expected: PASS.
- [ ] **Step 5: Commit** `feat(legal): EU validators for e-mail, address, name, VAT, register`.

### Task 6: Rule R1

**Files:**
- Create: `backend/internal/rules/r1/r1.go`, `r1_test.go`
- Create corpus cases `testcorpus/cases/r1-complete-at`, `r1-complete-en`, `r1-obfuscated-email`,
  `r1-no-email`, `r1-no-link`, `r1-no-address`. Each has `index.html` (with a footer link) + `impressum.html`
  (or `imprint.html`) + `expected.json` with `"spec"`, `"kind"`: `clean`/`defect`, `"expect": {"R1": …}`.
- Modify: `backend/internal/corpus/corpus_test.go` (`allRules` adds `r1.New()`)

**Interfaces:**
- Consumes: `legal.FindLinked`, `legal.ContentText`, `legal.FullText`, `legal.Language`, `legal.Email`, `Address`,
  `ProviderName`, `VATNumbers`, `RegisterNumber`, `lexicon.For`.
- Produces: `r1.New() rules.Rule`.
  - Meta: `ID "R1"`, `NameDe "Anbieterkennzeichnung (Impressum)"`, `SummaryDe` `Im Impressum fehlen Pflichtangaben oder es gibt kein Impressum.`,
    `ExplanationDe` from the spec.
  - Subs: `no-legal-notice-page`, `missing-email`, `missing-address`, `missing-name` (the last is unused while name
    is never missing; it is kept for the AI fallback).
- Per field, search ContentText first. If not Found there, search FullText of the same legal page (Review Focus 1)
  and keep the better state.
- `DecidedBy`: `structural` for no-page, otherwise `pattern`.

- [ ] **Step 1: Failing tests** (`r1_test.go`, inline pages; helper builds `rules.Input` with a start page that has
  a footer link).
  - `// Spec: R1-1` `TestR1Complete` → passed. The note contains `Nur geprüft, ob die Angaben vorhanden sind.` and
    no `UID-Nummer` sentence when VAT and FN are present.
  - `// Spec: R1-2` `TestR1MissingEmail` → failed, one finding `Sub "missing-email"`, `PageURL` = Impressum URL,
    the fix code contains `E-Mail: {{E-Mail}}` and not `Firmenname`.
  - `// Spec: R1-3` `TestR1NoLink` → failed, `Sub "no-legal-notice-page"`, `PageURL` = start page, the fix text is
    `Lege eine Seite „Impressum“ an und verlinke sie im Footer jeder Seite.`
  - `// Spec: R1-7` `TestR1EmailImage` → not_assessable, the note contains `Bitte manuell prüfen: E-Mail.`
  - `// Spec: R1-8` `TestR1NameWithoutLegalForm` (`Max Muster`, address, e-mail) → not_assessable, not failed.
  - `// Spec: R1-9` `TestR1LinkNotLoaded` (link to an off-site Impressum) → not_assessable, note
    `Das Impressum ist verlinkt, konnte aber nicht geladen werden.`
  - `// Spec: R1-10` `TestR1FooterEmailCounts`: Impressum content without an e-mail, its `<footer>` has
    `info@shop.at` → passed.
  - Conditional note: no VAT and no FN → the note contains `UID-Nummer bzw. Firmenbuchnummer prüfen, ob zutreffend.`
    and the state is still passed.
  - French Impressum → not_assessable with the global language note.
  - `// Spec: R1-F` every finding in these tests has a non-empty `Fix.TextDe`.
- [ ] **Step 2:** `cd backend && go test ./internal/rules/r1`. Expected: FAIL (package missing).
- [ ] **Step 3: Implement** `r1.go`. State order:
  1. no link → failed;
  2. not loaded → not_assessable;
  3. language not ok → not_assessable;
  4. any field Missing → failed;
  5. any field Unsure → not_assessable;
  6. otherwise passed.
- [ ] **Step 4:** `cd backend && go test ./internal/rules/r1`. Expected: PASS.
- [ ] **Step 5: Corpus.** Write the six cases (the AT case uses e.U. + `FN` + `ATU`; the EN case uses `Ltd`, a UK
  postcode and `Company number`). Register R1 in `allRules`. Run
  `cd backend && CHROME_PATH=/usr/bin/chromium go test ./internal/corpus -run 'TestCorpus/r1' -v 2>&1 | tail -20`.
  Expected: PASS, log `R1: recall 3/3, false alarms 0/3`.
- [ ] **Step 6: Commit** `feat(rules): R1 provider identification (deterministic)`.

### Task 7: Rule R2

**Files:**
- Create: `backend/internal/rules/r2/r2.go`, `r2_test.go`
- Create corpus cases `r2-complete-de`, `r2-complete-en`, `r2-numbered-headings`, `r2-no-link`,
  `r2-heading-only`, `r2-four-sections`
- Modify: `corpus_test.go` `allRules`

**Interfaces:**
- Consumes: `legal.FindLinked(in, "privacy")`, `ContentText`, `Language`, `lexicon.List.Sections`.
- Produces:
  - `r2.New()`; Meta `ID "R2"`, `NameDe "Datenschutzerklärung vorhanden"`, `Severity warning`, `SummaryDe`
    `Die Datenschutzerklärung fehlt oder ihr fehlen Pflichtabschnitte.`
  - Subs `no-privacy-page`, `incomplete-sections` (one finding; `Evidence` empty; the German missing names are in
    the fix code).
- Heading hit: text of `h1–h6`, `strong`/`b` that is the only child of a `p`, or `dt`. The text is lowercased, a
  leading `\d+\.|§ ?\d+|[IVX]+\.` is stripped, then it must contain a section word. Text hits are counted in
  ContentText.

- [ ] **Step 1: Failing tests.**
  - `// Spec: R2-1` `TestR2Complete` (6 sections as h2) → passed, note
    `Nur geprüft, ob die Abschnitte vorhanden sind, nicht ob der Inhalt stimmt.`
  - `// Spec: R2-2` `TestR2NoLink` → failed, `no-privacy-page`.
  - `// Spec: R2-3` `TestR2HeadingOnly` (h1 `Datenschutz` + 600 chars of filler with no markers) → failed,
    `incomplete-sections`, the fix code has all six `- …` lines.
  - `// Spec: R2-4` `TestR2English` → passed.
  - `// Spec: R2-5` `TestR2Short` (≤ 500 chars) → failed with six missing.
  - `// Spec: R2-6` `TestR2NumberedHeadings` (`1. VERANTWORTLICHER`, `§ 3 Ihre Rechte`, …) → passed.
  - Four of six sections → failed, and the code lists exactly the two missing in the Task 1 order.
  - Five of six → passed.
- [ ] **Step 2:** `cd backend && go test ./internal/rules/r2`. Expected: FAIL.
- [ ] **Step 3: Implement** `r2.go`.
- [ ] **Step 4:** Same command. Expected: PASS.
- [ ] **Step 5: Corpus.** Six cases; register R2. Run `… go test ./internal/corpus -run 'TestCorpus/r2' -v`.
  Expected: PASS, `R2: recall 3/3, false alarms 0/3`.
- [ ] **Step 6: Commit** `feat(rules): R2 privacy notice present (deterministic)`.

### Task 8: Rule R5

**Files:**
- Create: `backend/internal/rules/r5/r5.go`, `r5_test.go`
- Create corpus cases `r5-both-pages`, `r5-withdrawal-in-terms`, `r5-at-ruecktritt`, `r5-no-terms`,
  `r5-no-withdrawal`, `r5-terms-only-general`
- Modify: `corpus_test.go` `allRules`

**Interfaces:**
- Consumes: `FindLinked(in, "terms")`, `FindLinked(in, "withdrawal")`, `ContentText`, `Language`,
  `lexicon.List.Withdrawal/Periods/Exempt`.
- Produces:
  - `r5.New()`; Meta `ID "R5"`, `NameDe "AGB und Widerrufsinformation vorhanden"`, `Severity warning`,
    `SummaryDe` `AGB oder Widerrufsbelehrung fehlen.`
  - Subs `no-terms`, `no-withdrawal-info`.
- Withdrawal search: on the withdrawal page if loaded, else in the terms. Each page uses its own `Language`
  (Review Focus 5). A hit is a withdrawal word whose index is within 300 chars (either side) of a period phrase in
  the lowercased ContentText. The quote is ±60 chars around the period, stored in `Finding.Evidence` only.
- State:
  1. Any linked page not loaded and its content needed → not_assessable.
  2. Terms missing → failed (`no-terms`, PageURL = start page).
  3. No withdrawal word on either page → failed (`no-withdrawal-info`).
  4. Words but no period → not_assessable.
  5. Both present with the period → passed.
  6. When both failures apply, there are two findings.
  7. Language not ok on the page that matters → not_assessable.

- [ ] **Step 1: Failing tests.**
  - `// Spec: R5-1` `TestR5Both` → passed. The note starts with `Annahme: Der Shop verkauft an Verbraucher.`
  - `// Spec: R5-2` `TestR5NoTerms` → failed `no-terms`, fix text `Lege AGB an und verlinke sie im Footer und im Bestellprozess.`
  - `// Spec: R5-3` `TestR5WithdrawalInTerms` (no withdrawal link; the terms contain `Widerrufsrecht … binnen 14 Tagen`) → passed.
  - `// Spec: R5-4` `TestR5English` → passed.
  - `// Spec: R5-5` `TestR5NoPeriod` (`Widerruf` without a period) → not_assessable, note contains
    `Widerrufsfrist nicht gefunden – bitte manuell prüfen.`
  - `// Spec: R5-6` `TestR5Ruecktritt` (`Rücktrittsrecht … innerhalb von 14 Tagen`) → passed.
  - `// Spec: R5-7` `TestR5EnglishTermsOnGermanShop`: German start page, English terms page with
    `right of withdrawal … 14 days` → passed.
  - `TestR5FooterLinkNotWithdrawalInfo`: the terms page's footer has the link text `Widerrufsbelehrung` but no
    withdrawal page is loaded and the content has no withdrawal words → not_assessable when the link exists but
    isn't loaded; failed `no-withdrawal-info` when no such link exists.
  - Exempt keywords (`digitale Inhalte`) → the note contains `Ausnahmen vom Widerrufsrecht prüfen.`
  - `// Spec: R5-F` all findings have fixes; the `no-withdrawal-info` code starts with `Widerrufsbelehrung`.
- [ ] **Step 2:** `cd backend && go test ./internal/rules/r5`. Expected: FAIL.
- [ ] **Step 3: Implement** `r5.go`.
- [ ] **Step 4:** Same command. Expected: PASS.
- [ ] **Step 5: Corpus.** Six cases; register R5. Run `… go test ./internal/corpus -run 'TestCorpus/r5' -v`.
  Expected: PASS, `R5: recall 3/3, false alarms 0/3`.
- [ ] **Step 6: Commit** `feat(rules): R5 terms and withdrawal information present (deterministic)`.

### Task 9: Wiring, test shops, docs

**Files:**
- Modify: `backend/cmd/api/main.go:133`, `backend/internal/e2e/testshops_test.go:66` (rule list),
  `testshops/gen/main.go` (Expected maps, ExpectedScore, Note), regenerate `testshops/site/*`
- Modify: `specs/rules/R1…`, `R2…`, `R5…` Status lines, `specs/10-roadmap.md` M4, `CLAUDE.md` Status,
  `specs/09-testing.md` (the test-shop note)
- Leave `internal/e2e/report_test.go` on accessibility (design).

**Interfaces:**
- Consumes: `r1.New()`, `r2.New()`, `r5.New()`.

- [ ] **Step 1: Failing e2e expectation.**
  - In `testshops/gen/main.go`, add to each shop's `Expected`:
    - Teehaus R1/R2/R5 `passed`;
    - Radwerk R1/R2/R5 `failed`;
    - Seifen R1/R2/R5 `passed`.
  - Set ExpectedScore to Teehaus 100, Radwerk 0, Seifen 70 (S-1: passed 3+3+1+3+2+2 = 14 of 20).
  - Note text: `expected/expectedScore cover the implemented rules (A1-A6, R1, R2, R5). Other R-rule defects are planted and listed for later.`
  - Regenerate (`cd testshops/gen && go run .`). Add r1/r2/r5 to the e2e `ruleList` and run
    `cd backend && CHROME_PATH=/usr/bin/chromium go test ./internal/e2e -run 'TestShops$' -v 2>&1 | tail -30`.
  - Expected: PASS. If a state differs, debug the rule (systematic-debugging) against the shop page, **not** the
    expectation. The exception is a generator page that truly contradicts the planted list; that needs a ledgered
    ruling.
- [ ] **Step 2:** Register R1, R2, R5 in `cmd/api/main.go`. Run
  `cd backend && go vet ./... && CHROME_PATH=/usr/bin/chromium go test -count=1 ./... > /tmp/legal-go.txt 2>&1; tail -5 /tmp/legal-go.txt`.
  Expected: all `ok`, no FAIL.
- [ ] **Step 3: Frontend check.** Run `cd frontend && flutter analyze && flutter test`. Expected: green, no change
  (R names already in `lib/strings.dart`).
- [ ] **Step 4: Docs.**
  - The Status line of each rule: `implemented (2026-11-21, deterministic; AI fallback not built → not_assessable): unit tests, corpus 3 defect + 3 clean (recall 3/3, 0 false alarms), test shops. Code: backend/internal/rules/rN, backend/internal/legal, backend/internal/lexicon.`
    `legal-checked: no` is unchanged.
  - Roadmap M4 progress line.
  - CLAUDE.md: replace "Missing: all R rules" with `R1, R2, R5 done (deterministic, de/en); missing R3, R4, R6, R7 and the AI fallback.`
    Add a short bullet on `internal/legal` + `internal/lexicon`, and update the "Next" line.
- [ ] **Step 5: Commit** `feat: register R1, R2, R5; test-shop expectations and docs`.
