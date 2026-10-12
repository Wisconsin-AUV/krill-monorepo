package tracker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/ingest"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	// embedChunk frames go to the worker at a time, so a click waits for at
	// most one chunk of embedding to finish.
	embedChunk = 16
	// embedPickupTimeout is long because embed tasks wait behind every click.
	embedPickupTimeout = 10 * time.Minute
)

type EmbedWorker struct {
	river.WorkerDefaults[ingest.EmbedArgs]
	pool       *pgxpool.Pool
	store      *storage.Store
	dispatcher *Dispatcher
}

func NewEmbedWorker(pool *pgxpool.Pool, store *storage.Store, dispatcher *Dispatcher) *EmbedWorker {
	return &EmbedWorker{pool: pool, store: store, dispatcher: dispatcher}
}

func (w *EmbedWorker) Timeout(*river.Job[ingest.EmbedArgs]) time.Duration { return time.Hour }

func (w *EmbedWorker) Work(ctx context.Context, job *river.Job[ingest.EmbedArgs]) error {
	q := db.New(w.pool)
	clipID := job.Args.ClipID
	clip, err := q.GetClip(ctx, clipID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load clip: %w", err)
	}
	frames, err := q.ListClipFrames(ctx, clipID)
	if err != nil {
		return fmt.Errorf("list frames: %w", err)
	}

	var model string
	for start := 0; start < len(frames); start += embedChunk {
		chunk := frames[start:min(start+embedChunk, len(frames))]
		msg := &krillv1.EmbedTask{Id: w.dispatcher.nextID(), Frames: make([]*krillv1.EmbedFrame, len(chunk))}
		for i, f := range chunk {
			get, err := w.store.PresignGetInternal(ctx, storage.FrameKey(clip.VideoID, f.Idx), frameURLExpiry)
			if err != nil {
				return err
			}
			put, err := w.store.PresignPutInternal(ctx, storage.EmbeddingKey(clip.VideoID, f.Idx), frameURLExpiry)
			if err != nil {
				return err
			}
			msg.Frames[i] = &krillv1.EmbedFrame{Id: f.ID, Url: get, UploadUrl: put}
		}
		t := newEmbedTask(msg.GetId(), msg)
		if err := run(ctx, w.dispatcher, t, embedPickupTimeout); err != nil {
			return fmt.Errorf("embed frames %d to %d: %w", chunk[0].Idx, chunk[len(chunk)-1].Idx, err)
		}
		got := *t.model.Load()
		if model != "" && got != model {
			return fmt.Errorf("GPU worker switched from %s to %s mid-clip", model, got)
		}
		model = got
	}
	if err := q.SetClipEmbedded(ctx, db.SetClipEmbeddedParams{ID: clipID, EmbedModel: model}); err != nil {
		return fmt.Errorf("mark clip embedded: %w", err)
	}
	slog.Info("clip embedded", "clip_id", clipID, "frames", len(frames), "model", model)
	return nil
}
