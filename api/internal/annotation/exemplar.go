package annotation

import (
	"context"
	"encoding/json"
	"errors"

	"connectrpc.com/connect"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// ExemplarArgs finds objects like Box on FrameID and starts a track of the
// same type as TrackID for each.
type ExemplarArgs struct {
	TrackID int64 `json:"track_id"`
	FrameID int64 `json:"frame_id"`
	Box     Box   `json:"box"`
}

func (ExemplarArgs) Kind() string { return "exemplar" }

func (ExemplarArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: TrackQueue, MaxAttempts: 1}
}

// ExemplarOutput is recorded on an exemplar job when it finishes.
type ExemplarOutput struct {
	TrackIDs []int64 `json:"track_ids"`
}

var errUnknownSearch = connect.NewError(connect.CodeNotFound, errors.New("search not found"))

func (s *Service) SearchExemplar(ctx context.Context, req *krillv1.SearchExemplarRequest) (*krillv1.SearchExemplarResponse, error) {
	clipID, err := s.sameClip(ctx, req.GetTrackId(), req.GetFrameId())
	if err != nil {
		return nil, err
	}
	b, err := s.q.GetTrackBox(ctx, db.GetTrackBoxParams{TrackID: req.GetTrackId(), FrameID: req.GetFrameId()})
	if err != nil {
		return nil, rpc.DBError(err, "box")
	}
	args := ExemplarArgs{
		TrackID: req.GetTrackId(),
		FrameID: req.GetFrameId(),
		Box:     Box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height},
	}
	res, err := s.jobs.Insert(ctx, args, nil)
	if err != nil {
		return nil, rpc.Internal(err, "queue search")
	}
	s.claim(ctx, clipID)
	return &krillv1.SearchExemplarResponse{SearchId: res.Job.ID}, nil
}

func (s *Service) GetExemplarSearch(ctx context.Context, req *krillv1.GetExemplarSearchRequest) (*krillv1.GetExemplarSearchResponse, error) {
	job, err := s.jobs.JobGet(ctx, req.GetId())
	if errors.Is(err, river.ErrNotFound) {
		return nil, errUnknownSearch
	}
	if err != nil {
		return nil, rpc.Internal(err, "load search")
	}
	return exemplarSearch(job)
}

// exemplarSearch reports on an exemplar job. Other kinds of job are hidden
// because their errors can carry details labelers should not see.
func exemplarSearch(job *rivertype.JobRow) (*krillv1.GetExemplarSearchResponse, error) {
	if job.Kind != (ExemplarArgs{}).Kind() {
		return nil, errUnknownSearch
	}
	switch job.State {
	case rivertype.JobStateCompleted:
		var out ExemplarOutput
		if raw := job.Output(); raw != nil {
			if err := json.Unmarshal(raw, &out); err != nil {
				return nil, rpc.Internal(err, "decode search")
			}
		}
		return &krillv1.GetExemplarSearchResponse{Done: true, TrackIds: out.TrackIDs}, nil
	case rivertype.JobStateDiscarded, rivertype.JobStateCancelled:
		msg := "search was cancelled"
		if n := len(job.Errors); n > 0 {
			msg = job.Errors[n-1].Error
		}
		return &krillv1.GetExemplarSearchResponse{Done: true, Error: msg}, nil
	default:
		return &krillv1.GetExemplarSearchResponse{}, nil
	}
}
