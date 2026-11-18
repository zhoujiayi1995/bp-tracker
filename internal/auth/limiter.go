package auth

import (
	"sync"
	"time"
)

// Limiter throttles repeated failures per key (for example username+IP or user ID).
// It is in-memory on purpose: a single-process family app does not need shared state.
type Limiter struct {
	MaxFailures int
	Window      time.Duration
	Lockout     time.Duration
	Now         func() time.Time

	mu      sync.Mutex
	entries map[string]*limitEntry
}

type limitEntry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

func NewLimiter(maxFailures int, window, lockout time.Duration) *Limiter {
	return &Limiter{MaxFailures: maxFailures, Window: window, Lockout: lockout}
}

func (l *Limiter) now() time.Time {
	if l.Now != nil {
		return l.Now()
	}
	return time.Now()
}

// Allow reports whether another attempt is permitted and, if not, how long to wait.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return true, 0
	}
	if wait := e.lockedUntil.Sub(l.now()); wait > 0 {
		return false, wait
	}
	return true, 0
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.entries == nil {
		l.entries = make(map[string]*limitEntry)
	}
	now := l.now()
	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) > l.Window {
		e = &limitEntry{windowStart: now}
		l.entries[key] = e
	}
	e.failures++
	if e.failures >= l.MaxFailures {
		e.lockedUntil = now.Add(l.Lockout)
		e.failures = 0
		e.windowStart = now
	}
	if len(l.entries) > 10000 {
		l.pruneLocked(now)
	}
}

func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *Limiter) pruneLocked(now time.Time) {
	for k, e := range l.entries {
		if now.After(e.lockedUntil) && now.Sub(e.windowStart) > l.Window {
			delete(l.entries, k)
		}
	}
}
