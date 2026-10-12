package tracker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	// sameObjectIoU is the overlap above which a found box is taken to be an
	// object that already has a box, such as the exemplar itself.
	sameObjectIoU = 0.5
	// maxInstances bounds how many tracks one search can start, since each
	// one queues a tracking job for the rest of the clip.
	maxInstances = 50
)

type ExemplarWorker struct {
	river.WorkerDefaults[annotation.ExemplarArgs]
	pool       *pgxpool.Pool
	store      *storage.Store
	dispatcher *Dispatcher
}

func NewExemplarWorker(pool *pgxpool.Pool, store *storage.Store, dispatcher *Dispatcher) *ExemplarWorker {
	return &ExemplarWorker{pool: pool, store: store, dispatcher: dispatcher}
}

func (w *ExemplarWorker) Timeout(*river.Job[annotation.ExemplarArgs]) time.Duration {
	return 10 * time.Minute
}

func (w *ExemplarWorker) Work(ctx context.Context, job *river.Job[annotation.ExemplarArgs]) error {
	ids, err := w.search(ctx, job.ID, job.Args)
	if err != nil {
		slog.Error("exemplar search failed", "track_id", job.Args.TrackID, "frame_id", job.Args.FrameID, "err", err)
		return err
	}
	return river.RecordOutput(ctx, annotation.ExemplarOutput{TrackIDs: ids})
}

func (w *ExemplarWorker) search(ctx context.Context, id int64, args annotation.ExemplarArgs) ([]int64, error) {
	q := db.New(w.pool)
	frame, err := q.GetFrame(ctx, args.FrameID)
	if err != nil {
		return nil, fmt.Errorf("load frame: %w", err)
	}
	clip, err := q.GetClip(ctx, frame.ClipID)
	if err != nil {
		return nil, fmt.Errorf("load clip: %w", err)
	}
	url, err := w.store.PresignGetInternal(ctx, storage.FrameKey(clip.VideoID, frame.Idx), frameURLExpiry)
	if err != nil {
		return nil, err
	}
	b := args.Box
	t := newExemplarTask(id, &krillv1.ExemplarTask{
		Id:    id,
		Frame: &krillv1.TaskFrame{Id: frame.ID, Url: url},
		Box:   &krillv1.Box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height},
	})
	if err := w.dispatcher.run(ctx, t); err != nil {
		return nil, err
	}
	var res exemplarResult
	select {
	case res = <-t.found:
	default:
		return nil, errors.New("GPU worker finished without reporting boxes")
	}

	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q = q.WithTx(tx)

	exemplar, err := q.GetTrack(ctx, args.TrackID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New("the exemplar's track was deleted during the search")
	}
	if err != nil {
		return nil, fmt.Errorf("load track: %w", err)
	}
	rows, err := q.ListFrameBoxes(ctx, frame.ID)
	if err != nil {
		return nil, fmt.Errorf("list boxes: %w", err)
	}
	existing := make([]annotation.Box, len(rows))
	for i, r := range rows {
		existing[i] = annotation.Box{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}
	}

	found := newInstances(res.boxes, existing)
	ids := make([]int64, 0, len(found))
	jobs := make([]river.InsertManyParams, 0, len(found))
	for _, f := range found {
		track, err := q.CreateTrack(ctx, db.CreateTrackParams{
			ClipID: exemplar.ClipID, LabelTypeID: exemplar.LabelTypeID, Attributes: exemplar.Attributes,
		})
		if err != nil {
			return nil, fmt.Errorf("create track: %w", err)
		}
		_, err = q.UpsertProposals(ctx, db.UpsertProposalsParams{
			TrackID:      track.ID,
			FrameIds:     []int64{frame.ID},
			Xs:           []float64{f.X},
			Ys:           []float64{f.Y},
			Widths:       []float64{f.Width},
			Heights:      []float64{f.Height},
			Source:       "sam",
			ModelVersion: res.model,
		})
		if err != nil {
			return nil, fmt.Errorf("save box: %w", err)
		}
		ids = append(ids, track.ID)
		box := f
		jobs = append(jobs, river.InsertManyParams{
			Args: annotation.TrackArgs{TrackID: track.ID, FrameID: frame.ID, Box: &box},
		})
	}
	if len(jobs) > 0 {
		if _, err := river.ClientFromContext[pgx.Tx](ctx).InsertManyTx(ctx, tx, jobs); err != nil {
			return nil, fmt.Errorf("queue tracking: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return ids, nil
}

// newInstances drops found boxes that cover an object that already has a box,
// including one found earlier in the list, and keeps at most maxInstances.
func newInstances(found, existing []annotation.Box) []annotation.Box {
	taken := append([]annotation.Box(nil), existing...)
	var out []annotation.Box
	for _, f := range found {
		if len(out) == maxInstances {
			break
		}
		dup := false
		for _, e := range taken {
			if iou(f, e) > sameObjectIoU {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, f)
			taken = append(taken, f)
		}
	}
	return out
}

func iou(a, b annotation.Box) float64 {
	w := math.Min(a.X+a.Width, b.X+b.Width) - math.Max(a.X, b.X)
	h := math.Min(a.Y+a.Height, b.Y+b.Height) - math.Max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	inter := w * h
	return inter / (a.Width*a.Height + b.Width*b.Height - inter)
}
