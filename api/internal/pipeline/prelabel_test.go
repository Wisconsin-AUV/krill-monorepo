package pipeline

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/db"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	return pool
}

func scalar[T any](t *testing.T, pool *pgxpool.Pool, sql string, args ...any) T {
	t.Helper()
	var v T
	if err := pool.QueryRow(t.Context(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

type savedBox struct {
	frameID int64
	trackID int64
	source  string
	status  string
	model   string
	x       float64
	conf    pgtype.Float4
}

func clipBoxes(t *testing.T, pool *pgxpool.Pool, clipID int64) []savedBox {
	t.Helper()
	rows, err := pool.Query(t.Context(), `
		SELECT a.frame_id, a.track_id, a.source, a.status, a.model_version, a.x, a.confidence
		FROM annotations a JOIN tracks tr ON tr.id = a.track_id
		WHERE tr.clip_id = $1 ORDER BY a.frame_id, a.x`, clipID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []savedBox
	for rows.Next() {
		var b savedBox
		if err := rows.Scan(&b.frameID, &b.trackID, &b.source, &b.status, &b.model, &b.x, &b.conf); err != nil {
			t.Fatal(err)
		}
		out = append(out, b)
	}
	return out
}

func TestSavePrelabelsKeepsPeoplesBoxesAndReplacesOldProposals(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	p := New(Deps{Pool: pool})

	// Registered first so it is deleted after the tracks that use it.
	typeID := scalar[int64](t, pool, `INSERT INTO label_types (name, color, position) VALUES ('prelabel_test_buoy', '#ff0000', 99) RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, typeID) })
	videoID := scalar[int64](t, pool, `INSERT INTO videos (name, filename, status, frame_count) VALUES ('pool.mp4', 'pool.mp4', 'ready', 4) RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	clipID := scalar[int64](t, pool, `INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 4) RETURNING id`, videoID)
	frames := make([]db.ListUnlabeledClipFramesRow, 4)
	for i := range frames {
		frames[i].Idx = int32(i)
		frames[i].ID = scalar[int64](t, pool, `INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, $3, 0) RETURNING id`, videoID, clipID, i)
	}
	if _, err := pool.Exec(ctx, `UPDATE frames SET status = 'labeled' WHERE id = $1`, frames[3].ID); err != nil {
		t.Fatal(err)
	}

	// Someone drew a box on frame 0, and an older model proposed one on
	// frame 1.
	human := scalar[int64](t, pool, `INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, clipID, typeID)
	if _, err := pool.Exec(ctx, `INSERT INTO annotations (track_id, frame_id, x, y, width, height) VALUES ($1, $2, 0.1, 0.2, 0.2, 0.2)`, human, frames[0].ID); err != nil {
		t.Fatal(err)
	}
	stale := scalar[int64](t, pool, `INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, clipID, typeID)
	if _, err := pool.Exec(ctx, `
		INSERT INTO annotations (track_id, frame_id, x, y, width, height, source, status, model_version, confidence)
		VALUES ($1, $2, 0.7, 0.7, 0.1, 0.1, 'yolo', 'proposed', '1', 0.9)`, stale, frames[1].ID); err != nil {
		t.Fatal(err)
	}
	runID := scalar[int64](t, pool, `INSERT INTO prelabel_runs (source, status) VALUES ('yolo', 'running') RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM prelabel_runs WHERE id = $1`, runID) })

	l := &labeler{source: sourceYolo, version: "2", targets: map[string]target{
		"buoy": {index: 0, labelTypeID: typeID, attributes: []byte(`{}`)},
	}}
	box := func(frame int32, x float64, conf float32) detection {
		return detection{
			frameID: frames[frame].ID, frameIdx: frame, conf: conf, model: "2",
			box: annotation.Box{X: x, Y: 0.2, Width: 0.2, Height: 0.2},
		}
	}
	dets := []detection{
		box(0, 0.11, 0.8), // on top of the drawn box
		box(0, 0.5, 0.4),
		box(1, 0.52, 0.6),
		box(3, 0.5, 0.9), // frame already labeled
	}
	clip := db.ListPrelabelClipsRow{ID: clipID, VideoID: videoID}
	n, err := p.savePrelabels(ctx, runID, l, clip, frames, dets)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("saved %d proposals, want 2", n)
	}

	got := clipBoxes(t, pool, clipID)
	if len(got) != 3 {
		t.Fatalf("boxes = %+v, want the drawn box and two proposals", got)
	}
	if b := got[0]; b.trackID != human || b.source != "human" || b.status != "verified" || b.x != 0.1 {
		t.Errorf("drawn box = %+v, want it untouched", b)
	}
	if got[1].trackID != got[2].trackID {
		t.Errorf("proposals on frames 0 and 1 are in tracks %d and %d, want one", got[1].trackID, got[2].trackID)
	}
	for _, b := range got[1:] {
		if b.source != "yolo" || b.status != "proposed" || b.model != "2" || !b.conf.Valid {
			t.Errorf("proposal = %+v, want an open yolo proposal from model 2 with its confidence", b)
		}
	}
	if got[1].conf.Float32 != 0.4 {
		t.Errorf("confidence = %v, want 0.4", got[1].conf.Float32)
	}
	if n := scalar[int](t, pool, `SELECT count(*)::int FROM tracks WHERE id = $1`, stale); n != 0 {
		t.Error("the old model's emptied track was left behind")
	}

	// A run cancelled by pinning another model must not write over it.
	if _, err := pool.Exec(ctx, `UPDATE prelabel_runs SET status = 'cancelled' WHERE id = $1`, runID); err != nil {
		t.Fatal(err)
	}
	if _, err := p.savePrelabels(ctx, runID, l, clip, frames, []detection{box(2, 0.3, 0.5)}); !errors.Is(err, errRunStopped) {
		t.Errorf("err = %v, want errRunStopped", err)
	}
	if after := clipBoxes(t, pool, clipID); !slices.Equal(after, got) {
		t.Errorf("a cancelled run changed boxes to %+v", after)
	}
}

func TestHeldOutVideosAreNeverPrelabeled(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	var clips []int64
	for _, split := range []string{"auto", "test"} {
		videoID := scalar[int64](t, pool, `INSERT INTO videos (name, filename, status, split, frame_count) VALUES ('v', 'v.mp4', 'ready', $1, 1) RETURNING id`, split)
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
		clipID := scalar[int64](t, pool, `INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 1) RETURNING id`, videoID)
		if _, err := pool.Exec(ctx, `INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, 0, 0)`, videoID, clipID); err != nil {
			t.Fatal(err)
		}
		clips = append(clips, clipID)
	}
	rows, err := db.New(pool).ListPrelabelClips(ctx)
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	if !slices.Contains(ids, clips[0]) || slices.Contains(ids, clips[1]) {
		t.Errorf("clips to pre-label = %v, want %d and not held-out %d", ids, clips[0], clips[1])
	}
}
