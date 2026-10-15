package storagescan

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

// cleanupBatch is how many orphans are checked against a fresh load of the
// database before they're deleted, which keeps the time between the check
// and the delete short.
const cleanupBatch = 1000

type cleanupResult struct {
	deleted int64
	bytes   int64
	skipped int64
}

// cleanUp deletes the scan's orphans that are still orphans now.
func cleanUp(ctx context.Context, store objectStore, q *db.Queries, scanID int64, now func() time.Time) (cleanupResult, error) {
	var res cleanupResult
	var afterKey, afterUpload string
	for {
		rows, err := q.ListStorageOrphans(ctx, db.ListStorageOrphansParams{
			ScanID: scanID, AfterKey: afterKey, AfterUploadID: afterUpload, Limit: cleanupBatch,
		})
		if err != nil {
			return res, fmt.Errorf("list orphans: %w", err)
		}
		if len(rows) == 0 {
			return res, nil
		}
		last := rows[len(rows)-1]
		afterKey, afterUpload = last.Key, last.UploadID

		s, err := load(ctx, q, now())
		if err != nil {
			return res, err
		}
		var keys []string
		var bytes int64
		for _, r := range rows {
			if r.UploadID != "" {
				ok, err := cleanUpUpload(ctx, store, s, r.Key, r.UploadID, &res)
				if err != nil {
					return res, err
				}
				if !ok {
					res.skipped++
				}
				continue
			}
			o, exists, err := store.Stat(ctx, r.Key)
			if err != nil {
				return res, err
			}
			orphan := false
			if exists {
				if orphan, err = s.orphan(ctx, parseKey(r.Key), r.Key, o.Modified); err != nil {
					return res, err
				}
			}
			if !orphan {
				res.skipped++
				continue
			}
			keys = append(keys, r.Key)
			bytes += o.Size
		}
		if len(keys) > 0 {
			if err := store.RemoveKeys(ctx, keys); err != nil {
				return res, err
			}
			res.deleted += int64(len(keys))
			res.bytes += bytes
		}
	}
}

func cleanUpUpload(ctx context.Context, store objectStore, s *state, key, uploadID string, res *cleanupResult) (bool, error) {
	for u, err := range store.ListUploads(ctx, key) {
		if err != nil {
			return false, err
		}
		if u.Key != key || u.ID != uploadID {
			continue
		}
		if !s.orphanUpload(u) {
			return false, nil
		}
		if err := store.AbortUpload(ctx, key, uploadID); err != nil {
			return false, err
		}
		res.deleted++
		res.bytes += u.Size
		return true, nil
	}
	return false, nil
}

type CleanupArgs struct {
	ScanID int64 `json:"scan_id"`
}

func (CleanupArgs) Kind() string { return "storage_cleanup" }

func (CleanupArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Queue, MaxAttempts: 1}
}

type CleanupWorker struct {
	river.WorkerDefaults[CleanupArgs]
	pool  *pgxpool.Pool
	store objectStore
}

func NewCleanupWorker(pool *pgxpool.Pool, store *storage.Store) *CleanupWorker {
	return &CleanupWorker{pool: pool, store: store}
}

func (w *CleanupWorker) Timeout(*river.Job[CleanupArgs]) time.Duration { return 6 * time.Hour }

func (w *CleanupWorker) Work(ctx context.Context, job *river.Job[CleanupArgs]) error {
	id := job.Args.ScanID
	q := db.New(w.pool)
	n, err := q.StartStorageCleanup(ctx, id)
	if err != nil {
		return fmt.Errorf("start cleanup of scan %d: %w", id, err)
	}
	if n == 0 {
		return nil
	}
	res, err := cleanUp(ctx, w.store, q, id, time.Now)
	// Whatever was deleted before a failure is gone either way, so it's
	// recorded with the error.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err != nil {
		ferr := q.FailStorageCleanup(saveCtx, db.FailStorageCleanupParams{
			ID: id, CleanupError: err.Error(),
			DeletedObjects: res.deleted, DeletedBytes: res.bytes, SkippedObjects: res.skipped,
		})
		return errors.Join(err, ferr)
	}
	slog.Info("storage cleanup finished", "scan_id", id, "deleted", res.deleted, "bytes", res.bytes, "skipped", res.skipped)
	return q.FinishStorageCleanup(saveCtx, db.FinishStorageCleanupParams{
		ID: id, DeletedObjects: res.deleted, DeletedBytes: res.bytes, SkippedObjects: res.skipped,
	})
}
