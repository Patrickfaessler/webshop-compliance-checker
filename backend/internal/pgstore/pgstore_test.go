package pgstore

import (
	"context"
	"os"
	"testing"
	"time"

	"compliancechecker/internal/config"
	"compliancechecker/internal/scan"
	"compliancechecker/internal/scan/storetest"
)

// openTest connects to TEST_DATABASE_URL (from the environment or deploy/.env) and empties the tables.
// Without it the test is skipped, so the suite never needs a database (specs/09-testing.md).
func openTest(t *testing.T) *Store {
	t.Helper()
	_, _ = config.LoadDotEnv("../../../deploy/.env")
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	st, err := Open(ctx, url)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if _, err := st.pool.Exec(ctx, `TRUNCATE scans CASCADE`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(st.Close)
	return st
}

// Spec: ADR-5 — the PostgreSQL store fulfils the scan.Store contract.
func TestStoreContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) (scan.Store, func(time.Time)) {
		st := openTest(t)
		return st, func(now time.Time) { st.Now = func() time.Time { return now } }
	})
}

// Spec: ADR-5 — migrations are idempotent and recorded.
func TestMigrateTwice(t *testing.T) {
	st := openTest(t)
	ctx := context.Background()
	if err := Migrate(ctx, st.pool); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	var n int
	if err := st.pool.QueryRow(ctx, `SELECT count(*) FROM schema_migrations WHERE name = '0001_init.sql'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("migration recorded %d times, err %v", n, err)
	}
}

// Spec: N4 — Purge removes expired scans and their child rows.
func TestPurge(t *testing.T) {
	st := openTest(t)
	t0 := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	st.Now = func() time.Time { return t0 }
	old := scan.Scan{ID: "old", Host: "a.example", StartURL: "https://a.example/", Areas: []string{"legal"},
		Status: scan.Done, CreatedAt: t0.Add(-31 * 24 * time.Hour), ExpiresAt: t0.Add(-24 * time.Hour),
		RuleResults: []scan.RuleResult{{RuleID: "A6", State: "passed"}}}
	fresh := scan.Scan{ID: "fresh", Host: "b.example", StartURL: "https://b.example/", Areas: []string{"legal"},
		Status: scan.Done, CreatedAt: t0, ExpiresAt: t0.Add(scan.Retention)}
	for _, s := range []scan.Scan{old, fresh} {
		if err := st.Create(s); err != nil {
			t.Fatal(err)
		}
	}
	n, err := st.Purge(context.Background())
	if err != nil || n != 1 {
		t.Fatalf("purged %d, err %v", n, err)
	}
	var left int
	_ = st.pool.QueryRow(context.Background(), `SELECT count(*) FROM rule_results`).Scan(&left)
	if left != 0 {
		t.Fatalf("child rows left: %d", left)
	}
	if _, ok := st.Get("fresh"); !ok {
		t.Fatal("fresh scan removed")
	}
}
