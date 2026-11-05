# ADR-3 Algorithms first, Claude as fallback

Status: accepted (2026-11-05)

**Decision.** All checks, scoring and fix suggestions use deterministic algorithms (DOM analysis, axe-core, dictionaries, regex, tracker lists, templates). Claude (Anthropic API) is called only when an algorithm reports confidence below threshold and the rule allows fallback (`06-detection-and-ai.md`).

**Why.** Reproducible results, low cost and latency, easy testing, clearer story for legal reviewers; AI where text is truly free-form.

**Consequences.**
- Every result stores `decided_by` (structural, dictionary, pattern, ai).
- Claude can never turn a deterministic `failed` into `passed` (D-6) and must quote evidence (D-5).
- Tests use a fake LLM; the corpus run reports the share of decisions that needed Claude (N9).
- Provider sits behind `llm.Client` so it can be swapped.
