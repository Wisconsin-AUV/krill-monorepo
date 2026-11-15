package tracker

import (
	"context"
	"os"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

func TestReportOrientLeavesBoxesChangedSinceTheFitStarted(t *testing.T) {
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	scalar := func(sql string, args ...any) int64 {
		t.Helper()
		var v int64
		if err := pool.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return v
	}
	labelType := func(oriented bool) int64 {
		id := scalar(`INSERT INTO label_types (name, color, position, oriented) VALUES ($1, '#fff', 99, $2) RETURNING id`,
			"orient-test-"+uuid.NewString()[:8], oriented)
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, id) })
		return id
	}
	gate, buoy := labelType(true), labelType(false)
	videoID := scalar(`INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'ready') RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	clipID := scalar(`INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 1) RETURNING id`, videoID)
	frameID := scalar(`INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, 0, 0) RETURNING id`, videoID, clipID)
	box := func(x float64) (annotationID, trackID int64) {
		trackID = scalar(`INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, clipID, gate)
		annotationID = scalar(`INSERT INTO annotations (track_id, frame_id, x, y, width, height) VALUES ($1, $2, $3, 0.1, 0.2, 0.2) RETURNING id`,
			trackID, frameID, x)
		return annotationID, trackID
	}
	untouched, _ := box(0.1)
	moved, _ := box(0.4)
	relabeled, relabeledTrack := box(0.7)

	q := db.New(pool)
	rows, err := q.ListUnrotatedBoxes(ctx, gate)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("listed %d boxes, want 3", len(rows))
	}
	d := NewDispatcher()
	s := &Service{dispatcher: d, q: q}
	task := newBareTask(7, background, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Orient{Orient: &krillv1.OrientTask{Id: 7}}})
	task.orient = &orientBoxes{labelTypeID: gate, updatedAt: map[int64]pgtype.Timestamptz{}}
	for _, r := range rows {
		task.orient.updatedAt[r.ID] = r.UpdatedAt
	}
	d.add(task)

	// While the worker fits, a labeler moves one box and relabels another.
	if _, err := pool.Exec(ctx, `UPDATE annotations SET x = 0.45, updated_at = now() WHERE id = $1`, moved); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE tracks SET label_type_id = $2 WHERE id = $1`, relabeledTrack, buoy); err != nil {
		t.Fatal(err)
	}

	fitted := func(id int64) *krillv1.OrientBox {
		return &krillv1.OrientBox{AnnotationId: id, Box: &krillv1.Box{Corners: []*krillv1.Point{
			{X: 0.2, Y: 0.1}, {X: 0.3, Y: 0.2}, {X: 0.2, Y: 0.3}, {X: 0.1, Y: 0.2},
		}}}
	}
	_, err = s.ReportOrient(ctx, &krillv1.ReportOrientRequest{TaskId: 7, Boxes: []*krillv1.OrientBox{fitted(9999999)}})
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Errorf("box outside task: err = %v, want invalid argument", err)
	}
	_, err = s.ReportOrient(ctx, &krillv1.ReportOrientRequest{
		TaskId: 7, Done: true,
		Boxes: []*krillv1.OrientBox{fitted(untouched), fitted(moved), fitted(relabeled)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-task.done; err != nil {
		t.Errorf("done = %v, want nil", err)
	}

	got := map[int64][]float64{}
	var x, width float64
	for _, id := range []int64{untouched, moved, relabeled} {
		var c []float64
		if err := pool.QueryRow(ctx, `SELECT corners, x, width FROM annotations WHERE id = $1`, id).Scan(&c, &x, &width); err != nil {
			t.Fatal(err)
		}
		got[id] = c
		if id == untouched && (x != 0.1 || width < 0.2-1e-9 || width > 0.2+1e-9) {
			t.Errorf("fitted box is at x %v with width %v, want the corners' bounds", x, width)
		}
	}
	if want := []float64{0.2, 0.1, 0.3, 0.2, 0.2, 0.3, 0.1, 0.2}; !slices.Equal(got[untouched], want) {
		t.Errorf("untouched box corners = %v, want %v", got[untouched], want)
	}
	if got[moved] != nil || got[relabeled] != nil {
		t.Errorf("moved box corners = %v, relabeled box corners = %v, want both unrotated", got[moved], got[relabeled])
	}
}
