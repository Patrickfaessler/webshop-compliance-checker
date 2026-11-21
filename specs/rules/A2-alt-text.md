# A2 Alternative text for images

- **Status:** verified (2026-11-05): corpus 3 defect + 3 clean, recall 3/3, 0 false alarms; axe image rules plus DOM check for file-name alt texts (A2-3). Code: `backend/internal/rules/a2`. Fix templates implemented and tested (2026-11-21, A2-F; code in `backend/internal/fixes` and the rule). ·  **legal-checked:** n/a · **Area:** accessibility · **Severity:** warning
- **Basis:** WCAG 2.1 SC 1.1.1 (EN 301 549 clause 9.1.1.1)
- **applies_when:** page has `img`, `input[type=image]`, `area`, or `svg[role=img]`
- **ai_fallback:** no (detection); suggestion text for alt may use AI only if no template fits (D-3)

## Logic
1. axe-core `image-alt`, `input-image-alt`, `area-alt`, `svg-img-alt`.
2. `alt=""` is valid only for decorative images (`role="presentation"`/`aria-hidden`) or when the image is inside a link/button that already has text.
3. Alt that equals the file name (e.g. `IMG_1234.jpg`) → violation (algorithmic filename pattern).

## Result mapping
passed / failed / not_applicable as for A1; no `not_assessable` case.

## Finding
URL, selector, snippet (img tag), `src`.

## Explanation (German)
Bilder ohne Beschreibung sind für blinde Menschen unsichtbar. Jedes inhaltliche Bild braucht einen kurzen Alternativtext.

## Fix templates
Text for all sub-rules: `Beschreibe das Bild in wenigen Worten, z. B. mit dem Produktnamen.`
- Product image: code `alt="{{productName}}"` (deterministic, trimmed, ≤ 80 characters). The name is a heading or `[itemprop=name]` in the image's card: the closest of at most 4 ancestors, stopping at an ancestor with more than one image (no longer one card) and at body/main/header/footer/nav; a heading that comes before the image only counts inside the image's direct parent. The image is located by its `src`; when several images share that `src` (lazy-load placeholders), no name is used.
- `svg-img-alt`: text `Gib der Grafik einen kurzen Namen.`, code `aria-label="Beschreibung der Grafik"`, note `Beschreibung bitte anpassen.`
- Otherwise (incl. `filename-alt`): code `alt="Beschreibung des Bildes"`, note `Beschreibung bitte anpassen.`
- Decorative images are mentioned in the note when no product name is found: `Beschreibung bitte anpassen. Rein dekorative Bilder bekommen alt="".`
- AI suggestion only on user request (later, D-3).

## Acceptance
- A2-1: `<img src=a.jpg>` fails; `alt="Rotes T-Shirt"` passes.
- A2-2: decorative `alt=""` with `role=presentation` passes.
- A2-3: `alt="IMG_0012.jpg"` fails.
- A2-F: every finding carries a fix (origin `template`, non-empty German text); page values in the code are HTML-escaped.
