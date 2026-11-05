# 06 Detection and AI fallback

Status: draft · Principle: **algorithm first, Claude only if the algorithm is unsure** (ADR-3)

## Detection pipeline (D-1)

Every rule that needs to understand text runs these stages in order and stops at the first confident result:

1. **Structural**: DOM, attributes, schema.org data, known CMP/plugin signatures (e.g. WooCommerce, Complianz, Cookiebot, OneTrust, Borlabs).
2. **Dictionary**: per-language term lists (German and English first; one file per language in `backend/internal/lexicon/`).
   Examples: legal-notice link texts, withdrawal terms, order-button phrases, reject/accept labels.
3. **Pattern**: regex/validators (e-mail, EU VAT number formats per country, postal address heuristics, URL patterns).
4. **Score**: each stage yields a confidence 0..1. Result is `confident` if ≥ 0.8 (threshold in config, tuned on the corpus).
5. **Fallback (Claude)**: only if confidence < 0.8 **and** the rule's spec allows fallback (field `ai_fallback: yes`).
6. If the fallback also stays below threshold or is disabled → `not_assessable`.

## When Claude is allowed (D-2)

| Use | Allowed | Why |
| --- | --- | --- |
| Extract provider fields from a legal-notice page in a language without dictionary or with odd layout (R1) | yes | free-form text |
| Decide whether an unknown banner has a reject option (R3) | yes | arbitrary UI |
| Classify an unknown third-party domain as tracker/non-tracker (R4) | yes, cached per domain | unknown domains |
| Find withdrawal information in terms text (R5, R7) | yes | free-form text |
| Contrast, alt text, labels, headings, lang, keyboard order (A1–A6) | **no** | deterministic |
| Computing score or severity | **no** | deterministic (S-4) |
| Writing a fix for a finding with a **template** | **no** | use template |
| Writing a fix for a finding without a template | yes, on demand | no template exists |

## Fix suggestions (D-3)

- Each rule defines `fix_templates` (German text, code snippet with placeholders such as `{{selector}}`, `{{lang}}`). Templates are used first.
- Claude writes a suggestion only if no template fits or the template needs context it cannot get from the DOM. Generated lazily when the user opens the finding, then cached by hash.
- Every suggestion carries `origin: template | ai`. AI suggestions show "KI-Vorschlag – vor Einsatz prüfen".

## Claude integration (D-4)

- Provider: Anthropic API, behind a Go interface `llm.Client` so tests use a fake.
- Models configurable per use (env `LLM_MODEL_EXTRACT`, `LLM_MODEL_SUGGEST`); defaults to be set in `deploy/.env.example`: a small fast model for classification/extraction, a stronger one for suggestions.
- Input limit: public HTML/text excerpts ≤ 4 000 characters; never account data (none exists).
- Output: JSON validated against a schema; invalid JSON → one retry → `not_assessable`.
- **Evidence rule (D-5):** an extraction must quote the source text. A field without quote counts as "not found".
- **No promotion (D-6):** the model may fill in unknown facts but may never turn a deterministic `failed` into `passed`.
- Max 2 extraction calls per scan for the MVP (N1/N9); a global daily call budget stops the fallback and yields `not_assessable`.
- Every AI-derived result stores model name, prompt version and the quoted evidence.
- API key from environment only; never committed.

## Metrics (D-7)

The corpus run reports, per rule: share of results decided at each stage and share needing Claude. Target for the MVP: ≥ 85 % of
R-rule decisions without Claude on the test corpus (to be revised once measured).
