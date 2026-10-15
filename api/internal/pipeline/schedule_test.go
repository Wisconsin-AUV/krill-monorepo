package pipeline

import (
	"testing"
	"time"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/config"
)

func TestRetrainDue(t *testing.T) {
	cfg := config.Config{RetrainHour: 3, RetrainMinNewLabels: 100, WorkerToken: "token"}
	at3 := time.Date(2026, 10, 14, 3, 0, 5, 0, time.UTC)
	tests := []struct {
		name   string
		cfg    func(c *config.Config)
		now    time.Time
		active int
		last   *lastRun
		labels int
		want   bool
	}{
		{name: "due", now: at3, labels: 100, want: true},
		{name: "off", cfg: func(c *config.Config) { c.RetrainHour = -1 }, now: at3, labels: 500},
		{name: "no worker", cfg: func(c *config.Config) { c.WorkerToken = "" }, now: at3, labels: 500},
		{name: "another hour", now: at3.Add(time.Hour), labels: 500},
		{name: "run unfinished", now: at3, active: 1, labels: 500},
		{name: "too few new labels", now: at3, labels: 99},
		{
			name:   "already ran on schedule today",
			now:    at3,
			last:   &lastRun{createdAt: at3.Add(-time.Minute), trigger: triggerSchedule},
			labels: 500,
		},
		{
			name:   "ran yesterday",
			now:    at3,
			last:   &lastRun{createdAt: at3.Add(-24 * time.Hour), trigger: triggerSchedule},
			labels: 500,
			want:   true,
		},
		{
			name:   "trained by hand earlier",
			now:    at3,
			last:   &lastRun{createdAt: at3.Add(-time.Hour), trigger: triggerManual},
			labels: 500,
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := cfg
			if tt.cfg != nil {
				tt.cfg(&c)
			}
			if got := retrainDue(c, tt.now, tt.active, tt.last, tt.labels); got != tt.want {
				t.Errorf("retrainDue = %v, want %v", got, tt.want)
			}
		})
	}
}

func metrics(m50_95 float64) *krillv1.EvalMetrics {
	return &krillv1.EvalMetrics{All: &krillv1.ClassMetrics{Map50_95: m50_95}}
}

func TestBeatsBaselinePrefersTheHeldOutTest(t *testing.T) {
	tests := []struct {
		name string
		run  *krillv1.TrainingRun
		want bool
	}{
		{name: "nothing to compare", run: &krillv1.TrainingRun{Val: metrics(0.9)}},
		{
			name: "better on val but worse on test",
			run: &krillv1.TrainingRun{
				BaselineRunId: 1,
				Val:           metrics(0.8), BaselineVal: metrics(0.5),
				Test: metrics(0.4), BaselineTest: metrics(0.45),
			},
		},
		{
			name: "better on test",
			run: &krillv1.TrainingRun{
				BaselineRunId: 1,
				Val:           metrics(0.5), BaselineVal: metrics(0.6),
				Test: metrics(0.5), BaselineTest: metrics(0.45),
			},
			want: true,
		},
		{
			name: "no test videos",
			run:  &krillv1.TrainingRun{BaselineRunId: 1, Val: metrics(0.7), BaselineVal: metrics(0.6)},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := beatsBaseline(tt.run); got != tt.want {
				t.Errorf("beatsBaseline = %v, want %v", got, tt.want)
			}
		})
	}
}
