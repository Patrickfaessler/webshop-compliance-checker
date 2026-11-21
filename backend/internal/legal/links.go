package legal

import (
	"net/url"
	"sort"
	"strings"

	"golang.org/x/net/html"

	"compliancechecker/internal/domutil"
	"compliancechecker/internal/lexicon"
	"compliancechecker/internal/rules"
)

// Link is where a kind of legal page was found among the crawled pages.
type Link struct {
	Found  bool        // a link, or a crawled page whose path matches, exists
	Href   string      // absolute target without fragment; "" if !Found
	Loaded bool        // the target is one of Input.Pages
	Page   *rules.Page // the loaded target; nil if !Loaded
}

// FindLinked finds the page of the given kind (R1/R2/R5 Logic 1): the best-ranked candidate (see FindAllLinked).
// If it was not crawled, the result is Found but not Loaded, never a weaker loaded page.
func FindLinked(in rules.Input, kind lexicon.Kind) Link {
	if all := FindAllLinked(in, kind); len(all) > 0 {
		return all[0]
	}
	return Link{}
}

// FindAllLinked lists the candidate pages of a kind, best first, one per target. Ranking (R1 Logic 1): links in
// page chrome (header/nav/footer, role=contentinfo, class/id with "footer") first; then link text that is nearly
// just the lexicon word; then specific words before generic ones ("Rechtliches"); among equal ranks a loaded target
// first; then document order. A link "#", "javascript:" or a fragment without target counts as found, not loaded.
// Without any link, a crawled page whose path or main heading names the kind counts.
func FindAllLinked(in rules.Input, kind lexicon.Kind) []Link {
	var cands []cand
	order := 0
	for _, p := range in.Pages {
		for _, a := range anchors(p) {
			m, ok := match(kind, a.text, a.href)
			if !ok {
				continue
			}
			c := cand{rank: [3]int{boolInt(!a.chrome), m.loose, m.generic}, order: order, link: Link{Found: true, Href: a.href}}
			order++
			if !a.modal {
				if i := pageIndex(in.Pages, a.href); i >= 0 {
					c.link.Loaded, c.link.Page = true, &in.Pages[i]
				}
			}
			cands = append(cands, c)
		}
	}
	if len(cands) == 0 {
		for i, p := range in.Pages {
			if u, err := url.Parse(p.URL); err == nil && (pathMatches(kind, u.Path) || headingMatches(kind, p)) {
				return []Link{{Found: true, Href: p.URL, Loaded: true, Page: &in.Pages[i]}}
			}
		}
		return nil
	}
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.rank != b.rank {
			return less(a.rank, b.rank)
		}
		if a.link.Loaded != b.link.Loaded {
			return a.link.Loaded
		}
		return a.order < b.order
	})
	var out []Link
	seen := map[string]bool{}
	for _, c := range cands {
		k := key(c.link.Href)
		if c.link.Page == nil {
			k = "unloaded:" + k
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, c.link)
	}
	return out
}

type cand struct {
	rank  [3]int // chrome, loose text match, generic word: 0 is better
	order int
	link  Link
}

func less(a, b [3]int) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type anchor struct {
	href, text    string
	chrome, modal bool
}

// anchors lists the links of a page with absolute hrefs (fragment removed). chrome marks links inside page chrome;
// modal marks "#", "javascript:" and fragments that point at no element of the page.
func anchors(p rules.Page) []anchor {
	doc := domutil.Parse(p.HTML)
	ids := map[string]bool{}
	domutil.Walk(doc, func(n *html.Node) bool {
		if id, ok := domutil.Attr(n, "id"); ok {
			ids[id] = true
		}
		if n.Type == html.ElementNode && domutil.IsElement(n, "a") {
			if name, ok := domutil.Attr(n, "name"); ok {
				ids[name] = true
			}
		}
		return true
	})
	var out []anchor
	var walk func(n *html.Node, chrome bool)
	walk = func(n *html.Node, chrome bool) {
		if isChrome(n) {
			chrome = true
		}
		if domutil.IsElement(n, "a") {
			if h, ok := domutil.Attr(n, "href"); ok {
				h = strings.TrimSpace(h)
				a := anchor{text: strings.ToLower(collapse(domutil.Text(n))), chrome: chrome}
				low := strings.ToLower(h)
				switch {
				case h == "" || h == "#" || strings.HasPrefix(low, "javascript:"):
					a.href, a.modal = p.URL, true
				case strings.HasPrefix(h, "#"):
					a.href, a.modal = p.URL, !ids[strings.TrimPrefix(h, "#")]
				default:
					abs, ok := resolve(p.URL, h)
					if !ok {
						return
					}
					a.href = abs
				}
				out = append(out, a)
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, chrome)
		}
	}
	walk(doc, false)
	return out
}

func isChrome(n *html.Node) bool {
	if domutil.IsElement(n, "header", "nav", "footer") {
		return true
	}
	if n.Type != html.ElementNode {
		return false
	}
	if r, _ := domutil.Attr(n, "role"); r == "contentinfo" || r == "navigation" {
		return true
	}
	c, _ := domutil.Attr(n, "class")
	id, _ := domutil.Attr(n, "id")
	return strings.Contains(strings.ToLower(c+" "+id), "footer")
}

func resolve(base, href string) (string, bool) {
	b, err := url.Parse(base)
	if err != nil {
		return "", false
	}
	u, err := b.Parse(strings.TrimSpace(href))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "file") {
		return "", false
	}
	u.Fragment = ""
	return u.String(), true
}

type matchInfo struct{ loose, generic int }

// match reports whether a link is one to a page of this kind, and how well. A link that names only the withdrawal
// page never counts as the terms link; "AGB & Widerrufsbelehrung" counts for both (R5-9).
func match(kind lexicon.Kind, text, href string) (matchInfo, bool) {
	if kind == lexicon.Terms {
		if _, wd := textMatch(lexicon.Withdrawal, text); wd {
			if _, t := textMatch(lexicon.Terms, text); !t {
				return matchInfo{}, false
			}
		}
	}
	if w, ok := textMatch(kind, text); ok {
		return matchInfo{loose: boolInt(len(text) > len(w)+12)}, true
	}
	if kind == lexicon.LegalNotice || kind == lexicon.Privacy {
		if w, ok := genericMatch(text); ok {
			return matchInfo{loose: boolInt(len(text) > len(w)+12), generic: 1}, true
		}
	}
	u, err := url.Parse(href)
	if err != nil || !pathMatches(kind, u.Path) {
		return matchInfo{}, false
	}
	return matchInfo{loose: 1}, true
}

// textMatch returns the first de/en link word of kind that the link text contains at a word start.
func textMatch(kind lexicon.Kind, text string) (string, bool) {
	if text == "" {
		return "", false
	}
	for _, lang := range []string{"de", "en"} {
		l, _ := lexicon.For(lang)
		for _, w := range l.Links[kind] {
			if lexicon.ContainsWord(text, w) {
				return w, true
			}
		}
	}
	return "", false
}

func genericMatch(text string) (string, bool) {
	if text == "" {
		return "", false
	}
	for _, lang := range []string{"de", "en"} {
		l, _ := lexicon.For(lang)
		for _, w := range l.Generic {
			if lexicon.ContainsWord(text, w) {
				return w, true
			}
		}
	}
	return "", false
}

// headingMatches: the page's first h1/h2 names the kind (a page reached without a link).
func headingMatches(kind lexicon.Kind, p rules.Page) bool {
	var h string
	domutil.Walk(domutil.Parse(p.HTML), func(n *html.Node) bool {
		if h != "" {
			return false
		}
		if domutil.IsElement(n, "h1", "h2") {
			h = strings.ToLower(collapse(domutil.Text(n)))
			return false
		}
		return true
	})
	w, ok := textMatch(kind, h)
	return ok && len(h) <= len(w)+12
}

// pathMatches checks the URL path against the link words written without spaces or with hyphens.
func pathMatches(kind lexicon.Kind, path string) bool {
	p := strings.ToLower(path)
	if dec, err := url.PathUnescape(p); err == nil {
		p = dec
	}
	for _, lang := range []string{"de", "en"} {
		l, _ := lexicon.For(lang)
		for _, w := range l.Links[kind] {
			for _, v := range []string{strings.ReplaceAll(w, " ", "-"), strings.ReplaceAll(w, " ", "")} {
				if !strings.ContainsAny(v, "&") && lexicon.ContainsWord(p, v) {
					return true
				}
			}
		}
	}
	return false
}

// pageIndex finds the crawled page whose URL or requested URL is target, ignoring a trailing slash.
func pageIndex(pages []rules.Page, target string) int {
	t := key(target)
	for i, p := range pages {
		if key(p.URL) == t || (p.RequestedURL != "" && key(p.RequestedURL) == t) {
			return i
		}
	}
	return -1
}

func key(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Fragment = ""
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	if u.Path == "" {
		u.Path = "/"
	}
	if len(u.Path) > 1 {
		u.Path = strings.TrimSuffix(u.Path, "/")
	}
	return u.String()
}
