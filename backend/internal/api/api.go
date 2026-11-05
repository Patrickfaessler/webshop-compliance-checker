// Package api implements the REST contract in specs/08-api.yaml. No authentication (ADR-1).
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"compliancechecker/internal/ratelimit"
	"compliancechecker/internal/scan"
	"compliancechecker/internal/ssrf"
	"compliancechecker/internal/worker"
)

type Server struct {
	Store   scan.Store
	Limiter *ratelimit.Limiter
	Jobs    chan<- worker.Job
	Lookup  ssrf.LookupFunc // nil = system resolver
	Salt    []byte          // rotating salt for hashing client IPs (N4)
	Now     func() time.Time
	// HostCooldown: a scan of the same host within this time returns the existing result (C-RL).
	HostCooldown time.Duration
	// AllowOrigin enables CORS for the Flutter web dev server (e.g. http://localhost:5000). Empty = off.
	AllowOrigin string
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Handler returns the routed handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("POST /api/scans", s.createScan)
	mux.HandleFunc("GET /api/scans/{id}", s.getScan)
	mux.HandleFunc("GET /api/scans/{id}/findings", s.getFindings)
	return s.cors(mux)
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.AllowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.AllowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Vary", "Origin")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ---- JSON shapes (specs/08-api.yaml) ----

type scanRequest struct {
	URL            string   `json:"url"`
	Areas          []string `json:"areas"`
	PreviousScanID string   `json:"previousScanId"`
}

type scanStatus struct {
	ID           string    `json:"id"`
	Status       string    `json:"status"`
	PagesDone    int       `json:"pagesDone"`
	PagesTotal   int       `json:"pagesTotal"`
	Score        *int      `json:"score"`
	ErrorDe      string    `json:"errorDe,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
	DisclaimerDe string    `json:"disclaimerDe"`
}

type ruleResultJSON struct {
	RuleID        string `json:"ruleId"`
	State         string `json:"state"`
	DecidedBy     string `json:"decidedBy"`
	PagesAffected int    `json:"pagesAffected"`
}

type findingJSON struct {
	ID        string `json:"id"`
	RuleID    string `json:"ruleId"`
	Severity  string `json:"severity"`
	URL       string `json:"url"`
	SummaryDe string `json:"summaryDe"`
}

func toStatus(sc scan.Scan) scanStatus {
	return scanStatus{
		ID: sc.ID, Status: string(sc.Status), PagesDone: sc.PagesDone, PagesTotal: sc.PagesTotal,
		Score: sc.Score, ErrorDe: sc.ErrorDe, ExpiresAt: sc.ExpiresAt, DisclaimerDe: scan.DisclaimerDe,
	}
}

// ---- handlers ----

func (s *Server) createScan(w http.ResponseWriter, r *http.Request) {
	var req scanRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		problem(w, http.StatusBadRequest, "Die Anfrage war nicht lesbar.", 0)
		return
	}
	areas, ok := normalizeAreas(req.Areas)
	if !ok {
		problem(w, http.StatusBadRequest, "Unbekannter Prüfbereich. Erlaubt: accessibility, legal.", 0)
		return
	}
	u, _, err := ssrf.Validate(r.Context(), req.URL, s.Lookup)
	if err != nil {
		msg := "Diese Adresse konnte nicht geprüft werden. Bitte eine öffentliche http- oder https-Adresse eingeben."
		if !errors.Is(err, ssrf.ErrBlocked) {
			msg = "Die Adresse konnte nicht aufgelöst werden."
		}
		problem(w, http.StatusBadRequest, msg, 0)
		return
	}
	host := strings.ToLower(u.Hostname())

	// Same host recently scanned: return that result instead of scanning again (C-RL).
	if s.HostCooldown > 0 {
		if prev, found := s.Store.LatestForHost(host, s.now().Add(-s.HostCooldown)); found {
			writeJSON(w, http.StatusOK, toStatus(prev))
			return
		}
	}

	key := s.clientKey(r)
	if ok, retry, reason := s.Limiter.Acquire(key); !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		msg := "Zu viele Prüfungen in kurzer Zeit. Bitte später erneut versuchen."
		if reason == ratelimit.ReasonRunning {
			msg = "Es läuft bereits eine Prüfung von Ihrem Anschluss. Bitte warten Sie, bis sie fertig ist."
		}
		problem(w, http.StatusTooManyRequests, msg, int(retry.Seconds())+1)
		return
	}

	id, err := scan.NewID()
	if err != nil {
		s.Limiter.Release(key)
		problem(w, http.StatusInternalServerError, "Interner Fehler.", 0)
		return
	}
	now := s.now()
	sc := scan.Scan{
		ID: id, Host: host, StartURL: u.String(), Areas: areas, Status: scan.Queued,
		PreviousScanID: req.PreviousScanID, CreatedAt: now, ExpiresAt: now.Add(scan.Retention),
	}
	if err := s.Store.Create(sc); err != nil {
		s.Limiter.Release(key)
		problem(w, http.StatusInternalServerError, "Interner Fehler.", 0)
		return
	}
	select {
	case s.Jobs <- worker.Job{ScanID: id, ClientKey: key}:
	default:
		s.Limiter.Release(key)
		s.Store.Update(id, func(x *scan.Scan) { x.Status = scan.Failed; x.ErrorDe = "Warteschlange voll." })
		problem(w, http.StatusServiceUnavailable, "Gerade sind zu viele Prüfungen in der Warteschlange. Bitte später erneut versuchen.", 30)
		return
	}
	writeJSON(w, http.StatusAccepted, toStatus(sc))
}

func (s *Server) getScan(w http.ResponseWriter, r *http.Request) {
	sc, ok := s.Store.Get(r.PathValue("id"))
	if !ok {
		problem(w, http.StatusNotFound, "Diese Prüfung gibt es nicht (mehr).", 0)
		return
	}
	writeJSON(w, http.StatusOK, toStatus(sc))
}

func (s *Server) getFindings(w http.ResponseWriter, r *http.Request) {
	sc, ok := s.Store.Get(r.PathValue("id"))
	if !ok {
		problem(w, http.StatusNotFound, "Diese Prüfung gibt es nicht (mehr).", 0)
		return
	}
	sev, rule := r.URL.Query().Get("severity"), r.URL.Query().Get("rule")
	if sev != "" && !slices.Contains([]string{"critical", "warning", "notice"}, sev) {
		problem(w, http.StatusBadRequest, "Unbekannter Schweregrad.", 0)
		return
	}

	out := struct {
		RuleResults []ruleResultJSON `json:"ruleResults"`
		Findings    []findingJSON    `json:"findings"`
	}{RuleResults: []ruleResultJSON{}, Findings: []findingJSON{}}
	for _, rr := range sc.RuleResults {
		out.RuleResults = append(out.RuleResults, ruleResultJSON{
			RuleID: rr.RuleID, State: string(rr.State), DecidedBy: string(rr.DecidedBy), PagesAffected: rr.PagesAffected,
		})
	}
	for _, f := range sc.Findings {
		if (sev != "" && string(f.Severity) != sev) || (rule != "" && f.RuleID != rule) {
			continue
		}
		out.Findings = append(out.Findings, findingJSON{
			ID: f.ID, RuleID: f.RuleID, Severity: string(f.Severity), URL: f.URL, SummaryDe: f.SummaryDe,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- helpers ----

func normalizeAreas(in []string) ([]string, bool) {
	if len(in) == 0 {
		return []string{"accessibility", "legal"}, true
	}
	var out []string
	for _, a := range in {
		if a != "accessibility" && a != "legal" {
			return nil, false
		}
		if !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out, true
}

// clientKey hashes the client IP with the rotating salt. The IP itself is never stored (N4).
// X-Forwarded-For is deliberately ignored; a trusted reverse proxy setting is a later decision.
func (s *Server) clientKey(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	sum := sha256.Sum256(append(append([]byte{}, s.Salt...), ip...))
	return hex.EncodeToString(sum[:8])
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// problem writes RFC 9457 problem details; title is a German user message.
func problem(w http.ResponseWriter, code int, titleDe string, retryAfter int) {
	body := map[string]any{"type": "about:blank", "title": titleDe, "status": code}
	if retryAfter > 0 {
		body["retryAfterSeconds"] = retryAfter
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
