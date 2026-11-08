// Package pgstore is the PostgreSQL implementation of scan.Store (ADR-5). It works with a local
// PostgreSQL or Supabase alike; only the connection string differs.
package pgstore

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"compliancechecker/internal/rules"
	"compliancechecker/internal/scan"
	"compliancechecker/migrations"
)

// Store implements scan.Store.
type Store struct {
	pool *pgxpool.Pool
	Now  func() time.Time
	// Timeout bounds every database call, because scan.Store has no context parameter.
	Timeout time.Duration
}

var _ scan.Store = (*Store)(nil)

// Open connects, checks the connection and applies pending migrations.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgstore: bad DATABASE_URL: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgstore: cannot reach database: %w", err)
	}
	if err := Migrate(ctx, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool, Now: time.Now, Timeout: 10 * time.Second}, nil
}

// Close releases the connections.
func (s *Store) Close() { s.pool.Close() }

// Migrate applies every embedded migration that is not yet recorded in schema_migrations.
// A transaction-level advisory lock keeps two starting processes from migrating at the same time.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	names, err := fs.Glob(migrations.FS, "*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pgstore: migrate: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7212026)`); err != nil {
		return fmt.Errorf("pgstore: migrate lock: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("pgstore: migrate: %w", err)
	}
	for _, name := range names {
		var done bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE name = $1)`, name).Scan(&done); err != nil {
			return fmt.Errorf("pgstore: migrate: %w", err)
		}
		if done {
			continue
		}
		sqlText, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlText)); err != nil {
			return fmt.Errorf("pgstore: migration %s: %w", name, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, name); err != nil {
			return fmt.Errorf("pgstore: migration %s: %w", name, err)
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), s.Timeout)
}

const scanColumns = `id, host, target_key, start_url, areas, status, pages_done, pages_total, score, error_de,
	previous_scan_id, created_at, finished_at, expires_at`

// Create implements scan.Store.
func (s *Store) Create(sc scan.Scan) error {
	ctx, cancel := s.ctx()
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := insertScan(ctx, tx, sc); err != nil {
		return err
	}
	if err := writeChildren(ctx, tx, sc); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Get implements scan.Store. Expired scans are treated as missing (F11).
func (s *Store) Get(id string) (scan.Scan, bool) {
	ctx, cancel := s.ctx()
	defer cancel()
	sc, err := loadScan(ctx, s.pool, `WHERE id = $1 AND expires_at > $2`, id, s.Now())
	return sc, err == nil
}

// Update implements scan.Store: it locks the row, applies fn and writes the result back in one transaction.
// Child rows (rule results, findings) are only rewritten when fn changed them.
func (s *Store) Update(id string, fn func(*scan.Scan)) bool {
	ctx, cancel := s.ctx()
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false
	}
	defer func() { _ = tx.Rollback(ctx) }()

	before, err := loadScan(ctx, tx, `WHERE id = $1 FOR UPDATE`, id)
	if err != nil {
		return false
	}
	after := before
	after.Areas = append([]string(nil), before.Areas...)
	fn(&after)
	after.ID = id // the id is the key and never changes

	if _, err := tx.Exec(ctx, `UPDATE scans SET host=$2, target_key=$3, start_url=$4, areas=$5, status=$6, pages_done=$7,
		pages_total=$8, score=$9, error_de=$10, previous_scan_id=$11, created_at=$12, finished_at=$13, expires_at=$14
		WHERE id=$1`, scanArgs(after)...); err != nil {
		return false
	}
	if !reflect.DeepEqual(before.RuleResults, after.RuleResults) || !reflect.DeepEqual(before.Findings, after.Findings) {
		if _, err := tx.Exec(ctx, `DELETE FROM rule_results WHERE scan_id = $1`, id); err != nil {
			return false
		}
		if _, err := tx.Exec(ctx, `DELETE FROM findings WHERE scan_id = $1`, id); err != nil {
			return false
		}
		if err := writeChildren(ctx, tx, after); err != nil {
			return false
		}
	}
	return tx.Commit(ctx) == nil
}

// LatestForTarget implements scan.Store.
func (s *Store) LatestForTarget(key string, since time.Time) (scan.Scan, bool) {
	ctx, cancel := s.ctx()
	defer cancel()
	sc, err := loadScan(ctx, s.pool, `WHERE target_key = $1 AND status <> 'failed' AND created_at >= $2 AND expires_at > $3
		ORDER BY created_at DESC LIMIT 1`, key, since, s.Now())
	return sc, err == nil
}

// Purge deletes expired scans with their results and returns how many were removed (N4: 30-day retention).
func (s *Store) Purge(ctx context.Context) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM scans WHERE expires_at <= $1`, s.Now())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ---- helpers ----

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func scanArgs(sc scan.Scan) []any {
	var finished *time.Time
	if !sc.FinishedAt.IsZero() {
		finished = &sc.FinishedAt
	}
	areas := sc.Areas
	if areas == nil {
		areas = []string{}
	}
	return []any{sc.ID, sc.Host, sc.TargetKey, sc.StartURL, areas, string(sc.Status), sc.PagesDone, sc.PagesTotal, sc.Score,
		sc.ErrorDe, sc.PreviousScanID, sc.CreatedAt, finished, sc.ExpiresAt}
}

func insertScan(ctx context.Context, tx pgx.Tx, sc scan.Scan) error {
	_, err := tx.Exec(ctx, `INSERT INTO scans (`+scanColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, scanArgs(sc)...)
	return err
}

func writeChildren(ctx context.Context, tx pgx.Tx, sc scan.Scan) error {
	for i, rr := range sc.RuleResults {
		if _, err := tx.Exec(ctx, `INSERT INTO rule_results (scan_id, position, rule_id, state, decided_by, pages_affected)
			VALUES ($1,$2,$3,$4,$5,$6)`, sc.ID, i, rr.RuleID, string(rr.State), string(rr.DecidedBy), rr.PagesAffected); err != nil {
			return err
		}
	}
	for i, f := range sc.Findings {
		if _, err := tx.Exec(ctx, `INSERT INTO findings (id, scan_id, position, rule_id, severity, url, selector, snippet,
			evidence, sub, summary_de) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
			f.ID, sc.ID, i, f.RuleID, string(f.Severity), f.URL, f.Selector, f.Snippet, f.Evidence, f.Sub, f.SummaryDe); err != nil {
			return err
		}
	}
	return nil
}

var errNotFound = errors.New("pgstore: not found")

func loadScan(ctx context.Context, q querier, where string, args ...any) (scan.Scan, error) {
	var (
		sc       scan.Scan
		status   string
		finished *time.Time
	)
	err := q.QueryRow(ctx, `SELECT `+scanColumns+` FROM scans `+where, args...).Scan(
		&sc.ID, &sc.Host, &sc.TargetKey, &sc.StartURL, &sc.Areas, &status, &sc.PagesDone, &sc.PagesTotal, &sc.Score, &sc.ErrorDe,
		&sc.PreviousScanID, &sc.CreatedAt, &finished, &sc.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return scan.Scan{}, errNotFound
	}
	if err != nil {
		return scan.Scan{}, err
	}
	sc.Status = scan.Status(status)
	if finished != nil {
		sc.FinishedAt = *finished
	}

	rows, err := q.Query(ctx, `SELECT rule_id, state, decided_by, pages_affected FROM rule_results
		WHERE scan_id = $1 ORDER BY position`, sc.ID)
	if err != nil {
		return scan.Scan{}, err
	}
	for rows.Next() {
		var rr scan.RuleResult
		var state, decided string
		if err := rows.Scan(&rr.RuleID, &state, &decided, &rr.PagesAffected); err != nil {
			rows.Close()
			return scan.Scan{}, err
		}
		rr.State, rr.DecidedBy = rules.State(state), rules.DecidedBy(decided)
		sc.RuleResults = append(sc.RuleResults, rr)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return scan.Scan{}, err
	}

	rows, err = q.Query(ctx, `SELECT id, rule_id, severity, url, selector, snippet, evidence, sub, summary_de
		FROM findings WHERE scan_id = $1 ORDER BY position`, sc.ID)
	if err != nil {
		return scan.Scan{}, err
	}
	for rows.Next() {
		var f scan.Finding
		var sev string
		if err := rows.Scan(&f.ID, &f.RuleID, &sev, &f.URL, &f.Selector, &f.Snippet, &f.Evidence, &f.Sub, &f.SummaryDe); err != nil {
			rows.Close()
			return scan.Scan{}, err
		}
		f.Severity = rules.Severity(sev)
		sc.Findings = append(sc.Findings, f)
	}
	rows.Close()
	return sc, rows.Err()
}
