// Package scan holds the scan model and its storage. No user data exists (ADR-1).
// Spec: specs/07-architecture.md (data model), F11 (30-day expiry).
package scan

import (
	"crypto/rand"
	"encoding/base64"
	"net/url"
	"strings"
	"sync"
	"time"

	"compliancechecker/internal/rules"
)

// DisclaimerDe must appear on report, detail view and PDF (N3, specs/01-legal-scope.md).
const DisclaimerDe = "Automatische Prüfungen erfassen nur einen Teil möglicher Probleme und ersetzen keine Rechtsberatung. Geprüft wurde auf EU-Ebene; nationale Zusatzpflichten sind nicht enthalten."

// Retention is how long a scan stays readable after it was created (F11, N4, ADR-6: in memory only).
const Retention = time.Hour

type Status string

const (
	Queued     Status = "queued"
	Running    Status = "running"
	Done       Status = "done"
	Incomplete Status = "incomplete"
	Failed     Status = "failed"
)

type RuleResult struct {
	RuleID        string
	State         rules.State
	DecidedBy     rules.DecidedBy
	PagesAffected int
}

type Finding struct {
	ID        string
	RuleID    string
	Severity  rules.Severity
	URL       string
	Selector  string
	Snippet   string
	Evidence  string
	Sub       string // sub-rule, e.g. "skipped-level"
	SummaryDe string
}

type Scan struct {
	ID             string
	Host           string
	TargetKey      string // scheme://host/path of the start URL, the key of the cooldown (C-RL)
	StartURL       string
	Areas          []string
	Status         Status
	PagesDone      int
	PagesTotal     int
	Score          *int // nil = not computable
	ErrorDe        string
	PreviousScanID string
	RuleResults    []RuleResult
	Findings       []Finding
	CreatedAt      time.Time
	FinishedAt     time.Time
	ExpiresAt      time.Time
}

// NewID returns a random 128-bit URL-safe id (F11).
func NewID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Store keeps scans: in memory by default (ADR-6), PostgreSQL optionally (ADR-5).
type Store interface {
	Create(s Scan) error
	Get(id string) (Scan, bool)
	// Update applies fn to the stored scan; false if it does not exist.
	Update(id string, fn func(*Scan)) bool
	// LatestForTarget returns the newest non-failed scan with this TargetKey created at or after since (C-RL).
	LatestForTarget(key string, since time.Time) (Scan, bool)
}

// MemoryStore is an in-memory Store used until PostgreSQL is wired in and by tests.
type MemoryStore struct {
	mu    sync.Mutex
	Now   func() time.Time
	scans map[string]Scan
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{Now: time.Now, scans: map[string]Scan{}}
}

func (m *MemoryStore) Create(s Scan) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scans[s.ID] = s
	return nil
}

func (m *MemoryStore) Get(id string) (Scan, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.scans[id]
	if !ok {
		return Scan{}, false
	}
	if !s.ExpiresAt.IsZero() && !m.Now().Before(s.ExpiresAt) {
		delete(m.scans, id)
		return Scan{}, false
	}
	return s, true
}

func (m *MemoryStore) Update(id string, fn func(*Scan)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.scans[id]
	if !ok {
		return false
	}
	fn(&s)
	m.scans[id] = s
	return true
}

// Purge removes expired scans from memory and returns how many were removed (N4, ADR-6).
func (m *MemoryStore) Purge() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	now, n := m.Now(), 0
	for id, s := range m.scans {
		if !s.ExpiresAt.IsZero() && !now.Before(s.ExpiresAt) {
			delete(m.scans, id)
			n++
		}
	}
	return n
}

func (m *MemoryStore) LatestForTarget(key string, since time.Time) (Scan, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var best Scan
	found := false
	for _, s := range m.scans {
		if s.TargetKey != key || s.Status == Failed || s.CreatedAt.Before(since) {
			continue
		}
		if !found || s.CreatedAt.After(best.CreatedAt) {
			best, found = s, true
		}
	}
	return best, found
}

// TargetKey normalises a start URL for the cooldown (C-RL): lower-case scheme and host, path with a
// trailing slash trimmed (empty path = "/"), query and fragment dropped. Several shops can share one
// host, so the key includes the path.
func TargetKey(u *url.URL) string {
	p := strings.TrimSuffix(u.EscapedPath(), "/")
	if p == "" {
		p = "/"
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + p
}
