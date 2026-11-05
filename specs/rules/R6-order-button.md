# R6 Order button states the payment obligation

- **Status:** draft · **legal-checked:** no · **Area:** legal · **Severity:** critical
- **Basis:** Directive 2011/83/EU Art. 8(2): the button must be labelled "order with obligation to pay" or an unambiguous equivalent
- **applies_when:** shop sells to consumers; checkout reachable (C-3)
- **ai_fallback:** yes (judge unknown phrasing)

## Input
Last checkout step (test shop: reached with test data, **not submitted**). Third-party shops: only if reachable without input.

## Logic
1. Identify the final submit control (button/input in the form containing payment/order fields; WooCommerce `#place_order`).
2. Normalise its label (lowercase, trim, strip icons).
3. Match against the lexicon of **accepted** phrases per language (`zahlungspflichtig bestellen`, `kaufen`-variants only if with payment wording, `order with obligation to pay`, `commande avec obligation de paiement`, …) and **rejected** neutral phrases (`bestellen`, `weiter`, `absenden`, `submit`, `confirm`, `place order`, `jetzt kaufen` alone).
4. Phrase in neither list → Claude fallback; evidence = the label itself.

## Result mapping
| Situation | State |
| --- | --- |
| label in accepted list | passed |
| label in rejected list | failed, label quoted |
| label unknown, fallback says unclear | not_assessable |
| checkout not reachable without input | not_assessable (`checkout-not-reachable`) |
| no checkout (catalogue only) | not_applicable |

## Finding
Checkout URL, selector, **label verbatim**, screenshot.

## Explanation (German)
Der letzte Knopf im Bestellvorgang muss klar sagen, dass der Kauf zahlungspflichtig ist, zum Beispiel „Zahlungspflichtig bestellen“.

## Fix templates
`WooCommerce: Filter woocommerce_order_button_text → 'Zahlungspflichtig bestellen'` snippet; generic: change button label.

## Acceptance
- R6-1: "Zahlungspflichtig bestellen" passes. R6-2: "Weiter" fails. R6-3: "Place order" fails. R6-4: unknown label → fake LLM path. R6-5: no input needed → not_assessable on third-party fixture.
