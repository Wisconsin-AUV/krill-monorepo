package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/export"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/taxonomy"
	"github.com/wauv/krill/api/internal/tracker"
)

const (
	// trainPickup is long because training waits until nothing else is
	// queued for the GPU.
	trainPickup = 6 * time.Hour
	// trainIdle allows for a slow epoch, or a long tracking job served
	// between batches.
	trainIdle = 20 * time.Minute
	// trainURLExpiry outlasts the longest run, since the weights are uploaded
	// at the end.
	trainURLExpiry = 48 * time.Hour
	trainTimeout   = 36 * time.Hour
	// maxLog keeps the end of a run's log.
	maxLog = 32 << 10
)

var sizes = map[string]krillv1.ModelSize{
	config.ModelNano:  krillv1.ModelSize_MODEL_SIZE_NANO,
	config.ModelSmall: krillv1.ModelSize_MODEL_SIZE_SMALL,
}

// classTarget is what a YOLO class maps back to.
type classTarget struct {
	Name        string            `json:"name"`
	LabelTypeID int64             `json:"label_type_id"`
	Attributes  map[string]string `json:"attributes"`
}

// classTargets lists every class the label types export, in the order
// export.Build numbers them.
func classTargets(types []db.LabelType) ([]classTarget, error) {
	var out []classTarget
	for _, t := range types {
		attrs, err := taxonomy.ParseAttributes(t.Attributes)
		if err != nil {
			return nil, fmt.Errorf("label type %d: %w", t.ID, err)
		}
		combos := []map[string]string{{}}
		for _, a := range attrs {
			next := make([]map[string]string, 0, len(combos)*len(a.Options))
			for _, c := range combos {
				for _, o := range a.Options {
					v := make(map[string]string, len(c)+1)
					for k, x := range c {
						v[k] = x
					}
					v[a.Name] = o
					next = append(next, v)
				}
			}
			combos = next
		}
		for _, values := range combos {
			out = append(out, classTarget{
				Name:        taxonomy.ClassName(t.Name, attrs, values),
				LabelTypeID: t.ID,
				Attributes:  values,
			})
		}
	}
	return out, nil
}

// resolveConfig fills unset fields from the settings and checks the rest.
func (p *Pipeline) resolveConfig(ctx context.Context, q *db.Queries, in *krillv1.TrainConfig) (*krillv1.TrainConfig, error) {
	cfg := p.Config()
	out := &krillv1.TrainConfig{}
	if in != nil {
		out = proto.Clone(in).(*krillv1.TrainConfig)
	}
	if out.GetSize() == krillv1.ModelSize_MODEL_SIZE_UNSPECIFIED {
		out.Size = sizes[cfg.RetrainModel]
	}
	if out.GetEpochs() == 0 {
		out.Epochs = int32(cfg.RetrainEpochs) //nolint:gosec // validated to at most 1000
	}
	if out.GetImageSize() == 0 {
		out.ImageSize = int32(cfg.RetrainImageSize) //nolint:gosec // validated to at most 1920
	}
	if out.GetBatch() == 0 {
		out.Batch = int32(cfg.RetrainBatch) //nolint:gosec // validated to at most 256
	}
	if out.Dataset == nil {
		out.Dataset = export.Defaults(cfg)
	}
	var err error
	if out.Dataset, err = export.Normalize(out.GetDataset()); err != nil {
		return nil, invalidConfig{err}
	}
	if err := config.CheckTraining(int(out.GetEpochs()), int(out.GetImageSize()), int(out.GetBatch())); err != nil {
		var fe *config.FieldError
		if errors.As(err, &fe) {
			return nil, invalidConfig{fmt.Errorf("%s %s", trainFields[fe.Key], fe.Message)}
		}
		return nil, err
	}
	if out.GetBaseRunId() != 0 {
		base, err := q.GetTrainingRun(ctx, out.GetBaseRunId())
		if errors.Is(err, pgx.ErrNoRows) || err == nil && base.TrainingRun.Status != "succeeded" {
			return nil, invalidConfig{fmt.Errorf("run %d has no model to start from", out.GetBaseRunId())}
		}
		if err != nil {
			return nil, err
		}
		baseCfg := &krillv1.TrainConfig{}
		if err := unmarshal(base.TrainingRun.Config, baseCfg); err != nil {
			return nil, fmt.Errorf("decode run %d config: %w", base.TrainingRun.ID, err)
		}
		out.Size = baseCfg.GetSize()
	}
	if _, ok := krillv1.ModelSize_name[int32(out.GetSize())]; !ok || out.GetSize() == krillv1.ModelSize_MODEL_SIZE_UNSPECIFIED {
		return nil, invalidConfig{errors.New("unknown model size")}
	}
	return out, nil
}

var trainFields = map[string]string{
	"KRILL_RETRAIN_EPOCHS":     "epochs",
	"KRILL_RETRAIN_IMAGE_SIZE": "image size",
	"KRILL_RETRAIN_BATCH":      "batch size",
}

// invalidConfig is a config the caller can fix.
type invalidConfig struct{ error }

// queueRun creates a run and its job in tx.
func (p *Pipeline) queueRun(ctx context.Context, tx pgx.Tx, trigger string, by pgtype.UUID, cfg *krillv1.TrainConfig) (db.TrainingRun, error) {
	q := p.q.WithTx(tx)
	verified, err := q.CountVerifiedBoxes(ctx)
	if err != nil {
		return db.TrainingRun{}, fmt.Errorf("count verified boxes: %w", err)
	}
	cfgJSON, err := protojson.Marshal(cfg)
	if err != nil {
		return db.TrainingRun{}, fmt.Errorf("encode config: %w", err)
	}
	run, err := q.CreateTrainingRun(ctx, db.CreateTrainingRunParams{
		Trigger: trigger, CreatedBy: by, Config: cfgJSON, VerifiedBoxes: verified,
	})
	if err != nil {
		return db.TrainingRun{}, fmt.Errorf("create run: %w", err)
	}
	if _, err := p.jobs.InsertTx(ctx, tx, TrainArgs{RunID: run.ID}, nil); err != nil {
		return db.TrainingRun{}, fmt.Errorf("queue run: %w", err)
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.ModelTrain,
		TargetType: audit.TargetModel,
		TargetID:   strconv.FormatInt(run.ID, 10),
		TargetName: runName(run.ID),
		Details:    map[string]any{"trigger": trigger, "epochs": cfg.GetEpochs(), "size": sizeName(cfg.GetSize())},
	})
	if err != nil {
		return db.TrainingRun{}, err
	}
	return run, nil
}

func runName(id int64) string {
	return fmt.Sprintf("Run %d", id)
}

func sizeName(s krillv1.ModelSize) string {
	for k, v := range sizes {
		if v == s {
			return k
		}
	}
	return ""
}

type TrainArgs struct {
	RunID int64 `json:"run_id"`
}

func (TrainArgs) Kind() string { return "train" }

func (TrainArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: TrainQueue, MaxAttempts: 1}
}

type TrainWorker struct {
	river.WorkerDefaults[TrainArgs]
	p *Pipeline
}

func (w *TrainWorker) Timeout(*river.Job[TrainArgs]) time.Duration { return trainTimeout }

func (w *TrainWorker) Work(ctx context.Context, job *river.Job[TrainArgs]) error {
	p, id := w.p, job.Args.RunID
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer p.track("train", id, cancel)()

	err := p.train(ctx, id)
	bg, done := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer done()
	if err != nil {
		if run, gerr := p.q.GetTrainingRun(bg, id); gerr == nil && run.TrainingRun.Status == "cancelled" {
			return nil
		}
		slog.Error("training failed", "run_id", id, "err", err)
		if ferr := p.q.FailTrainingRun(bg, db.FailTrainingRunParams{ID: id, Error: err.Error()}); ferr != nil {
			return errors.Join(err, fmt.Errorf("record failure: %w", ferr))
		}
		return err
	}
	if err := p.autoPin(context.WithoutCancel(ctx), id); err != nil {
		slog.Error("pin trained model", "run_id", id, "err", err)
	}
	return nil
}

// trainState is what FinishTrain needs to save a run.
type trainState struct {
	runID    int64
	baseline int64
}

func (p *Pipeline) train(ctx context.Context, id int64) error {
	q := p.q
	n, err := q.StartTrainingRun(ctx, id)
	if err != nil {
		return fmt.Errorf("mark preparing: %w", err)
	}
	if n == 0 {
		return nil
	}
	row, err := q.GetTrainingRun(ctx, id)
	if err != nil {
		return fmt.Errorf("load run: %w", err)
	}
	run := row.TrainingRun
	cfg := &krillv1.TrainConfig{}
	if err := unmarshal(run.Config, cfg); err != nil {
		return fmt.Errorf("decode config: %w", err)
	}

	plan, err := export.Load(ctx, q, cfg.GetDataset())
	if err != nil {
		return err
	}
	stats := plan.Stats
	switch {
	case stats.GetTrainImages() == 0:
		return errors.New("no verified frames to train on")
	case stats.GetValImages() == 0:
		return errors.New("validation needs verified frames from a second video")
	}
	types, err := q.ListAllLabelTypes(ctx)
	if err != nil {
		return fmt.Errorf("list label types: %w", err)
	}
	targets, err := classTargets(types)
	if err != nil {
		return err
	}
	if !slices.EqualFunc(targets, plan.Classes, func(t classTarget, name string) bool { return t.Name == name }) {
		return errors.New("label types changed while planning the dataset")
	}
	statsJSON, err := protojson.Marshal(stats)
	if err != nil {
		return fmt.Errorf("encode stats: %w", err)
	}
	classesJSON, err := json.Marshal(targets)
	if err != nil {
		return fmt.Errorf("encode classes: %w", err)
	}
	err = q.SetTrainingDataset(ctx, db.SetTrainingDatasetParams{ID: id, Dataset: statsJSON, Classes: classesJSON, Epochs: cfg.GetEpochs()})
	if err != nil {
		return fmt.Errorf("save dataset stats: %w", err)
	}
	optsJSON, err := protojson.Marshal(cfg.GetDataset())
	if err != nil {
		return fmt.Errorf("encode options: %w", err)
	}
	manifest := export.Manifest{Name: runName(id), CreatedAt: time.Now(), Version: p.Version, Options: optsJSON, Stats: statsJSON}
	if _, err := export.Upload(ctx, p.Store, storage.TrainingDatasetKey(id), plan, manifest, func(int, int) {}); err != nil {
		return fmt.Errorf("write dataset: %w", err)
	}

	tid := p.Dispatcher.NextID()
	msg := &krillv1.TrainTask{
		Id:        tid,
		RunId:     id,
		Classes:   plan.Classes,
		Size:      cfg.GetSize(),
		Epochs:    cfg.GetEpochs(),
		ImageSize: cfg.GetImageSize(),
		Batch:     cfg.GetBatch(),
		HasTest:   stats.GetTestImages() > 0,
	}
	if msg.DatasetUrl, err = p.Store.PresignGetInternal(ctx, storage.TrainingDatasetKey(id), trainURLExpiry); err != nil {
		return err
	}
	if msg.WeightsUploadUrl, err = p.Store.PresignPutInternal(ctx, storage.TrainingWeightsKey(id), trainURLExpiry); err != nil {
		return err
	}
	if base := cfg.GetBaseRunId(); base != 0 {
		if msg.BaseWeightsUrl, err = p.Store.PresignGetInternal(ctx, storage.TrainingWeightsKey(base), trainURLExpiry); err != nil {
			return err
		}
	}
	state := trainState{runID: id}
	pinned, err := q.GetPinnedTrainingRun(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return fmt.Errorf("load pinned run: %w", err)
	case sameClasses(pinned.Classes, classesJSON):
		state.baseline = pinned.ID
		if msg.BaselineWeightsUrl, err = p.Store.PresignGetInternal(ctx, storage.TrainingWeightsKey(pinned.ID), trainURLExpiry); err != nil {
			return err
		}
	}

	t := tracker.NewTask(tid, tracker.Training, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Train{Train: msg}}, trainIdle)
	p.trains.Store(tid, state)
	defer p.trains.Delete(tid)
	if err := p.Dispatcher.Run(ctx, t, trainPickup); err != nil {
		return err
	}
	slog.Info("training finished", "run_id", id, "train_images", stats.GetTrainImages())
	return nil
}

func sameClasses(a, b []byte) bool {
	var x, y []classTarget
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	return slices.EqualFunc(x, y, func(s, t classTarget) bool { return s.Name == t.Name })
}

// beatsBaseline reports whether a run scored better than the pinned model
// did on the same images, preferring the held-out test videos.
func beatsBaseline(r *krillv1.TrainingRun) bool {
	if r.GetBaselineRunId() == 0 {
		return false
	}
	if r.GetTest() != nil && r.GetBaselineTest() != nil {
		return r.GetTest().GetAll().GetMap50_95() > r.GetBaselineTest().GetAll().GetMap50_95()
	}
	if r.GetVal() != nil && r.GetBaselineVal() != nil {
		return r.GetVal().GetAll().GetMap50_95() > r.GetBaselineVal().GetAll().GetMap50_95()
	}
	return false
}

// autoPin pins a scheduled run's model when settings allow and it beats the
// pinned one, or when nothing is pinned yet.
func (p *Pipeline) autoPin(ctx context.Context, id int64) error {
	if !p.Config().RetrainAutoPin {
		return nil
	}
	row, err := p.q.GetTrainingRun(ctx, id)
	if err != nil {
		return err
	}
	if row.TrainingRun.Trigger != triggerSchedule || row.TrainingRun.Status != "succeeded" {
		return nil
	}
	run, err := trainingRun(row.TrainingRun, "", false)
	if err != nil {
		return err
	}
	_, err = p.q.GetPinnedTrainingRun(ctx)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return err
	case !beatsBaseline(run):
		return nil
	}
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, cancelled, err := p.pin(ctx, tx, id, pgtype.UUID{})
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	p.stopAll(cancelled)
	return nil
}

var errNotPinnable = errors.New("only a finished run's model can be pinned")

// pin makes run id the pinned model in tx, cancels pre-labeling with the
// model it replaces, and queues pre-labeling with the new one. Once tx
// commits, call stopAll with the cancelled runs.
func (p *Pipeline) pin(ctx context.Context, tx pgx.Tx, id int64, by pgtype.UUID) (db.PrelabelRun, []int64, error) {
	q := p.q.WithTx(tx)
	var from any
	if old, err := q.GetPinnedTrainingRun(ctx); err == nil {
		from = runName(old.ID)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return db.PrelabelRun{}, nil, fmt.Errorf("load pinned run: %w", err)
	}
	if err := q.UnpinTrainingRuns(ctx); err != nil {
		return db.PrelabelRun{}, nil, fmt.Errorf("unpin: %w", err)
	}
	n, err := q.PinTrainingRun(ctx, id)
	if err != nil {
		return db.PrelabelRun{}, nil, fmt.Errorf("pin: %w", err)
	}
	if n == 0 {
		return db.PrelabelRun{}, nil, errNotPinnable
	}
	cancelled, err := q.CancelActivePrelabelRuns(ctx, sourceYolo)
	if err != nil {
		return db.PrelabelRun{}, nil, fmt.Errorf("cancel pre-labeling: %w", err)
	}
	pre, err := p.queuePrelabel(ctx, tx, sourceYolo, pgtype.Int8{Int64: id, Valid: true}, by)
	if err != nil {
		return db.PrelabelRun{}, nil, err
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.ModelPin,
		TargetType: audit.TargetModel,
		TargetID:   strconv.FormatInt(id, 10),
		TargetName: runName(id),
		Details:    audit.Change(from, runName(id)),
	})
	if err != nil {
		return db.PrelabelRun{}, nil, err
	}
	return pre, cancelled, nil
}

// stopAll stops the jobs working the given pre-label runs.
func (p *Pipeline) stopAll(prelabelRuns []int64) {
	for _, id := range prelabelRuns {
		p.stop("prelabel", id)
	}
}
