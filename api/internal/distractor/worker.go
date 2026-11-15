package distractor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

// Queue runs one distractor at a time, since decoding a camera's full-size
// image can hold over a hundred megabytes.
const Queue = "distractors"

type Args struct {
	DistractorID int64 `json:"distractor_id"`
}

func (Args) Kind() string { return "distractor" }

// A failed distractor is surfaced in the admin panel to retry, instead of
// River retrying silently in the background.
func (Args) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: Queue, MaxAttempts: 1}
}

type Worker struct {
	river.WorkerDefaults[Args]
	pool  *pgxpool.Pool
	store *storage.Store
}

func NewWorker(pool *pgxpool.Pool, store *storage.Store) *Worker {
	return &Worker{pool: pool, store: store}
}

func (w *Worker) Timeout(*river.Job[Args]) time.Duration { return 5 * time.Minute }

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	id := job.Args.DistractorID
	err := w.process(ctx, id)
	if err == nil {
		return nil
	}
	slog.Error("processing distractor failed", "distractor_id", id, "err", err)
	// ctx may already be cancelled, and the failure still needs recording.
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if ferr := db.New(w.pool).FailDistractor(failCtx, db.FailDistractorParams{ID: id, Error: err.Error()}); ferr != nil {
		return errors.Join(err, fmt.Errorf("record failure: %w", ferr))
	}
	return err
}

func (w *Worker) process(ctx context.Context, id int64) error {
	q := db.New(w.pool)
	n, err := q.StartDistractor(ctx, id)
	if err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}
	if n == 0 {
		slog.Info("distractor skipped, it was deleted", "distractor_id", id)
		return nil
	}

	upload, err := w.read(ctx, storage.DistractorUploadKey(id))
	if err != nil {
		return err
	}
	// The upload URL outlives ProcessDistractor's check, so the image could
	// have been replaced since.
	if _, err := checkImage(bytes.NewReader(upload)); err != nil {
		return err
	}
	out, err := process(upload)
	if err != nil {
		return err
	}
	if err := w.store.Put(ctx, storage.DistractorKey(id), bytes.NewReader(out.image), int64(len(out.image)), "image/jpeg"); err != nil {
		return err
	}
	if err := w.store.Put(ctx, storage.DistractorThumbnailKey(id), bytes.NewReader(out.thumbnail), int64(len(out.thumbnail)), "image/jpeg"); err != nil {
		return err
	}
	n, err = q.FinishDistractor(ctx, db.FinishDistractorParams{
		ID:        id,
		Width:     int32(out.width),  //nolint:gosec // at most maxSide
		Height:    int32(out.height), //nolint:gosec // at most maxSide
		SizeBytes: int64(len(out.image)),
	})
	if err != nil {
		return fmt.Errorf("finish: %w", err)
	}
	if n == 0 {
		// Deleted while processing, after DeleteDistractors removed its
		// objects, so the ones written here are removed too.
		return w.store.RemovePrefix(ctx, storage.DistractorPrefix(id))
	}
	if err := w.store.Remove(ctx, storage.DistractorUploadKey(id)); err != nil {
		slog.WarnContext(ctx, "remove processed distractor upload", "distractor_id", id, "err", err)
	}
	slog.Info("distractor processed", "distractor_id", id, "width", out.width, "height", out.height, "bytes", len(out.image))
	return nil
}

func (w *Worker) read(ctx context.Context, key string) ([]byte, error) {
	rc, err := w.store.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	defer func() { _ = rc.Close() }()
	b, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if len(b) > maxBytes {
		return nil, fmt.Errorf("the image is larger than %d MB", maxBytes>>20)
	}
	return b, nil
}
