package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
)

// ScheduleArgs checks whether a scheduled retrain is due. It runs at the top
// of every hour so a changed retrain hour applies without a restart.
type ScheduleArgs struct{}

func (ScheduleArgs) Kind() string { return "retrain_schedule" }

func (ScheduleArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1}
}

type hourly struct{}

func (hourly) Next(t time.Time) time.Time {
	return t.Truncate(time.Hour).Add(time.Hour)
}

// PeriodicJobs are the River periodic jobs the pipeline needs.
func PeriodicJobs() []*river.PeriodicJob {
	return []*river.PeriodicJob{
		river.NewPeriodicJob(hourly{}, func() (river.JobArgs, *river.InsertOpts) {
			return ScheduleArgs{}, nil
		}, &river.PeriodicJobOpts{ID: "retrain_schedule"}),
	}
}

type ScheduleWorker struct {
	river.WorkerDefaults[ScheduleArgs]
	p *Pipeline
}

// lastRun is what the schedule needs to know about the newest run.
type lastRun struct {
	createdAt time.Time
	trigger   string
}

// retrainDue decides whether a scheduled run should start at now. It waits
// for the configured hour, skips while a run is unfinished or a scheduled
// one already started today, and needs enough new labels to be worth it.
func retrainDue(cfg config.Config, now time.Time, active int, last *lastRun, newLabels int) bool {
	switch {
	case cfg.RetrainHour < 0, cfg.WorkerToken == "":
		return false
	case now.Hour() != cfg.RetrainHour:
		return false
	case active > 0:
		return false
	case last != nil && last.trigger == triggerSchedule && now.Sub(last.createdAt) < 20*time.Hour:
		return false
	}
	return newLabels >= cfg.RetrainMinNewLabels
}

// newLabels counts verified boxes added or changed since the newest run was
// queued, or all of them before the first run.
func (p *Pipeline) newLabels(ctx context.Context, q *db.Queries) (int, *lastRun, error) {
	latest, err := q.GetLatestTrainingRun(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		n, err := q.CountVerifiedBoxes(ctx)
		return int(n), nil, err
	}
	if err != nil {
		return 0, nil, err
	}
	n, err := q.CountVerifiedBoxesSince(ctx, latest.CreatedAt)
	return int(n), &lastRun{createdAt: latest.CreatedAt.Time, trigger: latest.Trigger}, err
}

func (w *ScheduleWorker) Work(ctx context.Context, _ *river.Job[ScheduleArgs]) error {
	p := w.p
	cfg := p.Config()
	if cfg.RetrainHour < 0 {
		return nil
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := p.q.WithTx(tx)
	// Serializes with a run being started by hand at the same moment.
	if _, err := tx.Exec(ctx, "LOCK TABLE training_runs IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return fmt.Errorf("lock runs: %w", err)
	}
	active, err := q.CountActiveTrainingRuns(ctx)
	if err != nil {
		return fmt.Errorf("count runs: %w", err)
	}
	labels, last, err := p.newLabels(ctx, q)
	if err != nil {
		return fmt.Errorf("count new labels: %w", err)
	}
	if !retrainDue(cfg, time.Now(), int(active), last, labels) {
		return nil
	}
	trainCfg, err := p.resolveConfig(ctx, q, nil)
	if err != nil {
		return err
	}
	run, err := p.queueRun(ctx, tx, triggerSchedule, pgtype.UUID{}, trainCfg)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	slog.Info("scheduled retrain queued", "run_id", run.ID, "new_labels", labels)
	return nil
}

// nextCheck is when the schedule next looks at whether to retrain.
func nextCheck(cfg config.Config, now time.Time) time.Time {
	if cfg.RetrainHour < 0 {
		return time.Time{}
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), cfg.RetrainHour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
