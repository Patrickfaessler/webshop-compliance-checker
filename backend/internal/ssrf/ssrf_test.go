package ssrf

import (
	"context"
	"errors"
	"net/netip"
	"testing"
)

func fakeLookup(m map[string][]string) LookupFunc {
	return func(_ context.Context, host string) ([]netip.Addr, error) {
		ips, ok := m[host]
		if !ok {
			return nil, errors.New("no such host")
		}
		var out []netip.Addr
		for _, s := range ips {
			out = append(out, netip.MustParseAddr(s))
		}
		return out, nil
	}
}

// Spec: C-SSRF (address ranges)
func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "127.255.255.254", "10.0.0.1", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "224.0.0.1", "255.255.255.255", "198.18.0.1",
		"::1", "::", "fc00::1", "fd12:3456::1", "fe80::1", "ff02::1", "64:ff9b::a00:1",
		"::ffff:127.0.0.1", "::ffff:10.1.2.3", "::ffff:169.254.169.254",
	}
	for _, s := range blocked {
		if !IsBlockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s should be blocked", s)
		}
	}
	allowed := []string{"8.8.8.8", "93.184.216.34", "172.32.0.1", "172.15.255.255", "100.63.255.255", "2606:4700:4700::1111", "::ffff:8.8.8.8"}
	for _, s := range allowed {
		if IsBlockedIP(netip.MustParseAddr(s)) {
			t.Errorf("%s should be allowed", s)
		}
	}
	if !IsBlockedIP(netip.Addr{}) {
		t.Error("invalid address must be blocked")
	}
}

// Spec: C-SSRF (scheme, port, credentials, resolution)
func TestValidate(t *testing.T) {
	lookup := fakeLookup(map[string][]string{
		"shop.example":     {"93.184.216.34"},
		"internal.example": {"10.0.0.5"},
		"mixed.example":    {"93.184.216.34", "127.0.0.1"}, // one bad record blocks the host
		"v6.example":       {"2606:4700:4700::1111"},
	})
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https ok", "https://shop.example/", false},
		{"http ok", "http://shop.example/path?q=1", false},
		{"explicit 443 ok", "https://shop.example:443/", false},
		{"explicit 80 ok", "http://shop.example:80/", false},
		{"ipv6 host ok", "https://v6.example/", false},
		{"public ip literal ok", "http://93.184.216.34/", false},
		{"surrounding spaces trimmed", "  https://shop.example/ ", false},

		{"ftp scheme", "ftp://shop.example/", true},
		{"file scheme", "file:///etc/passwd", true},
		{"javascript scheme", "javascript:alert(1)", true},
		{"no scheme", "shop.example", true},
		{"port 8080", "http://shop.example:8080/", true},
		{"port 22", "http://shop.example:22/", true},
		{"credentials", "https://user:pw@shop.example/", true},
		{"empty host", "https:///path", true},
		{"loopback literal", "http://127.0.0.1/", true},
		{"metadata literal", "http://169.254.169.254/latest/meta-data/", true},
		{"ipv6 loopback literal", "http://[::1]/", true},
		{"ipv4-mapped literal", "http://[::ffff:127.0.0.1]/", true},
		{"private resolves", "https://internal.example/", true},
		{"mixed records", "https://mixed.example/", true},
		{"unresolvable", "https://nope.example/", true},
		{"localhost by name", "http://localhost/", true},
	}
	// localhost needs a resolver answer; the fake has none, so it fails as unresolvable. Also test with a loopback answer:
	lookup2 := fakeLookup(map[string][]string{"localhost": {"127.0.0.1", "::1"}})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lookup
			if tt.name == "localhost by name" {
				l = lookup2
			}
			u, addrs, err := Validate(context.Background(), tt.url, l)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v %v", u, addrs)
				}
				if !errors.Is(err, ErrBlocked) {
					t.Fatalf("error must wrap ErrBlocked, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(addrs) == 0 || u == nil {
				t.Fatal("expected URL and vetted addresses")
			}
		})
	}
}

// Spec: C-SSRF (re-check after redirects: a public URL redirecting to a private one is rejected
// because the caller validates the Location target with the same function).
func TestValidateRedirectTarget(t *testing.T) {
	lookup := fakeLookup(map[string][]string{"shop.example": {"93.184.216.34"}})
	if _, _, err := Validate(context.Background(), "https://shop.example/", lookup); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Validate(context.Background(), "http://169.254.169.254/", lookup); err == nil {
		t.Fatal("redirect to metadata address must be rejected")
	}
}
