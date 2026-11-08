package scan_test

import (
	"testing"
	"time"

	"compliancechecker/internal/scan"
	"compliancechecker/internal/scan/storetest"
)

// Spec: ADR-5 — the in-memory store fulfils the same contract as the PostgreSQL store.
func TestMemoryStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) (scan.Store, func(time.Time)) {
		st := scan.NewMemoryStore()
		return st, func(now time.Time) { st.Now = func() time.Time { return now } }
	})
}
