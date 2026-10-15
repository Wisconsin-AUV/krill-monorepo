package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/taxonomy"
	"github.com/wauv/krill/api/internal/tracker"
)

const (
	listRuns         = 50
	listPrelabelRuns = 20
	weightsExpiry    = 12 * time.Hour
	previewPickup    = 30 * time.Second
	previewExpiry    = time.Hour
)

// Service backs the admin panel's pipeline pages.
type Service struct {
	krillv1connect.UnimplementedPipelineServiceHandler
	p *Pipeline
}

func NewService(p *Pipeline) *Service {
	return &Service{p: p}
}

func failed(format string, args ...any) error {
	return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf(format, args...))
}

func caller(ctx context.Context) pgtype.UUID {
	id := auth.CallerID(ctx)
	return pgtype.UUID{Bytes: id, Valid: id != uuid.Nil}
}

func (s *Service) ListTrainingRuns(ctx context.Context, _ *krillv1.ListTrainingRunsRequest) (*krillv1.ListTrainingRunsResponse, error) {
	q := s.p.q
	rows, err := q.ListTrainingRuns(ctx, listRuns)
	if err != nil {
		return nil, rpc.Internal(err, "list runs")
	}
	out := &krillv1.ListTrainingRunsResponse{Runs: make([]*krillv1.TrainingRun, len(rows))}
	for i, r := range rows {
		if out.Runs[i], err = trainingRun(r.TrainingRun, r.CreatedByName, false); err != nil {
			return nil, rpc.Internal(err, "list runs")
		}
		if r.TrainingRun.Pinned {
			out.PinnedRunId = r.TrainingRun.ID
		}
	}
	if out.Defaults, err = s.p.resolveConfig(ctx, q, nil); err != nil {
		return nil, rpc.Internal(err, "resolve defaults")
	}
	cfg := s.p.Config()
	labels, _, err := s.p.newLabels(ctx, q)
	if err != nil {
		return nil, rpc.Internal(err, "count new labels")
	}
	now := time.Now()
	zone, _ := now.Zone()
	out.Schedule = &krillv1.RetrainSchedule{
		Hour:         int32(cfg.RetrainHour),         //nolint:gosec // an hour of the day
		MinNewLabels: int32(cfg.RetrainMinNewLabels), //nolint:gosec // label counts fit in int32
		NewLabels:    int32(labels),                  //nolint:gosec // label counts fit in int32
		AutoPin:      cfg.RetrainAutoPin,
		TimeZone:     zone,
	}
	if next := nextCheck(cfg, now); !next.IsZero() {
		out.Schedule.NextCheck = timestamppb.New(next)
	}
	return out, nil
}

func (s *Service) GetTrainingRun(ctx context.Context, req *krillv1.GetTrainingRunRequest) (*krillv1.GetTrainingRunResponse, error) {
	row, err := s.p.q.GetTrainingRun(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "run")
	}
	run, err := trainingRun(row.TrainingRun, row.CreatedByName, true)
	if err != nil {
		return nil, rpc.Internal(err, "load run")
	}
	if row.TrainingRun.Status == "succeeded" {
		name := fmt.Sprintf("krill-run-%d.pt", run.GetId())
		if run.WeightsUrl, err = s.p.Store.PresignGet(ctx, storage.TrainingWeightsKey(run.GetId()), weightsExpiry, name); err != nil {
			return nil, rpc.Internal(err, "sign weights url")
		}
	}
	out := &krillv1.GetTrainingRunResponse{Run: run}
	pinned, err := s.p.q.GetPinnedTrainingRun(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return nil, rpc.Internal(err, "load pinned run")
	case pinned.ID != run.GetId():
		if out.Pinned, err = trainingRun(pinned, "", false); err != nil {
			return nil, rpc.Internal(err, "load pinned run")
		}
	}
	return out, nil
}

// lockRuns keeps two runs from starting at once.
func lockRuns(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "LOCK TABLE training_runs IN SHARE ROW EXCLUSIVE MODE")
	return err
}

func (s *Service) StartTraining(ctx context.Context, req *krillv1.StartTrainingRequest) (*krillv1.StartTrainingResponse, error) {
	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := lockRuns(ctx, tx); err != nil {
		return nil, rpc.Internal(err, "lock runs")
	}
	q := s.p.q.WithTx(tx)
	cfg, err := s.p.resolveConfig(ctx, q, req.GetConfig())
	var bad invalidConfig
	if errors.As(err, &bad) {
		return nil, rpc.Invalid("%s", bad.error)
	}
	if err != nil {
		return nil, rpc.Internal(err, "resolve config")
	}
	active, err := q.CountActiveTrainingRuns(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "count runs")
	}
	if active > 0 {
		return nil, failed("a run is already queued or training")
	}
	run, err := s.p.queueRun(ctx, tx, triggerManual, caller(ctx), cfg)
	if err != nil {
		return nil, rpc.Internal(err, "queue run")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	sess, _ := auth.SessionFrom(ctx)
	out, err := trainingRun(run, sess.User.Name, false)
	if err != nil {
		return nil, rpc.Internal(err, "load run")
	}
	return &krillv1.StartTrainingResponse{Run: out}, nil
}

func (s *Service) CancelTraining(ctx context.Context, req *krillv1.CancelTrainingRequest) (*krillv1.CancelTrainingResponse, error) {
	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.p.q.WithTx(tx)
	n, err := q.CancelTrainingRun(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "cancel run")
	}
	if n == 0 {
		return nil, failed("run %d is not queued or training", req.GetId())
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.ModelCancel,
		TargetType: audit.TargetModel,
		TargetID:   strconv.FormatInt(req.GetId(), 10),
		TargetName: runName(req.GetId()),
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	s.p.stop("train", req.GetId())
	return &krillv1.CancelTrainingResponse{}, nil
}

func (s *Service) PinModel(ctx context.Context, req *krillv1.PinModelRequest) (*krillv1.PinModelResponse, error) {
	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	pre, cancelled, err := s.p.pin(ctx, tx, req.GetId(), caller(ctx))
	if errors.Is(err, errNotPinnable) {
		return nil, failed("%s", err)
	}
	if err != nil {
		return nil, rpc.Internal(err, "pin model")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	s.p.stopAll(cancelled)
	sess, _ := auth.SessionFrom(ctx)
	return &krillv1.PinModelResponse{Prelabel: prelabelRun(pre, sess.User.Name)}, nil
}

func (s *Service) ListPrelabelRuns(ctx context.Context, _ *krillv1.ListPrelabelRunsRequest) (*krillv1.ListPrelabelRunsResponse, error) {
	rows, err := s.p.q.ListPrelabelRuns(ctx, listPrelabelRuns)
	if err != nil {
		return nil, rpc.Internal(err, "list pre-label runs")
	}
	open, err := s.p.q.CountOpenPrelabels(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "count proposals")
	}
	out := &krillv1.ListPrelabelRunsResponse{
		Runs:              make([]*krillv1.PrelabelRun, len(rows)),
		OpenYoloProposals: open.Yolo,
		OpenCvProposals:   open.Cv,
	}
	for i, r := range rows {
		out.Runs[i] = prelabelRun(r.PrelabelRun, r.CreatedByName)
	}
	return out, nil
}

func (s *Service) StartPrelabel(ctx context.Context, req *krillv1.StartPrelabelRequest) (*krillv1.StartPrelabelResponse, error) {
	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.p.q.WithTx(tx)

	var source string
	var model pgtype.Int8
	switch req.GetSource() {
	case krillv1.PrelabelSource_PRELABEL_SOURCE_YOLO:
		source = sourceYolo
		pinned, err := q.GetPinnedTrainingRun(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, failed("pin a model first")
		}
		if err != nil {
			return nil, rpc.Internal(err, "load pinned run")
		}
		model = pgtype.Int8{Int64: pinned.ID, Valid: true}
	case krillv1.PrelabelSource_PRELABEL_SOURCE_CV:
		source = sourceCV
		if _, err := s.p.cvLabeler(ctx); err != nil {
			return nil, failed("%s", err)
		}
	default:
		return nil, rpc.Invalid("unknown source")
	}
	cancelled, err := q.CancelActivePrelabelRuns(ctx, source)
	if err != nil {
		return nil, rpc.Internal(err, "cancel pre-labeling")
	}
	run, err := s.p.queuePrelabel(ctx, tx, source, model, caller(ctx))
	if err != nil {
		return nil, rpc.Internal(err, "queue pre-labeling")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	s.p.stopAll(cancelled)
	sess, _ := auth.SessionFrom(ctx)
	return &krillv1.StartPrelabelResponse{Run: prelabelRun(run, sess.User.Name)}, nil
}

func (s *Service) CancelPrelabel(ctx context.Context, req *krillv1.CancelPrelabelRequest) (*krillv1.CancelPrelabelResponse, error) {
	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.p.q.WithTx(tx)
	n, err := q.CancelPrelabelRun(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "cancel pre-labeling")
	}
	if n == 0 {
		return nil, failed("pre-label run %d is not queued or running", req.GetId())
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.PrelabelCancel,
		TargetType: audit.TargetPrelabel,
		TargetID:   strconv.FormatInt(req.GetId(), 10),
		TargetName: prelabelName(req.GetId()),
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	s.p.stop("prelabel", req.GetId())
	return &krillv1.CancelPrelabelResponse{}, nil
}

func (s *Service) ListPrelabelers(ctx context.Context, _ *krillv1.ListPrelabelersRequest) (*krillv1.ListPrelabelersResponse, error) {
	rows, err := s.p.q.ListPrelabelers(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list pre-labelers")
	}
	out := &krillv1.ListPrelabelersResponse{Prelabelers: make([]*krillv1.Prelabeler, len(rows))}
	for i, r := range rows {
		if out.Prelabelers[i], err = prelabeler(r.Prelabeler, r.UpdatedByName); err != nil {
			return nil, rpc.Internal(err, "list pre-labelers")
		}
	}
	return out, nil
}

// checkOutputs validates the label each output maps to and returns them as
// stored, keyed by output.
func checkOutputs(ctx context.Context, q *db.Queries, spec *krillv1.PrelabelerSpec, outputs []*krillv1.PrelabelerOutput) (map[string]savedOutput, error) {
	out := map[string]savedOutput{}
	for _, o := range outputs {
		if !slices.Contains(spec.GetOutputs(), o.GetName()) {
			return nil, rpc.Invalid("%s is not an output of %s", o.GetName(), spec.GetName())
		}
		if _, dup := out[o.GetName()]; dup {
			return nil, rpc.Invalid("%s is given twice", o.GetName())
		}
		if o.GetLabelTypeId() == 0 {
			if len(o.GetAttributes()) > 0 {
				return nil, rpc.Invalid("%s has attributes but no label type", o.GetName())
			}
			continue
		}
		lt, err := q.GetLabelType(ctx, o.GetLabelTypeId())
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, rpc.Invalid("label type %d does not exist", o.GetLabelTypeId())
		}
		if err != nil {
			return nil, rpc.Internal(err, "load label type")
		}
		defs, err := taxonomy.ParseAttributes(lt.Attributes)
		if err != nil {
			return nil, rpc.Internal(err, "load label type")
		}
		if err := taxonomy.CheckValues(defs, o.GetAttributes()); err != nil {
			return nil, rpc.Invalid("%s: %s", o.GetName(), err)
		}
		out[o.GetName()] = savedOutput{LabelTypeID: o.GetLabelTypeId(), Attributes: o.GetAttributes()}
	}
	return out, nil
}

func (s *Service) UpdatePrelabeler(ctx context.Context, req *krillv1.UpdatePrelabelerRequest) (*krillv1.UpdatePrelabelerResponse, error) {
	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.p.q.WithTx(tx)
	row, err := q.GetPrelabeler(ctx, req.GetName())
	if err != nil {
		return nil, rpc.DBError(err, "pre-labeler")
	}
	before, err := prelabeler(row, "")
	if err != nil {
		return nil, rpc.Internal(err, "load pre-labeler")
	}
	params, err := resolveParams(before.GetSpec(), req.GetParams())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	outputs, err := checkOutputs(ctx, q, before.GetSpec(), req.GetOutputs())
	if err != nil {
		return nil, err
	}
	if req.GetEnabled() && len(outputs) == 0 {
		return nil, rpc.Invalid("%s", errNoOutput)
	}
	paramsJSON, err := encodeParams(params)
	if err != nil {
		return nil, rpc.Internal(err, "encode params")
	}
	outputsJSON, err := json.Marshal(outputs)
	if err != nil {
		return nil, rpc.Internal(err, "encode outputs")
	}
	err = q.UpdatePrelabeler(ctx, db.UpdatePrelabelerParams{
		Name: req.GetName(), Enabled: req.GetEnabled(), Params: paramsJSON, Outputs: outputsJSON, UpdatedBy: caller(ctx),
	})
	if err != nil {
		return nil, rpc.Internal(err, "save pre-labeler")
	}
	if err := audit.Record(ctx, q, audit.Entry{
		Action:     audit.PrelabelerSave,
		TargetType: audit.TargetPlugin,
		TargetID:   req.GetName(),
		TargetName: before.GetSpec().GetTitle(),
		Details:    prelabelerChanges(before, req.GetEnabled(), params, outputs),
	}); err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	row, err = s.p.q.GetPrelabeler(ctx, req.GetName())
	if err != nil {
		return nil, rpc.Internal(err, "load pre-labeler")
	}
	sess, _ := auth.SessionFrom(ctx)
	out, err := prelabeler(row, sess.User.Name)
	if err != nil {
		return nil, rpc.Internal(err, "load pre-labeler")
	}
	return &krillv1.UpdatePrelabelerResponse{Prelabeler: out}, nil
}

// prelabelerChanges names what an update changed for the audit log.
func prelabelerChanges(before *krillv1.Prelabeler, enabled bool, params []*krillv1.ParamValue, outputs map[string]savedOutput) map[string]any {
	details := map[string]any{}
	if before.GetEnabled() != enabled {
		details["enabled"] = audit.Change(before.GetEnabled(), enabled)
	}
	var changed []string
	for i, v := range params {
		if !proto.Equal(v, before.GetParams()[i]) {
			changed = append(changed, v.GetName())
		}
	}
	if len(changed) > 0 {
		details["params"] = changed
	}
	var mapped []string
	for _, o := range before.GetOutputs() {
		now := outputs[o.GetName()]
		if now.LabelTypeID != o.GetLabelTypeId() || !maps.Equal(now.Attributes, o.GetAttributes()) {
			mapped = append(mapped, o.GetName())
		}
	}
	if len(mapped) > 0 {
		details["outputs"] = mapped
	}
	return details
}

func (s *Service) PreviewPrelabeler(ctx context.Context, req *krillv1.PreviewPrelabelerRequest) (*krillv1.PreviewPrelabelerResponse, error) {
	q := s.p.q
	row, err := q.GetPrelabeler(ctx, req.GetName())
	if err != nil {
		return nil, rpc.DBError(err, "pre-labeler")
	}
	pl, err := prelabeler(row, "")
	if err != nil {
		return nil, rpc.Internal(err, "load pre-labeler")
	}
	params, err := resolveParams(pl.GetSpec(), req.GetParams())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	frame, err := q.GetFrame(ctx, req.GetFrameId())
	if err != nil {
		return nil, rpc.DBError(err, "frame")
	}
	key := storage.FrameKey(frame.VideoID, frame.Idx)
	maskKey := storage.PrelabelerPreviewKey(req.GetName())
	out := &krillv1.PreviewPrelabelerResponse{}
	tid := s.p.Dispatcher.NextID()
	msg := &krillv1.PreviewTask{
		Id:     tid,
		Frame:  &krillv1.PrelabelFrame{Id: frame.ID},
		Plugin: &krillv1.PluginConfig{Name: req.GetName(), Params: params},
	}
	if msg.Frame.Url, err = s.p.Store.PresignGetInternal(ctx, key, previewExpiry); err == nil {
		msg.MaskUploadUrl, err = s.p.Store.PresignPutInternal(ctx, maskKey, previewExpiry)
	}
	if err == nil {
		out.FrameUrl, err = s.p.Store.PresignGet(ctx, key, previewExpiry, "")
	}
	if err != nil {
		return nil, rpc.Internal(err, "sign urls")
	}

	result := make(chan previewResult, 1)
	s.p.previews.Store(tid, result)
	defer s.p.previews.Delete(tid)
	t := tracker.NewTask(tid, tracker.Interactive, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Preview{Preview: msg}}, 0)
	if err := s.p.Dispatcher.Run(ctx, t, previewPickup); err != nil {
		if ctx.Err() != nil {
			return nil, connect.NewError(connect.CodeCanceled, err)
		}
		return nil, failed("%s", err)
	}
	var res previewResult
	select {
	case res = <-result:
	default:
		return nil, rpc.Internal(errors.New("worker finished without reporting boxes"), "preview")
	}
	out.Boxes, out.TookMs = res.boxes, res.tookMs
	if out.MaskUrl, err = s.p.Store.PresignGet(ctx, maskKey, previewExpiry, ""); err != nil {
		return nil, rpc.Internal(err, "sign mask url")
	}
	return out, nil
}
