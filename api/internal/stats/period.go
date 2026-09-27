package stats

import (
	"errors"
	"time"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

const dateLayout = time.DateOnly

var periods = []krillv1.Period{
	krillv1.Period_PERIOD_DAY,
	krillv1.Period_PERIOD_WEEK,
	krillv1.Period_PERIOD_MONTH,
	krillv1.Period_PERIOD_ALL_TIME,
}

func location(name string) (*time.Location, error) {
	if name == "" {
		return time.UTC, nil
	}
	// LoadLocation treats "Local" as the server's zone, not the caller's.
	if name == "Local" {
		return nil, errors.New("unknown time zone")
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, errors.New("unknown time zone")
	}
	return loc, nil
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	daysSinceMonday := (int(t.Weekday()) + 6) % 7
	return startOfDay(t.AddDate(0, 0, -daysSinceMonday))
}

// since returns when the period containing now started, in now's location,
// and false for PERIOD_ALL_TIME.
func since(p krillv1.Period, now time.Time) (time.Time, bool, error) {
	switch p {
	case krillv1.Period_PERIOD_DAY:
		return startOfDay(now), true, nil
	case krillv1.Period_PERIOD_WEEK:
		return startOfWeek(now), true, nil
	case krillv1.Period_PERIOD_MONTH:
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()), true, nil
	case krillv1.Period_PERIOD_ALL_TIME:
		return time.Time{}, false, nil
	default:
		return time.Time{}, false, errors.New("unknown period")
	}
}

type streaks struct {
	current, longest int32
}

// countStreaks takes active days oldest first as UTC midnights. A streak
// still counts as current until a whole day passes without a contribution.
func countStreaks(days []time.Time, today time.Time) streaks {
	var s streaks
	var run int32
	var prev time.Time
	for _, d := range days {
		if d.Sub(prev) == 24*time.Hour {
			run++
		} else {
			run = 1
		}
		s.longest = max(s.longest, run)
		prev = d
	}
	if len(days) > 0 && today.Sub(prev) <= 24*time.Hour {
		s.current = run
	}
	return s
}
