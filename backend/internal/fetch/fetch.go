// Package fetch is an HTTP client that cannot reach internal networks.
// Every connection, including those after redirects, is validated and pinned to the vetted IP.
// Spec: specs/05-crawler.md (C-SSRF), N6. The Playwright crawler gets the same protection through the egress proxy.
package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"compliancechecker/internal/ssrf"
)

const (
	maxRedirects = 5
	UserAgent    = "ComplianceCheckerBot/0.1 (research project; contact URL to be added)"
)

// Client fetches public pages only.
type Client struct {
	hc       *http.Client
	lookup   ssrf.LookupFunc
	MaxBytes int64
}

// New returns a client. lookup may be nil to use the system resolver.
func New(lookup ssrf.LookupFunc) *Client {
	if lookup == nil {
		lookup = ssrf.DefaultLookup
	}
	c := &Client{lookup: lookup, MaxBytes: 2 << 20}
	c.hc = &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			Proxy:                 nil, // never via environment proxies: they would bypass the checks
			DialContext:           c.dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          10,
		},
		CheckRedirect: c.checkRedirect,
	}
	return c
}

// dial resolves, validates and connects to a vetted address (pinning against DNS rebinding).
func (c *Client) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	scheme := "https"
	if port == "80" {
		scheme = "http"
	}
	_, addrs, err := ssrf.Validate(ctx, scheme+"://"+net.JoinHostPort(host, port)+"/", c.lookup)
	if err != nil {
		return nil, err
	}
	p, _ := strconv.ParseUint(port, 10, 16)
	var d net.Dialer
	var lastErr error
	for _, a := range addrs {
		conn, err := d.DialContext(ctx, network, netip.AddrPortFrom(a, uint16(p)).String())
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (c *Client) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("too many redirects")
	}
	_, _, err := ssrf.Validate(req.Context(), req.URL.String(), c.lookup)
	return err
}

// Page is a fetched document.
type Page struct {
	FinalURL string
	Status   int
	Body     []byte
}

// Get fetches rawURL. The URL is validated first so callers get a clean ErrBlocked.
func (c *Client) Get(ctx context.Context, rawURL string) (Page, error) {
	if _, _, err := ssrf.Validate(ctx, rawURL, c.lookup); err != nil {
		return Page{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Page{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	resp, err := c.hc.Do(req)
	if err != nil {
		return Page{}, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, c.MaxBytes))
	if err != nil {
		return Page{}, fmt.Errorf("fetch: reading body: %w", err)
	}
	return Page{FinalURL: resp.Request.URL.String(), Status: resp.StatusCode, Body: body}, nil
}
