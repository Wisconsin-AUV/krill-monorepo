package tracker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/taxonomy"
)

const (
	// orientChunk frames go to the worker at a time, so a click waits for at
	// most one chunk.
	orientChunk = 100
	// orientPickupTimeout is long because orient tasks wait behind every
	// click.
	orientPickupTimeout = 10 * time.Minute
)

// orientBoxes is what an orient task's reports are checked against.
type orientBoxes struct {
	labelTypeID int64
	// updatedAt is when each of the task's boxes last changed before the fit.
	updatedAt map[int64]pgtype.Timestamptz
}

type OrientWorker struct {
	river.WorkerDefaults[taxonomy.OrientArgs]
	pool       *pgxpool.Pool
	store      *storage.Store
	dispatcher *Dispatcher
}

func NewOrientWorker(pool *pgxpool.Pool, store *storage.Store, dispatcher *Dispatcher) *OrientWorker {
	return &OrientWorker{pool: pool, store: store, dispatcher: dispatcher}
}

func (w *OrientWorker) Timeout(*river.Job[taxonomy.OrientArgs]) time.Duration { return 6 * time.Hour }

func (w *OrientWorker) Work(ctx context.Context, job *river.Job[taxonomy.OrientArgs]) error {
	q := db.New(w.pool)
	lt, err := q.GetLabelType(ctx, job.Args.LabelTypeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load label type: %w", err)
	}
	if !lt.Oriented {
		return nil
	}
	ref, err := referenceURL(ctx, w.store, lt)
	if err != nil {
		return err
	}
	rows, err := q.ListUnrotatedBoxes(ctx, lt.ID)
	if err != nil {
		return fmt.Errorf("list boxes: %w", err)
	}
	frames := groupByFrame(rows)
	for start := 0; start < len(frames); start += orientChunk {
		chunk := frames[start:min(start+orientChunk, len(frames))]
		if err := w.orient(ctx, job.ID, lt.ID, ref, chunk); err != nil {
			return err
		}
	}
	slog.Info("rotations fit", "label_type_id", lt.ID, "frames", len(frames), "boxes", len(rows))
	return nil
}

// groupByFrame splits rows, which are ordered by frame, into one slice per
// frame.
func groupByFrame(rows []db.ListUnrotatedBoxesRow) [][]db.ListUnrotatedBoxesRow {
	var out [][]db.ListUnrotatedBoxesRow
	for i, r := range rows {
		if i == 0 || r.FrameID != rows[i-1].FrameID {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], r)
	}
	return out
}

func (w *OrientWorker) orient(ctx context.Context, jobID, labelTypeID int64, ref string, frames [][]db.ListUnrotatedBoxesRow) error {
	id := w.dispatcher.nextID()
	msg := &krillv1.OrientTask{Id: id, ReferenceUrl: ref, Frames: make([]*krillv1.OrientFrame, len(frames))}
	boxes := &orientBoxes{labelTypeID: labelTypeID, updatedAt: map[int64]pgtype.Timestamptz{}}
	for i, rows := range frames {
		f := rows[0]
		url, err := w.store.PresignGetInternal(ctx, storage.FrameKey(f.VideoID, f.FrameIdx), frameURLExpiry)
		if err != nil {
			return err
		}
		of := &krillv1.OrientFrame{Frame: &krillv1.TaskFrame{Id: f.FrameID, Url: url}, Boxes: make([]*krillv1.OrientBox, len(rows))}
		for j, r := range rows {
			of.Boxes[j] = &krillv1.OrientBox{AnnotationId: r.ID, Box: &krillv1.Box{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}}
			boxes.updatedAt[r.ID] = r.UpdatedAt
		}
		msg.Frames[i] = of
	}
	t := newBareTask(id, background, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Orient{Orient: msg}})
	t.jobID = jobID
	t.orient = boxes
	if err := run(ctx, w.dispatcher, t, orientPickupTimeout); err != nil {
		first, last := frames[0][0], frames[len(frames)-1][0]
		return fmt.Errorf("fit rotations from frame %d to %d: %w", first.FrameID, last.FrameID, err)
	}
	return nil
}
