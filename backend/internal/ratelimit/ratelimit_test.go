package ratelimit

import (
	"testing"
	"time"
)

// Spec: C-RL (max 3 scans per hour, max 1 running per client)
func TestLimiter(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	l := New(3, time.Hour, 1)
	l.Now = func() time.Time { return now }

	// 1 running at a time
	if ok, _, _ := l.Acquire("a"); !ok {
		t.Fatal("first scan must be allowed")
	}
	if ok, _, r := l.Acquire("a"); ok || r != ReasonRunning {
		t.Fatalf("second concurrent scan must be refused as running, got ok=%v reason=%q", ok, r)
	}
	l.Release("a")

	// other clients are independent
	if ok, _, _ := l.Acquire("b"); !ok {
		t.Fatal("other client must be allowed")
	}
	l.Release("b")

	// up to 3 per hour
	for i := 0; i < 2; i++ {
		if ok, _, _ := l.Acquire("a"); !ok {
			t.Fatalf("scan %d must be allowed", i+2)
		}
		l.Release("a")
	}
	ok, retry, r := l.Acquire("a")
	if ok || r != ReasonRate {
		t.Fatalf("4th scan must be refused by rate, got ok=%v reason=%q", ok, r)
	}
	if retry <= 0 || retry > time.Hour {
		t.Fatalf("retryAfter out of range: %v", retry)
	}

	// window expires
	now = now.Add(time.Hour + time.Second)
	if ok, _, _ := l.Acquire("a"); !ok {
		t.Fatal("scan after the window must be allowed")
	}
}

func TestReleaseNeverNegative(t *testing.T) {
	l := New(3, time.Hour, 1)
	l.Release("x")
	l.Release("x")
	if ok, _, _ := l.Acquire("x"); !ok {
		t.Fatal("must still allow after spurious releases")
	}
	if ok, _, _ := l.Acquire("x"); ok {
		t.Fatal("running limit must hold")
	}
}
