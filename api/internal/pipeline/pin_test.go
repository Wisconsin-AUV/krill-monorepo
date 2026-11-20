package pipeline

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/tracker"
)

func TestScheduledRunReplacesAPinnedModelOfAnotherTask(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		t.Fatal(err)
	}
	jobs, err := river.NewClient(riverpgxv5.New(pool), &river.Config{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(Deps{
		Pool:       pool,
		Dispatcher: tracker.NewDispatcher(),
		Config:     func() config.Config { return config.Config{RetrainAutoPin: true} },
	})
	p.SetJobs(jobs)

	before := scalar[int64](t, pool, `SELECT coalesce((SELECT id FROM training_runs WHERE pinned), 0)`)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `UPDATE training_runs SET pinned = (id = $1)`, before)
	})
	addRun := func(keypoints int, pinned bool) int64 {
		id := scalar[int64](t, pool, `INSERT INTO training_runs (trigger, status, config, classes, keypoints, pinned)
			VALUES ('schedule', 'succeeded', '{}', '[]', $1, $2) RETURNING id`, keypoints, pinned)
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM prelabel_runs WHERE training_run_id = $1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM training_runs WHERE id = $1`, id)
		})
		return id
	}
	if _, err := pool.Exec(ctx, `UPDATE training_runs SET pinned = false`); err != nil {
		t.Fatal(err)
	}
	detect := addRun(0, true)
	pinned := func() int64 { return scalar[int64](t, pool, `SELECT id FROM training_runs WHERE pinned`) }

	// A run of the same task with no baseline scores never beats it.
	if err := p.autoPin(ctx, addRun(0, false)); err != nil {
		t.Fatal(err)
	}
	if got := pinned(); got != detect {
		t.Fatalf("same task: pinned run %d, want %d", got, detect)
	}

	pose := addRun(8, false)
	if err := p.autoPin(ctx, pose); err != nil {
		t.Fatal(err)
	}
	if got := pinned(); got != pose {
		t.Errorf("pose run over a detection model: pinned run %d, want %d", got, pose)
	}
}
