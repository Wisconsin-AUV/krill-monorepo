package pipeline

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/tracker"
)

var errUnknownTask = connect.NewError(connect.CodeNotFound, errors.New("task is not running"))

// WorkerService takes reports from GPU workers.
type WorkerService struct {
	krillv1connect.UnimplementedPipelineWorkerServiceHandler
	p *Pipeline
}

func NewWorkerService(p *Pipeline) *WorkerService {
	return &WorkerService{p: p}
}

func (s *WorkerService) RegisterPrelabelers(ctx context.Context, req *krillv1.RegisterPrelabelersRequest) (*krillv1.RegisterPrelabelersResponse, error) {
	for _, spec := range req.GetPrelabelers() {
		if err := checkSpec(spec); err != nil {
			return nil, rpc.Invalid("%s", err)
		}
	}
	for _, spec := range req.GetPrelabelers() {
		raw, err := protojson.Marshal(spec)
		if err != nil {
			return nil, rpc.Internal(err, "encode spec")
		}
		if err := s.p.q.UpsertPrelabelerSpec(ctx, db.UpsertPrelabelerSpecParams{Name: spec.GetName(), Spec: raw}); err != nil {
			return nil, rpc.Internal(err, "save pre-labeler")
		}
	}
	return &krillv1.RegisterPrelabelersResponse{}, nil
}

// trainTask finds a running train task and what it trains.
func (s *WorkerService) trainTask(id int64) (*tracker.Task, trainState, error) {
	t, ok := s.p.Dispatcher.Lookup(id)
	if !ok {
		return nil, trainState{}, errUnknownTask
	}
	st, ok := s.p.trains.Load(id)
	if !ok || t.Msg().GetTrain() == nil {
		return nil, trainState{}, rpc.Invalid("task %d is not a train task", id)
	}
	return t, st.(trainState), nil
}

func (s *WorkerService) CheckIn(_ context.Context, req *krillv1.CheckInRequest) (*krillv1.CheckInResponse, error) {
	t, _, err := s.trainTask(req.GetTaskId())
	if err != nil {
		return nil, err
	}
	t.Touch()
	return &krillv1.CheckInResponse{Task: s.p.Dispatcher.TakeWaiting()}, nil
}

// tail keeps the end of a log, cut at a line break.
func tail(log string) string {
	if len(log) <= maxLog {
		return log
	}
	log = log[len(log)-maxLog:]
	if i := strings.IndexByte(log, '\n'); i >= 0 {
		log = log[i+1:]
	}
	return log
}

func (s *WorkerService) ReportTrainProgress(ctx context.Context, req *krillv1.ReportTrainProgressRequest) (*krillv1.ReportTrainProgressResponse, error) {
	t, st, err := s.trainTask(req.GetTaskId())
	if err != nil {
		return nil, err
	}
	p := db.SetTrainingProgressParams{
		ID:     st.runID,
		Status: "training",
		Log:    tail(req.GetLog()),
		Device: req.GetDevice(),
	}
	if req.GetEvaluating() {
		p.Status = "evaluating"
	}
	if e := req.GetEpoch(); e != nil {
		if p.Metrics, err = protojson.Marshal(e); err != nil {
			return nil, rpc.Internal(err, "encode metrics")
		}
		p.Epoch = e.GetEpoch()
	}
	if err := s.p.q.SetTrainingProgress(ctx, p); err != nil {
		return nil, rpc.Internal(err, "save progress")
	}
	t.Touch()
	return &krillv1.ReportTrainProgressResponse{}, nil
}

func marshalMetrics(m *krillv1.EvalMetrics) ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return protojson.Marshal(m)
}

func (s *WorkerService) FinishTrain(ctx context.Context, req *krillv1.FinishTrainRequest) (*krillv1.FinishTrainResponse, error) {
	t, st, err := s.trainTask(req.GetTaskId())
	if err != nil {
		return nil, err
	}
	if req.GetVal() == nil {
		return nil, rpc.Invalid("val metrics are required")
	}
	p := db.FinishTrainingRunParams{ID: st.runID, WeightsBytes: req.GetWeightsBytes(), Log: tail(req.GetLog())}
	for _, m := range []struct {
		src *krillv1.EvalMetrics
		dst *[]byte
	}{
		{req.GetVal(), &p.ValMetrics},
		{req.GetTest(), &p.TestMetrics},
		{req.GetBaselineVal(), &p.BaselineValMetrics},
		{req.GetBaselineTest(), &p.BaselineTestMetrics},
	} {
		if *m.dst, err = marshalMetrics(m.src); err != nil {
			return nil, rpc.Internal(err, "encode metrics")
		}
	}
	if st.baseline != 0 && (req.GetBaselineVal() != nil || req.GetBaselineTest() != nil) {
		p.BaselineRunID = pgtype.Int8{Int64: st.baseline, Valid: true}
	}
	if _, err := s.p.q.FinishTrainingRun(ctx, p); err != nil {
		t.Finish(err)
		return nil, rpc.Internal(err, "save run")
	}
	t.Finish(nil)
	return &krillv1.FinishTrainResponse{}, nil
}

func (s *WorkerService) ReportPrelabels(ctx context.Context, req *krillv1.ReportPrelabelsRequest) (*krillv1.ReportPrelabelsResponse, error) {
	t, ok := s.p.Dispatcher.Lookup(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	v, ok := s.p.prelabels.Load(req.GetTaskId())
	if !ok || t.Msg().GetPrelabel() == nil {
		return nil, rpc.Invalid("task %d is not a prelabel task", req.GetTaskId())
	}
	col := v.(*collector)
	for _, b := range req.GetBoxes() {
		if !col.frames[b.GetFrameId()] {
			return nil, rpc.Invalid("frame %d is not part of the task", b.GetFrameId())
		}
		if _, _, _, _, err := annotation.ClampBox(b.GetBox()); err != nil {
			return nil, rpc.Invalid("frame %d: %s", b.GetFrameId(), err)
		}
	}
	if err := col.add(ctx, req.GetBoxes(), req.GetFramesDone()); err != nil {
		return nil, rpc.Internal(err, "save boxes")
	}
	if req.GetDone() {
		t.Finish(nil)
	} else {
		t.Touch()
	}
	return &krillv1.ReportPrelabelsResponse{}, nil
}

type previewResult struct {
	boxes  []*krillv1.PreviewBox
	tookMs int32
}

func (s *WorkerService) ReportPreview(_ context.Context, req *krillv1.ReportPreviewRequest) (*krillv1.ReportPreviewResponse, error) {
	t, ok := s.p.Dispatcher.Lookup(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	v, ok := s.p.previews.Load(req.GetTaskId())
	if !ok || t.Msg().GetPreview() == nil {
		return nil, rpc.Invalid("task %d is not a preview task", req.GetTaskId())
	}
	boxes := make([]*krillv1.PreviewBox, 0, len(req.GetBoxes()))
	for _, b := range req.GetBoxes() {
		x, y, w, h, err := annotation.ClampBox(b.GetBox())
		if err != nil {
			continue
		}
		b = proto.CloneOf(b)
		b.Box = &krillv1.Box{X: x, Y: y, Width: w, Height: h}
		boxes = append(boxes, b)
	}
	select {
	case v.(chan previewResult) <- previewResult{boxes: boxes, tookMs: req.GetTookMs()}:
	default:
		return nil, rpc.Invalid("task %d was already reported", req.GetTaskId())
	}
	t.Finish(nil)
	return &krillv1.ReportPreviewResponse{}, nil
}
