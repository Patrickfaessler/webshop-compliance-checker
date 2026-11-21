package crawler

import (
	"net/url"
	"slices"
	"strings"
)

// Keyboard probe settings. Spec: specs/rules/A5-keyboard.md ("Modes and pages").
const (
	DefaultKeyboardPresses = 200
	MinKeyboardPresses     = 50
	MaxKeyboardPresses     = 1000
	MaxKeyboardSample      = 5 // pages in mode Stichprobe (A5-5)
)

// KeyboardOptions selects the A5 mode for one scan.
type KeyboardOptions struct {
	AllPages   bool // mode "alle Seiten" (A5-6)
	MaxPresses int  // Tab limit per page; 0 = DefaultKeyboardPresses (A5-8)
}

// Presses returns the Tab limit to use.
func (o KeyboardOptions) Presses() int {
	if o.MaxPresses <= 0 {
		return DefaultKeyboardPresses
	}
	return o.MaxPresses
}

// PageType is the kind of shop page, used to pick the keyboard sample (A5-5).
type PageType string

const (
	PageStart    PageType = "start"
	PageCategory PageType = "category"
	PageProduct  PageType = "product"
	PageCart     PageType = "cart"
	PageCheckout PageType = "checkout"
	PageOther    PageType = ""
)

// pageSignals are read from the rendered DOM for the page type.
type pageSignals struct {
	LDTypes      []string `json:"ldTypes"`      // JSON-LD @type values
	OGType       string   `json:"ogType"`       // <meta property="og:type">
	ProductLinks int      `json:"productLinks"` // distinct links to product-like URLs
}

var (
	checkoutWords = []string{"kasse", "checkout"}
	cartWords     = []string{"warenkorb", "cart", "basket"}
	productWords  = []string{"produkt", "product", "/p/"}
	categoryWords = []string{"kategorie", "category", "collection", "/c/"}
)

func pathHas(path string, words []string) bool {
	return slices.ContainsFunc(words, func(w string) bool { return strings.Contains(path, w) })
}

// isProductURL reports whether a URL path looks like a product page.
func isProductURL(path string) bool { return pathHas(strings.ToLower(path), productWords) }

// classifyPage returns the page type from the URL path and the DOM signals. It never returns PageStart.
func classifyPage(pageURL string, sig pageSignals) PageType {
	path := pageURL
	if u, err := url.Parse(pageURL); err == nil {
		path = u.Path
	}
	path = strings.ToLower(path)
	ld := func(types ...string) bool {
		return slices.ContainsFunc(sig.LDTypes, func(t string) bool { return slices.Contains(types, t) })
	}
	switch {
	case pathHas(path, checkoutWords):
		return PageCheckout
	case pathHas(path, cartWords):
		return PageCart
	case ld("Product") || strings.EqualFold(sig.OGType, "product") || pathHas(path, productWords):
		return PageProduct
	case ld("ItemList", "CollectionPage") || pathHas(path, categoryWords) || sig.ProductLinks >= 4:
		return PageCategory
	}
	return PageOther
}

// keyboardSampler decides per crawled page whether the keyboard probe runs (A5-5, A5-6). One per crawl.
type keyboardSampler struct {
	all   bool
	seen  bool // the start page was decided
	taken map[PageType]bool
}

func newKeyboardSampler(o KeyboardOptions) *keyboardSampler {
	return &keyboardSampler{all: o.AllPages, taken: map[PageType]bool{}}
}

// decide is called once per page in crawl order; the first call is the start page.
func (k *keyboardSampler) decide(pageURL string, sig pageSignals) (PageType, bool) {
	if !k.seen {
		k.seen = true
		k.taken[PageStart] = true
		return PageStart, true
	}
	t := classifyPage(pageURL, sig)
	if k.all {
		return t, true
	}
	if t == PageOther || k.taken[t] || len(k.taken) >= MaxKeyboardSample {
		return t, false
	}
	k.taken[t] = true
	return t, true
}
