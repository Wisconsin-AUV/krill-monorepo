package pipeline

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/tracker"
)

// trialFixture is a clip with a finished run that knows buoys and gates of
// two colors.
type trialFixture struct {
	pool    *pgxpool.Pool
	p       *Pipeline
	ctx     context.Context
	buoy    int64
	gate    int64
	videoID int64
	clipID  int64
	runID   int64
	frames  []db.Frame
	l       *labeler
}

func newTrialFixture(t *testing.T, frames int) *trialFixture {
	t.Helper()
	pool := testPool(t)
	f := &trialFixture{pool: pool, ctx: t.Context()}
	f.p = New(Deps{
		Pool:       pool,
		Dispatcher: tracker.NewDispatcher(),
		Config:     func() config.Config { return config.Config{PrelabelConfidence: 0.25} },
	})
	for _, lt := range []*int64{&f.buoy, &f.gate} {
		*lt = scalar[int64](t, pool, `INSERT INTO label_types (name, color, position) VALUES ('trial-test-' || gen_random_uuid(), '#ff0000', 99) RETURNING id`)
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, *lt) })
	}
	f.videoID = scalar[int64](t, pool, `INSERT INTO videos (name, filename, status, split, frame_count) VALUES ('pool.mp4', 'pool.mp4', 'ready', 'test', $1) RETURNING id`, frames)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, f.videoID) })
	f.clipID = scalar[int64](t, pool, `INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, $2) RETURNING id`, f.videoID, frames)
	for i := range frames {
		if _, err := pool.Exec(f.ctx, `INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, $3, 0)`, f.videoID, f.clipID, i); err != nil {
			t.Fatal(err)
		}
	}
	var err error
	if f.frames, err = db.New(pool).ListClipFrames(f.ctx, f.clipID); err != nil {
		t.Fatal(err)
	}
	classes := []classTarget{
		{Name: "buoy", LabelTypeID: f.buoy, Attributes: map[string]string{}},
		{Name: "gate_red", LabelTypeID: f.gate, Attributes: map[string]string{"color": "red"}},
		{Name: "gate_white", LabelTypeID: f.gate, Attributes: map[string]string{"color": "white"}},
	}
	raw, err := json.Marshal(classes)
	if err != nil {
		t.Fatal(err)
	}
	f.runID = scalar[int64](t, pool, `INSERT INTO training_runs (trigger, status, config, classes) VALUES ('manual', 'succeeded', '{}', $1) RETURNING id`, raw)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM training_runs WHERE id = $1`, f.runID) })
	f.l = &labeler{source: sourceYolo, targets: map[string]target{}, task: func(context.Context, *krillv1.PrelabelTask) error { return nil }}
	for i, c := range classes {
		f.l.targets[c.Name] = target{index: i, labelTypeID: c.LabelTypeID}
	}
	return f
}

func (f *trialFixture) trial(t *testing.T, status string) int64 {
	t.Helper()
	return scalar[int64](t, f.pool, `INSERT INTO model_trials (training_run_id, clip_id, status, frames_total) VALUES ($1, $2, $3, $4) RETURNING id`,
		f.runID, f.clipID, status, len(f.frames))
}

func (f *trialFixture) run(id int64) error {
	sign := func(context.Context, int32) (string, error) { return "http://frames.invalid/", nil }
	return f.p.workTrial(f.ctx, id, func(ctx context.Context) error { return f.p.runTrial(ctx, id, f.l, f.frames, sign) })
}

// worker hands each task the trial sends to report, until ctx ends.
func (f *trialFixture) worker(ctx context.Context, report func(task *krillv1.PrelabelTask, ws *WorkerService)) {
	ws := NewWorkerService(f.p)
	for ctx.Err() == nil {
		msg := f.p.Dispatcher.TakeWaiting()
		if msg == nil {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		report(msg.GetPrelabel(), ws)
	}
}

func box(frameID int64, label string, x float64, conf float32) *krillv1.PrelabelBox {
	return &krillv1.PrelabelBox{FrameId: frameID, Label: label, Confidence: conf, Box: &krillv1.Box{X: x, Y: 0.2, Width: 0.2, Height: 0.2}}
}

func TestTrialKeepsItsBoxesApartFromLabels(t *testing.T) {
	f := newTrialFixture(t, trialChunk+6)
	// A labeled frame with a verified box, which a trial must leave alone
	// even on a held-out video.
	if _, err := f.pool.Exec(f.ctx, `UPDATE frames SET status = 'labeled' WHERE id = $1`, f.frames[0].ID); err != nil {
		t.Fatal(err)
	}
	track := scalar[int64](t, f.pool, `INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, f.clipID, f.buoy)
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO annotations (track_id, frame_id, x, y, width, height) VALUES ($1, $2, 0.1, 0.2, 0.2, 0.2)`, track, f.frames[0].ID); err != nil {
		t.Fatal(err)
	}
	labels := func() [3]int {
		return [3]int{
			scalar[int](t, f.pool, `SELECT count(*)::int FROM tracks WHERE clip_id = $1`, f.clipID),
			scalar[int](t, f.pool, `SELECT count(*)::int FROM annotations a JOIN frames fr ON fr.id = a.frame_id WHERE fr.clip_id = $1`, f.clipID),
			scalar[int](t, f.pool, `SELECT count(*)::int FROM prelabel_runs`),
		}
	}
	before := labels()
	id := f.trial(t, "running")

	ctx, stop := context.WithCancel(f.ctx)
	defer stop()
	var progress []int32
	go f.worker(ctx, func(task *krillv1.PrelabelTask, ws *WorkerService) {
		frames := task.GetFrames()
		report := func(boxes []*krillv1.PrelabelBox, done bool, framesDone int32) {
			req := &krillv1.ReportPrelabelsRequest{TaskId: task.GetId(), Boxes: boxes, Done: done, FramesDone: framesDone}
			if _, err := ws.ReportPrelabels(f.ctx, req); err != nil {
				t.Error(err)
			}
			if !done {
				progress = append(progress, scalar[int32](t, f.pool, `SELECT frames_done FROM model_trials WHERE id = $1`, id))
			}
		}
		if frames[0].GetId() == f.frames[0].ID {
			report([]*krillv1.PrelabelBox{
				box(frames[0].GetId(), "buoy", 0.1, 0.12),
				box(frames[1].GetId(), "buoy", 0.11, 0.9),
			}, false, 2)
			report(nil, true, int32(len(frames))) //nolint:gosec // a chunk's length
			return
		}
		// The second chunk finds nothing.
		report(nil, false, 3)
		report(nil, true, int32(len(frames))) //nolint:gosec // a chunk's length
	})
	if err := f.run(id); err != nil {
		t.Fatal(err)
	}

	if want := []int32{2, trialChunk + 3}; len(progress) != 2 || progress[0] != want[0] || progress[1] != want[1] {
		t.Errorf("frames done while running = %v, want %v", progress, want)
	}
	var status string
	var done, total int32
	if err := f.pool.QueryRow(f.ctx, `SELECT status, frames_done, frames_total FROM model_trials WHERE id = $1`, id).Scan(&status, &done, &total); err != nil {
		t.Fatal(err)
	}
	if status != "succeeded" || done != total || int(total) != len(f.frames) {
		t.Errorf("trial is %s with %d of %d frames done, want succeeded with all %d", status, done, total, len(f.frames))
	}
	res, err := NewService(f.p).GetModelTrial(f.ctx, &krillv1.GetModelTrialRequest{Id: id})
	if err != nil {
		t.Fatal(err)
	}
	boxes := res.GetBoxes()
	if len(boxes) != 2 {
		t.Fatalf("boxes = %v, want both", boxes)
	}
	if c := boxes[0].GetConfidence(); math.Abs(float64(c)-0.12) > 1e-6 {
		t.Errorf("first box confidence = %v, want 0.12, kept though under the pre-label setting", c)
	}
	if boxes[0].GetTrack() != 1 || boxes[1].GetTrack() != 1 || boxes[0].GetLabelTypeId() != f.buoy {
		t.Errorf("boxes = %v, want buoys linked into track 1", boxes)
	}
	if after := labels(); after != before {
		t.Errorf("tracks, annotations, pre-label runs went from %v to %v", before, after)
	}
}

func TestCancelledTrialIsNotFailed(t *testing.T) {
	f := newTrialFixture(t, 2)
	id := f.trial(t, "running")
	svc := NewService(f.p)
	ctx, stop := context.WithCancel(f.ctx)
	defer stop()
	go f.worker(ctx, func(task *krillv1.PrelabelTask, ws *WorkerService) {
		_, err := ws.ReportPrelabels(f.ctx, &krillv1.ReportPrelabelsRequest{
			TaskId: task.GetId(), Boxes: []*krillv1.PrelabelBox{box(task.GetFrames()[0].GetId(), "buoy", 0.1, 0.9)}, FramesDone: 1,
		})
		if err != nil {
			t.Error(err)
		}
		if _, err := svc.CancelModelTrial(f.ctx, &krillv1.CancelModelTrialRequest{Id: id}); err != nil {
			t.Error(err)
		}
	})
	if err := f.run(id); err != nil {
		t.Errorf("run = %v, want nil once cancelled", err)
	}
	var status, msg string
	if err := f.pool.QueryRow(f.ctx, `SELECT status, error FROM model_trials WHERE id = $1`, id).Scan(&status, &msg); err != nil {
		t.Fatal(err)
	}
	if status != "cancelled" || msg != "" {
		t.Errorf("trial is %s with error %q, want cancelled", status, msg)
	}
}

func TestTrialSummaryScoresDoneFramesAgainstVerifiedBoxes(t *testing.T) {
	f := newTrialFixture(t, 3)
	fr := f.frames
	for i, status := range []string{"labeled", "empty"} {
		if _, err := f.pool.Exec(f.ctx, `UPDATE frames SET status = $2 WHERE id = $1`, fr[i].ID, status); err != nil {
			t.Fatal(err)
		}
	}
	verified := func(labelType int64, attrs string, x, y float64, status string) {
		t.Helper()
		track := scalar[int64](t, f.pool, `INSERT INTO tracks (clip_id, label_type_id, attributes) VALUES ($1, $2, $3) RETURNING id`, f.clipID, labelType, attrs)
		if _, err := f.pool.Exec(f.ctx, `INSERT INTO annotations (track_id, frame_id, x, y, width, height, status) VALUES ($1, $2, $3, $4, 0.2, 0.2, $5)`,
			track, fr[0].ID, x, y, status); err != nil {
			t.Fatal(err)
		}
	}
	verified(f.buoy, `{}`, 0.1, 0.1, "verified")
	verified(f.buoy, `{}`, 0.1, 0.4, "verified")               // missed
	verified(f.buoy, `{}`, 0.7, 0.4, "proposed")               // not ground truth
	verified(f.gate, `{}`, 0.5, 0.5, "verified")               // color unset
	verified(f.gate, `{"color": "red"}`, 0.1, 0.7, "verified") // color set

	id := f.trial(t, "succeeded")
	model := func(frame int, class int, x, y float64, conf float32) {
		t.Helper()
		if _, err := f.pool.Exec(f.ctx, `INSERT INTO model_trial_boxes (trial_id, frame_id, class, x, y, width, height, confidence) VALUES ($1, $2, $3, $4, $5, 0.2, 0.2, $6)`,
			id, fr[frame].ID, class, x, y, conf); err != nil {
			t.Fatal(err)
		}
	}
	model(0, 0, 0.1, 0.1, 0.9)  // correct
	model(0, 2, 0.5, 0.5, 0.9)  // white gate on the unset one: correct
	model(0, 2, 0.1, 0.7, 0.9)  // white gate on the red one: matched only
	model(0, 0, 0.7, 0.75, 0.9) // extra
	model(0, 0, 0.4, 0.1, 0.2)  // under the setting
	model(1, 0, 0.1, 0.1, 0.9)  // extra on an empty frame
	model(2, 0, 0.1, 0.1, 0.9)  // frame not done

	res, err := NewService(f.p).GetModelTrial(f.ctx, &krillv1.GetModelTrialRequest{Id: id})
	if err != nil {
		t.Fatal(err)
	}
	s := res.GetTrial().GetSummary()
	if s.GetLabeledFrames() != 2 || s.GetMinConfidence() != 0.25 {
		t.Errorf("summary over %d frames at %v, want 2 at 0.25", s.GetLabeledFrames(), s.GetMinConfidence())
	}
	type counts struct{ matched, correct, missed, extra int32 }
	of := func(c *krillv1.ModelTrialClass) counts {
		return counts{c.GetMatched(), c.GetCorrect(), c.GetMissed(), c.GetExtra()}
	}
	got := map[int64]counts{}
	for _, c := range s.GetClasses() {
		got[c.GetLabelTypeId()] = of(c)
	}
	if want := (counts{1, 1, 1, 2}); got[f.buoy] != want {
		t.Errorf("buoys = %+v, want %+v", got[f.buoy], want)
	}
	if want := (counts{2, 1, 0, 0}); got[f.gate] != want {
		t.Errorf("gates = %+v, want %+v", got[f.gate], want)
	}
	if want := (counts{3, 2, 1, 2}); of(s.GetAll()) != want {
		t.Errorf("all = %+v, want %+v", of(s.GetAll()), want)
	}
}

func TestStartModelTrial(t *testing.T) {
	f := newTrialFixture(t, 1)
	migrator, err := rivermigrate.New(riverpgxv5.New(f.pool), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.Migrate(f.ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient(riverpgxv5.New(f.pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	f.p.SetJobs(jobs)
	svc := NewService(f.p)
	start := func() (*krillv1.ModelTrial, error) {
		res, err := svc.StartModelTrial(f.ctx, &krillv1.StartModelTrialRequest{TrainingRunId: f.runID, ClipId: f.clipID})
		return res.GetTrial(), err
	}
	jobCount := func(id int64) int {
		return scalar[int](t, f.pool, `SELECT count(*)::int FROM river_job WHERE kind = 'model_trial' AND (args->>'trial_id')::bigint = $1`, id)
	}

	if _, err := f.pool.Exec(f.ctx, `UPDATE training_runs SET status = 'training' WHERE id = $1`, f.runID); err != nil {
		t.Fatal(err)
	}
	if _, err := start(); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("trying an unfinished run: err = %v, want failed precondition", err)
	}
	if _, err := f.pool.Exec(f.ctx, `UPDATE training_runs SET status = 'succeeded' WHERE id = $1`, f.runID); err != nil {
		t.Fatal(err)
	}

	tr, err := start()
	if err != nil {
		t.Fatal(err)
	}
	id := tr.GetId()
	t.Cleanup(func() {
		_, _ = f.pool.Exec(context.Background(), `DELETE FROM river_job WHERE kind = 'model_trial' AND (args->>'trial_id')::bigint = $1`, id)
	})
	if tr.GetStatus() != krillv1.ModelTrialStatus_MODEL_TRIAL_STATUS_QUEUED || jobCount(id) != 1 {
		t.Fatalf("first start: trial %v with %d jobs, want queued with one", tr.GetStatus(), jobCount(id))
	}

	if _, err := f.pool.Exec(f.ctx, `UPDATE model_trials SET status = 'succeeded' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	tr, err = start()
	if err != nil {
		t.Fatal(err)
	}
	if tr.GetId() != id || tr.GetStatus() != krillv1.ModelTrialStatus_MODEL_TRIAL_STATUS_SUCCEEDED || jobCount(id) != 1 {
		t.Errorf("starting a finished trial gave trial %d %v with %d jobs, want %d unchanged", tr.GetId(), tr.GetStatus(), jobCount(id), id)
	}

	// A failed trial runs again from scratch.
	if _, err := f.pool.Exec(f.ctx, `UPDATE model_trials SET status = 'failed', error = 'boom' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(f.ctx, `INSERT INTO model_trial_boxes (trial_id, frame_id, class, x, y, width, height, confidence) VALUES ($1, $2, 0, 0.1, 0.1, 0.2, 0.2, 0.5)`, id, f.frames[0].ID); err != nil {
		t.Fatal(err)
	}
	if tr, err = start(); err != nil {
		t.Fatal(err)
	}
	left := scalar[int](t, f.pool, `SELECT count(*)::int FROM model_trial_boxes WHERE trial_id = $1`, id)
	if tr.GetStatus() != krillv1.ModelTrialStatus_MODEL_TRIAL_STATUS_QUEUED || tr.GetError() != "" || left != 0 || jobCount(id) != 2 {
		t.Errorf("retrying a failed trial: %v %q with %d old boxes and %d jobs, want queued, clean, and a second job", tr.GetStatus(), tr.GetError(), left, jobCount(id))
	}
}
