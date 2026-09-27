package annotation

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// TrackQueue holds jobs for the GPU worker. It runs one at a time because the
// worker holds one model on one card.
const TrackQueue = "gpu"

// Point and Box are normalized to the frame.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type Box struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// TrackArgs tracks a track's object from FrameID to the end of its clip.
// Exactly one of Point and Box is set.
type TrackArgs struct {
	TrackID int64  `json:"track_id"`
	FrameID int64  `json:"frame_id"`
	Point   *Point `json:"point,omitempty"`
	Box     *Box   `json:"box,omitempty"`
}

func (TrackArgs) Kind() string { return "track" }

func (TrackArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: TrackQueue, MaxAttempts: 1}
}

// Tracking returns which of the tracks have a track job that has not finished.
func Tracking(ctx context.Context, jobs *river.Client[pgx.Tx], trackIDs []int64) ([]int64, error) {
	want := make(map[int64]bool, len(trackIDs))
	for _, id := range trackIDs {
		want[id] = true
	}
	params := river.NewJobListParams().
		Kinds(TrackArgs{}.Kind()).
		States(rivertype.JobStateAvailable, rivertype.JobStateRunning, rivertype.JobStateRetryable, rivertype.JobStateScheduled, rivertype.JobStatePending).
		First(1000)
	res, err := jobs.JobList(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list track jobs: %w", err)
	}
	var out []int64
	for _, job := range res.Jobs {
		var args TrackArgs
		if err := json.Unmarshal(job.EncodedArgs, &args); err != nil {
			return nil, fmt.Errorf("decode job %d: %w", job.ID, err)
		}
		if want[args.TrackID] {
			out = append(out, args.TrackID)
			delete(want, args.TrackID)
		}
	}
	return out, nil
}

func (s *Service) TrackObject(ctx context.Context, req *krillv1.TrackObjectRequest) (*krillv1.TrackObjectResponse, error) {
	args := TrackArgs{FrameID: req.GetFrameId()}
	switch p := req.GetPrompt().(type) {
	case *krillv1.TrackObjectRequest_Point:
		x, y := p.Point.GetX(), p.Point.GetY()
		if !(x >= 0 && x <= 1 && y >= 0 && y <= 1) {
			return nil, rpc.Invalid("point is outside the frame")
		}
		args.Point = &Point{X: x, Y: y}
	case *krillv1.TrackObjectRequest_Box:
		x, y, w, h, err := ClampBox(p.Box)
		if err != nil {
			return nil, rpc.Invalid("%s", err)
		}
		args.Box = &Box{X: x, Y: y, Width: w, Height: h}
	default:
		return nil, rpc.Invalid("a point or box is required")
	}

	frame, err := s.q.GetFrame(ctx, req.GetFrameId())
	if err != nil {
		return nil, rpc.DBError(err, "frame")
	}
	var attrs []byte
	if req.GetTrackId() == 0 {
		if attrs, err = s.newTrackAttributes(ctx, req.GetLabelTypeId(), req.GetAttributes()); err != nil {
			return nil, err
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	var track db.Track
	if req.GetTrackId() == 0 {
		track, err = q.CreateTrack(ctx, db.CreateTrackParams{ClipID: frame.ClipID, LabelTypeID: req.GetLabelTypeId(), Attributes: attrs})
		if err != nil {
			return nil, rpc.Internal(err, "create track")
		}
	} else {
		if track, err = q.GetTrack(ctx, req.GetTrackId()); err != nil {
			return nil, rpc.DBError(err, "track")
		}
		if track.ClipID != frame.ClipID {
			return nil, errWrongClip
		}
	}
	if b := args.Box; b != nil {
		_, err := q.UpsertAnnotation(ctx, db.UpsertAnnotationParams{
			TrackID: track.ID, FrameID: frame.ID, X: b.X, Y: b.Y, Width: b.Width, Height: b.Height, UserID: auth.CallerID(ctx),
		})
		if err != nil {
			return nil, rpc.Internal(err, "save box")
		}
		if err := q.ClearEmptyFrame(ctx, frame.ID); err != nil {
			return nil, rpc.Internal(err, "update frame status")
		}
	}
	args.TrackID = track.ID
	if _, err := s.jobs.InsertTx(ctx, tx, args, nil); err != nil {
		return nil, rpc.Internal(err, "queue tracking")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	s.claim(ctx, frame.ClipID)

	out, err := Track(track)
	if err != nil {
		return nil, rpc.Internal(err, "decode track")
	}
	return &krillv1.TrackObjectResponse{Track: out}, nil
}
