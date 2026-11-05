# A2 Alternative text for images

- **Status:** verified (2026-11-05): corpus 3 defect + 3 clean, recall 3/3, 0 false alarms; axe image rules plus DOM check for file-name alt texts (A2-3). Code: `backend/internal/rules/a2`. Fix templates not yet implemented. ·  **legal-checked:** n/a · **Area:** accessibility · **Severity:** warning
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
- Product image: `alt="{{productName}}"` where the name comes from the surrounding product title (deterministic).
- Otherwise: `alt="Beschreibung des Bildes"` with note; AI suggestion allowed only on user request.

## Acceptance
- A2-1: `<img src=a.jpg>` fails; `alt="Rotes T-Shirt"` passes.
- A2-2: decorative `alt=""` with `role=presentation` passes.
- A2-3: `alt="IMG_0012.jpg"` fails.
