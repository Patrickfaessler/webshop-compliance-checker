package fetch

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"compliancechecker/internal/ssrf"
)

// Spec: C-SSRF — a local test server (loopback) must be unreachable.
func TestGetBlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("blocked request reached the server")
	}))
	defer srv.Close()

	_, err := New(nil).Get(context.Background(), srv.URL)
	if !errors.Is(err, ssrf.ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
}

// Spec: C-SSRF — redirects are validated.
func TestRedirectToInternalAddressRejected(t *testing.T) {
	c := New(nil)
	req, _ := http.NewRequest(http.MethodGet, "http://169.254.169.254/latest/meta-data/", nil)
	if err := c.checkRedirect(req, nil); !errors.Is(err, ssrf.ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
}

// Spec: C-SSRF — redirect chain length is limited.
func TestRedirectLimit(t *testing.T) {
	c := New(nil)
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/", nil)
	via := make([]*http.Request, maxRedirects)
	if err := c.checkRedirect(req, via); err == nil {
		t.Fatal("expected too-many-redirects error")
	}
}

// Spec: C-SSRF — the dialer itself refuses internal targets (covers sub-requests and rebinding).
func TestDialRefusesPrivate(t *testing.T) {
	c := New(nil)
	if _, err := c.dial(context.Background(), "tcp", "10.0.0.5:443"); !errors.Is(err, ssrf.ErrBlocked) {
		t.Fatalf("want ErrBlocked, got %v", err)
	}
	if _, err := c.dial(context.Background(), "tcp", "93.184.216.34:22"); !errors.Is(err, ssrf.ErrBlocked) {
		t.Fatalf("port 22 must be blocked, got %v", err)
	}
}
