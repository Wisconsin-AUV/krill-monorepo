package taxonomy

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// MaxCheckBoxes and MaxCheckFrames keep a keypoint check to a few minutes
// of GPU time.
const (
	MaxCheckBoxes  = 500
	MaxCheckFrames = 3000
)

// Kinds of keypoint check, as stored.
const (
	CheckFit      = "fit"
	CheckTracking = "tracking"
)

// errCheckStopped is shown for a check whose job is gone while the check
// still says it is running.
const errCheckStopped = "stopped before it finished"

// CheckResult is a keypoint check's result column. Errors are in video
// pixels.
type CheckResult struct {
	// Compared is the found boxes with a keypoint labeled in both.
	Compared  int             `json:"compared"`
	Median    float64         `json:"median"`
	P90       float64         `json:"p90"`
	Keypoints []KeypointError `json:"keypoints"`
	Worst     []CheckedBox    `json:"worst"`
	// Tracking checks only.
	Tracks int `json:"tracks,omitempty"`
	Frames int `json:"frames,omitempty"`
	Missed int `json:"missed,omitempty"`
	Extra  int `json:"extra,omitempty"`
}

type KeypointError struct {
	Name   string  `json:"name"`
	Count  int     `json:"count"`
	Median float64 `json:"median"`
	P90    float64 `json:"p90"`
}

type CheckedBox struct {
	AnnotationID int64   `json:"annotation_id"`
	ClipID       int64   `json:"clip_id"`
	FrameIdx     int32   `json:"frame_idx"`
	Error        float64 `json:"error"`
	Turned       bool    `json:"turned"`
	// FramesTracked is from the track's first box posed by hand, in tracking
	// checks.
	FramesTracked int32 `json:"frames_tracked,omitempty"`
}

var checkKindToProto = map[string]krillv1.KeypointCheckKind{
	CheckFit:      krillv1.KeypointCheckKind_KEYPOINT_CHECK_KIND_FIT,
	CheckTracking: krillv1.KeypointCheckKind_KEYPOINT_CHECK_KIND_TRACKING,
}

var checkStatusToProto = map[string]krillv1.KeypointCheckStatus{
	"running":   krillv1.KeypointCheckStatus_KEYPOINT_CHECK_STATUS_RUNNING,
	"succeeded": krillv1.KeypointCheckStatus_KEYPOINT_CHECK_STATUS_SUCCEEDED,
	"failed":    krillv1.KeypointCheckStatus_KEYPOINT_CHECK_STATUS_FAILED,
}

func (s *Service) CheckKeypoints(ctx context.Context, req *krillv1.CheckKeypointsRequest) (*krillv1.CheckKeypointsResponse, error) {
	lt, err := s.q.GetLabelType(ctx, req.GetLabelTypeId())
	if err != nil {
		return nil, rpc.DBError(err, "label type")
	}
	kind := CheckFit
	var n int32
	if req.GetKind() == krillv1.KeypointCheckKind_KEYPOINT_CHECK_KIND_TRACKING {
		kind = CheckTracking
		pose, err := ParsePose(lt)
		if err != nil {
			return nil, rpc.Internal(err, "decode label type")
		}
		if pose.Count() == 0 {
			return nil, rpc.Invalid("label type has no keypoints")
		}
		rows, err := s.q.ListHandPosedTrackBoxes(ctx, lt.ID)
		if err != nil {
			return nil, rpc.Internal(err, "list boxes")
		}
		for _, r := range SampleCheckRuns(CheckRuns(rows), MaxCheckFrames) {
			n += int32(len(r) - 1) //nolint:gosec // capped by MaxCheckFrames
		}
	} else {
		if err := fittable(lt); err != nil {
			return nil, err
		}
		if n, err = s.q.CountHandPosedBoxes(ctx, lt.ID); err != nil {
			return nil, rpc.Internal(err, "count boxes")
		}
		n = min(n, MaxCheckBoxes)
	}
	if n == 0 {
		return &krillv1.CheckKeypointsResponse{}, nil
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	running, err := q.LockRunningKeypointCheck(ctx, db.LockRunningKeypointCheckParams{LabelTypeID: lt.ID, Kind: kind})
	switch {
	case err == nil:
		alive, err := s.jobAlive(ctx, running.JobID)
		if err != nil {
			return nil, rpc.Internal(err, "load check job")
		}
		if alive {
			return &krillv1.CheckKeypointsResponse{BoxCount: running.BoxCount}, nil
		}
		if err := q.FailKeypointCheck(ctx, db.FailKeypointCheckParams{ID: running.ID, Error: errCheckStopped}); err != nil {
			return nil, rpc.Internal(err, "end stopped check")
		}
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, rpc.Internal(err, "load running check")
	}
	args := OrientArgs{LabelTypeID: lt.ID, Keypoints: true, Check: true, Tracking: kind == CheckTracking}
	res, err := s.jobs.InsertTx(ctx, tx, args, nil)
	if err != nil {
		return nil, rpc.Internal(err, "queue check")
	}
	sess, signedIn := auth.SessionFrom(ctx)
	_, err = q.CreateKeypointCheck(ctx, db.CreateKeypointCheckParams{
		LabelTypeID: lt.ID,
		Kind:        kind,
		JobID:       res.Job.ID,
		BoxCount:    n,
		CreatedBy:   pgtype.UUID{Bytes: sess.User.ID, Valid: signedIn},
	})
	if err != nil {
		return nil, rpc.Internal(err, "record check")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.CheckKeypointsResponse{BoxCount: n}, nil
}

func (s *Service) ListKeypointChecks(ctx context.Context, _ *krillv1.ListKeypointChecksRequest) (*krillv1.ListKeypointChecksResponse, error) {
	types, err := s.q.ListKeypointCheckTypes(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list label types")
	}
	checks, err := s.q.ListLatestKeypointChecks(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list checks")
	}
	out := &krillv1.ListKeypointChecksResponse{MaxBoxes: MaxCheckBoxes, MaxTrackedFrames: MaxCheckFrames}
	var boxes []*krillv1.KeypointCheckBox
	byType := map[int64]*krillv1.KeypointCheckType{}
	for _, r := range types {
		pose, err := ParsePose(r.LabelType)
		if err != nil {
			return nil, rpc.Internal(err, "decode label type")
		}
		t := &krillv1.KeypointCheckType{
			LabelTypeId:     r.LabelType.ID,
			Name:            r.LabelType.Name,
			Title:           r.LabelType.Title,
			Color:           r.LabelType.Color,
			PosedBoxCount:   r.PosedBoxCount,
			PosedTrackCount: r.PosedTrackCount,
			Fittable:        pose.HasReference() && r.LabelType.ReferenceKey.Valid,
		}
		byType[t.GetLabelTypeId()] = t
		out.Types = append(out.Types, t)
	}
	// Rows come finished first, so a stopped check replaces the one before
	// it.
	for _, r := range checks {
		t, ok := byType[r.KeypointCheck.LabelTypeID]
		if !ok {
			continue
		}
		c, err := checkProto(r)
		if err != nil {
			return nil, rpc.Internal(err, "decode check")
		}
		boxes = append(boxes, c.GetWorst()...)
		last, running := &t.Last, &t.Running
		if r.KeypointCheck.Kind == CheckTracking {
			last, running = &t.LastTracking, &t.RunningTracking
		}
		if c.GetStatus() != krillv1.KeypointCheckStatus_KEYPOINT_CHECK_STATUS_RUNNING {
			*last = c
			continue
		}
		alive, err := s.jobAlive(ctx, r.KeypointCheck.JobID)
		if err != nil {
			return nil, rpc.Internal(err, "load check job")
		}
		if alive {
			*running = c
			continue
		}
		c.Status = krillv1.KeypointCheckStatus_KEYPOINT_CHECK_STATUS_FAILED
		c.Error = errCheckStopped
		*last = c
	}
	if err := s.nameClips(ctx, boxes); err != nil {
		return nil, rpc.Internal(err, "load clips")
	}
	return out, nil
}

func (s *Service) nameClips(ctx context.Context, boxes []*krillv1.KeypointCheckBox) error {
	if len(boxes) == 0 {
		return nil
	}
	ids := make([]int64, len(boxes))
	for i, b := range boxes {
		ids[i] = b.GetClipId()
	}
	clips, err := s.q.ListJobClips(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]db.ListJobClipsRow, len(clips))
	for _, c := range clips {
		byID[c.ID] = c
	}
	for _, b := range boxes {
		if c, ok := byID[b.GetClipId()]; ok {
			b.VideoName, b.ClipIndex = c.VideoName, c.Idx
		}
	}
	return nil
}

// jobAlive reports whether a job is waiting or running. River deletes
// finished jobs after a while.
func (s *Service) jobAlive(ctx context.Context, id int64) (bool, error) {
	job, err := s.jobs.JobGet(ctx, id)
	if errors.Is(err, river.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	switch job.State {
	case rivertype.JobStateCompleted, rivertype.JobStateCancelled, rivertype.JobStateDiscarded:
		return false, nil
	}
	return true, nil
}

func checkProto(r db.ListLatestKeypointChecksRow) (*krillv1.KeypointCheck, error) {
	k := r.KeypointCheck
	c := &krillv1.KeypointCheck{
		Id:          k.ID,
		Status:      checkStatusToProto[k.Status],
		Kind:        checkKindToProto[k.Kind],
		Error:       k.Error,
		CreatedAt:   timestamppb.New(k.CreatedAt.Time),
		BoxCount:    k.BoxCount,
		FoundCount:  k.FoundCount,
		TurnedCount: k.TurnedCount,
	}
	if r.CreatorID.Valid {
		c.CreatedBy = auth.PublicProfile(r.CreatorID.Bytes, r.CreatorName.String, r.CreatorUsername.String, r.CreatorRole.String, r.CreatorCreatedAt)
	}
	if k.FinishedAt.Valid {
		c.FinishedAt = timestamppb.New(k.FinishedAt.Time)
	}
	if k.Result == nil {
		return c, nil
	}
	var res CheckResult
	if err := json.Unmarshal(k.Result, &res); err != nil {
		return nil, err
	}
	c.ComparedCount = int32(res.Compared) //nolint:gosec // at most MaxCheckBoxes
	c.MedianError = res.Median
	c.P90Error = res.P90
	c.TrackCount = int32(res.Tracks)  //nolint:gosec // capped by MaxCheckFrames
	c.FrameCount = int32(res.Frames)  //nolint:gosec // capped by MaxCheckFrames
	c.MissedCount = int32(res.Missed) //nolint:gosec // capped by MaxCheckFrames
	c.ExtraCount = int32(res.Extra)   //nolint:gosec // capped by MaxCheckFrames
	for _, e := range res.Keypoints {
		c.Keypoints = append(c.Keypoints, &krillv1.KeypointError{
			Name: e.Name, Count: int32(e.Count), Median: e.Median, P90: e.P90, //nolint:gosec // at most MaxCheckBoxes
		})
	}
	for _, b := range res.Worst {
		c.Worst = append(c.Worst, &krillv1.KeypointCheckBox{
			AnnotationId: b.AnnotationID, ClipId: b.ClipID, FrameIndex: b.FrameIdx, Error: b.Error, Turned: b.Turned,
			FramesTracked: b.FramesTracked,
		})
	}
	return c, nil
}

// CheckRuns splits ListHandPosedTrackBoxes's rows by track, keeping tracks
// with two or more boxes. A tracking check tracks each from its first box
// and scores the rest.
func CheckRuns(rows []db.ListHandPosedTrackBoxesRow) [][]db.ListHandPosedTrackBoxesRow {
	var out [][]db.ListHandPosedTrackBoxesRow
	for i := 0; i < len(rows); {
		j := i + 1
		for j < len(rows) && rows[j].TrackID == rows[i].TrackID {
			j++
		}
		if j-i >= 2 {
			out = append(out, rows[i:j])
		}
		i = j
	}
	return out
}

// RunFrames is how many frames tracking a run takes, its first box's
// through its last's.
func RunFrames(run []db.ListHandPosedTrackBoxesRow) int {
	return int(run[len(run)-1].FrameIdx-run[0].FrameIdx) + 1
}

// SampleCheckRuns picks as many runs as fit in maxFrames, spread evenly over
// them and keeping their order.
func SampleCheckRuns(runs [][]db.ListHandPosedTrackBoxesRow, maxFrames int) [][]db.ListHandPosedTrackBoxesRow {
	for n := len(runs); n > 0; n-- {
		picked := make([][]db.ListHandPosedTrackBoxesRow, n)
		frames := 0
		for i := range picked {
			picked[i] = runs[i*len(runs)/n]
			frames += RunFrames(picked[i])
		}
		if frames <= maxFrames {
			return picked
		}
	}
	return nil
}
