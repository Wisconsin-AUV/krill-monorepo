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
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/compute"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

// pollTimeout stays under common proxy idle timeouts.
const pollTimeout = 30 * time.Second

var errUnknownTask = connect.NewError(connect.CodeNotFound, errors.New("task is not running"))

type Service struct {
	krillv1connect.UnimplementedWorkerServiceHandler
	pool       *pgxpool.Pool
	q          *db.Queries
	dispatcher *Dispatcher
	nodes      *compute.Registry
	model      func() string
}

func NewService(pool *pgxpool.Pool, dispatcher *Dispatcher, nodes *compute.Registry) *Service {
	return &Service{pool: pool, q: db.New(pool), dispatcher: dispatcher, nodes: nodes}
}

// WithModel tells workers in heartbeat responses to run the SAM model model
// returns.
func (s *Service) WithModel(model func() string) *Service {
	s.model = model
	return s
}

func (s *Service) NextTask(ctx context.Context, _ *krillv1.NextTaskRequest) (*krillv1.NextTaskResponse, error) {
	t, err := s.dispatcher.next(ctx, pollTimeout)
	if err != nil {
		return nil, connect.NewError(connect.CodeCanceled, err)
	}
	if t == nil {
		return &krillv1.NextTaskResponse{}, nil
	}
	return t.msg, nil
}

func (s *Service) ReportTrack(ctx context.Context, req *krillv1.ReportTrackRequest) (*krillv1.ReportTrackResponse, error) {
	t, ok := s.dispatcher.get(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	if t.msg.GetTrack() == nil {
		return nil, rpc.Invalid("task %d is not a track task", t.id)
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
		if err := s.saveProposals(ctx, p); err != nil {
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

func (s *Service) saveProposals(ctx context.Context, p db.UpsertProposalsParams) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if _, err := q.UpsertProposals(ctx, p); err != nil {
		return err
	}
	boxes, err := q.ListTrackBoxes(ctx, p.TrackID)
	if err != nil {
		return err
	}
	if d := staleDrift(boxes); len(d.Ids) > 0 {
		if err := q.SetDrift(ctx, d); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// staleDrift compares each proposal with the track's box on the frame before
// it, whoever drew that one, and returns the proposals whose stored flag is
// out of date.
func staleDrift(boxes []db.ListTrackBoxesRow) db.SetDriftParams {
	var p db.SetDriftParams
	var prev annotation.Rect
	for i, b := range boxes {
		cur := annotation.Rect{X: b.X, Y: b.Y, W: b.Width, H: b.Height}
		flagged := i > 0 && annotation.Drifted(prev, cur)
		prev = cur
		if b.Status != "proposed" {
			continue
		}
		want := "ok"
		if flagged {
			want = "flagged"
		}
		if b.Drift.String == want {
			continue
		}
		p.Ids = append(p.Ids, b.ID)
		if flagged {
			p.FlaggedIds = append(p.FlaggedIds, b.ID)
		}
	}
	return p
}

func (s *Service) ReportExemplars(_ context.Context, req *krillv1.ReportExemplarsRequest) (*krillv1.ReportExemplarsResponse, error) {
	t, ok := s.dispatcher.get(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	if t.msg.GetExemplar() == nil {
		return nil, rpc.Invalid("task %d is not an exemplar task", t.id)
	}
	res := exemplarResult{model: req.GetModel(), boxes: make([]annotation.Box, 0, len(req.GetBoxes()))}
	for i, b := range req.GetBoxes() {
		x, y, w, h, err := annotation.ClampBox(b)
		if err != nil {
			return nil, rpc.Invalid("box %d: %s", i, err)
		}
		res.boxes = append(res.boxes, annotation.Box{X: x, Y: y, Width: w, Height: h})
	}
	select {
	case t.found <- res:
	default:
		return nil, rpc.Invalid("task %d was already reported", t.id)
	}
	t.finish(nil)
	return &krillv1.ReportExemplarsResponse{}, nil
}

func (s *Service) ReportEmbed(_ context.Context, req *krillv1.ReportEmbedRequest) (*krillv1.ReportEmbedResponse, error) {
	t, ok := s.dispatcher.get(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	if t.msg.GetEmbed() == nil {
		return nil, rpc.Invalid("task %d is not an embed task", t.id)
	}
	if req.GetModel() == "" {
		return nil, rpc.Invalid("model is required")
	}
	model := req.GetModel()
	t.model.Store(&model)
	t.finish(nil)
	return &krillv1.ReportEmbedResponse{}, nil
}

func (s *Service) FailTask(_ context.Context, req *krillv1.FailTaskRequest) (*krillv1.FailTaskResponse, error) {
	t, ok := s.dispatcher.get(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	t.finish(errors.New(req.GetError()))
	return &krillv1.FailTaskResponse{}, nil
}

func (s *Service) Heartbeat(ctx context.Context, req *krillv1.HeartbeatRequest) (*krillv1.HeartbeatResponse, error) {
	if req.GetInstanceId() == "" {
		return nil, rpc.Invalid("instance_id is required")
	}
	token, _ := auth.WorkerFrom(ctx)
	cmd := s.nodes.RecordWorker(req.GetInstanceId(), compute.Report{
		Hostname:    req.GetHostname(),
		Version:     req.GetVersion(),
		Model:       req.GetModel(),
		Accelerator: req.GetAccelerator(),
		StartedAt:   req.GetStartedAt().AsTime(),
		Host:        req.GetHost(),
		Gpus:        req.GetGpus(),
		TaskID:      req.GetTaskId(),
		Sam: compute.Sam{
			State:             req.GetModelState(),
			Error:             req.GetModelError(),
			AllocatedBytes:    req.GetModelMemoryAllocatedBytes(),
			ReservedBytes:     req.GetModelMemoryReservedBytes(),
			Pinned:            req.GetPinnedModel() != "",
			SupportsExemplars: req.GetSupportsExemplars(),
			Cached:            req.GetCachedModels(),
		},
		TokenName: token.TokenName,
		CommandID: req.GetCommandId(),
	})
	res := &krillv1.HeartbeatResponse{Command: cmd}
	if s.model != nil {
		res.Model = s.model()
	}
	// The task's job was cancelled, timed out, or ran before an API restart,
	// so nothing will accept its results.
	if id := req.GetTaskId(); id != 0 {
		if _, ok := s.dispatcher.get(id); !ok {
			res.CancelTaskId = id
		}
	}
	return res, nil
}
