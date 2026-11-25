package tracker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/taxonomy"
)

// trackingCheck collects the keypoints tracking places on boxes labelers
// posed, across all of a tracking check's runs.
type trackingCheck struct {
	pose taxonomy.Pose
	// runs are each track's boxes posed by hand, tracked from the first.
	runs [][]db.ListHandPosedTrackBoxesRow
	mu   sync.Mutex
	// tracked are the keypoints tracking placed, by the annotation posed on
	// the same frame.
	tracked map[int64][]float64
}

func newTrackingCheck(pose taxonomy.Pose, runs [][]db.ListHandPosedTrackBoxesRow) *trackingCheck {
	return &trackingCheck{pose: pose, runs: runs, tracked: map[int64][]float64{}}
}

func (c *trackingCheck) add(annotationID int64, keypoints []float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tracked[annotationID] = keypoints
}

// trackedRun matches what a tracking check's task reports to the boxes it
// scores.
type trackedRun struct {
	check *trackingCheck
	// posed is the annotation posed by hand on each frame after the first.
	posed map[int64]int64
}

// result compares the tracked keypoints with the labelers' on every posed
// box after each run's first. Boxes tracking placed no keypoints on are ones
// it lost.
func (c *trackingCheck) result() (found int, res taxonomy.CheckResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	derived := map[int]bool{}
	for _, q := range c.pose.Quads {
		if q.Center != nil {
			derived[int(*q.Center)] = true
		}
	}
	var boxes []scoredBox
	var frames, missed, extra int
	for _, run := range c.runs {
		frames += taxonomy.RunFrames(run)
		for _, b := range run[1:] {
			tracked := c.tracked[b.ID]
			if !anyLabeled(tracked) {
				continue
			}
			found++
			for i := range c.pose.Count() {
				if derived[i] {
					continue
				}
				switch hand, placed := labeled(b.Keypoints, i), labeled(tracked, i); {
				case hand && !placed:
					missed++
				case placed && !hand:
					extra++
				}
			}
			boxes = append(boxes, scoredBox{
				box: taxonomy.CheckedBox{
					AnnotationID: b.ID, ClipID: b.ClipID, FrameIdx: b.FrameIdx,
					FramesTracked: b.FrameIdx - run[0].FrameIdx,
				},
				hand:   b.Keypoints,
				placed: tracked,
				w:      float64(b.VideoWidth),
				h:      float64(b.VideoHeight),
			})
		}
	}
	res = score(c.pose, boxes)
	res.Tracks, res.Frames, res.Missed, res.Extra = len(c.runs), frames, missed, extra
	return found, res
}

func anyLabeled(kps []float64) bool {
	for i := range len(kps) / 3 {
		if labeled(kps, i) {
			return true
		}
	}
	return false
}

func (w *OrientWorker) runTrackingCheck(ctx context.Context, q *db.Queries, jobID int64, lt db.LabelType, checkID int64) error {
	pose, err := taxonomy.ParsePose(lt)
	if err != nil {
		return err
	}
	if pose.Count() == 0 {
		return errors.New("the label type no longer has keypoints")
	}
	rows, err := q.ListHandPosedTrackBoxes(ctx, lt.ID)
	if err != nil {
		return fmt.Errorf("list boxes: %w", err)
	}
	runs := taxonomy.SampleCheckRuns(taxonomy.CheckRuns(rows), taxonomy.MaxCheckFrames)
	check := newTrackingCheck(pose, runs)
	var ref string
	if lt.Oriented || pose.HasReference() {
		if ref, err = referenceURL(ctx, w.store, lt); err != nil {
			return err
		}
	}
	scored := 0
	for _, boxes := range runs {
		if err := w.trackRun(ctx, q, jobID, lt, pose, ref, boxes, check); err != nil {
			return err
		}
		scored += len(boxes) - 1
	}
	found, res := check.result()
	raw, err := json.Marshal(res)
	if err != nil {
		return err
	}
	err = q.FinishKeypointCheck(ctx, db.FinishKeypointCheckParams{
		ID:         checkID,
		BoxCount:   int32(scored), //nolint:gosec // capped by MaxCheckFrames
		FoundCount: int32(found),  //nolint:gosec // capped by MaxCheckFrames
		Result:     raw,
	})
	if err != nil {
		return fmt.Errorf("save check: %w", err)
	}
	slog.Info("tracking checked", "label_type_id", lt.ID, "tracks", len(runs), "frames", res.Frames, "boxes", scored, "found", found, "median", res.Median, "p90", res.P90)
	return nil
}

// trackRun tracks a track from its first box posed by hand through its last,
// as tracking from that box would but without the labeler's later keypoints
// to carry on from.
func (w *OrientWorker) trackRun(ctx context.Context, q *db.Queries, jobID int64, lt db.LabelType, pose taxonomy.Pose, ref string, boxes []db.ListHandPosedTrackBoxesRow, check *trackingCheck) error {
	first, last := boxes[0], boxes[len(boxes)-1]
	clip, err := q.GetClip(ctx, first.ClipID)
	if err != nil {
		return fmt.Errorf("load clip: %w", err)
	}
	frames, err := q.ListFramesFrom(ctx, db.ListFramesFromParams{ClipID: first.ClipID, Idx: first.FrameIdx})
	if err != nil {
		return fmt.Errorf("list frames: %w", err)
	}
	n := 0
	for n < len(frames) && frames[n].Idx <= last.FrameIdx {
		n++
	}
	msg := &krillv1.TrackTask{
		Id:            w.dispatcher.nextID(),
		Oriented:      lt.Oriented,
		ReferenceUrl:  ref,
		KeypointCount: int32(pose.Count()), //nolint:gosec // keypoints per type are capped
		Quads:         pose.QuadsProto(),
		Prompt: &krillv1.TrackTask_Box{Box: &krillv1.Box{
			X: first.X, Y: first.Y, Width: first.Width, Height: first.Height,
			Keypoints: annotation.Keypoints(annotation.PadKeypoints(first.Keypoints, pose.Count())),
		}},
	}
	if clip.EmbeddedAt.Valid {
		msg.EmbeddingModel = clip.EmbedModel
	}
	if msg.Frames, err = taskFrames(ctx, w.store, clip, frames[:n]); err != nil {
		return err
	}
	posed := make(map[int64]int64, len(boxes)-1)
	for _, b := range boxes[1:] {
		posed[b.FrameID] = b.ID
	}
	t := newTask(msg.GetId(), 0, msg)
	t.priority = background
	t.jobID = jobID
	t.tracking = &trackedRun{check: check, posed: posed}
	if err := run(ctx, w.dispatcher, t, orientPickupTimeout); err != nil {
		return fmt.Errorf("track %d from frame %d: %w", first.TrackID, first.FrameIdx, err)
	}
	return nil
}
