package scan_test

import (
	"net/url"
	"testing"

	"compliancechecker/internal/scan"
)

// Spec: C-RL — the cooldown key is scheme, host and path; query, fragment, case and trailing slash do not matter.
func TestTargetKey(t *testing.T) {
	tests := map[string]string{
		"https://Shop.Example":                           "https://shop.example/",
		"https://shop.example/":                          "https://shop.example/",
		"https://shop.example/?utm=x#top":                "https://shop.example/",
		"HTTPS://shop.example/de/":                       "https://shop.example/de",
		"https://user.github.io/repo/teehaus/":           "https://user.github.io/repo/teehaus",
		"https://user.github.io/repo/radwerk/index.html": "https://user.github.io/repo/radwerk/index.html",
		"http://shop.example:80/a":                       "http://shop.example:80/a",
	}
	for in, want := range tests {
		u, err := url.Parse(in)
		if err != nil {
			t.Fatal(err)
		}
		if got := scan.TargetKey(u); got != want {
			t.Errorf("%s: got %q, want %q", in, got, want)
		}
	}
}
