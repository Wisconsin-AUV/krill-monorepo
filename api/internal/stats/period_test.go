package stats

import (
	"testing"
	"time"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func TestSinceUsesCallerTimeZone(t *testing.T) {
	chicago, err := location("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	// Sunday night in Chicago is already Monday in UTC.
	now := time.Date(2026, 9, 27, 23, 30, 0, 0, chicago)

	for _, tt := range []struct {
		period krillv1.Period
		want   time.Time
	}{
		{krillv1.Period_PERIOD_DAY, time.Date(2026, 9, 27, 0, 0, 0, 0, chicago)},
		{krillv1.Period_PERIOD_WEEK, time.Date(2026, 9, 21, 0, 0, 0, 0, chicago)},
		{krillv1.Period_PERIOD_MONTH, time.Date(2026, 9, 1, 0, 0, 0, 0, chicago)},
	} {
		got, bounded, err := since(tt.period, now)
		if err != nil || !bounded || !got.Equal(tt.want) {
			t.Errorf("since(%v) = %v, %v, %v; want %v", tt.period, got, bounded, err, tt.want)
		}
	}
	if _, bounded, _ := since(krillv1.Period_PERIOD_ALL_TIME, now); bounded {
		t.Error("all time should be unbounded")
	}
	if _, _, err := since(krillv1.Period_PERIOD_UNSPECIFIED, now); err == nil {
		t.Error("unspecified period accepted")
	}
}

func TestLocationRejectsServerZone(t *testing.T) {
	for _, name := range []string{"Local", "Mars/Olympus", "../../etc/passwd"} {
		if _, err := location(name); err == nil {
			t.Errorf("location(%q) accepted", name)
		}
	}
}

func TestCountStreaks(t *testing.T) {
	d := func(s string) time.Time {
		v, err := time.Parse(dateLayout, s)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	days := []time.Time{d("2026-09-01"), d("2026-09-02"), d("2026-09-03"), d("2026-09-10"), d("2026-09-11")}

	for _, tt := range []struct {
		today            string
		current, longest int32
	}{
		{"2026-09-11", 2, 3},
		{"2026-09-12", 2, 3},
		{"2026-09-13", 0, 3},
	} {
		got := countStreaks(days, d(tt.today))
		if got != (streaks{tt.current, tt.longest}) {
			t.Errorf("today %s: got %+v, want current %d longest %d", tt.today, got, tt.current, tt.longest)
		}
	}
	if got := countStreaks(nil, d("2026-09-13")); got != (streaks{}) {
		t.Errorf("no days: got %+v", got)
	}
}
