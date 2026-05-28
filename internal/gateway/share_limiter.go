package gateway

import (
	"sync"
	"time"
)

const (
	maxSharePasscodeAttempts   = 5
	sharePasscodeLockoutWindow = 15 * time.Minute
)

type passcodeAttempt struct {
	failures    int
	windowStart time.Time
}

type sharePasscodeLimiter struct {
	mu      sync.Mutex
	entries map[string]*passcodeAttempt
	now     func() time.Time
}

func newSharePasscodeLimiter(now func() time.Time) *sharePasscodeLimiter {
	if now == nil {
		now = time.Now
	}
	return &sharePasscodeLimiter{
		entries: make(map[string]*passcodeAttempt),
		now:     now,
	}
}

func (l *sharePasscodeLimiter) isLocked(token string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[token]
	if !ok {
		return false
	}
	if l.now().Sub(e.windowStart) > sharePasscodeLockoutWindow {
		delete(l.entries, token)
		return false
	}
	return e.failures >= maxSharePasscodeAttempts
}

func (l *sharePasscodeLimiter) recordFailure(token string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[token]
	if !ok || now.Sub(e.windowStart) > sharePasscodeLockoutWindow {
		l.entries[token] = &passcodeAttempt{failures: 1, windowStart: now}
		return false
	}
	e.failures++
	return e.failures > maxSharePasscodeAttempts
}

func (l *sharePasscodeLimiter) resetOnSuccess(token string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, token)
}
