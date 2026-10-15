package backup

import (
	"strings"
	"testing"
	"time"
)

func TestDue(t *testing.T) {
	chicago, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Skip(err)
	}
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, chicago)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	tests := []struct {
		name string
		now  string
		hour int
		last time.Time
		want bool
	}{
		{"never backed up", "2026-10-14 12:00", 3, time.Time{}, true},
		{"before the hour, done yesterday", "2026-10-14 02:59", 3, at("2026-10-13 03:05"), false},
		{"at the hour", "2026-10-14 03:00", 3, at("2026-10-13 03:05"), true},
		{"done today", "2026-10-14 03:10", 3, at("2026-10-14 03:00"), false},
		{"missed while down", "2026-10-14 09:00", 3, at("2026-10-12 03:00"), true},
		{"midnight", "2026-10-14 00:05", 0, at("2026-10-13 00:01"), true},
		{"off", "2026-10-14 12:00", -1, time.Time{}, false},
		// 2:30 doesn't exist on the night clocks spring forward.
		{"spring forward", "2026-03-08 04:00", 2, at("2026-03-07 02:00"), true},
		{"fall back", "2026-11-01 01:30", 1, at("2026-11-01 01:00"), false},
	}
	for _, tt := range tests {
		if got := due(at(tt.now), tt.hour, tt.last); got != tt.want {
			t.Errorf("%s: due = %v, want %v", tt.name, got, tt.want)
		}
	}
	if next := nextDue(at("2026-10-14 03:00"), 3); !next.Equal(at("2026-10-15 03:00")) {
		t.Errorf("nextDue = %v", next)
	}
}

// pg_dump's arguments are visible to every process on the host.
func TestConnectionKeepsPasswordOffTheCommandLine(t *testing.T) {
	dsn, pw := connection("postgres://krill:s3cr%40t@postgres:5432/krill?sslmode=disable")
	if pw != "s3cr@t" || strings.Contains(dsn, "s3cr") || dsn != "postgres://krill@postgres:5432/krill?sslmode=disable" {
		t.Errorf("connection = %q, %q", dsn, pw)
	}
	if dsn, pw := connection("host=db user=krill"); dsn != "host=db user=krill" || pw != "" {
		t.Errorf("key-value connection = %q, %q", dsn, pw)
	}
}
