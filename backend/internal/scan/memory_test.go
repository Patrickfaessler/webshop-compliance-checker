package scan_test

import (
	"testing"
	"time"

	"compliancechecker/internal/scan"
	"compliancechecker/internal/scan/storetest"
)

// Spec: N4, ADR-6 — Purge drops expired scans from memory, keeps the rest.
func TestMemoryPurge(t *testing.T) {
	t0 := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	st := scan.NewMemoryStore()
	st.Now = func() time.Time { return t0 }
	_ = st.Create(scan.Scan{ID: "old", CreatedAt: t0.Add(-2 * time.Hour), ExpiresAt: t0.Add(-time.Hour)})
	_ = st.Create(scan.Scan{ID: "new", CreatedAt: t0, ExpiresAt: t0.Add(scan.Retention)})
	if n := st.Purge(); n != 1 {
		t.Fatalf("purged %d, want 1", n)
	}
	if _, ok := st.Get("new"); !ok {
		t.Fatal("fresh scan removed")
	}
	if scan.Retention != time.Hour {
		t.Fatalf("retention = %v, want 1h (ADR-6)", scan.Retention)
	}
}

// Spec: ADR-5 — the in-memory store fulfils the same contract as the PostgreSQL store.
func TestMemoryStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) (scan.Store, func(time.Time)) {
		st := scan.NewMemoryStore()
		return st, func(now time.Time) { st.Now = func() time.Time { return now } }
	})
}
