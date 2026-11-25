package tracker

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/compute"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/logs"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/taxonomy"
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
	logs       *logs.Hub
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

// WithLogs keeps the log records workers send with heartbeats in hub.
func (s *Service) WithLogs(hub *logs.Hub) *Service {
	s.logs = hub
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
	if t.tracking != nil {
		return reportCheckedTrack(t, req)
	}
	msg := t.msg.GetTrack()
	pose := taxonomy.Pose{Keypoints: make([]taxonomy.Keypoint, msg.GetKeypointCount()), Quads: taxonomy.QuadsFromProto(msg.GetQuads())}
	n := len(req.GetBoxes())
	p := db.UpsertProposalsParams{
		TrackID:      t.trackID,
		FrameIds:     make([]int64, 0, n),
		Xs:           make([]float64, 0, n),
		Ys:           make([]float64, 0, n),
		Widths:       make([]float64, 0, n),
		Heights:      make([]float64, 0, n),
		HasCorners:   make([]bool, 0, n),
		Corners:      make([]float64, 0, 8*n),
		KeypointEnds: make([]int32, 0, n),
		Keypoints:    make([]float64, 0, 3*pose.Count()*n),
		Source:       "sam",
		ModelVersion: req.GetModel(),
	}
	var prompt *promptKeypoints
	for _, b := range req.GetBoxes() {
		if !t.frames[b.GetFrameId()] {
			return nil, rpc.Invalid("frame %d is not part of the task", b.GetFrameId())
		}
		x, y, w, h, corners, err := annotation.ClampCorners(b.GetBox(), msg.GetOriented())
		if err != nil {
			return nil, rpc.Invalid("frame %d: %s", b.GetFrameId(), err)
		}
		kps, err := annotation.ClampKeypoints(b.GetBox().GetKeypoints(), pose)
		if err != nil {
			return nil, rpc.Invalid("frame %d: %s", b.GetFrameId(), err)
		}
		if b.GetFrameId() == msg.GetFrames()[0].GetId() {
			prompt = &promptKeypoints{frameID: b.GetFrameId(), keypoints: kps, refilled: pose.QuadSlots(msg.GetFill())}
		}
		p.FrameIds = append(p.FrameIds, b.GetFrameId())
		p.Xs = append(p.Xs, x)
		p.Ys = append(p.Ys, y)
		p.Widths = append(p.Widths, w)
		p.Heights = append(p.Heights, h)
		p.HasCorners = append(p.HasCorners, corners != nil)
		if corners == nil {
			corners = make([]float64, 8)
		}
		p.Corners = append(p.Corners, corners...)
		p.Keypoints = append(p.Keypoints, kps...)
		p.KeypointEnds = append(p.KeypointEnds, int32(len(p.Keypoints))) //nolint:gosec // a report's keypoints fit in int32
	}
	if len(p.FrameIds) > 0 {
		if err := s.saveProposals(ctx, p, pose, prompt); err != nil {
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

// promptKeypoints are what the worker found on the frame it was prompted on.
type promptKeypoints struct {
	frameID   int64
	keypoints []float64
	// The keypoints of the quad the labeler asked to fill, which replace
	// theirs.
	refilled []int32
}

func (s *Service) saveProposals(ctx context.Context, p db.UpsertProposalsParams, pose taxonomy.Pose, prompt *promptKeypoints) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if _, err := q.UpsertProposals(ctx, p); err != nil {
		return err
	}
	if prompt != nil && prompt.keypoints != nil {
		if err := mergePromptKeypoints(ctx, q, p, pose, prompt); err != nil {
			return err
		}
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

// mergePromptKeypoints adds the keypoints the worker found on the prompt
// frame, such as a quad it filled, to a box a labeler drew or accepted there,
// which UpsertProposals leaves alone. The box keeps its bounds and every
// keypoint already labeled except the filled quad's, and goes back to being
// a proposal when any changes.
func mergePromptKeypoints(ctx context.Context, q *db.Queries, p db.UpsertProposalsParams, pose taxonomy.Pose, prompt *promptKeypoints) error {
	a, err := q.LockTrackAnnotation(ctx, db.LockTrackAnnotationParams{TrackID: p.TrackID, FrameID: prompt.frameID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if a.Status != "verified" {
		return nil
	}
	merged := annotation.MergeKeypoints(a.Keypoints, prompt.keypoints, pose, prompt.refilled)
	if merged == nil {
		return nil
	}
	return q.ProposeKeypoints(ctx, db.ProposeKeypointsParams{ID: a.ID, Keypoints: merged, ModelVersion: p.ModelVersion})
}

// staleDrift compares each proposal with the track's box on the frame before
// it, whoever drew that one, and returns the proposals whose stored flag is
// out of date.
func staleDrift(boxes []db.ListTrackBoxesRow) db.SetDriftParams {
	var p db.SetDriftParams
	var prev annotation.Rect
	var prevKeypoints []float64
	for i, b := range boxes {
		cur := annotation.Rect{X: b.X, Y: b.Y, W: b.Width, H: b.Height}
		flagged := i > 0 && annotation.Drifted(prev, cur)
		if i > 0 && !flagged {
			flagged = annotation.KeypointsDrifted(prevKeypoints, b.Keypoints, prev, float64(b.FrameWidth), float64(b.FrameHeight))
		}
		prev, prevKeypoints = cur, b.Keypoints
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

func (s *Service) ReportOrient(ctx context.Context, req *krillv1.ReportOrientRequest) (*krillv1.ReportOrientResponse, error) {
	t, ok := s.dispatcher.get(req.GetTaskId())
	if !ok {
		return nil, errUnknownTask
	}
	if t.msg.GetOrient() == nil {
		return nil, rpc.Invalid("task %d is not an orient task", t.id)
	}
	if t.orient.check != nil {
		return s.reportCheckedKeypoints(t, req)
	}
	if t.orient.pose != nil {
		return s.reportFittedKeypoints(ctx, t, req)
	}
	n := len(req.GetBoxes())
	p := db.SetFittedCornersParams{
		LabelTypeID: t.orient.labelTypeID,
		Ids:         make([]int64, 0, n),
		UpdatedAts:  make([]pgtype.Timestamptz, 0, n),
		Xs:          make([]float64, 0, n),
		Ys:          make([]float64, 0, n),
		Widths:      make([]float64, 0, n),
		Heights:     make([]float64, 0, n),
		Corners:     make([]float64, 0, 8*n),
	}
	for _, b := range req.GetBoxes() {
		updatedAt, ok := t.orient.updatedAt[b.GetAnnotationId()]
		if !ok {
			return nil, rpc.Invalid("box %d is not part of the task", b.GetAnnotationId())
		}
		x, y, w, h, corners, err := annotation.ClampCorners(b.GetBox(), true)
		if err != nil {
			return nil, rpc.Invalid("box %d: %s", b.GetAnnotationId(), err)
		}
		if corners == nil {
			return nil, rpc.Invalid("box %d has no corners", b.GetAnnotationId())
		}
		p.Ids = append(p.Ids, b.GetAnnotationId())
		p.UpdatedAts = append(p.UpdatedAts, updatedAt)
		p.Xs = append(p.Xs, x)
		p.Ys = append(p.Ys, y)
		p.Widths = append(p.Widths, w)
		p.Heights = append(p.Heights, h)
		p.Corners = append(p.Corners, corners...)
	}
	if len(p.Ids) > 0 {
		if _, err := s.q.SetFittedCorners(ctx, p); err != nil {
			t.finish(err)
			return nil, rpc.Internal(err, "save boxes")
		}
	}
	if req.GetDone() {
		t.finish(nil)
	} else {
		t.touch()
	}
	return &krillv1.ReportOrientResponse{}, nil
}

func (s *Service) reportFittedKeypoints(ctx context.Context, t *task, req *krillv1.ReportOrientRequest) (*krillv1.ReportOrientResponse, error) {
	n := len(req.GetBoxes())
	p := db.SetFittedKeypointsParams{
		LabelTypeID:  t.orient.labelTypeID,
		Ids:          make([]int64, 0, n),
		UpdatedAts:   make([]pgtype.Timestamptz, 0, n),
		KeypointEnds: make([]int32, 0, n),
		Keypoints:    make([]float64, 0, 3*t.orient.pose.Count()*n),
	}
	for _, b := range req.GetBoxes() {
		updatedAt, ok := t.orient.updatedAt[b.GetAnnotationId()]
		if !ok {
			return nil, rpc.Invalid("box %d is not part of the task", b.GetAnnotationId())
		}
		kps, err := annotation.ClampKeypoints(b.GetBox().GetKeypoints(), *t.orient.pose)
		if err != nil {
			return nil, rpc.Invalid("box %d: %s", b.GetAnnotationId(), err)
		}
		if kps == nil {
			continue
		}
		p.Ids = append(p.Ids, b.GetAnnotationId())
		p.UpdatedAts = append(p.UpdatedAts, updatedAt)
		p.Keypoints = append(p.Keypoints, kps...)
		p.KeypointEnds = append(p.KeypointEnds, int32(len(p.Keypoints))) //nolint:gosec // a report's keypoints fit in int32
	}
	if len(p.Ids) > 0 {
		if _, err := s.q.SetFittedKeypoints(ctx, p); err != nil {
			t.finish(err)
			return nil, rpc.Internal(err, "save boxes")
		}
	}
	if req.GetDone() {
		t.finish(nil)
	} else {
		t.touch()
	}
	return &krillv1.ReportOrientResponse{}, nil
}

// reportCheckedKeypoints adds the keypoints fitted for a keypoint check to it
// without saving them.
func (s *Service) reportCheckedKeypoints(t *task, req *krillv1.ReportOrientRequest) (*krillv1.ReportOrientResponse, error) {
	for _, b := range req.GetBoxes() {
		if _, ok := t.orient.updatedAt[b.GetAnnotationId()]; !ok {
			return nil, rpc.Invalid("box %d is not part of the task", b.GetAnnotationId())
		}
		kps, err := annotation.ClampKeypoints(b.GetBox().GetKeypoints(), *t.orient.pose)
		if err != nil {
			return nil, rpc.Invalid("box %d: %s", b.GetAnnotationId(), err)
		}
		if kps != nil {
			t.orient.check.add(b.GetAnnotationId(), kps)
		}
	}
	if req.GetDone() {
		t.finish(nil)
	} else {
		t.touch()
	}
	return &krillv1.ReportOrientResponse{}, nil
}

// reportCheckedTrack adds the keypoints tracked for a tracking check to it
// without saving them.
func reportCheckedTrack(t *task, req *krillv1.ReportTrackRequest) (*krillv1.ReportTrackResponse, error) {
	for _, b := range req.GetBoxes() {
		if !t.frames[b.GetFrameId()] {
			return nil, rpc.Invalid("frame %d is not part of the task", b.GetFrameId())
		}
		kps, err := annotation.ClampKeypoints(b.GetBox().GetKeypoints(), t.tracking.check.pose)
		if err != nil {
			return nil, rpc.Invalid("frame %d: %s", b.GetFrameId(), err)
		}
		if id, ok := t.tracking.posed[b.GetFrameId()]; ok && kps != nil {
			t.tracking.check.add(id, kps)
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
	if s.logs != nil {
		s.logs.RecordWorker(req.GetInstanceId(), req.GetHostname(), req.GetLogs())
	}
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
