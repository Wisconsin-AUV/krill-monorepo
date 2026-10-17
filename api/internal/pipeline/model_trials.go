package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/gold"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/tracker"
)

const (
	// trialConfidence is the floor the worker drops boxes below. It is low so
	// the browser can try thresholds above it without running the model again.
	trialConfidence = 0.1
	// trialChunk frames go to the worker per task, so the trial shows
	// progress and a cancel stops it within a chunk.
	trialChunk = 64
)

var errTrialStopped = errors.New("model trial was cancelled")

type ModelTrialArgs struct {
	TrialID int64 `json:"trial_id"`
}

func (ModelTrialArgs) Kind() string { return "model_trial" }

func (ModelTrialArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: TrialQueue, MaxAttempts: 1}
}

func trialName(id int64) string {
	return fmt.Sprintf("Model trial %d", id)
}

type ModelTrialWorker struct {
	river.WorkerDefaults[ModelTrialArgs]
	p *Pipeline
}

func (w *ModelTrialWorker) Timeout(*river.Job[ModelTrialArgs]) time.Duration { return prelabelTimeout }

func (w *ModelTrialWorker) Work(ctx context.Context, job *river.Job[ModelTrialArgs]) error {
	id := job.Args.TrialID
	return w.p.workTrial(ctx, id, func(ctx context.Context) error { return w.p.trial(ctx, id) })
}

// workTrial runs a trial so that cancelling it stops run, and records why it
// failed unless it was cancelled.
func (p *Pipeline) workTrial(ctx context.Context, id int64, run func(context.Context) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer p.track("trial", id, cancel)()

	err := run(ctx)
	if err == nil {
		return nil
	}
	bg, done := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer done()
	if row, gerr := p.q.GetModelTrial(bg, id); gerr == nil && row.ModelTrial.Status == "cancelled" {
		return nil
	}
	slog.Error("model trial failed", "model_trial_id", id, "err", err)
	if ferr := p.q.FailModelTrial(bg, db.FailModelTrialParams{ID: id, Error: err.Error()}); ferr != nil {
		return errors.Join(err, fmt.Errorf("record failure: %w", ferr))
	}
	return err
}

func (p *Pipeline) trial(ctx context.Context, id int64) error {
	row, err := p.q.GetModelTrial(ctx, id)
	if err != nil {
		return fmt.Errorf("load trial: %w", err)
	}
	if row.ModelTrial.Status != "queued" {
		return nil
	}
	l, err := p.yoloLabeler(ctx, row.ModelTrial.TrainingRunID, trialConfidence)
	if err != nil {
		return err
	}
	frames, err := p.q.ListClipFrames(ctx, row.ModelTrial.ClipID)
	if err != nil {
		return fmt.Errorf("list frames: %w", err)
	}
	n, err := p.q.RunModelTrial(ctx, db.RunModelTrialParams{
		ID:              id,
		FramesTotal:     int32(len(frames)), //nolint:gosec // frame counts fit in int32
		ConfidenceFloor: trialConfidence,
	})
	if err != nil {
		return fmt.Errorf("mark running: %w", err)
	}
	if n == 0 {
		return nil
	}
	sign := func(ctx context.Context, idx int32) (string, error) {
		return p.Store.PresignGetInternal(ctx, storage.FrameKey(row.VideoID, idx), prelabelURLExpiry)
	}
	if err := p.runTrial(ctx, id, l, frames, sign); err != nil {
		return err
	}
	slog.Info("model trial finished", "model_trial_id", id, "training_run_id", row.ModelTrial.TrainingRunID, "frames", len(frames))
	return nil
}

// runTrial sends frames to the worker a chunk at a time, saving boxes as
// they are reported, then links them into tracks and marks the trial done.
func (p *Pipeline) runTrial(ctx context.Context, id int64, l *labeler, frames []db.Frame, sign func(context.Context, int32) (string, error)) error {
	for start := 0; start < len(frames); start += trialChunk {
		chunk := frames[start:min(start+trialChunk, len(frames))]
		if err := p.runTrialChunk(ctx, id, l, chunk, int32(start), sign); err != nil { //nolint:gosec // frame counts fit in int32
			return err
		}
		n, err := p.q.AdvanceModelTrial(ctx, db.AdvanceModelTrialParams{ID: id, FramesDone: int32(start + len(chunk))}) //nolint:gosec // frame counts fit in int32
		if err != nil {
			return fmt.Errorf("save progress: %w", err)
		}
		if n == 0 {
			return errTrialStopped
		}
	}
	if err := p.linkTrial(ctx, id); err != nil {
		return err
	}
	n, err := p.q.FinishModelTrial(ctx, id)
	if err != nil {
		return fmt.Errorf("finish trial: %w", err)
	}
	if n == 0 {
		return errTrialStopped
	}
	return nil
}

func (p *Pipeline) runTrialChunk(ctx context.Context, id int64, l *labeler, chunk []db.Frame, offset int32, sign func(context.Context, int32) (string, error)) error {
	tid := p.Dispatcher.NextID()
	msg := &krillv1.PrelabelTask{Id: tid, Frames: make([]*krillv1.PrelabelFrame, len(chunk))}
	col := &collector{frames: make(map[int64]bool, len(chunk))}
	for i, f := range chunk {
		url, err := sign(ctx, f.Idx)
		if err != nil {
			return err
		}
		msg.Frames[i] = &krillv1.PrelabelFrame{Id: f.ID, Url: url}
		col.frames[f.ID] = true
	}
	if err := l.task(ctx, msg); err != nil {
		return err
	}
	size := int32(len(chunk)) //nolint:gosec // at most trialChunk
	col.save = func(ctx context.Context, boxes []*krillv1.PrelabelBox, framesDone int32) error {
		if err := p.saveTrialBoxes(ctx, id, l, boxes); err != nil {
			return err
		}
		if framesDone <= 0 {
			return nil
		}
		_, err := p.q.AdvanceModelTrial(ctx, db.AdvanceModelTrialParams{ID: id, FramesDone: offset + min(framesDone, size)})
		return err
	}
	t := tracker.NewTask(tid, tracker.Background, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Prelabel{Prelabel: msg}}, 0)
	p.prelabels.Store(tid, col)
	defer p.prelabels.Delete(tid)
	return p.Dispatcher.Run(ctx, t, prelabelPickup)
}

func (p *Pipeline) saveTrialBoxes(ctx context.Context, id int64, l *labeler, boxes []*krillv1.PrelabelBox) error {
	ins := db.InsertModelTrialBoxesParams{TrialID: id}
	for _, b := range boxes {
		tg, ok := l.targets[b.GetLabel()]
		if !ok {
			continue
		}
		x, y, w, h, err := annotation.ClampBox(b.GetBox())
		if err != nil {
			continue
		}
		ins.FrameIds = append(ins.FrameIds, b.GetFrameId())
		ins.Classes = append(ins.Classes, int32(tg.index)) //nolint:gosec // class counts fit in int32
		ins.Xs = append(ins.Xs, x)
		ins.Ys = append(ins.Ys, y)
		ins.Widths = append(ins.Widths, w)
		ins.Heights = append(ins.Heights, h)
		ins.Confidences = append(ins.Confidences, min(max(b.GetConfidence(), 0), 1))
	}
	if len(ins.FrameIds) == 0 {
		return nil
	}
	if _, err := p.q.InsertModelTrialBoxes(ctx, ins); err != nil {
		return fmt.Errorf("save trial boxes: %w", err)
	}
	return nil
}

// linkTrial numbers the trial's boxes into tracks the way pre-labeling links
// its proposals.
func (p *Pipeline) linkTrial(ctx context.Context, id int64) error {
	rows, err := p.q.ListModelTrialBoxes(ctx, id)
	if err != nil {
		return fmt.Errorf("list trial boxes: %w", err)
	}
	dets := make([]detection, len(rows))
	for i, r := range rows {
		dets[i] = detection{
			frameID:  r.FrameID,
			frameIdx: r.FrameIdx,
			box:      annotation.Box{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height},
			conf:     r.Confidence,
			target:   int(r.Class),
		}
	}
	set := db.SetModelTrialTracksParams{TrialID: id}
	for n, group := range linkTracks(dets) {
		for _, i := range group {
			set.Ids = append(set.Ids, rows[i].ID)
			set.Tracks = append(set.Tracks, int32(n+1)) //nolint:gosec // track counts fit in int32
		}
	}
	if len(set.Ids) == 0 {
		return nil
	}
	if err := p.q.SetModelTrialTracks(ctx, set); err != nil {
		return fmt.Errorf("save tracks: %w", err)
	}
	return nil
}

var trialStatuses = map[string]krillv1.ModelTrialStatus{
	"queued":    krillv1.ModelTrialStatus_MODEL_TRIAL_STATUS_QUEUED,
	"running":   krillv1.ModelTrialStatus_MODEL_TRIAL_STATUS_RUNNING,
	"succeeded": krillv1.ModelTrialStatus_MODEL_TRIAL_STATUS_SUCCEEDED,
	"failed":    krillv1.ModelTrialStatus_MODEL_TRIAL_STATUS_FAILED,
	"cancelled": krillv1.ModelTrialStatus_MODEL_TRIAL_STATUS_CANCELLED,
}

func modelTrial(r db.GetModelTrialRow) *krillv1.ModelTrial {
	t := r.ModelTrial
	return &krillv1.ModelTrial{
		Id:              t.ID,
		TrainingRunId:   t.TrainingRunID,
		ClipId:          t.ClipID,
		VideoId:         r.VideoID,
		VideoName:       r.VideoName,
		ClipIndex:       r.ClipIdx,
		Status:          trialStatuses[t.Status],
		FramesTotal:     t.FramesTotal,
		FramesDone:      t.FramesDone,
		ConfidenceFloor: t.ConfidenceFloor,
		Error:           t.Error,
		CreatedBy:       r.CreatedByName,
		CreatedAt:       ts(t.CreatedAt),
		FinishedAt:      ts(t.FinishedAt),
	}
}

// trialReader loads what converting trials needs once per request.
type trialReader struct {
	p       *Pipeline
	q       *db.Queries
	classes map[int64][]classTarget
	names   map[int64]string
	order   []int64
}

func (p *Pipeline) trialReader(q *db.Queries) *trialReader {
	return &trialReader{p: p, q: q, classes: map[int64][]classTarget{}}
}

func (r *trialReader) runClasses(ctx context.Context, runID int64) ([]classTarget, error) {
	if c, ok := r.classes[runID]; ok {
		return c, nil
	}
	row, err := r.q.GetTrainingRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load run: %w", err)
	}
	c, err := runClasses(row.TrainingRun)
	if err != nil {
		return nil, err
	}
	r.classes[runID] = c
	return c, nil
}

// trial converts row, with its summary once it has succeeded.
func (r *trialReader) trial(ctx context.Context, row db.GetModelTrialRow) (*krillv1.ModelTrial, error) {
	out := modelTrial(row)
	if row.ModelTrial.Status != "succeeded" {
		return out, nil
	}
	classes, err := r.runClasses(ctx, row.ModelTrial.TrainingRunID)
	if err != nil {
		return nil, err
	}
	out.Summary, err = r.summary(ctx, row.ModelTrial, classes)
	return out, err
}

type tally struct{ matched, correct, missed, extra int }

func (t *tally) add(o tally) {
	t.matched += o.matched
	t.correct += o.correct
	t.missed += o.missed
	t.extra += o.extra
}

func (t tally) class(labelTypeID int64, name string) *krillv1.ModelTrialClass {
	ratio := func(a, b int) float64 {
		if b == 0 {
			return 0
		}
		return float64(a) / float64(b)
	}
	return &krillv1.ModelTrialClass{
		LabelTypeId: labelTypeID,
		Name:        name,
		Matched:     int32(t.matched), //nolint:gosec // box counts fit in int32
		Correct:     int32(t.correct), //nolint:gosec // box counts fit in int32
		Missed:      int32(t.missed),  //nolint:gosec // box counts fit in int32
		Extra:       int32(t.extra),   //nolint:gosec // box counts fit in int32
		Precision:   ratio(t.matched, t.matched+t.extra),
		Recall:      ratio(t.matched, t.matched+t.missed),
	}
}

// summary scores the trial's boxes at the live pre-label threshold against
// the verified boxes on the clip's done frames, or returns nil when none is
// done.
func (r *trialReader) summary(ctx context.Context, t db.ModelTrial, classes []classTarget) (*krillv1.ModelTrialSummary, error) {
	done, err := r.q.ListDoneClipFrames(ctx, t.ClipID)
	if err != nil {
		return nil, fmt.Errorf("list done frames: %w", err)
	}
	if len(done) == 0 {
		return nil, nil
	}
	minConf := float32(r.p.Config().PrelabelConfidence)
	refRows, err := r.q.ListDoneVerifiedBoxes(ctx, t.ClipID)
	if err != nil {
		return nil, fmt.Errorf("list verified boxes: %w", err)
	}
	ansRows, err := r.q.ListDoneTrialBoxes(ctx, db.ListDoneTrialBoxesParams{TrialID: t.ID, MinConfidence: minConf})
	if err != nil {
		return nil, fmt.Errorf("list trial boxes: %w", err)
	}
	refs := map[int64][]gold.Box{}
	for _, b := range refRows {
		var attrs map[string]string
		if err := json.Unmarshal(b.Attributes, &attrs); err != nil {
			return nil, fmt.Errorf("decode attributes: %w", err)
		}
		refs[b.FrameID] = append(refs[b.FrameID], gold.Box{
			LabelTypeID: b.LabelTypeID, Attributes: attrs, X: b.X, Y: b.Y, Width: b.Width, Height: b.Height,
		})
	}
	answers := map[int64][]gold.Box{}
	for _, b := range ansRows {
		if int(b.Class) >= len(classes) || b.Class < 0 {
			continue
		}
		c := classes[b.Class]
		answers[b.FrameID] = append(answers[b.FrameID], gold.Box{
			LabelTypeID: c.LabelTypeID, Attributes: c.Attributes, X: b.X, Y: b.Y, Width: b.Width, Height: b.Height,
		})
	}

	byType := map[int64]*tally{}
	get := func(id int64) *tally {
		if byType[id] == nil {
			byType[id] = &tally{}
		}
		return byType[id]
	}
	for _, f := range done {
		ref, ans := refs[f], answers[f]
		res := gold.ScoreSetAttributes(ref, ans)
		matchedRef := make([]bool, len(ref))
		matchedAns := make([]bool, len(ans))
		for _, m := range res.Matches {
			matchedRef[m.Reference], matchedAns[m.Answer] = true, true
			tl := get(ref[m.Reference].LabelTypeID)
			tl.matched++
			if m.Correct {
				tl.correct++
			}
		}
		for i, b := range ref {
			if !matchedRef[i] {
				get(b.LabelTypeID).missed++
			}
		}
		for i, b := range ans {
			if !matchedAns[i] {
				get(b.LabelTypeID).extra++
			}
		}
	}

	if err := r.loadNames(ctx); err != nil {
		return nil, err
	}
	out := &krillv1.ModelTrialSummary{MinConfidence: minConf, LabeledFrames: int32(len(done))} //nolint:gosec // frame counts fit in int32
	var all tally
	for _, id := range r.order {
		if tl := byType[id]; tl != nil {
			all.add(*tl)
			out.Classes = append(out.Classes, tl.class(id, r.names[id]))
			delete(byType, id)
		}
	}
	// Types deleted since keep their counts under no name.
	for _, id := range slices.Sorted(maps.Keys(byType)) {
		all.add(*byType[id])
		out.Classes = append(out.Classes, byType[id].class(id, ""))
	}
	out.All = all.class(0, "")
	return out, nil
}

func (r *trialReader) loadNames(ctx context.Context) error {
	if r.names != nil {
		return nil
	}
	types, err := r.q.ListAllLabelTypes(ctx)
	if err != nil {
		return fmt.Errorf("list label types: %w", err)
	}
	r.names = make(map[int64]string, len(types))
	for _, t := range types {
		r.names[t.ID] = t.Name
		r.order = append(r.order, t.ID)
	}
	return nil
}

func (s *Service) StartModelTrial(ctx context.Context, req *krillv1.StartModelTrialRequest) (*krillv1.StartModelTrialResponse, error) {
	q := s.p.q
	run, err := q.GetTrainingRun(ctx, req.GetTrainingRunId())
	if err != nil {
		return nil, rpc.DBError(err, "run")
	}
	if run.TrainingRun.Status != "succeeded" {
		return nil, failed("only a finished run's model can be tried")
	}
	if _, err := q.GetClip(ctx, req.GetClipId()); err != nil {
		return nil, rpc.DBError(err, "clip")
	}

	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q = q.WithTx(tx)
	key := db.QueueModelTrialParams{TrainingRunID: req.GetTrainingRunId(), ClipID: req.GetClipId(), CreatedBy: caller(ctx)}
	id, err := q.QueueModelTrial(ctx, key)
	queued := err == nil
	if errors.Is(err, pgx.ErrNoRows) {
		id, err = q.GetModelTrialID(ctx, db.GetModelTrialIDParams{TrainingRunID: key.TrainingRunID, ClipID: key.ClipID})
	}
	if err != nil {
		return nil, rpc.Internal(err, "queue trial")
	}
	if queued {
		if err := q.DeleteModelTrialBoxes(ctx, id); err != nil {
			return nil, rpc.Internal(err, "clear old boxes")
		}
		if _, err := s.p.jobs.InsertTx(ctx, tx, ModelTrialArgs{TrialID: id}, nil); err != nil {
			return nil, rpc.Internal(err, "queue trial")
		}
		err = audit.Record(ctx, q, audit.Entry{
			Action:     audit.ModelTrialStart,
			TargetType: audit.TargetModelTrial,
			TargetID:   strconv.FormatInt(id, 10),
			TargetName: trialName(id),
			Details:    map[string]any{"model": runName(key.TrainingRunID), "clip_id": key.ClipID},
		})
		if err != nil {
			return nil, rpc.Internal(err, "record audit entry")
		}
	}
	row, err := q.GetModelTrial(ctx, id)
	if err != nil {
		return nil, rpc.Internal(err, "load trial")
	}
	out, err := s.p.trialReader(q).trial(ctx, row)
	if err != nil {
		return nil, rpc.Internal(err, "load trial")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.StartModelTrialResponse{Trial: out}, nil
}

func (s *Service) GetModelTrial(ctx context.Context, req *krillv1.GetModelTrialRequest) (*krillv1.GetModelTrialResponse, error) {
	q := s.p.q
	row, err := q.GetModelTrial(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "trial")
	}
	r := s.p.trialReader(q)
	trial, err := r.trial(ctx, row)
	if err != nil {
		return nil, rpc.Internal(err, "load trial")
	}
	classes, err := r.runClasses(ctx, row.ModelTrial.TrainingRunID)
	if err != nil {
		return nil, rpc.Internal(err, "load trial")
	}
	rows, err := q.ListModelTrialBoxes(ctx, row.ModelTrial.ID)
	if err != nil {
		return nil, rpc.Internal(err, "list trial boxes")
	}
	out := &krillv1.GetModelTrialResponse{Trial: trial, Boxes: make([]*krillv1.ModelTrialBox, 0, len(rows))}
	for _, b := range rows {
		if int(b.Class) >= len(classes) || b.Class < 0 {
			continue
		}
		c := classes[b.Class]
		out.Boxes = append(out.Boxes, &krillv1.ModelTrialBox{
			FrameId:     b.FrameID,
			LabelTypeId: c.LabelTypeID,
			Attributes:  c.Attributes,
			Box:         &krillv1.Box{X: b.X, Y: b.Y, Width: b.Width, Height: b.Height},
			Confidence:  b.Confidence,
			Track:       b.Track,
		})
	}
	return out, nil
}

func (s *Service) ListModelTrials(ctx context.Context, req *krillv1.ListModelTrialsRequest) (*krillv1.ListModelTrialsResponse, error) {
	var params db.ListModelTrialsParams
	switch run, clip := req.GetTrainingRunId(), req.GetClipId(); {
	case run != 0 && clip == 0:
		params.TrainingRunID = pgtype.Int8{Int64: run, Valid: true}
	case clip != 0 && run == 0:
		params.ClipID = pgtype.Int8{Int64: clip, Valid: true}
	default:
		return nil, rpc.Invalid("give a run or a clip")
	}
	q := s.p.q
	rows, err := q.ListModelTrials(ctx, params)
	if err != nil {
		return nil, rpc.Internal(err, "list trials")
	}
	r := s.p.trialReader(q)
	out := &krillv1.ListModelTrialsResponse{Trials: make([]*krillv1.ModelTrial, len(rows))}
	for i, row := range rows {
		if out.Trials[i], err = r.trial(ctx, db.GetModelTrialRow(row)); err != nil {
			return nil, rpc.Internal(err, "list trials")
		}
	}
	return out, nil
}

func (s *Service) CancelModelTrial(ctx context.Context, req *krillv1.CancelModelTrialRequest) (*krillv1.CancelModelTrialResponse, error) {
	tx, err := s.p.Pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.p.q.WithTx(tx)
	n, err := q.CancelModelTrial(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "cancel trial")
	}
	if n == 0 {
		return nil, failed("model trial %d is not queued or running", req.GetId())
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.ModelTrialCancel,
		TargetType: audit.TargetModelTrial,
		TargetID:   strconv.FormatInt(req.GetId(), 10),
		TargetName: trialName(req.GetId()),
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	s.p.stop("trial", req.GetId())
	return &krillv1.CancelModelTrialResponse{}, nil
}
