package tracker

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// pollTimeout stays under common proxy idle timeouts.
const pollTimeout = 30 * time.Second

var errUnknownTask = connect.NewError(connect.CodeNotFound, errors.New("task is not running"))

type Service struct {
	krillv1connect.UnimplementedWorkerServiceHandler
	q          *db.Queries
	dispatcher *Dispatcher
}

func NewService(pool *pgxpool.Pool, dispatcher *Dispatcher) *Service {
	return &Service{q: db.New(pool), dispatcher: dispatcher}
}

func (s *Service) NextTask(ctx context.Context, _ *krillv1.NextTaskRequest) (*krillv1.NextTaskResponse, error) {
	timer := time.NewTimer(pollTimeout)
	defer timer.Stop()
	select {
	case t := <-s.dispatcher.queue:
		return &krillv1.NextTaskResponse{Track: t.msg}, nil
	case <-timer.C:
		return &krillv1.NextTaskResponse{}, nil
	case <-ctx.Done():
		return nil, connect.NewError(connect.CodeCanceled, ctx.Err())
	}
}

func (s *Service) ReportTrack(ctx context.Context, req *krillv1.ReportTrackRequest) (*krillv1.ReportTrackResponse, error) {
	t, ok := s.dispatcher.get(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	n := len(req.GetBoxes())
	p := db.UpsertProposalsParams{
		TrackID:      t.trackID,
		FrameIds:     make([]int64, 0, n),
		Xs:           make([]float64, 0, n),
		Ys:           make([]float64, 0, n),
		Widths:       make([]float64, 0, n),
		Heights:      make([]float64, 0, n),
		Source:       "sam",
		ModelVersion: req.GetModel(),
	}
	for _, b := range req.GetBoxes() {
		if !t.frames[b.GetFrameId()] {
			return nil, rpc.Invalid("frame %d is not part of the task", b.GetFrameId())
		}
		x, y, w, h, err := annotation.ClampBox(b.GetBox())
		if err != nil {
			return nil, rpc.Invalid("frame %d: %s", b.GetFrameId(), err)
		}
		p.FrameIds = append(p.FrameIds, b.GetFrameId())
		p.Xs = append(p.Xs, x)
		p.Ys = append(p.Ys, y)
		p.Widths = append(p.Widths, w)
		p.Heights = append(p.Heights, h)
	}
	if len(p.FrameIds) > 0 {
		if _, err := s.q.UpsertProposals(ctx, p); err != nil {
			// Usually the track was deleted while tracking ran.
			t.finish(err)
			return nil, rpc.Internal(err, "save boxes")
		}
	}
	if req.GetDone() {
		t.finish(nil)
	} else {
		t.touch()
	}
	return &krillv1.ReportTrackResponse{}, nil
}

func (s *Service) FailTask(_ context.Context, req *krillv1.FailTaskRequest) (*krillv1.FailTaskResponse, error) {
	t, ok := s.dispatcher.get(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	t.finish(errors.New(req.GetError()))
	return &krillv1.FailTaskResponse{}, nil
}
