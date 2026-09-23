package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"google.golang.org/protobuf/encoding/protojson"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

type Args struct {
	DatasetID int64 `json:"dataset_id"`
}

func (Args) Kind() string { return "export" }

func (Args) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 1}
}

type Worker struct {
	river.WorkerDefaults[Args]
	pool    *pgxpool.Pool
	store   *storage.Store
	version string
}

func NewWorker(pool *pgxpool.Pool, store *storage.Store, version string) *Worker {
	return &Worker{pool: pool, store: store, version: version}
}

func (w *Worker) Timeout(*river.Job[Args]) time.Duration { return 2 * time.Hour }

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	id := job.Args.DatasetID
	err := w.export(ctx, id)
	if err == nil {
		return nil
	}
	slog.Error("export failed", "dataset_id", id, "err", err)
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if ferr := db.New(w.pool).FailDataset(failCtx, db.FailDatasetParams{ID: id, Error: err.Error()}); ferr != nil {
		return errors.Join(err, fmt.Errorf("record failure: %w", ferr))
	}
	return err
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

func (w *Worker) export(ctx context.Context, id int64) error {
	q := db.New(w.pool)
	n, err := q.StartDataset(ctx, id)
	if err != nil {
		return fmt.Errorf("mark running: %w", err)
	}
	if n == 0 {
		return nil
	}
	ds, err := q.GetDataset(ctx, id)
	if err != nil {
		return fmt.Errorf("load dataset: %w", err)
	}
	opts := &krillv1.ExportOptions{}
	if err := protojson.Unmarshal(ds.Options, opts); err != nil {
		return fmt.Errorf("decode options: %w", err)
	}

	// Labels may have changed since the export was queued, so plan again
	// and store the stats that match what is actually written.
	plan, err := Load(ctx, q, opts)
	if err != nil {
		return err
	}
	statsJSON, err := protojson.Marshal(plan.Stats)
	if err != nil {
		return fmt.Errorf("encode stats: %w", err)
	}
	if err := q.SetDatasetStats(ctx, db.SetDatasetStatsParams{ID: id, Stats: statsJSON}); err != nil {
		return fmt.Errorf("save stats: %w", err)
	}
	if len(plan.Items) == 0 {
		return errors.New("no frames to export")
	}

	pr, pw := io.Pipe()
	upload := make(chan error, 1)
	go func() {
		err := w.store.Put(ctx, storage.DatasetKey(id), pr, -1, "application/zip")
		_ = pr.CloseWithError(err)
		upload <- err
	}()

	counter := &countingWriter{w: pw}
	lastSave := time.Now()
	manifest := Manifest{Name: ds.Name, CreatedAt: ds.CreatedAt.Time, Version: w.version, Options: ds.Options, Stats: statsJSON}
	open := func(ctx context.Context, f Frame) (io.ReadCloser, error) {
		return w.store.Get(ctx, storage.FrameKey(f.VideoID, f.Idx))
	}
	writeErr := Write(ctx, counter, plan, manifest, open, func(done, total int) {
		if time.Since(lastSave) < time.Second {
			return
		}
		lastSave = time.Now()
		p := float32(done) / float32(total)
		if err := q.SetDatasetProgress(ctx, db.SetDatasetProgressParams{ID: id, Progress: p}); err != nil {
			slog.Warn("save export progress", "dataset_id", id, "err", err)
		}
	})
	_ = pw.CloseWithError(writeErr)
	if err := <-upload; err != nil {
		return errors.Join(writeErr, err)
	}
	if writeErr != nil {
		return writeErr
	}

	if err := q.FinishDataset(ctx, db.FinishDatasetParams{ID: id, SizeBytes: counter.n}); err != nil {
		return fmt.Errorf("finish dataset: %w", err)
	}
	slog.Info("export finished", "dataset_id", id, "images", len(plan.Items), "bytes", counter.n)
	return nil
}
