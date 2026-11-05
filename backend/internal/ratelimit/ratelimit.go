// Package ratelimit limits scans per client without accounts.
// Spec: specs/05-crawler.md (C-RL), F12.
package ratelimit

import (
	"sync"
	"time"
)

// Reason says why Acquire refused.
type Reason string

const (
	ReasonNone    Reason = ""
	ReasonRunning Reason = "running" // too many scans of this client are running
	ReasonRate    Reason = "rate"    // too many scans started in the window
)

// Limiter allows at most max starts per window and maxRunning concurrent scans per key.
// Keys are hashed client identifiers, never raw IPs (N4).
type Limiter struct {
	mu         sync.Mutex
	Now        func() time.Time
	max        int
	window     time.Duration
	maxRunning int
	hits       map[string][]time.Time
	running    map[string]int
}

func New(max int, window time.Duration, maxRunning int) *Limiter {
	return &Limiter{
		Now: time.Now, max: max, window: window, maxRunning: maxRunning,
		hits: map[string][]time.Time{}, running: map[string]int{},
	}
}

// Acquire records a start for key if allowed. When refused, retryAfter is a hint for the client.
func (l *Limiter) Acquire(key string) (ok bool, retryAfter time.Duration, reason Reason) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.Now()

	recent := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < l.window {
			recent = append(recent, t)
		}
	}
	l.hits[key] = recent

	if l.running[key] >= l.maxRunning {
		return false, 10 * time.Second, ReasonRunning
	}
	if len(recent) >= l.max {
		return false, l.window - now.Sub(recent[0]), ReasonRate
	}
	l.hits[key] = append(recent, now)
	l.running[key]++
	return true, 0, ReasonNone
}

// Release marks a scan of key as finished.
func (l *Limiter) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.running[key] > 0 {
		l.running[key]--
	}
	if l.running[key] == 0 {
		delete(l.running, key)
	}
}
