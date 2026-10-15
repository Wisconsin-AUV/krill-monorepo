package backup

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

// Queue runs one backup at a time.
const Queue = "backup"

// ErrBusy is returned when a backup is already queued or running.
var ErrBusy = errors.New("a backup is already running")

// create queues a backup. by is uuid.Nil for scheduled backups.
func create(ctx context.Context, tx pgx.Tx, jobs *river.Client[pgx.Tx], scheduled bool, by uuid.UUID) (db.Backup, error) {
	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return db.Backup{}, fmt.Errorf("generate key: %w", err)
	}
	key := storage.BackupKey(fmt.Sprintf("krill-%s-%s.dump", time.Now().UTC().Format("20060102-150405"), hex.EncodeToString(suffix)))
	b, err := db.New(tx).CreateBackup(ctx, db.CreateBackupParams{
		Scheduled: scheduled,
		ObjectKey: key,
		CreatedBy: pgtype.UUID{Bytes: by, Valid: by != uuid.Nil},
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return db.Backup{}, ErrBusy
	}
	if err != nil {
		return db.Backup{}, fmt.Errorf("create backup: %w", err)
	}
	if _, err := jobs.InsertTx(ctx, tx, Args{BackupID: b.ID}, nil); err != nil {
		return db.Backup{}, fmt.Errorf("queue backup: %w", err)
	}
	return b, nil
}

type Args struct {
	BackupID int64 `json:"backup_id"`
}

func (Args) Kind() string { return "backup" }

func (Args) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Queue, MaxAttempts: 1}
}

type Worker struct {
	river.WorkerDefaults[Args]
	pool        *pgxpool.Pool
	store       *storage.Store
	databaseURL string
	settings    func() config.Config
}

func NewWorker(pool *pgxpool.Pool, store *storage.Store, databaseURL string, settings func() config.Config) *Worker {
	return &Worker{pool: pool, store: store, databaseURL: databaseURL, settings: settings}
}

func (w *Worker) Timeout(*river.Job[Args]) time.Duration { return 6 * time.Hour }

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	q := db.New(w.pool)
	id := job.Args.BackupID
	n, err := q.StartBackup(ctx, id)
	if err != nil {
		return fmt.Errorf("start backup %d: %w", id, err)
	}
	if n == 0 {
		return nil
	}
	b, err := q.GetBackup(ctx, id)
	if err != nil {
		return fmt.Errorf("load backup %d: %w", id, err)
	}
	start := time.Now()
	size, err := dump(ctx, w.databaseURL, func(ctx context.Context, r io.Reader) error {
		return w.store.PutStream(ctx, b.ObjectKey, r, "application/octet-stream")
	})
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err != nil {
		slog.Error("backup failed", "backup_id", id, "err", err)
		if rerr := w.store.Remove(saveCtx, b.ObjectKey); rerr != nil {
			slog.Warn("remove partial backup", "backup_id", id, "err", rerr)
		}
		return errors.Join(err, q.FailBackup(saveCtx, db.FailBackupParams{ID: id, Error: err.Error()}))
	}
	if err := q.FinishBackup(saveCtx, db.FinishBackupParams{ID: id, SizeBytes: size}); err != nil {
		return fmt.Errorf("finish backup %d: %w", id, err)
	}
	slog.Info("backup finished", "backup_id", id, "bytes", size, "took", time.Since(start).Round(time.Millisecond))
	if b.Scheduled {
		if err := w.prune(saveCtx, w.settings().BackupKeep); err != nil {
			slog.Error("delete old backups", "err", err)
		}
	}
	return nil
}

// prune deletes scheduled backups beyond the newest keep.
func (w *Worker) prune(ctx context.Context, keep int) error {
	expired, err := db.New(w.pool).ListExpiredScheduledBackups(ctx, int32(keep)) //nolint:gosec // keep is a small setting
	if err != nil {
		return err
	}
	for _, b := range expired {
		if err := remove(ctx, w.pool, w.store, b.ID, map[string]any{"expired": true}); err != nil {
			return err
		}
	}
	return nil
}

// remove deletes a backup's row, audited, and then its object.
func remove(ctx context.Context, pool *pgxpool.Pool, store *storage.Store, id int64, details map[string]any) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	b, err := q.DeleteBackup(ctx, id)
	if err != nil {
		return err
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.BackupDelete,
		TargetType: audit.TargetBackup,
		TargetID:   strconv.FormatInt(b.ID, 10),
		TargetName: filename(b.ObjectKey),
		Details:    details,
	})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if err := store.Remove(ctx, b.ObjectKey); err != nil {
		// The row is gone, so a storage scan will find the object.
		slog.Error("remove backup object", "backup_id", id, "err", err)
	}
	return nil
}

// ScheduleArgs checks whether a scheduled backup is due. It runs often, so a
// changed schedule applies without a restart.
type ScheduleArgs struct{}

func (ScheduleArgs) Kind() string { return "backup_schedule" }

func (ScheduleArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Queue, MaxAttempts: 1}
}

// ScheduleInterval is how often the schedule is checked.
const ScheduleInterval = 10 * time.Minute

type ScheduleWorker struct {
	river.WorkerDefaults[ScheduleArgs]
	pool     *pgxpool.Pool
	settings func() config.Config
}

func NewScheduleWorker(pool *pgxpool.Pool, settings func() config.Config) *ScheduleWorker {
	return &ScheduleWorker{pool: pool, settings: settings}
}

func (w *ScheduleWorker) Work(ctx context.Context, _ *river.Job[ScheduleArgs]) error {
	hour := w.settings().BackupHour
	if hour < 0 || Available() != nil {
		return nil
	}
	last, err := db.New(w.pool).LastScheduledBackupAt(ctx)
	if err != nil {
		return fmt.Errorf("find last scheduled backup: %w", err)
	}
	if !due(time.Now(), hour, last.Time) {
		return nil
	}
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	client, err := river.ClientFromContextSafely[pgx.Tx](ctx)
	if err != nil {
		return err
	}
	if _, err := create(ctx, tx, client, true, uuid.Nil); errors.Is(err, ErrBusy) {
		// A backup made by hand is running. The next check starts this one.
		return nil
	} else if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
