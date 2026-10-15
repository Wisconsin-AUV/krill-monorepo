package trash

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/wauv/krill/api/internal/db"
)

// AutoPurgeArgs purges what has been in the trash longer than the retention
// setting.
type AutoPurgeArgs struct{}

func (AutoPurgeArgs) Kind() string { return "auto_purge_trash" }

// AutoPurgeJob runs hourly so a changed retention applies within the hour.
func AutoPurgeJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(time.Hour),
		func() (river.JobArgs, *river.InsertOpts) { return AutoPurgeArgs{}, nil },
		&river.PeriodicJobOpts{ID: "auto_purge_trash", RunOnStart: true},
	)
}

type AutoPurgeWorker struct {
	river.WorkerDefaults[AutoPurgeArgs]
	pool *pgxpool.Pool
	// days is the live retention setting. 0 keeps everything.
	days func() int
}

func NewAutoPurgeWorker(pool *pgxpool.Pool, days func() int) *AutoPurgeWorker {
	return &AutoPurgeWorker{pool: pool, days: days}
}

func (w *AutoPurgeWorker) Work(ctx context.Context, _ *river.Job[AutoPurgeArgs]) error {
	days := w.days()
	if days <= 0 {
		return nil
	}
	before := time.Now().AddDate(0, 0, -days)
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	rows, err := q.ListTrashIDs(ctx, pgtype.Timestamptz{Time: before, Valid: true})
	if err != nil {
		return fmt.Errorf("list expired trash: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	var videos, clips []int64
	for _, r := range rows {
		if r.Kind == "video" {
			videos = append(videos, r.ID)
		} else {
			clips = append(clips, r.ID)
		}
	}
	details := map[string]any{"automatic": true, "retention_days": days}
	p, err := Purge(ctx, q, tx, river.ClientFromContext[pgx.Tx](ctx), videos, clips, true, details)
	if err != nil {
		return err
	}
	if len(p.Jobs) > 0 {
		_, err := q.CreateContentJob(ctx, db.CreateContentJobParams{Kind: JobKind, Summary: p.Summary(), RiverJobIds: p.Jobs})
		if err != nil {
			return fmt.Errorf("record purge: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	slog.InfoContext(ctx, "purged expired trash", "videos", p.Videos, "clips", p.Clips, "retention_days", days)
	return nil
}
