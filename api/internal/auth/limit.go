package auth

import (
	"sync"
	"time"
)

// failureLimiter counts failed sign-ins per key in a fixed window. It lives in
// memory, which is enough for the single API process and resets on restart.
type failureLimiter struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]*failures
}

type failures struct {
	count int
	reset time.Time
}

func newFailureLimiter(maxFailures int, window time.Duration) *failureLimiter {
	return &failureLimiter{max: maxFailures, window: window, now: time.Now, entries: map[string]*failures{}}
}

// blocked reports whether key has used up its failures, and how long until it
// may try again.
func (l *failureLimiter) blocked(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[key]
	if !ok {
		return false, 0
	}
	now := l.now()
	if !now.Before(e.reset) {
		delete(l.entries, key)
		return false, 0
	}
	return e.count >= l.max, e.reset.Sub(now)
}

func (l *failureLimiter) fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	e, ok := l.entries[key]
	if !ok || !now.Before(e.reset) {
		// Keeps a flood of made-up usernames from growing the map forever.
		if len(l.entries) > 10_000 {
			l.prune(now)
		}
		e = &failures{reset: now.Add(l.window)}
		l.entries[key] = e
	}
	e.count++
}

func (l *failureLimiter) clear(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.entries, key)
}

func (l *failureLimiter) prune(now time.Time) {
	for k, e := range l.entries {
		if !now.Before(e.reset) {
			delete(l.entries, k)
		}
	}
}
