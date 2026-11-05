// Package ssrf validates user-supplied URLs before anything fetches them.
// Spec: specs/05-crawler.md (C-SSRF), N6.
package ssrf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strings"
)

// ErrBlocked is wrapped by every rejection so callers can map it to a German user message (F1).
var ErrBlocked = errors.New("url not allowed")

// LookupFunc resolves a host name. Tests inject a fake; production uses DefaultLookup.
type LookupFunc func(ctx context.Context, host string) ([]netip.Addr, error)

// DefaultLookup resolves with the system resolver.
func DefaultLookup(ctx context.Context, host string) ([]netip.Addr, error) {
	return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
}

var blockedPrefixes = mustPrefixes(
	// IPv4
	"0.0.0.0/8",      // "this" network
	"10.0.0.0/8",     // private
	"100.64.0.0/10",  // carrier-grade NAT
	"127.0.0.0/8",    // loopback
	"169.254.0.0/16", // link-local, cloud metadata
	"172.16.0.0/12",  // private
	"192.0.0.0/24",   // IETF protocol assignments
	"192.168.0.0/16", // private
	"198.18.0.0/15",  // benchmarking
	"224.0.0.0/4",    // multicast
	"240.0.0.0/4",    // reserved, incl. broadcast
	// IPv6
	"::/128",       // unspecified
	"::1/128",      // loopback
	"64:ff9b::/96", // NAT64 (can embed private IPv4)
	"fc00::/7",     // unique local
	"fe80::/10",    // link-local
	"ff00::/8",     // multicast
)

func mustPrefixes(list ...string) []netip.Prefix {
	out := make([]netip.Prefix, len(list))
	for i, s := range list {
		out[i] = netip.MustParsePrefix(s)
	}
	return out
}

// IsBlockedIP reports whether addr is in a range that must never be fetched (C-SSRF).
// IPv4-mapped IPv6 addresses are checked as IPv4.
func IsBlockedIP(addr netip.Addr) bool {
	addr = addr.Unmap()
	if !addr.IsValid() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// Validate checks scheme, port and credentials, resolves the host and rejects it if any
// resolved address is blocked. It returns the parsed URL and the vetted addresses; the caller
// must connect to one of these addresses (pinning) instead of resolving again, which defeats
// DNS rebinding. Call it again for every redirect and every browser sub-request.
// Spec: C-SSRF.
func Validate(ctx context.Context, raw string, lookup LookupFunc) (*url.URL, []netip.Addr, error) {
	if lookup == nil {
		lookup = DefaultLookup
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return nil, nil, fmt.Errorf("%w: unparsable URL", ErrBlocked)
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return nil, nil, fmt.Errorf("%w: scheme %q", ErrBlocked, u.Scheme)
	}
	if u.User != nil {
		return nil, nil, fmt.Errorf("%w: credentials in URL", ErrBlocked)
	}
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return nil, nil, fmt.Errorf("%w: port %s", ErrBlocked, p)
	}
	host := u.Hostname()
	if host == "" {
		return nil, nil, fmt.Errorf("%w: no host", ErrBlocked)
	}

	var addrs []netip.Addr
	if ip, err := netip.ParseAddr(host); err == nil {
		addrs = []netip.Addr{ip}
	} else {
		addrs, err = lookup(ctx, host)
		if err != nil || len(addrs) == 0 {
			return nil, nil, fmt.Errorf("%w: host does not resolve", ErrBlocked)
		}
	}
	for _, a := range addrs {
		if IsBlockedIP(a) {
			return nil, nil, fmt.Errorf("%w: address %s is not public", ErrBlocked, a)
		}
	}
	return u, addrs, nil
}
