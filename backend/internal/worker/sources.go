package worker

import (
	"context"
	"errors"

	"compliancechecker/internal/crawler"
	"compliancechecker/internal/fetch"
	"compliancechecker/internal/rules"
)

// CrawlSource crawls the site in a headless browser (up to Options.MaxPages pages, axe-core on each).
type CrawlSource struct {
	Crawler *crawler.Crawler
	Options crawler.Options
	Robots  crawler.Getter // fetches robots.txt; nil = ignore robots.txt
}

func (c CrawlSource) Pages(ctx context.Context, startURL string, kb crawler.KeyboardOptions, progress func(done, total int)) ([]rules.Page, bool, error) {
	opt := c.Options
	opt.Keyboard = kb // A5 mode and Tab limit of this scan
	res, err := c.Crawler.Crawl(ctx, startURL, opt, c.Robots, progress)
	if err != nil && len(res.Pages) == 0 {
		return nil, res.Incomplete, err
	}
	return res.Pages, res.Incomplete, nil
}

// FetchSource loads only the start page over plain HTTP. No browser, so no axe-core results:
// rules that need axe report not_assessable. Fallback when no browser is installed.
type FetchSource struct {
	Client *fetch.Client
}

// No browser, so no keyboard probe either: A5 reports not_assessable.
func (f FetchSource) Pages(ctx context.Context, startURL string, _ crawler.KeyboardOptions, progress func(done, total int)) ([]rules.Page, bool, error) {
	p, err := f.Client.Get(ctx, startURL)
	if err != nil {
		return nil, false, err
	}
	if p.Status >= 400 {
		return nil, false, errors.New("start page returned an error status")
	}
	if progress != nil {
		progress(1, 1)
	}
	return []rules.Page{{URL: p.FinalURL, RequestedURL: startURL, HTML: string(p.Body)}}, false, nil
}
