package legal

import (
	"net/url"
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

// FindLinked finds the page of the given kind (R1/R2/R5 Logic 1): a link whose text or URL path matches the de/en
// lexicon, footer/header/nav links first, matched against the crawled pages by URL or requested URL (ignoring the
// fragment and a trailing slash). The first loaded match wins; else the first match is returned unloaded. Without a
// link, a crawled page whose path matches counts.
func FindLinked(in rules.Input, kind lexicon.Kind) Link {
	var first *Link
	for _, p := range in.Pages {
		for _, a := range anchors(p) {
			if !matches(kind, a.text, a.href) {
				continue
			}
			if i := pageIndex(in.Pages, a.href); i >= 0 {
				return Link{Found: true, Href: a.href, Loaded: true, Page: &in.Pages[i]}
			}
			if first == nil {
				first = &Link{Found: true, Href: a.href}
			}
		}
	}
	if first != nil {
		return *first
	}
	for i, p := range in.Pages {
		if u, err := url.Parse(p.URL); err == nil && pathMatches(kind, u.Path) {
			return Link{Found: true, Href: p.URL, Loaded: true, Page: &in.Pages[i]}
		}
	}
	return Link{}
}

type anchor struct{ href, text string }

// anchors lists the links of a page, those inside header/nav/footer first; hrefs are absolute without fragment.
func anchors(p rules.Page) []anchor {
	var top, rest []anchor
	var walk func(n *html.Node, inChrome bool)
	walk = func(n *html.Node, inChrome bool) {
		if domutil.IsElement(n, "header", "nav", "footer") {
			inChrome = true
		}
		if domutil.IsElement(n, "a") {
			if h, ok := domutil.Attr(n, "href"); ok {
				if abs, ok := resolve(p.URL, h); ok {
					a := anchor{abs, strings.ToLower(collapse(domutil.Text(n)))}
					if inChrome {
						top = append(top, a)
					} else {
						rest = append(rest, a)
					}
				}
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c, inChrome)
		}
	}
	walk(domutil.Parse(p.HTML), false)
	return append(top, rest...)
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

// matches reports whether a link of the given text and target is a link to a page of this kind. A link that
// names the withdrawal page never counts as the terms link.
func matches(kind lexicon.Kind, text, href string) bool {
	if kind == lexicon.Terms && textMatches(lexicon.Withdrawal, text) {
		return false
	}
	if textMatches(kind, text) {
		return true
	}
	u, err := url.Parse(href)
	return err == nil && pathMatches(kind, u.Path) && !(kind == lexicon.Terms && pathMatches(lexicon.Withdrawal, u.Path))
}

func textMatches(kind lexicon.Kind, text string) bool {
	if text == "" {
		return false
	}
	for _, lang := range []string{"de", "en"} {
		l, _ := lexicon.For(lang)
		for _, w := range l.Links[kind] {
			if lexicon.ContainsWord(text, w) {
				return true
			}
		}
	}
	return false
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
