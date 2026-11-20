package tracker

import (
	"context"
	"encoding/json"
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
	// pose is set when the task fits keypoints rather than corners.
	pose *taxonomy.Pose
	// updatedAt is when each of the task's boxes last changed before the fit.
	updatedAt map[int64]pgtype.Timestamptz
	// check is set when the task scores keypoints rather than saving them.
	check *keypointCheck
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
	if job.Args.Check {
		return w.check(ctx, job.ID, lt)
	}
	var pose *taxonomy.Pose
	var rows []db.ListUnrotatedBoxesRow
	if job.Args.Keypoints {
		p, err := taxonomy.ParsePose(lt)
		if err != nil {
			return err
		}
		if !p.HasReference() || !lt.ReferenceKey.Valid {
			return nil
		}
		pose = &p
		unposed, err := q.ListUnposedBoxes(ctx, lt.ID)
		if err != nil {
			return fmt.Errorf("list boxes: %w", err)
		}
		rows = make([]db.ListUnrotatedBoxesRow, len(unposed))
		for i, r := range unposed {
			rows[i] = db.ListUnrotatedBoxesRow(r)
		}
	} else {
		if !lt.Oriented {
			return nil
		}
		if rows, err = q.ListUnrotatedBoxes(ctx, lt.ID); err != nil {
			return fmt.Errorf("list boxes: %w", err)
		}
	}
	frames, err := w.fit(ctx, job.ID, lt, pose, rows, nil)
	if err != nil {
		return err
	}
	if pose != nil {
		slog.Info("keypoints fit", "label_type_id", lt.ID, "frames", frames, "boxes", len(rows))
	} else {
		slog.Info("rotations fit", "label_type_id", lt.ID, "frames", frames, "boxes", len(rows))
	}
	return nil
}

// check fits keypoints to boxes labelers posed and records how far they land
// in the check the job belongs to. The check fails with the job.
func (w *OrientWorker) check(ctx context.Context, jobID int64, lt db.LabelType) error {
	q := db.New(w.pool)
	c, err := q.GetRunningKeypointCheckByJob(ctx, jobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load check: %w", err)
	}
	err = w.runCheck(ctx, q, jobID, lt, c.ID)
	if err != nil {
		if ferr := q.FailKeypointCheck(context.WithoutCancel(ctx), db.FailKeypointCheckParams{ID: c.ID, Error: err.Error()}); ferr != nil {
			slog.ErrorContext(ctx, "fail keypoint check", "id", c.ID, "err", ferr)
		}
	}
	return err
}

func (w *OrientWorker) runCheck(ctx context.Context, q *db.Queries, jobID int64, lt db.LabelType, checkID int64) error {
	pose, err := taxonomy.ParsePose(lt)
	if err != nil {
		return err
	}
	if !pose.HasReference() || !lt.ReferenceKey.Valid {
		return errors.New("the label type no longer has a quad covered by a reference design")
	}
	posed, err := q.ListHandPosedBoxes(ctx, lt.ID)
	if err != nil {
		return fmt.Errorf("list boxes: %w", err)
	}
	posed = sampleEvenly(posed, taxonomy.MaxCheckBoxes)
	check := newKeypointCheck(pose, posed)
	rows := make([]db.ListUnrotatedBoxesRow, len(posed))
	for i, r := range posed {
		rows[i] = db.ListUnrotatedBoxesRow{
			ID: r.ID, FrameID: r.FrameID, FrameIdx: r.FrameIdx, VideoID: r.VideoID,
			X: r.X, Y: r.Y, Width: r.Width, Height: r.Height, UpdatedAt: r.UpdatedAt,
		}
	}
	if _, err := w.fit(ctx, jobID, lt, &pose, rows, check); err != nil {
		return err
	}
	found, turned, res := check.result()
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	err = q.FinishKeypointCheck(ctx, db.FinishKeypointCheckParams{
		ID:          checkID,
		BoxCount:    int32(len(posed)), //nolint:gosec // at most MaxCheckBoxes
		FoundCount:  int32(found),      //nolint:gosec // at most MaxCheckBoxes
		TurnedCount: int32(turned),     //nolint:gosec // at most MaxCheckBoxes
		Result:      raw,
	})
	if err != nil {
		return fmt.Errorf("save check: %w", err)
	}
	slog.Info("keypoints checked", "label_type_id", lt.ID, "boxes", len(posed), "found", found, "turned", turned, "median", res.Median, "p90", res.P90)
	return nil
}

// fit sends rows to the worker a chunk of frames at a time and returns how
// many frames they're on.
func (w *OrientWorker) fit(ctx context.Context, jobID int64, lt db.LabelType, pose *taxonomy.Pose, rows []db.ListUnrotatedBoxesRow, check *keypointCheck) (int, error) {
	ref, err := referenceURL(ctx, w.store, lt)
	if err != nil {
		return 0, err
	}
	frames := groupByFrame(rows)
	for start := 0; start < len(frames); start += orientChunk {
		chunk := frames[start:min(start+orientChunk, len(frames))]
		if err := w.orient(ctx, jobID, lt.ID, ref, pose, chunk, check); err != nil {
			return 0, err
		}
	}
	return len(frames), nil
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

func (w *OrientWorker) orient(ctx context.Context, jobID, labelTypeID int64, ref string, pose *taxonomy.Pose, frames [][]db.ListUnrotatedBoxesRow, check *keypointCheck) error {
	id := w.dispatcher.nextID()
	msg := &krillv1.OrientTask{Id: id, ReferenceUrl: ref, Frames: make([]*krillv1.OrientFrame, len(frames))}
	if pose != nil {
		msg.KeypointCount = int32(pose.Count()) //nolint:gosec // keypoints per type are capped
		msg.Quads = pose.QuadsProto()
	}
	boxes := &orientBoxes{labelTypeID: labelTypeID, pose: pose, updatedAt: map[int64]pgtype.Timestamptz{}, check: check}
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
		return fmt.Errorf("fit frames %d to %d: %w", first.FrameID, last.FrameID, err)
	}
	return nil
}
