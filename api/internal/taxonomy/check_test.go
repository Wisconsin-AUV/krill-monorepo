package taxonomy

import (
	"context"
	"os"
	"slices"
	"strconv"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

func TestCheckKeypoints(t *testing.T) {
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
	scalar := func(sql string, args ...any) int64 {
		t.Helper()
		var v int64
		if err := pool.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return v
	}
	labelType := func(reference any) int64 {
		id := scalar(`INSERT INTO label_types (name, color, position, keypoints, quads, reference_key) VALUES ($1, '#fff', 99, $2, $3, $4) RETURNING id`,
			"check-test-"+uuid.NewString()[:8],
			`[{"name": "tl"}, {"name": "tr"}, {"name": "br"}, {"name": "bl"}]`,
			`[{"corners": [0, 1, 2, 3], "reference": true}]`, reference)
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, id) })
		return id
	}
	sheet, unprinted := labelType("reference.png"), labelType(nil)
	videoID := scalar(`INSERT INTO videos (name, filename, status, width, height) VALUES ('v', 'v.mp4', 'ready', 1000, 1000) RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	clipID := scalar(`INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 1) RETURNING id`, videoID)
	frameID := scalar(`INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, 0, 0) RETURNING id`, videoID, clipID)
	trackID := scalar(`INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, clipID, sheet)
	pose := func(frameID int64) {
		t.Helper()
		scalar(`INSERT INTO annotations (track_id, frame_id, x, y, width, height, keypoints, source)
			VALUES ($1, $2, 0.1, 0.1, 0.3, 0.3, '{0.1, 0.1, 2, 0.2, 0.1, 2, 0.2, 0.2, 2, 0.1, 0.2, 2}', 'human') RETURNING id`, trackID, frameID)
	}
	pose(frameID)

	s := NewService(pool, nil, jobs)
	if _, err := s.CheckKeypoints(ctx, &krillv1.CheckKeypointsRequest{LabelTypeId: unprinted}); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("type without a design: err = %v, want failed precondition", err)
	}
	res, err := s.CheckKeypoints(ctx, &krillv1.CheckKeypointsRequest{LabelTypeId: sheet})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetBoxCount() != 1 {
		t.Errorf("box count = %d, want 1", res.GetBoxCount())
	}
	if _, err := s.CheckKeypoints(ctx, &krillv1.CheckKeypointsRequest{LabelTypeId: sheet}); err != nil {
		t.Errorf("checking again while one runs: %v", err)
	}
	if n := scalar(`SELECT count(*) FROM keypoint_checks WHERE label_type_id = $1`, sheet); n != 1 {
		t.Errorf("%d checks after checking twice, want the running one", n)
	}

	find := func() *krillv1.KeypointCheckType {
		t.Helper()
		list, err := s.ListKeypointChecks(ctx, &krillv1.ListKeypointChecksRequest{})
		if err != nil {
			t.Fatal(err)
		}
		var found *krillv1.KeypointCheckType
		for _, ct := range list.GetTypes() {
			if ct.GetLabelTypeId() == unprinted && ct.GetFittable() {
				t.Error("type without a design is fittable")
			}
			if ct.GetLabelTypeId() == sheet {
				found = ct
			}
		}
		if found == nil || !found.GetFittable() {
			t.Fatalf("type with a design listed as %v, want it fittable", found)
		}
		return found
	}
	ct := find()
	if ct.GetPosedBoxCount() != 1 || ct.GetRunning() == nil || ct.GetLast() != nil {
		t.Errorf("listed %v, want one posed box and a check running", ct)
	}

	// The job is cancelled from the jobs page, so the check never finishes.
	jobID := scalar(`SELECT job_id FROM keypoint_checks WHERE label_type_id = $1`, sheet)
	if _, err := jobs.JobCancel(ctx, jobID); err != nil {
		t.Fatal(err)
	}
	ct = find()
	if ct.GetRunning() != nil || ct.GetLast().GetStatus() != krillv1.KeypointCheckStatus_KEYPOINT_CHECK_STATUS_FAILED {
		t.Errorf("after cancelling, listed %v, want the check failed", ct)
	}
	if _, err := s.CheckKeypoints(ctx, &krillv1.CheckKeypointsRequest{LabelTypeId: sheet}); err != nil {
		t.Fatal(err)
	}
	if n := scalar(`SELECT count(*) FROM keypoint_checks WHERE label_type_id = $1 AND status = 'running'`, sheet); n != 1 {
		t.Errorf("%d checks running after checking again, want 1", n)
	}
	if ct := find(); ct.GetRunning() == nil || ct.GetLast().GetError() != errCheckStopped {
		t.Errorf("listed %v, want the new check running and the cancelled one stopped", ct)
	}

	scalar(`INSERT INTO keypoint_checks (label_type_id, job_id, status, finished_at, box_count, found_count, result)
		VALUES ($1, 0, 'succeeded', now(), 1, 1, $2) RETURNING id`, sheet,
		`{"compared": 1, "median": 3.5, "p90": 3.5, "keypoints": [{"name": "tl", "count": 1, "median": 3.5, "p90": 3.5}],
		"worst": [{"annotation_id": 1, "clip_id": `+strconv.FormatInt(clipID, 10)+`, "frame_idx": 0, "error": 3.5}]}`)
	last := find().GetLast()
	if last.GetMedianError() != 3.5 || len(last.GetKeypoints()) != 1 || last.GetWorst()[0].GetVideoName() != "v" {
		t.Errorf("finished check = %v, want its result with the worst box's video named", last)
	}

	tracking := &krillv1.CheckKeypointsRequest{LabelTypeId: sheet, Kind: krillv1.KeypointCheckKind_KEYPOINT_CHECK_KIND_TRACKING}
	if res, err := s.CheckKeypoints(ctx, tracking); err != nil || res.GetBoxCount() != 0 {
		t.Errorf("tracking a track posed on one box: %v, %v, want nothing to check", res, err)
	}
	pose(scalar(`INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, 5, 0) RETURNING id`, videoID, clipID))
	if res, err := s.CheckKeypoints(ctx, tracking); err != nil || res.GetBoxCount() != 1 {
		t.Fatalf("tracking a track posed on two boxes: %v, %v, want its second box scored", res, err)
	}
	ct = find()
	if ct.GetPosedTrackCount() != 1 || ct.GetRunningTracking().GetKind() != krillv1.KeypointCheckKind_KEYPOINT_CHECK_KIND_TRACKING || ct.GetRunning() == nil {
		t.Errorf("listed %v, want a tracking check running beside the fit check", ct)
	}
	if ct.GetRunning().GetKind() != krillv1.KeypointCheckKind_KEYPOINT_CHECK_KIND_FIT {
		t.Errorf("fit check's kind = %v", ct.GetRunning().GetKind())
	}
}

func TestSampleCheckRuns(t *testing.T) {
	box := func(track int64, idx int32) db.ListHandPosedTrackBoxesRow {
		return db.ListHandPosedTrackBoxesRow{TrackID: track, FrameIdx: idx}
	}
	rows := []db.ListHandPosedTrackBoxesRow{
		box(1, 0), box(1, 9),
		box(2, 4),
		box(3, 0), box(3, 2), box(3, 19),
		box(4, 10), box(4, 14),
	}
	runs := CheckRuns(rows)
	if len(runs) != 3 || runs[0][0].TrackID != 1 || len(runs[1]) != 3 || runs[2][0].TrackID != 4 {
		t.Fatalf("runs = %v, want tracks 1, 3, and 4", runs)
	}
	frames := func(runs [][]db.ListHandPosedTrackBoxesRow) (out []int) {
		for _, r := range runs {
			out = append(out, RunFrames(r))
		}
		return out
	}
	if got := frames(SampleCheckRuns(runs, 35)); !slices.Equal(got, []int{10, 20, 5}) {
		t.Errorf("35 frames: %v, want every run", got)
	}
	// Two runs spread evenly over three are the first two, which don't fit.
	if got := frames(SampleCheckRuns(runs, 20)); !slices.Equal(got, []int{10}) {
		t.Errorf("20 frames: %v, want the first run", got)
	}
	if got := SampleCheckRuns(runs, 4); got != nil {
		t.Errorf("4 frames: %v, want none", got)
	}
}
