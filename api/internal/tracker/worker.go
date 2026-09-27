package tracker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	// pickupTimeout is how long a task waits for a GPU worker to poll.
	pickupTimeout = 2 * time.Minute
	// idleTimeout fails a task whose worker stopped reporting, for example
	// because it crashed.
	idleTimeout    = 2 * time.Minute
	frameURLExpiry = time.Hour
)

var errNoWorker = errors.New("no GPU worker picked up the task")

type Worker struct {
	river.WorkerDefaults[annotation.TrackArgs]
	pool       *pgxpool.Pool
	store      *storage.Store
	dispatcher *Dispatcher
}

func NewWorker(pool *pgxpool.Pool, store *storage.Store, dispatcher *Dispatcher) *Worker {
	return &Worker{pool: pool, store: store, dispatcher: dispatcher}
}

func (w *Worker) Timeout(*river.Job[annotation.TrackArgs]) time.Duration { return 30 * time.Minute }

func (w *Worker) Work(ctx context.Context, job *river.Job[annotation.TrackArgs]) error {
	args := job.Args
	err := w.track(ctx, job.ID, args)
	if err != nil {
		slog.Error("tracking failed", "track_id", args.TrackID, "frame_id", args.FrameID, "err", err)
	}
	// A new track whose object was never found has no boxes to show.
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if _, cerr := db.New(w.pool).DeleteTrackIfEmpty(cleanupCtx, args.TrackID); cerr != nil {
		err = errors.Join(err, fmt.Errorf("delete empty track: %w", cerr))
	}
	return err
}

func (w *Worker) track(ctx context.Context, id int64, args annotation.TrackArgs) error {
	q := db.New(w.pool)
	frame, err := q.GetFrame(ctx, args.FrameID)
	if err != nil {
		return fmt.Errorf("load frame: %w", err)
	}
	clip, err := q.GetClip(ctx, frame.ClipID)
	if err != nil {
		return fmt.Errorf("load clip: %w", err)
	}
	frames, err := q.ListFramesFrom(ctx, db.ListFramesFromParams{ClipID: frame.ClipID, Idx: frame.Idx})
	if err != nil {
		return fmt.Errorf("list frames: %w", err)
	}
	if err := q.DeleteTrackProposalsFrom(ctx, db.DeleteTrackProposalsFromParams{TrackID: args.TrackID, FromIdx: frame.Idx}); err != nil {
		return fmt.Errorf("clear old proposals: %w", err)
	}

	msg := &krillv1.TrackTask{Id: id, Frames: make([]*krillv1.TaskFrame, len(frames))}
	for i, f := range frames {
		url, err := w.store.PresignGetInternal(ctx, storage.FrameKey(clip.VideoID, f.Idx), frameURLExpiry)
		if err != nil {
			return err
		}
		msg.Frames[i] = &krillv1.TaskFrame{Id: f.ID, Url: url}
	}
	switch {
	case args.Point != nil:
		msg.Prompt = &krillv1.TrackTask_Point{Point: &krillv1.Point{X: args.Point.X, Y: args.Point.Y}}
	case args.Box != nil:
		b := args.Box
		msg.Prompt = &krillv1.TrackTask_Box{Box: &krillv1.Box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height}}
	default:
		return errors.New("job has no prompt")
	}

	t := newTask(id, args.TrackID, msg)
	w.dispatcher.add(t)
	defer w.dispatcher.remove(id)

	pickup := time.NewTimer(pickupTimeout)
	defer pickup.Stop()
	select {
	case w.dispatcher.queue <- t:
	case <-pickup.C:
		return errNoWorker
	case <-ctx.Done():
		return ctx.Err()
	}

	idle := time.NewTimer(idleTimeout)
	defer idle.Stop()
	for {
		select {
		case err := <-t.done:
			return err
		case <-t.progress:
			idle.Reset(idleTimeout)
		case <-idle.C:
			return errors.New("GPU worker stopped reporting")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
