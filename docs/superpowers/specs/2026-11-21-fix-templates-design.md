# Fix templates and finding detail: design

Date: 2026-11-21. Requirement F8 (`specs/02-requirements.md`), milestone M5, D-3 (`specs/06-detection-and-ai.md`).
The "Fix templates" and "Explanation (German)" sections of each rule spec `specs/rules/A1…A6` are the source.

## Intent

A shop owner who gets a finding should see **what is wrong, where, and what to change**. The fix must be
copyable and must carry its origin (`template`). Scans are kept for only 1 hour (ADR-6), so the downloaded
report is what lasts. The fix therefore goes into the report and into a new detail view in the app.

## Goals

- Every finding of A1–A6 gets a fix from a template: a German text, optionally a code snippet, and optionally a note.
  Origin is `template`.
- The finding carries everything F8 needs: the German explanation of the rule, URL, selector, snippet, the fix,
  and the crop screenshot when one exists.
- The fix is shown in:
  - the HTML and PDF report, on the existing finding card;
  - the JSON report and the API;
  - a new **detail page in the app** with a "Kopieren" button for the code.
- The result page's finding list links to the detail page.

## Non-goals

- **No AI suggestions** (decided by the user: later). `POST …/suggestion` stays reserved for that.
- No automatic fixing, no shop-system-specific guides (Shopify, WooCommerce …), no re-scan.
- No fixes for R rules. They do not exist yet; each R rule brings its own templates.

## Anti-goals

- **A wrong fix is worse than none.**
  - A1's suggested colour must always reach the required ratio (A1-3, property test).
  - When a value cannot be derived (for example, no background colour because there is an image), the fix says so
    in a note instead of guessing silently.
- **Page content is untrusted.** Values taken from the page (placeholder, name, product title, id) are escaped:
  - as HTML attribute values inside the code snippet;
  - again when the HTML report renders the snippet. Flutter `Text` is safe.
- Fix texts are suggestions ("Vorschlag"). They never promise that the shop is then "konform" (S-3, CLAUDE.md rule 7).

## Constraints

- Spec → test → code. Every test cites a spec ID. The template texts come from the rule specs; where a spec has no
  template for a sub-rule, the spec gets one first.
- User-facing text is German, code is English.
- Scans are in memory only (ADR-6). pgstore is off by default.

## Approaches (picked by the user: 1)

1. **Each rule writes its fix while checking (chosen).** The rule already has the data at hand: axe colours for A1,
   placeholder/name for A3, the product title for A2, the selector for A5, the page text for A6. A small shared
   package `internal/fixes` holds the placeholder filling, the escaping and the colour maths.
2. **A separate step after the rules** reads each page's HTML again by selector. Rejected: the page HTML and the
   axe data would have to be kept, and the work would be done twice.
3. **Lazily when a finding is opened** (`POST /suggestion`). Rejected: the report needs every fix anyway, and the
   page data is gone after the scan. This path fits the later AI suggestions.

## Design

### Data

- `rules.Fix{TextDe, Code, NoteDe string}`, and `rules.Finding.Fix *Fix`. The origin is implicitly `template` now; the
  JSON says `"origin":"template"`.
- `rules.Meta.ExplanationDe`: the German explanation from the rule spec.
- `scan.Finding.Fix` and `scan.Finding.ExplanationDe` are carried by the worker.
- pgstore does not persist the fix, like `Position`/`Box`. This is documented in CLAUDE.md, and pgstore is off by
  default. *(Builder's call; no migration.)*

### Templates per rule (from the specs; additions marked "new" go into the spec first)

| Rule / sub | Text | Code |
| --- | --- | --- |
| A1 | `Ändere die Textfarbe von {{fg}} auf {{suggestedFg}} (Kontrast {{ratio}}:1).` | `color: {{suggestedFg}};` |
| A1, background unknown (new) | text without a colour, plus a note: "Hintergrund nicht eindeutig (z. B. Bild) – Kontrast bitte selbst prüfen." | none |
| A2, product image | `Beschreibe das Bild, z. B. mit dem Produktnamen.` | `alt="{{productName}}"` |
| A2, other / `filename-alt` | same text, with the note "Beschreibung bitte anpassen." | `alt="Beschreibung des Bildes"` |
| A3 (incl. `placeholder-only`) | `Gib dem Feld eine sichtbare Beschriftung.` | `<label for="{{id}}">{{labelText}}</label>` (+ `id="{{id}}"` on the field if it has none) |
| A4 `skipped-level` | `Ändere <h{{from}}> zu <h{{to}}>.` | none |
| A4 no h1 / two h1 / `empty` (new where missing) | `Füge genau eine <h1> mit dem Seitentitel hinzu.` / `Behalte nur eine <h1> …` / `Gib der Überschrift einen Text oder entferne sie.` | none |
| A5 `no-focus-style` | `Mach den Fokus sichtbar.` | `{{selector}}:focus-visible { outline: 2px solid currentColor; outline-offset: 2px; }` |
| A5 `not-focusable` | `Verwende ein <button>-Element oder ergänze role und tabindex samt Tastenbedienung (Enter/Leertaste).` | `<button type="button">…</button>` |
| A5 `unreachable` / `keyboard-trap` (new) | text only: remove `tabindex="-1"` or hidden-focus tricks / let Tab leave the element | none |
| A6 | `Setze das Attribut lang im html-Element.` | `<html lang="{{lang}}">` (note "bitte anpassen" when the detector is unsure) |

The exact wording is left to the builder, within the spec. The builder also checks each rule's real sub-rule names
in the code.

### Helpers (`internal/fixes`)

- `Fill(template, values)` fills placeholders and HTML-escapes every value.
- A1 colour: the nearest colour (same hue, lightness changed towards the far end) reaching the required ratio
  (4.5:1, or 3:1 for large text). Input comes from axe `color-contrast` check data (`fgColor`, `bgColor`, `fontSize`,
  `fontWeight`). Property test A1-3: for random fg/bg pairs, the suggestion reaches the ratio.
- A2 product name: the nearest heading or the product title in the same card or ancestor (`h1–h4`, `[itemprop=name]`).
  If there is none, use the generic template.
- A3 label text: from `placeholder`, else `name` ("plz" → "Plz"), else "Beschriftung" with a note. The id comes from the
  field, else `name`, else `feld-{{n}}`.
- A6 language detector: stopword counts over the visible text for de, en, fr, it, es, nl. If unsure, use "de" with a note.

### API

- `GET /api/scans/{id}/findings` additionally returns per finding:
  - `selector`, `snippet`, `sub`, `explanationDe`;
  - `fix {origin, textDe, code, noteDe}`;
  - `hasCrop`.
  This is additive.
- **New** `GET /api/scans/{id}/findings/{findingId}/crop.png`: the cropped screenshot of the element, from
  `scan.Shots`. Returns 404 if there is none.
- `specs/08-api.yaml`: the planned `/api/findings/{id}` becomes scan-scoped, because finding ids are unique only
  within a scan and there is no global store (ADR-6). `FindingDetail` gets `fix`. *(My call.)*
- The JSON report (`report.json`, version stays 1) gets `explanationDe` and `fix` per finding. This is additive.

### Report

- HTML card: below the existing text, a block "So können Sie es beheben (Vorlage)" with the text, a `<pre><code>`
  block (escaped) and the note. No script; it must still pass A1–A6 itself (RP-A2/RP-A3 self-scan).
- PDF: printed from the HTML, so it comes along. The fpdf fallback adds "Vorschlag: …" plus the code line.

### App

- The result page's finding tile becomes tappable and goes to `/r/:id/f/:findingId`.
- The detail page shows:
  - severity and rule name;
  - explanation;
  - page URL (link), selector and snippet (monospace);
  - the crop image when `hasCrop`;
  - the fix text, the code in a monospace box with a "Kopieren" button (clipboard, then a SnackBar "Kopiert"), and
    the note;
  - the label "Vorlage";
  - a back link "Zurück zum Ergebnis".
- Data comes from the findings list already loaded for the scan. A direct link reloads it. After expiry the page
  shows the existing 404 text.

```
← Zurück zum Ergebnis
Kritisch · A3 Formularfelder beschriftet
Dieses Eingabefeld hat keine Beschriftung, …
Seite: https://shop.at/kasse.html
Element: #plz      <input id="plz" placeholder="PLZ">
[ Ausschnitt-Screenshot ]
So können Sie es beheben  (Vorlage)
Gib dem Feld eine sichtbare Beschriftung.
┌──────────────────────────────────┐
│ <label for="plz">PLZ</label>     │  [Kopieren]
└──────────────────────────────────┘
```

### Tests

- Per rule: a unit test per template or sub-rule with the spec's exact values (`// Spec: A1-3`, `A3-x` …).
- A new acceptance ID per rule for "every finding carries a fix".
- Corpus: every finding of every `a*` case has a non-empty `Fix.TextDe`.
- `fixes`:
  - escaping (a placeholder with `"` and `<`);
  - the A1 property test;
  - the language detector on German, English and mixed text.
- API: findings JSON fields and crop 200/404. Report: the card shows the fix, the code is escaped, the self-scan
  stays green.
- Flutter: tile → detail page; Kopieren puts the code on the clipboard; the 404 case.
- E2E (test shops): every finding in the Radwerk report has a fix.

## Left to the builder

- Exact German wording within the spec.
- The colour search method, as long as A1-3 holds.
- The stopword lists.
- Layout details of the card and the detail page.
- Whether `ExplanationDe` gets per-sub-rule variants (only where a spec gives one).
