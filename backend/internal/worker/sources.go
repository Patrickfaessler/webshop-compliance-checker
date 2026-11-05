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

func (c CrawlSource) Pages(ctx context.Context, startURL string, progress func(done, total int)) ([]rules.Page, bool, error) {
	res, err := c.Crawler.Crawl(ctx, startURL, c.Options, c.Robots, progress)
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

func (f FetchSource) Pages(ctx context.Context, startURL string, progress func(done, total int)) ([]rules.Page, bool, error) {
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
	return []rules.Page{{URL: p.FinalURL, HTML: string(p.Body)}}, false, nil
}
