package auth

import (
	"testing"
	"time"
)

func TestFailureLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := newFailureLimiter(3, time.Minute)
	l.now = func() time.Time { return now }

	for range 3 {
		if b, _ := l.blocked("ada"); b {
			t.Fatal("blocked before reaching the limit")
		}
		l.fail("ada")
	}
	if b, wait := l.blocked("ada"); !b || wait != time.Minute {
		t.Fatalf("blocked = %v, wait = %v; want blocked for a minute", b, wait)
	}
	if b, _ := l.blocked("grace"); b {
		t.Error("another key was blocked")
	}

	now = now.Add(time.Minute)
	if b, _ := l.blocked("ada"); b {
		t.Error("still blocked after the window")
	}

	l.fail("ada")
	l.clear("ada")
	if b, _ := l.blocked("ada"); b {
		t.Error("still blocked after clear")
	}
}
