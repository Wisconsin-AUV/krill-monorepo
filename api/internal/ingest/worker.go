package ingest

import (
	"context"
	"errors"
	"fmt"
	"image/jpeg"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"golang.org/x/sync/errgroup"

	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	Queue           = "ingest"
	uploadWorkers   = 8
	progressEvery   = time.Second
	extractFraction = 0.5
)

type Args struct {
	VideoID int64 `json:"video_id"`
}

func (Args) Kind() string { return "ingest" }

// A failed ingest is surfaced on the video so the user can retry it from the
// UI, instead of River retrying silently in the background.
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

func (w *Worker) Timeout(*river.Job[Args]) time.Duration { return 2 * time.Hour }

func (w *Worker) Work(ctx context.Context, job *river.Job[Args]) error {
	id := job.Args.VideoID
	err := w.ingest(ctx, id)
	if err == nil {
		return nil
	}
	slog.Error("ingest failed", "video_id", id, "err", err)
	// ctx may already be cancelled, and the failure still needs recording.
	failCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if ferr := db.New(w.pool).FailIngest(failCtx, db.FailIngestParams{ID: id, Error: err.Error()}); ferr != nil {
		return errors.Join(err, fmt.Errorf("record failure: %w", ferr))
	}
	return err
}

func (w *Worker) ingest(ctx context.Context, id int64) error {
	q := db.New(w.pool)
	n, err := q.StartProcessing(ctx, id)
	if err != nil {
		return fmt.Errorf("mark processing: %w", err)
	}
	if n == 0 {
		slog.Info("ingest skipped, video was deleted", "video_id", id)
		return nil
	}
	video, err := q.GetVideo(ctx, id)
	if err != nil {
		return fmt.Errorf("load video: %w", err)
	}

	if err := q.DeleteClips(ctx, id); err != nil {
		return fmt.Errorf("clear old clips: %w", err)
	}
	if err := w.store.RemovePrefix(ctx, storage.FramesPrefix(id)); err != nil {
		return fmt.Errorf("clear old frames: %w", err)
	}

	tmp, err := os.MkdirTemp("", fmt.Sprintf("krill-ingest-%d-", id))
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	src := filepath.Join(tmp, "source")
	if err := w.store.Download(ctx, storage.VideoSourceKey(id), src); err != nil {
		return err
	}
	info, err := probe(ctx, src)
	if err != nil {
		return err
	}

	fps := info.FPS
	extractFPS := 0.0
	if video.ExtractFps > 0 && video.ExtractFps < info.FPS {
		fps, extractFPS = video.ExtractFps, video.ExtractFps
	}
	expected := max(float64(info.DurationMs)/1000*fps, 1)

	progress := newProgressReporter(ctx, q, id)
	defer progress.stop()
	framesDir := filepath.Join(tmp, "frames")
	if err := os.Mkdir(framesDir, 0o750); err != nil {
		return fmt.Errorf("create frames dir: %w", err)
	}
	err = extractFrames(ctx, src, framesDir, extractFPS, func(n int) {
		progress.set(extractFraction * min(float64(n)/expected, 1))
	})
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(framesDir, "*.jpg"))
	if err != nil {
		return fmt.Errorf("list frames: %w", err)
	}
	if len(files) == 0 {
		return errors.New("ffmpeg produced no frames")
	}
	sort.Strings(files)

	hashes, width, height, err := w.uploadFrames(ctx, id, files, func(done int) {
		progress.set(extractFraction + (1-extractFraction)*float64(done)/float64(len(files)))
	})
	if err != nil {
		return err
	}
	progress.stop()

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := q.WithTx(tx)

	if err := qtx.SetVideoProbe(ctx, db.SetVideoProbeParams{
		ID: id, Width: width, Height: height, Fps: fps, DurationMs: info.DurationMs,
	}); err != nil {
		return fmt.Errorf("save probe: %w", err)
	}
	if err := insertClips(ctx, qtx, id, hashes, fps); err != nil {
		return err
	}
	if err := qtx.FinishIngest(ctx, db.FinishIngestParams{ID: id, FrameCount: int32(len(files))}); err != nil { //nolint:gosec // frame counts fit in int32
		return fmt.Errorf("finish ingest: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	slog.Info("ingest finished", "video_id", id, "frames", len(files), "fps", fps)
	return nil
}

func insertClips(ctx context.Context, q *db.Queries, videoID int64, hashes []uint64, fps float64) error {
	for i, c := range chunkClips(len(hashes), fps) {
		clipID, err := q.CreateClip(ctx, db.CreateClipParams{
			VideoID:    videoID,
			Idx:        int32(i),       //nolint:gosec // clip counts fit in int32
			StartFrame: int32(c.Start), //nolint:gosec // frame counts fit in int32
			FrameCount: int32(c.Count), //nolint:gosec // frame counts fit in int32
		})
		if err != nil {
			return fmt.Errorf("create clip %d: %w", i, err)
		}
		rows := make([]db.InsertFramesParams, c.Count)
		for j := range rows {
			idx := c.Start + j
			rows[j] = db.InsertFramesParams{
				VideoID: videoID,
				ClipID:  clipID,
				Idx:     int32(idx),         //nolint:gosec // frame counts fit in int32
				Phash:   int64(hashes[idx]), //nolint:gosec // stored bit-for-bit in a bigint
			}
		}
		if _, err := q.InsertFrames(ctx, rows); err != nil {
			return fmt.Errorf("insert frames for clip %d: %w", i, err)
		}
	}
	return nil
}

func (w *Worker) uploadFrames(ctx context.Context, videoID int64, files []string, onDone func(int)) ([]uint64, int32, int32, error) {
	hashes := make([]uint64, len(files))
	var width, height int32
	var done atomic.Int64

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(uploadWorkers)
	for i, path := range files {
		g.Go(func() error {
			hash, w2, h2, err := hashFile(path)
			if err != nil {
				return err
			}
			hashes[i] = hash
			if i == 0 {
				width, height = w2, h2
			}
			f, err := os.Open(path) //nolint:gosec // path comes from our own temp dir
			if err != nil {
				return fmt.Errorf("open frame: %w", err)
			}
			defer func() { _ = f.Close() }()
			st, err := f.Stat()
			if err != nil {
				return fmt.Errorf("stat frame: %w", err)
			}
			//nolint:gosec // i is bounded by the frame count
			if err := w.store.Put(gctx, storage.FrameKey(videoID, int32(i)), f, st.Size(), "image/jpeg"); err != nil {
				return err
			}
			onDone(int(done.Add(1)))
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, 0, 0, err
	}
	return hashes, width, height, nil
}

func hashFile(path string) (uint64, int32, int32, error) {
	f, err := os.Open(path) //nolint:gosec // path comes from our own temp dir
	if err != nil {
		return 0, 0, 0, fmt.Errorf("open frame: %w", err)
	}
	defer func() { _ = f.Close() }()
	img, err := jpeg.Decode(f)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	b := img.Bounds()
	return DHash(img), int32(b.Dx()), int32(b.Dy()), nil //nolint:gosec // image sizes fit in int32
}

// progressReporter throttles progress writes so frame callbacks do not hit
// the database for every frame.
type progressReporter struct {
	ctx     context.Context
	q       *db.Queries
	id      int64
	value   atomic.Uint64
	stopped chan struct{}
	done    chan struct{}
}

func newProgressReporter(ctx context.Context, q *db.Queries, id int64) *progressReporter {
	p := &progressReporter{ctx: ctx, q: q, id: id, stopped: make(chan struct{}), done: make(chan struct{})}
	go p.loop()
	return p
}

func (p *progressReporter) set(v float64) {
	p.value.Store(uint64(v * 1e6))
}

func (p *progressReporter) loop() {
	defer close(p.done)
	ticker := time.NewTicker(progressEvery)
	defer ticker.Stop()
	var last uint64
	for {
		select {
		case <-p.stopped:
			return
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			v := p.value.Load()
			if v == last {
				continue
			}
			last = v
			if err := p.q.SetIngestProgress(p.ctx, db.SetIngestProgressParams{ID: p.id, IngestProgress: float32(v) / 1e6}); err != nil {
				slog.Warn("save ingest progress", "video_id", p.id, "err", err)
			}
		}
	}
}

func (p *progressReporter) stop() {
	select {
	case <-p.stopped:
	default:
		close(p.stopped)
	}
	<-p.done
}
