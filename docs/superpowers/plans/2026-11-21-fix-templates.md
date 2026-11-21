# Fix templates and finding detail: implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every A1–A6 finding carries a German fix from a template. The fix is shown on the HTML/PDF/JSON report card and on a new finding detail page in the Flutter app (with a "Kopieren" button).

**Architecture:**
- Each rule builds its `rules.Fix` while checking (design approach 1).
- The shared package `backend/internal/fixes` does three things:
  - fills and escapes the placeholders;
  - computes the A1 colour;
  - detects the page language for A6.
- The worker copies the fix and the rule's `ExplanationDe` onto `scan.Finding`. From there the existing paths carry it: API, report JSON, HTML card and the app.
- The app detail page reads the findings list it already loads.
- A new API route serves the element crop as JPEG.

**Tech Stack:** Go (golang.org/x/net/html, chromedp/axe-core data), Flutter (go_router, http, `Clipboard`).

**Spec:** `docs/superpowers/specs/2026-11-21-fix-templates-design.md`, plus rule specs `specs/rules/A1…A6`, `specs/08-api.yaml`, `specs/11-report.md`, `specs/06-detection-and-ai.md` (D-3).

## Global Constraints

- Spec → test → code. Every test has `// Spec: <ID>`. The template texts come from the rule specs. A sub-rule without a template gets one in the spec first (Task 1).
- User-facing text is German, code and comments are English. Never call a result or fix "konform" or "compliant" (S-3, CLAUDE.md rule 7). Fix texts are suggestions.
- Page values are untrusted. Every value from the page that goes into `Fix.Code` is HTML-escaped (`&`, `<`, `>`, `"`, `'`). The HTML report escapes `Fix.Code` again when it renders it (html/template default).
- Origin is always `"template"` in this plan. No LLM calls, and `POST …/suggestion` stays unimplemented (AI comes later).
- JSON changes are additive. Report `version` stays 1.
- pgstore does not persist `Fix`/`ExplanationDe`, like `Position`/`Box`. There is no migration.
- Browser tests run with `CHROME_PATH=/usr/bin/chromium`. Backend checks: `cd backend && go vet ./... && go test ./...`. Frontend checks: `cd frontend && flutter analyze && flutter test`.

## Review Focus

1. **Hostile page values:** a field with `placeholder="\"><script>x</script>"` or a product title with `<`/`"` must not break out of the code snippet, the HTML report or the app. Covered in Task 2 by `TestFillEscapes`, and in Task 7 by `TestHTMLFixEscaped`.
2. **A1 without a usable background:** an axe node with no `bgColor` (background image, gradient) or an unparsable colour gets a fix with a note and no code. It must not guess a colour. Covered in Task 3 by `TestA1FixUnknownBackground`.
3. **Finding without a screenshot:** the crop route returns 404, the app shows no image and no error, and the report card has no crop. Covered in Task 6 by `TestFindingCrop` (404 case) and in Task 8 by the widget test with `hasCrop: false`.
4. **Direct link or reload on `/r/:id/f/:fid`, an expired scan, or an unknown finding id:** the page shows the existing "Diese Prüfung gibt es nicht (mehr)." text, or "Diese Fundstelle gibt es nicht.". It must never show a blank page or crash. Covered in Task 8.
5. **Long code or snippet on a phone-width screen (360 px):** the detail page must not overflow (no RenderFlex overflow error). The code scrolls horizontally. Covered in Task 8 by `detail page fits 360 px`.

---

### Task 1: Spec updates

**Files:**
- Modify:
  - `specs/rules/A1-contrast.md`, `A2-alt-text.md`, `A3-form-labels.md`, `A4-heading-structure.md`, `A5-keyboard.md`, `A6-html-lang.md` (Fix templates + Acceptance)
  - `specs/08-api.yaml`
  - `specs/11-report.md`
  - `specs/02-requirements.md` (F8 row status/trace)

**Interfaces:**
- Produces: acceptance IDs `A1-F`, `A2-F`, `A3-F`, `A4-F`, `A5-F`, `A6-F`. Each means "every finding of the rule carries a fix with non-empty `textDe`; values from the page are escaped". Also the API paths and the `Fix` schema below.

- [ ] **Step 1: Rule specs.** Under each rule's "Fix templates", list one template per sub-rule. Add an acceptance line `<ID>-F: every finding carries a fix (origin template, non-empty textDe); page values in code are HTML-escaped.` Additions to the specs (exact German copy):
  - A1, when the background is unknown: text `Wähle eine Textfarbe mit ausreichend Kontrast zum Hintergrund.`, note `Hintergrund nicht eindeutig (z. B. Bild) – Kontrast bitte selbst prüfen.`, no code.
  - A1 code: `color: {{suggestedFg}};`. A1 text stays `Ändere die Textfarbe von {{fg}} auf {{suggestedFg}} (Kontrast {{ratio}}:1).`. The ratio is the suggestion's own ratio, written with a German decimal comma and one decimal (for example `4,6`).
  - A2: text `Beschreibe das Bild in wenigen Worten, z. B. mit dem Produktnamen.`
    - Product image code: `alt="{{productName}}"`.
    - Otherwise code: `alt="Beschreibung des Bildes"` with the note `Beschreibung bitte anpassen.`
    - Decorative image: the same text plus the note `Rein dekorative Bilder bekommen alt="".`
  - A3: text `Gib dem Feld eine sichtbare Beschriftung.`, code `<label for="{{id}}">{{labelText}}</label>`.
    - If the field has no id, add a second code line `id="{{id}}" am Feld ergänzen`.
    - If no label text could be derived: labelText `Beschriftung`, note `Beschriftungstext bitte anpassen.`
  - A4:
    - `skipped-level`: `Ändere <h{{from}}> zu <h{{to}}>.`
    - `no-h1`: `Füge genau eine <h1> mit dem Seitentitel hinzu.`
    - `multi-h1`: `Behalte nur eine <h1> und mache die anderen zu <h2>.`
    - `empty`: `Gib der Überschrift einen Text oder entferne das leere Element.`
    - No code for any of them.
  - A5:
    - `no-focus-style`: text `Mach sichtbar, welches Element den Fokus hat.`, code `{{selector}}:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }`.
    - `not-focusable`: text `Verwende ein <button>-Element. Alternativ role="button" und tabindex="0" ergänzen und Enter/Leertaste per Skript bedienen.`, code `<button type="button">…</button>`.
    - `unreachable`: text `Mach das Element mit der Tab-Taste erreichbar: entferne tabindex="-1" oder Skripte, die den Fokus wegnehmen.`
    - `keyboard-trap`: text `Sorge dafür, dass man das Element mit der Tab-Taste wieder verlassen kann.`
    - `unreachable` and `keyboard-trap` have no code.
  - A6: text `Setze das Attribut lang im html-Element.`, code `<html lang="{{lang}}">`. When the detector is unsure: lang `de`, note `Sprache bitte anpassen.`
- [ ] **Step 2: `specs/08-api.yaml`.**
  - Replace `/api/findings/{id}` and `/api/findings/{id}/suggestion` with `/api/scans/{id}/findings/{findingId}/suggestion` (unchanged summary, marked "not implemented, AI later").
  - Add `GET /api/scans/{id}/findings/{findingId}/crop.jpg`: `image/jpeg`; 404 when there is no crop or no such finding.
  - The `Finding` items of `GET /api/scans/{id}/findings` gain these fields:
    - `selector`, `snippet`, `sub` (nullable), `explanationDe`, `hasCrop: boolean`;
    - `fix: {origin: template|ai, textDe, code (nullable), noteDe (nullable)}`.
  - Keep `Suggestion` and use it as the `fix` schema.
- [ ] **Step 3: `specs/11-report.md`.** Add RP-13: "Every finding card shows the fix block 'So können Sie es beheben (Vorlage)' with the text, the code in `<pre><code>` (escaped) and the note. The JSON report has `explanationDe` and `fix` per finding." `specs/02-requirements.md` F8: trace to A*-F, RP-13 and the detail page.
- [ ] **Step 4: Commit.** `git add specs && git commit -m "Specs: fix templates per sub-rule, finding detail API, RP-13 (F8)"`

### Task 2: Fix model, explanations, `fixes` helpers, worker plumbing

**Files:**
- Modify:
  - `backend/internal/rules/rule.go` (types)
  - each `backend/internal/rules/a*/a*.go` `Meta()` (`ExplanationDe` copied verbatim from the spec's "Explanation (German)")
  - `backend/internal/scan/scan.go`
  - `backend/internal/worker/worker.go:117`
- Create:
  - `backend/internal/fixes/fixes.go`, `backend/internal/fixes/lang.go`
- Test:
  - `backend/internal/fixes/fixes_test.go`, `lang_test.go`
  - `backend/internal/worker/worker_test.go` (add)

**Interfaces:**
- Produces:
  - `type rules.Fix struct { TextDe, Code, NoteDe string }`
  - `rules.Finding.Fix *rules.Fix`
  - `rules.Meta.ExplanationDe string`
  - `scan.Finding.Fix *rules.Fix`, `scan.Finding.ExplanationDe string`
  - `fixes.Attr(s string) string`: HTML-escapes `& < > " '`.
  - `fixes.Fill(tmpl string, vals map[string]string) string`: replaces `{{key}}` with `Attr(val)`. Unknown keys stay as they are.
  - `fixes.DetectLang(text string) (lang string, sure bool)`: covers de, en, fr, it, es, nl.

- [ ] **Step 1: Failing tests.**
  - `TestFillEscapes`, `// Spec: A3-F` (Review Focus 1): `Fill(`<label for="{{id}}">{{t}}</label>`, {"id":"plz","t":"\"><script>x</script>"})` must contain neither `<script>` nor `"><`. It must equal `<label for="plz">&#34;&gt;&lt;script&gt;x&lt;/script&gt;</label>` (html.EscapeString output).
  - `TestDetectLang`, `// Spec: A6-F`:
    - A German shop paragraph (≥ 40 words) → `de, true`.
    - An English paragraph → `en, true`.
    - `"Shop 123 €"` → `_, false`.
    - A 50/50 de/en mix → `_, false`. "Sure" means the top language has ≥ 8 stopword hits and ≥ 2× the second.
  - `TestRunCopiesFixAndExplanation`, `// Spec: F8`, in worker: a fake rule returns one finding with `Fix{TextDe:"t"}` and Meta `ExplanationDe:"e"`. The stored `scan.Finding` has `Fix.TextDe=="t"` and `ExplanationDe=="e"`.
  - `TestMetaHasExplanation`, `// Spec: F8`, in `internal/rules` (or the registry test): every registered rule's `Meta().ExplanationDe` is non-empty.
- [ ] **Step 2: Run** `cd backend && go test ./internal/fixes ./internal/worker ./internal/rules/...`. Expected: FAIL (undefined `fixes`, fields).
- [ ] **Step 3: Implement** the types, the helpers and the worker copy (`Fix: f.Fix, ExplanationDe: m.ExplanationDe`). Fill `ExplanationDe` for A1–A6 from the specs. Stopword lists live in `lang.go`, about 30 short function words per language. The text is lower-cased and split on non-letters.
- [ ] **Step 4: Run** the same command. Expected: PASS. Then run `go vet ./...`.
- [ ] **Step 5: Commit** `"Fix model, rule explanations, fixes helpers; worker carries them (F8)"`.

### Task 3: A1 suggested colour

**Files:**
- Create: `backend/internal/fixes/color.go`
- Modify:
  - `backend/internal/rules/axecheck/axecheck.go`
  - `backend/internal/rules/a1/a1.go`
  - callers of `axecheck.Check` (a2, a3)
- Test:
  - `backend/internal/fixes/color_test.go`
  - `backend/internal/rules/a1/a1_test.go`

**Interfaces:**
- Consumes: `rules.Fix`, `fixes.Fill` (Task 2).
- Produces:
  - `fixes.ParseColor(s string) (r, g, b uint8, ok bool)`: accepts `#rgb`, `#rrggbb`, `rgb(…)`. Anything with alpha < 1 → `ok=false`.
  - `fixes.Contrast(fg, bg [3]uint8) float64`: WCAG relative luminance.
  - `fixes.SuggestColor(fg, bg string, min float64) (hex string, ratio float64, ok bool)`.
  - `axecheck.Check(ids []string, in rules.Input, fix func(p rules.Page, n rules.AxeNode) *rules.Fix) rules.Result`. `fix` may be nil. It is called per violation node and sets `Finding.Fix`.
  - `a1.fixFor(n rules.AxeNode) *rules.Fix`.

- [ ] **Step 1: Failing tests.**
  - `TestSuggestColorReachesRatio`, `// Spec: A1-3` (property): `testing/quick`, or 2000 seeded random fg/bg pairs × min ∈ {4.5, 3.0}. Whenever `ok`, `Contrast(ParseColor(hex), bg) >= min`. `ok` is true for every opaque pair.
  - `TestSuggestColorNearest`, `// Spec: A1-3`: `SuggestColor("#aaaaaa","#ffffff",4.5)` returns a grey (r==g==b) with ratio in [4.5, 4.7). `SuggestColor("#767676","#ffffff",4.5)` returns `#767676` (already passes).
  - `TestA1Fix`, `// Spec: A1-F`:
    - Input node Data `{"fgColor":"#aaaaaa","bgColor":"#ffffff","contrastRatio":2.32,"expectedContrastRatio":"4.5:1"}`.
    - `TextDe` starts with `Ändere die Textfarbe von #aaaaaa auf #` and ends with `:1).`.
    - `Code` is `color: <hex>;`. The ratio is printed with a comma.
  - `TestA1FixUnknownBackground`, `// Spec: A1-F` (Review Focus 2): Data without `bgColor` (and separately `bgColor:"rgba(0, 0, 0, 0)"`) gives `TextDe=="Wähle eine Textfarbe mit ausreichend Kontrast zum Hintergrund."`, `Code==""`, `NoteDe=="Hintergrund nicht eindeutig (z. B. Bild) – Kontrast bitte selbst prüfen."`.
- [ ] **Step 2: Run** `go test ./internal/fixes ./internal/rules/...`. Expected: FAIL.
- [ ] **Step 3: Implement.**
  - `SuggestColor` keeps the hue and saturation of fg (HSL). It binary-searches the lightness towards 0 and towards 1, and takes the passing candidate closer to fg's lightness. If neither passes at the ends, it falls back to black or white, whichever passes (one always does for min ≤ 4.5).
  - `min` comes from `expectedContrastRatio` (`"4.5:1"` → 4.5), defaulting to 4.5.
  - Check the real axe 4.10 key names on a live node first. Use a throwaway test that prints `Data` for a testshop low-contrast node, then delete it. Ledger the key names.
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `"A1 fix: nearest colour reaching the ratio (A1-3, A1-F)"`.

### Task 4: A2 and A3 fixes

**Files:**
- Modify: `backend/internal/rules/a2/a2.go`, `backend/internal/rules/a3/a3.go`
- Test: `a2_test.go`, `a3_test.go`

**Interfaces:**
- Consumes: `axecheck.Check(..., fix)`, `fixes.Fill`, `domutil.*`.
- Produces:
  - `a2.productName(doc *html.Node, img *html.Node) string`: nearest ancestor (max 4 levels) that contains an `h1–h4` or `[itemprop=name]`; its text, trimmed, at most 80 runes. Empty string if there is none.
  - `a3.labelFix(field *html.Node, n int) *rules.Fix`.
  - Axe nodes are mapped to the DOM node by parsing the snippet (`AxeNode.HTML`) for its attributes. A2 then finds the `img` in the page DOM with the same `src`, taking the first match. A3 needs only the snippet's attributes.

- [ ] **Step 1: Failing tests.**
  - `TestA2FixProductName`, `// Spec: A2-F`:
    - Page `<div class="card"><img src="a.jpg"><h3>Lampe "Alpin" <b>X</b></h3></div>` with an `image-alt` violation on `a.jpg` → `Code == "alt=\"Lampe &#34;Alpin&#34; X\""`, `NoteDe == ""`.
    - Without a heading → `Code == "alt=\"Beschreibung des Bildes\""`, `NoteDe == "Beschreibung bitte anpassen."`.
  - `TestA2FixFilenameAlt`, `// Spec: A2-F`: the existing `filename-alt` case gets a fix with a non-empty `TextDe`.
  - `TestA3Fix`, `// Spec: A3-F`:
    - `<input id="plz" placeholder="PLZ">` (placeholder-only) → `Code == "<label for=\"plz\">PLZ</label>"`.
    - `<input name="email">` with no id → code line 1 `<label for="email">Email</label>`, line 2 `id="email" am Feld ergänzen`.
    - `<input type="text">` with no id, name or placeholder → `for="feld-1"`, label `Beschriftung`, `NoteDe == "Beschriftungstext bitte anpassen."`.
- [ ] **Step 2: Run** `go test ./internal/rules/a2 ./internal/rules/a3`. Expected: FAIL.
- [ ] **Step 3: Implement.** The label text from `name` is capitalised, with `_`/`-` turned into spaces. `n` counts fields without id or name per page, starting at 1.
- [ ] **Step 4: Run.** Expected: PASS.
- [ ] **Step 5: Commit** `"A2/A3 fixes: product-name alt, label from placeholder/name (A2-F, A3-F)"`.

### Task 5: A4, A5, A6 fixes and the corpus gate

**Files:**
- Modify:
  - `backend/internal/rules/a4/a4.go`, `a5/a5.go`, `a6/a6.go`
  - `backend/internal/corpus/corpus_test.go`
- Test: `a4_test.go`, `a5_test.go`, `a6_test.go`, corpus test

**Interfaces:**
- Consumes: `fixes.Fill`, `fixes.DetectLang`, `domutil.Text`.
- Produces: every A4/A5/A6 finding has a `Fix`. A4 `skipped-level` needs `from`/`to`: `from` is the found level, `to` is the previous heading level + 1.

- [ ] **Step 1: Failing tests** (all with the exact copy from Task 1):
  - `TestA4Fix`, `// Spec: A4-F`: `<h1>T</h1><h3>U</h3>` → `TextDe == "Ändere <h3> zu <h2>."`. Also cases for `no-h1`, `multi-h1` and `empty`.
  - `TestA5Fix`, `// Spec: A5-F`:
    - `no-focus-style` on `#b` → `Code == "#b:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }"`. The selector goes through `Fill`, so `"` in a selector is escaped.
    - `not-focusable` → `Code == "<button type=\"button\">…</button>"`.
    - `unreachable` and `keyboard-trap` → non-empty `TextDe`, empty `Code`.
  - `TestA6Fix`, `// Spec: A6-F`:
    - A page without `lang` and with German text → `Code == "<html lang=\"de\">"`, `NoteDe == ""`.
    - With English text → `lang="en"`.
    - With little text → `lang="de"`, `NoteDe == "Sprache bitte anpassen."`.
  - `TestCorpusFindingsHaveFix`, `// Spec: A1-F…A6-F`, in corpus_test: for every case, every finding of every rule has `Fix != nil && Fix.TextDe != ""`.
- [ ] **Step 2: Run** `CHROME_PATH=/usr/bin/chromium go test ./internal/rules/... ./internal/corpus`. Expected: FAIL.
- [ ] **Step 3: Implement.** A6 reads the visible text with `domutil.Text` on `body`.
- [ ] **Step 4: Run.** Expected: PASS. The corpus recall and false-alarm numbers stay unchanged.
- [ ] **Step 5: Commit** `"A4/A5/A6 fixes; corpus: every finding carries a fix (A4-F, A5-F, A6-F)"`.

### Task 6: API: finding detail fields and crop route

**Files:**
- Modify:
  - `backend/internal/api/api.go` (`findingJSON`, routes)
  - `backend/internal/report/html.go` (export the crop bytes)
- Test: `backend/internal/api/api_test.go`

**Interfaces:**
- Consumes: `scan.Finding.Fix/ExplanationDe/Box/URL`, `scan.Scan.Shots`.
- Produces:
  - `findingJSON` gains:
    - `Selector string json:"selector"`, `Snippet json:"snippet"`, `Sub json:"sub,omitempty"`;
    - `ExplanationDe json:"explanationDe"`, `HasCrop bool json:"hasCrop"`;
    - `Fix *fixJSON json:"fix,omitempty"` with `fixJSON{Origin "origin"; TextDe "textDe"; Code "code,omitempty"; NoteDe "noteDe,omitempty"}`, where Origin is always `"template"`.
  - `report.CropJPEG(img image.Image, b report.Box, pageW float64) ([]byte, bool)`: the existing `crop` uses it.
  - `report.FindingCrop(sc scan.Scan, findingID string) ([]byte, bool)`: decodes the finding's page shot and crops.
  - Route `GET /api/scans/{id}/findings/{findingId}/crop.jpg`.
  - `HasCrop` = the finding has a `Box` and a shot exists for its URL.

- [ ] **Step 1: Failing tests.**
  - `TestFindingsDetailFields`, `// Spec: F8, A3-F`: a stored scan with one finding (`Fix{TextDe:"t",Code:"c"}`, `ExplanationDe:"e"`, selector, snippet) → JSON has `fix.origin=="template"`, `fix.textDe=="t"`, `fix.code=="c"`, `explanationDe=="e"`, `selector`, `snippet`, `hasCrop==false`.
  - `TestFindingCrop`, `// Spec: F8` (Review Focus 3):
    - A scan with a 200×200 PNG shot for the URL and a finding with Box `{10,10,50,20}` → 200 `image/jpeg`, decodable, and `hasCrop==true` in the list.
    - A finding without a Box → 404.
    - An unknown finding id → 404.
    - An unknown scan → 404.
- [ ] **Step 2: Run** `go test ./internal/api ./internal/report`. Expected: FAIL.
- [ ] **Step 3: Implement.** Keep the existing 404 and expiry handling of the findings route.
- [ ] **Step 4: Run.** Expected: PASS. All report tests still pass.
- [ ] **Step 5: Commit** `"API: finding detail fields and crop.jpg route (F8)"`.

### Task 7: Report: fix block on cards, JSON, PDF fallback

**Files:**
- Modify:
  - `backend/internal/report/report.go` (`Finding` gains `ExplanationDe string json:"explanationDe,omitempty"`, `Fix *Fix json:"fix,omitempty"` with `Fix{Origin, TextDe, Code, NoteDe}` and the same JSON names as the API)
  - `html.go`, `report.html.tmpl`, `pdf.go`
- Test: `report_test.go`, `html_test.go`

**Interfaces:**
- Consumes: `scan.Finding.Fix/ExplanationDe`.
- Produces: the card markup `<div class="fix"><h4>So können Sie es beheben <span class="origin">(Vorlage)</span></h4><p>{{TextDe}}</p>{{if Code}}<pre><code>{{Code}}</code></pre>{{end}}{{if NoteDe}}<p class="note">{{NoteDe}}</p>{{end}}</div>`. The heading level follows the card's own heading +1, so the RP-A3/A4 self-scan stays green. The builder checks the card's level.

- [ ] **Step 1: Failing tests.**
  - `TestReportJSONFix`, `// Spec: RP-13`: the JSON finding has `fix.textDe` and `explanationDe`.
  - `TestHTMLFixBlock`, `// Spec: RP-13`: the HTML contains `So können Sie es beheben` and `<pre><code>color: #595959;</code></pre>`.
  - `TestHTMLFixEscaped`, `// Spec: RP-13` (Review Focus 1): a Fix.Code `<label for="x">&#34;&gt;&lt;script&gt;</label>`, already escaped by `fixes`, is rendered so that the HTML contains no `<script>`. The visible code shows the original escaped string, because html/template escapes again.
  - `TestPDFFallbackFix`, `// Spec: RP-13`: the fpdf text PDF contains `Vorschlag:` (cp1252 encoding as in the existing tests).
- [ ] **Step 2: Run** `go test ./internal/report`. Expected: FAIL.
- [ ] **Step 3: Implement.** CSS for `.fix pre`: monospace, `white-space: pre-wrap; overflow-wrap: anywhere`, text contrast ≥ 4.5:1, and printable.
- [ ] **Step 4: Run** `CHROME_PATH=/usr/bin/chromium go test ./internal/report ./internal/e2e -run 'Report'`. Expected: PASS, including the self-scan (TestShopReports).
- [ ] **Step 5: Commit** `"Report: fix block on finding cards, JSON fix, PDF fallback (RP-13)"`.

### Task 8: Flutter finding detail page

**Files:**
- Modify:
  - `frontend/lib/api/api_client.dart` (`Finding` fields, `cropUrl`)
  - `frontend/lib/main.dart` (route)
  - `frontend/lib/screens/result_page.dart` (tile tap)
  - `frontend/test/fake_api.dart`
- Create: `frontend/lib/screens/finding_page.dart`
- Test: `frontend/test/finding_page_test.dart`, `api_client_test.dart`

**Interfaces:**
- Consumes: the findings JSON from Task 6.
- Produces:
  - `class FindingFix {origin, textDe, code?, noteDe?}`.
  - `Finding` gains `selector, snippet, sub?, explanationDe, hasCrop, fix?` (missing fields → `''`/`false`/null).
  - `ApiClient.cropUrl(String scanId, String findingId) → String`.
  - Route `/r/:id/f/:fid` → `FindingPage(api, scanId, findingId)`.

- [ ] **Step 1: Failing tests.**
  - `api_client_test`, `// Spec: F8`: parses `fix` and `hasCrop`. An old JSON without them → `fix == null`, `hasCrop == false`.
  - `finding_page_test`, `// Spec: F8`:
    - Tapping the finding tile on the result page opens the detail page.
    - It shows the explanation, the URL, the selector, the snippet, the fix text, the code, the note and the label `Vorlage`.
    - Tapping `Kopieren` puts the code on the clipboard (mock `SystemChannels.platform`, `Clipboard.setData`) and shows the SnackBar `Kopiert`.
    - `Zurück zum Ergebnis` goes back to `/r/:id`.
  - `// Spec: F8` (Review Focus 3): `hasCrop: false` → no `Image` widget. `hasCrop: true` → an `Image.network` with `cropUrl`.
  - `// Spec: F8` (Review Focus 4):
    - Opening `/r/x/f/missing` directly with FakeApi findings that lack it → `Diese Fundstelle gibt es nicht.`.
    - A 404 scan → `Diese Prüfung gibt es nicht (mehr).`.
  - `detail page fits 360 px`, `// Spec: F8` (Review Focus 5): view 360×800, a code of 300 chars without spaces → no exception (`tester.takeException()` is null). The code sits in a horizontal `SingleChildScrollView`.
- [ ] **Step 2: Run** `cd frontend && flutter test`. Expected: FAIL.
- [ ] **Step 3: Implement.**
  - The page loads with `api.getFindings(scanId)` (existing call) and picks the finding by id.
  - Selector, snippet and code use `SelectableText` in monospace.
  - A fix without code shows no box and no Kopieren button.
- [ ] **Step 4: Run** `flutter analyze && flutter test`. Expected: PASS.
- [ ] **Step 5: Commit** `"App: finding detail page with explanation, crop and copyable fix (F8)"`.

### Task 9: End-to-end, status docs, full verification

**Files:**
- Modify:
  - `backend/internal/e2e/report_test.go` (or testshops_test.go)
  - `CLAUDE.md` Status
  - `specs/10-roadmap.md` M5
  - rule spec Status lines (drop "Fix templates not yet implemented")

- [ ] **Step 1: Failing test.** `TestShopFindingsHaveFixes`, `// Spec: A1-F…A6-F, RP-13`: scan Radwerk like `scanShop` (all A rules). Every finding has `Fix.TextDe != ""`. The HTML report contains as many `class="fix"` blocks as there are finding cards.
- [ ] **Step 2: Run** `CHROME_PATH=/usr/bin/chromium go test ./internal/e2e -run TestShopFindingsHaveFixes`. Expected: FAIL only if `scanShop` drops `Fix`. In that case pass it through like the worker does. If it passes at once, ledger that the plumbing was already complete.
- [ ] **Step 3: Docs.**
  - CLAUDE.md Status: add a fix-templates line (`internal/fixes`; the detail route `/r/:id/f/:fid`; `crop.jpg`; not in pgstore).
  - Update the frontend test count.
  - Roadmap M5: fix templates done, AI suggestions open.
- [ ] **Step 4: Full verification.** `cd backend && go vet ./... && CHROME_PATH=/usr/bin/chromium go test -count=1 ./... && cd ../frontend && flutter analyze && flutter test`. Expected: all green.
- [ ] **Step 5: Commit** `"E2E: test-shop findings carry fixes; M5 fix templates done (F8)"`.
