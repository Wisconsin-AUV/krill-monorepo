package tracker

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/taxonomy"
)

// keypointFixture is a clip of three frames and a label type whose four
// keypoints are a quad.
type keypointFixture struct {
	pool      *pgxpool.Pool
	q         *db.Queries
	labelType int64
	clip      int64
	frames    []int64
}

func newKeypointFixture(t *testing.T) *keypointFixture {
	t.Helper()
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	f := &keypointFixture{pool: pool, q: db.New(pool)}
	f.labelType = f.scalar(t, `INSERT INTO label_types (name, color, position, keypoints, quads) VALUES ($1, '#fff', 99, $2, $3) RETURNING id`,
		"kp-test-"+uuid.NewString()[:8],
		`[{"name": "tl"}, {"name": "tr"}, {"name": "br"}, {"name": "bl"}]`,
		`[{"corners": [0, 1, 2, 3], "reference": true}]`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, f.labelType) })
	videoID := f.scalar(t, `INSERT INTO videos (name, filename, status, width, height) VALUES ('v', 'v.mp4', 'ready', 1000, 1000) RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	f.clip = f.scalar(t, `INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 3) RETURNING id`, videoID)
	for i := range 3 {
		f.frames = append(f.frames, f.scalar(t, `INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, $3, 0) RETURNING id`, videoID, f.clip, i))
	}
	return f
}

func (f *keypointFixture) scalar(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var v int64
	if err := f.pool.QueryRow(t.Context(), sql, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func (f *keypointFixture) keypoints(t *testing.T, id int64) (kps []float64, status, source string) {
	t.Helper()
	err := f.pool.QueryRow(t.Context(), `SELECT keypoints, status, source FROM annotations WHERE id = $1`, id).Scan(&kps, &status, &source)
	if err != nil {
		t.Fatal(err)
	}
	return kps, status, source
}

func quad(x, y float64) []*krillv1.Keypoint {
	v := krillv1.KeypointVisibility_KEYPOINT_VISIBILITY_VISIBLE
	return []*krillv1.Keypoint{
		{X: x, Y: y, Visibility: v},
		{X: x + 0.1, Y: y, Visibility: v},
		{X: x + 0.1, Y: y + 0.1, Visibility: v},
		{X: x, Y: y + 0.1, Visibility: v},
	}
}

func TestReportTrackMergesPromptFrameKeypoints(t *testing.T) {
	f := newKeypointFixture(t)
	ctx := t.Context()
	track := f.scalar(t, `INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, f.clip, f.labelType)
	// The labeler drew the box and placed its top left keypoint, then asked
	// SAM to fill the quad.
	prompt := f.scalar(t, `INSERT INTO annotations (track_id, frame_id, x, y, width, height, keypoints, status, source)
		VALUES ($1, $2, 0.1, 0.1, 0.3, 0.3, '{0.11, 0.1, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0}', 'verified', 'human') RETURNING id`, track, f.frames[0])

	d := NewDispatcher()
	s := &Service{pool: f.pool, dispatcher: d, q: f.q}
	frames := make([]*krillv1.TaskFrame, len(f.frames))
	for i, id := range f.frames {
		frames[i] = &krillv1.TaskFrame{Id: id}
	}
	task := newTask(9, track, &krillv1.TrackTask{
		Id: 9, Frames: frames, KeypointCount: 4,
		Quads: []*krillv1.KeypointQuad{{Corners: []int32{0, 1, 2, 3}, Reference: true}},
	})
	d.add(task)
	boxes := []*krillv1.TrackedBox{
		{FrameId: f.frames[0], Box: &krillv1.Box{X: 0.12, Y: 0.1, Width: 0.25, Height: 0.25, Keypoints: quad(0.1, 0.1)}},
		{FrameId: f.frames[1], Box: &krillv1.Box{X: 0.12, Y: 0.1, Width: 0.3, Height: 0.3, Keypoints: quad(0.1, 0.1)}},
		// The quad jumped across the box.
		{FrameId: f.frames[2], Box: &krillv1.Box{X: 0.12, Y: 0.1, Width: 0.3, Height: 0.3, Keypoints: quad(0.3, 0.3)}},
	}
	if _, err := s.ReportTrack(ctx, &krillv1.ReportTrackRequest{TaskId: 9, Model: "sam", Done: true, Boxes: boxes}); err != nil {
		t.Fatal(err)
	}
	if err := <-task.done; err != nil {
		t.Fatal(err)
	}

	kps, status, source := f.keypoints(t, prompt)
	if want := []float64{0.11, 0.1, 2, 0.2, 0.1, 2, 0.2, 0.2, 2, 0.1, 0.2, 2}; !slices.Equal(kps, want) || status != "proposed" || source != "sam" {
		t.Errorf("prompt frame = %v %s %s, want %v proposed by sam", kps, status, source, want)
	}
	var x, width float64
	if err := f.pool.QueryRow(ctx, `SELECT x, width FROM annotations WHERE id = $1`, prompt).Scan(&x, &width); err != nil {
		t.Fatal(err)
	}
	if x != 0.1 || width != 0.3 {
		t.Errorf("prompt box moved to x %v width %v, want the labeler's", x, width)
	}
	var drift []string
	rows, err := f.pool.Query(ctx, `SELECT a.drift FROM annotations a JOIN frames fr ON fr.id = a.frame_id
		WHERE a.track_id = $1 AND fr.idx > 0 ORDER BY fr.idx`, track)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var d pgtype.Text
		if err := rows.Scan(&d); err != nil {
			t.Fatal(err)
		}
		drift = append(drift, d.String)
	}
	if want := []string{"ok", "flagged"}; !slices.Equal(drift, want) {
		t.Errorf("drift = %v, want %v", drift, want)
	}

	// Reporting the same keypoints again changes nothing, so a labeler's
	// accepted box stays accepted.
	if _, err := f.pool.Exec(ctx, `UPDATE annotations SET status = 'verified', source = 'human' WHERE id = $1`, prompt); err != nil {
		t.Fatal(err)
	}
	task = newTask(10, track, &krillv1.TrackTask{Id: 10, Frames: frames[:1], KeypointCount: 4})
	d.add(task)
	if _, err := s.ReportTrack(ctx, &krillv1.ReportTrackRequest{TaskId: 10, Model: "sam", Done: true, Boxes: boxes[:1]}); err != nil {
		t.Fatal(err)
	}
	if _, status, _ := f.keypoints(t, prompt); status != "verified" {
		t.Errorf("status after an unchanged report = %s, want verified", status)
	}
}

func TestReportOrientFitsKeypointsOnlyOnUnposedBoxes(t *testing.T) {
	f := newKeypointFixture(t)
	ctx := t.Context()
	box := func(kps any) int64 {
		track := f.scalar(t, `INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, f.clip, f.labelType)
		return f.scalar(t, `INSERT INTO annotations (track_id, frame_id, x, y, width, height, keypoints, status)
			VALUES ($1, $2, 0.1, 0.1, 0.3, 0.3, $3, 'verified') RETURNING id`, track, f.frames[0], kps)
	}
	unposed := box(nil)
	labeledMeanwhile := box(nil)
	rows, err := f.q.ListUnposedBoxes(ctx, f.labelType)
	if err != nil {
		t.Fatal(err)
	}
	pose := taxonomy.Pose{Keypoints: make([]taxonomy.Keypoint, 4)}
	d := NewDispatcher()
	s := &Service{dispatcher: d, q: f.q}
	task := newBareTask(11, background, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Orient{Orient: &krillv1.OrientTask{Id: 11, KeypointCount: 4}}})
	task.orient = &orientBoxes{labelTypeID: f.labelType, pose: &pose, updatedAt: map[int64]pgtype.Timestamptz{}}
	for _, r := range rows {
		task.orient.updatedAt[r.ID] = r.UpdatedAt
	}
	d.add(task)
	if _, err := f.pool.Exec(ctx, `UPDATE annotations SET keypoints = '{0.2, 0.2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0}' WHERE id = $1`, labeledMeanwhile); err != nil {
		t.Fatal(err)
	}
	_, err = s.ReportOrient(ctx, &krillv1.ReportOrientRequest{TaskId: 11, Done: true, Boxes: []*krillv1.OrientBox{
		{AnnotationId: unposed, Box: &krillv1.Box{Keypoints: quad(0.1, 0.1)}},
		{AnnotationId: labeledMeanwhile, Box: &krillv1.Box{Keypoints: quad(0.1, 0.1)}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-task.done; err != nil {
		t.Fatal(err)
	}
	if kps, status, _ := f.keypoints(t, unposed); len(kps) != 12 || status != "proposed" {
		t.Errorf("unposed box = %v %s, want fitted keypoints proposed", kps, status)
	}
	if kps, status, _ := f.keypoints(t, labeledMeanwhile); kps[0] != 0.2 || status != "verified" {
		t.Errorf("box labeled meanwhile = %v %s, want the labeler's keypoints", kps, status)
	}
}
