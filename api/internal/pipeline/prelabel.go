package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/tracker"
	"github.com/wauv/krill/api/internal/video"
)

const (
	// prelabelPickup is long because pre-labeling waits behind tracking.
	prelabelPickup    = 30 * time.Minute
	prelabelURLExpiry = time.Hour
	prelabelTimeout   = 24 * time.Hour
)

type PrelabelArgs struct {
	RunID int64 `json:"run_id"`
}

func (PrelabelArgs) Kind() string { return "prelabel" }

func (PrelabelArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: PrelabelQueue, MaxAttempts: 1}
}

// queuePrelabel creates a pre-label run and its job in tx.
func (p *Pipeline) queuePrelabel(ctx context.Context, tx pgx.Tx, source string, model pgtype.Int8, by pgtype.UUID) (db.PrelabelRun, error) {
	q := p.q.WithTx(tx)
	run, err := q.CreatePrelabelRun(ctx, db.CreatePrelabelRunParams{Source: source, TrainingRunID: model, CreatedBy: by})
	if err != nil {
		return db.PrelabelRun{}, fmt.Errorf("create pre-label run: %w", err)
	}
	if _, err := p.jobs.InsertTx(ctx, tx, PrelabelArgs{RunID: run.ID}, nil); err != nil {
		return db.PrelabelRun{}, fmt.Errorf("queue pre-labeling: %w", err)
	}
	details := map[string]any{"source": source}
	if model.Valid {
		details["model"] = runName(model.Int64)
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.PrelabelStart,
		TargetType: audit.TargetPrelabel,
		TargetID:   strconv.FormatInt(run.ID, 10),
		TargetName: prelabelName(run.ID),
		Details:    details,
	})
	if err != nil {
		return db.PrelabelRun{}, err
	}
	return run, nil
}

func prelabelName(id int64) string {
	return fmt.Sprintf("Pre-label %d", id)
}

type PrelabelWorker struct {
	river.WorkerDefaults[PrelabelArgs]
	p *Pipeline
}

func (w *PrelabelWorker) Timeout(*river.Job[PrelabelArgs]) time.Duration { return prelabelTimeout }

func (w *PrelabelWorker) Work(ctx context.Context, job *river.Job[PrelabelArgs]) error {
	p, id := w.p, job.Args.RunID
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer p.track("prelabel", id, cancel)()

	err := p.prelabel(ctx, id)
	if err == nil {
		return nil
	}
	bg, done := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer done()
	if run, gerr := p.q.GetPrelabelRun(bg, id); gerr == nil && run.Status == "cancelled" {
		return nil
	}
	slog.Error("pre-labeling failed", "prelabel_run_id", id, "err", err)
	if ferr := p.q.FailPrelabelRun(bg, db.FailPrelabelRunParams{ID: id, Error: err.Error()}); ferr != nil {
		return errors.Join(err, fmt.Errorf("record failure: %w", ferr))
	}
	return err
}

// labeler is what one pre-label run proposes with.
type labeler struct {
	source string
	// targets maps a reported box to what it becomes. Keys are a YOLO class
	// name, or plugin + "/" + output for CV.
	targets map[string]target
	// version is stored as each box's model version. CV boxes store their
	// plugin instead.
	version string
	// task fills in the model part of a task.
	task func(ctx context.Context, msg *krillv1.PrelabelTask) error
}

// target is the label a detection becomes.
type target struct {
	index       int
	labelTypeID int64
	attributes  []byte
}

func targetKey(plugin, label string) string {
	if plugin == "" {
		return label
	}
	return plugin + "/" + label
}

func (p *Pipeline) yoloLabeler(ctx context.Context, runID int64) (*labeler, error) {
	row, err := p.q.GetTrainingRun(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}
	run := row.TrainingRun
	if run.Status != "succeeded" {
		return nil, errNotPinnable
	}
	var classes []classTarget
	if err := json.Unmarshal(run.Classes, &classes); err != nil {
		return nil, fmt.Errorf("decode classes: %w", err)
	}
	cfg := &krillv1.TrainConfig{}
	if err := unmarshal(run.Config, cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	l := &labeler{source: sourceYolo, version: strconv.FormatInt(runID, 10), targets: map[string]target{}}
	for i, c := range classes {
		attrs, err := json.Marshal(c.Attributes)
		if err != nil {
			return nil, err
		}
		l.targets[c.Name] = target{index: i, labelTypeID: c.LabelTypeID, attributes: attrs}
	}
	minConf := float32(p.Config().PrelabelConfidence)
	l.task = func(ctx context.Context, msg *krillv1.PrelabelTask) error {
		url, err := p.Store.PresignGetInternal(ctx, storage.TrainingWeightsKey(runID), prelabelURLExpiry)
		if err != nil {
			return err
		}
		msg.Model = &krillv1.PrelabelTask_Yolo{Yolo: &krillv1.YoloModel{
			Version:       l.version,
			WeightsUrl:    url,
			MinConfidence: minConf,
			ImageSize:     cfg.GetImageSize(),
		}}
		return nil
	}
	return l, nil
}

func (p *Pipeline) cvLabeler(ctx context.Context) (*labeler, error) {
	rows, err := p.q.ListEnabledPrelabelers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list pre-labelers: %w", err)
	}
	l := &labeler{source: sourceCV, targets: map[string]target{}}
	var plugins []*krillv1.PluginConfig
	for _, r := range rows {
		pl, err := prelabeler(r, "")
		if err != nil {
			return nil, err
		}
		mapped := false
		for _, o := range pl.GetOutputs() {
			if o.GetLabelTypeId() == 0 {
				continue
			}
			attrs, err := json.Marshal(o.GetAttributes())
			if err != nil {
				return nil, err
			}
			l.targets[targetKey(r.Name, o.GetName())] = target{index: len(l.targets), labelTypeID: o.GetLabelTypeId(), attributes: attrs}
			mapped = true
		}
		if mapped {
			plugins = append(plugins, &krillv1.PluginConfig{Name: r.Name, Params: pl.GetParams()})
		}
	}
	if len(plugins) == 0 {
		return nil, errors.New("no enabled pre-labeler has an output mapped to a label type")
	}
	l.task = func(_ context.Context, msg *krillv1.PrelabelTask) error {
		msg.Model = &krillv1.PrelabelTask_Cv{Cv: &krillv1.CvPlugins{Plugins: plugins}}
		return nil
	}
	return l, nil
}

func (p *Pipeline) prelabel(ctx context.Context, id int64) error {
	q := p.q
	run, err := q.GetPrelabelRun(ctx, id)
	if err != nil {
		return fmt.Errorf("load run: %w", err)
	}
	if run.Status != "queued" {
		return nil
	}
	var l *labeler
	if run.Source == sourceYolo {
		if !run.TrainingRunID.Valid {
			return errors.New("the model was deleted")
		}
		l, err = p.yoloLabeler(ctx, run.TrainingRunID.Int64)
	} else {
		l, err = p.cvLabeler(ctx)
	}
	if err != nil {
		return err
	}
	clips, err := q.ListPrelabelClips(ctx)
	if err != nil {
		return fmt.Errorf("list clips: %w", err)
	}
	n, err := q.StartPrelabelRun(ctx, db.StartPrelabelRunParams{ID: id, ClipsTotal: int32(len(clips))}) //nolint:gosec // clip counts fit in int32
	if err != nil {
		return fmt.Errorf("mark running: %w", err)
	}
	if n == 0 {
		return nil
	}
	for _, c := range clips {
		if err := ctx.Err(); err != nil {
			return err
		}
		frames, proposals, err := p.prelabelClip(ctx, id, l, c)
		if err != nil {
			return fmt.Errorf("clip %d: %w", c.ID, err)
		}
		err = q.AdvancePrelabelRun(ctx, db.AdvancePrelabelRunParams{ID: id, Frames: int32(frames), Proposals: int32(proposals)}) //nolint:gosec // per-clip counts fit in int32
		if err != nil {
			return fmt.Errorf("save progress: %w", err)
		}
	}
	if err := q.FinishPrelabelRun(ctx, id); err != nil {
		return fmt.Errorf("finish run: %w", err)
	}
	slog.Info("pre-labeling finished", "prelabel_run_id", id, "source", run.Source, "clips", len(clips))
	return nil
}

// collector gathers the boxes a worker reports for one prelabel task.
type collector struct {
	mu     sync.Mutex
	frames map[int64]bool
	boxes  []*krillv1.PrelabelBox
}

func (c *collector) add(boxes []*krillv1.PrelabelBox) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.boxes = append(c.boxes, boxes...)
}

func (p *Pipeline) prelabelClip(ctx context.Context, runID int64, l *labeler, c db.ListPrelabelClipsRow) (frames, proposals int, err error) {
	rows, err := p.q.ListUnlabeledClipFrames(ctx, c.ID)
	if err != nil {
		return 0, 0, fmt.Errorf("list frames: %w", err)
	}
	if len(rows) == 0 {
		return 0, 0, nil
	}
	tid := p.Dispatcher.NextID()
	msg := &krillv1.PrelabelTask{Id: tid, Frames: make([]*krillv1.PrelabelFrame, len(rows))}
	col := &collector{frames: make(map[int64]bool, len(rows))}
	idx := make(map[int64]int32, len(rows))
	for i, f := range rows {
		url, err := p.Store.PresignGetInternal(ctx, storage.FrameKey(c.VideoID, f.Idx), prelabelURLExpiry)
		if err != nil {
			return 0, 0, err
		}
		msg.Frames[i] = &krillv1.PrelabelFrame{Id: f.ID, Url: url}
		col.frames[f.ID] = true
		idx[f.ID] = f.Idx
	}
	if err := l.task(ctx, msg); err != nil {
		return 0, 0, err
	}
	t := tracker.NewTask(tid, tracker.Background, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Prelabel{Prelabel: msg}}, 0)
	p.prelabels.Store(tid, col)
	defer p.prelabels.Delete(tid)
	if err := p.Dispatcher.Run(ctx, t, prelabelPickup); err != nil {
		return 0, 0, err
	}

	var dets []detection
	for _, b := range col.boxes {
		tg, ok := l.targets[targetKey(b.GetPlugin(), b.GetLabel())]
		if !ok {
			continue
		}
		x, y, w, h, err := annotation.ClampBox(b.GetBox())
		if err != nil {
			continue
		}
		model := l.version
		if model == "" {
			model = b.GetPlugin()
		}
		dets = append(dets, detection{
			frameID:  b.GetFrameId(),
			frameIdx: idx[b.GetFrameId()],
			box:      annotation.Box{X: x, Y: y, Width: w, Height: h},
			conf:     min(max(b.GetConfidence(), 0), 1),
			target:   tg.index,
			model:    model,
		})
	}
	n, err := p.savePrelabels(ctx, runID, l, c, rows, dets)
	if err != nil {
		return 0, 0, err
	}
	return len(rows), n, nil
}

var errRunStopped = errors.New("pre-label run was cancelled")

// savePrelabels replaces the source's open proposals on the clip's frames
// that are still unlabeled with dets, grouped into tracks. Boxes anyone
// drew, accepted, or another source proposed stay, and dets on top of them
// are dropped.
func (p *Pipeline) savePrelabels(ctx context.Context, runID int64, l *labeler, c db.ListPrelabelClipsRow, frames []db.ListUnlabeledClipFramesRow, dets []detection) (int, error) {
	tx, err := p.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := p.q.WithTx(tx)

	// Pinning a new model cancels this run in its own transaction, which
	// this lock waits for, so an older model's boxes never land after the
	// new run cleared them.
	status, err := q.LockPrelabelRun(ctx, runID)
	if err != nil {
		return 0, fmt.Errorf("lock run: %w", err)
	}
	if status != "running" {
		return 0, errRunStopped
	}
	v, err := q.GetVideo(ctx, c.VideoID)
	if err != nil {
		return 0, fmt.Errorf("load video: %w", err)
	}
	if video.HeldOut(v.Split) {
		return 0, nil
	}

	ids := make([]int64, len(frames))
	for i, f := range frames {
		ids[i] = f.ID
	}
	locked, err := q.LockUnlabeledFrames(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("lock frames: %w", err)
	}
	open := make(map[int64]bool, len(locked))
	for _, id := range locked {
		open[id] = true
	}
	stale, err := q.DeleteOpenPrelabels(ctx, db.DeleteOpenPrelabelsParams{FrameIds: locked, Source: l.source})
	if err != nil {
		return 0, fmt.Errorf("clear old proposals: %w", err)
	}
	if len(stale) > 0 {
		if err := q.DeleteEmptyTracks(ctx, stale); err != nil {
			return 0, fmt.Errorf("delete emptied tracks: %w", err)
		}
	}
	rows, err := q.ListBoxesOnFrames(ctx, locked)
	if err != nil {
		return 0, fmt.Errorf("list boxes: %w", err)
	}
	existing := map[int64][]annotation.Box{}
	for _, r := range rows {
		existing[r.FrameID] = append(existing[r.FrameID], annotation.Box{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height})
	}
	kept := dets[:0:0]
	for _, d := range dets {
		if open[d.frameID] {
			kept = append(kept, d)
		}
	}
	kept = dropCovered(kept, existing)

	byIndex := map[int]target{}
	for _, tg := range l.targets {
		byIndex[tg.index] = tg
	}
	ins := db.InsertPrelabelsParams{Source: l.source}
	for _, group := range linkTracks(kept) {
		tg := byIndex[kept[group[0]].target]
		track, err := q.CreateTrack(ctx, db.CreateTrackParams{ClipID: c.ID, LabelTypeID: tg.labelTypeID, Attributes: tg.attributes})
		if err != nil {
			return 0, fmt.Errorf("create track: %w", err)
		}
		for _, i := range group {
			d := kept[i]
			ins.TrackIds = append(ins.TrackIds, track.ID)
			ins.FrameIds = append(ins.FrameIds, d.frameID)
			ins.Xs = append(ins.Xs, d.box.X)
			ins.Ys = append(ins.Ys, d.box.Y)
			ins.Widths = append(ins.Widths, d.box.Width)
			ins.Heights = append(ins.Heights, d.box.Height)
			ins.ModelVersions = append(ins.ModelVersions, d.model)
			ins.Confidences = append(ins.Confidences, d.conf)
		}
	}
	n := int64(0)
	if len(ins.TrackIds) > 0 {
		if n, err = q.InsertPrelabels(ctx, ins); err != nil {
			return 0, fmt.Errorf("save proposals: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(n), nil
}
