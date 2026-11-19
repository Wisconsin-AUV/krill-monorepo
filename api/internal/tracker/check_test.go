package tracker

import (
	"math"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/taxonomy"
)

// square is a quad's keypoints, clockwise from (x, y), with each corner
// labeled visible.
func square(x, y, side float64) []float64 {
	return []float64{x, y, 2, x + side, y, 2, x + side, y + side, 2, x, y + side, 2}
}

func TestKeypointErrors(t *testing.T) {
	hand := []float64{0.1, 0.1, 2, 0.2, 0.1, 1, 0, 0, 0}
	fitted := []float64{0.1, 0.13, 2, 0.2, 0.1, 2, 0.3, 0.3, 2}
	got := keypointErrors(hand, fitted, 4, 1000, 500)
	if math.Abs(got[0]-15) > 1e-9 || got[1] != 0 || !math.IsNaN(got[2]) || !math.IsNaN(got[3]) {
		t.Errorf("errors = %v, want [15 0 NaN NaN]", got)
	}
}

func TestTurnedQuad(t *testing.T) {
	corners := []int32{0, 1, 2, 3}
	hand := square(0.1, 0.1, 0.1)
	// The fit put the design's top left on the labeler's top right, a
	// quarter turn.
	turned := []float64{0.1, 0.2, 2, 0.1, 0.1, 2, 0.2, 0.1, 2, 0.2, 0.2, 2}
	shifted := square(0.12, 0.11, 0.1)
	hidden := slices.Clone(turned)
	hidden[5], hidden[8] = 0, 0
	for _, tc := range []struct {
		name   string
		fitted []float64
		want   bool
	}{
		{"matching", square(0.101, 0.1, 0.1), false},
		{"turned", turned, true},
		{"shifted", shifted, false},
		{"two corners", hidden, false},
	} {
		if got := turnedQuad(hand, tc.fitted, corners, 1000, 1000); got != tc.want {
			t.Errorf("%s: turned = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestMedianP90(t *testing.T) {
	median, p90 := medianP90([]float64{5, 1, 3, 2, 4, 10, 6, 7, 8, 9, 0})
	if median != 5 || p90 != 9 {
		t.Errorf("median, p90 = %v, %v, want 5, 9", median, p90)
	}
	median, p90 = medianP90([]float64{1, 2})
	if median != 1.5 || math.Abs(p90-1.9) > 1e-9 {
		t.Errorf("median, p90 of two = %v, %v, want 1.5, 1.9", median, p90)
	}
	if median, p90 := medianP90(nil); median != 0 || p90 != 0 {
		t.Errorf("median, p90 of none = %v, %v, want 0, 0", median, p90)
	}
}

func TestSampleEvenly(t *testing.T) {
	rows := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	if got := sampleEvenly(rows, 4); !slices.Equal(got, []int{0, 2, 5, 7}) {
		t.Errorf("4 of 10 = %v", got)
	}
	if got := sampleEvenly(rows, 20); !slices.Equal(got, rows) {
		t.Errorf("20 of 10 = %v", got)
	}
}

func TestReportOrientChecksKeypointsWithoutSavingThem(t *testing.T) {
	f := newKeypointFixture(t)
	ctx := t.Context()
	box := func(frame int64, kps []float64, source string) int64 {
		track := f.scalar(t, `INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, f.clip, f.labelType)
		return f.scalar(t, `INSERT INTO annotations (track_id, frame_id, x, y, width, height, keypoints, status, source)
			VALUES ($1, $2, 0.1, 0.1, 0.3, 0.3, $3, 'verified', $4) RETURNING id`, track, frame, kps, source)
	}
	near := box(f.frames[0], square(0.1, 0.1, 0.1), "human")
	off := box(f.frames[1], square(0.1, 0.1, 0.1), "human")
	turned := box(f.frames[1], square(0.4, 0.4, 0.1), "human")
	box(f.frames[2], square(0.1, 0.1, 0.1), "human")
	box(f.frames[2], square(0.1, 0.1, 0.1), "sam")
	box(f.frames[2], nil, "human")

	rows, err := f.q.ListHandPosedBoxes(ctx, f.labelType)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("listed %d boxes, want the 4 posed by hand", len(rows))
	}
	pose := taxonomy.Pose{
		Keypoints: []taxonomy.Keypoint{{Name: "tl"}, {Name: "tr"}, {Name: "br"}, {Name: "bl"}},
		Quads:     []taxonomy.Quad{{Corners: []int32{0, 1, 2, 3}, Reference: true}},
	}
	check := newKeypointCheck(pose, rows)
	d := NewDispatcher()
	s := &Service{dispatcher: d, q: f.q}
	task := newBareTask(12, background, &krillv1.NextTaskResponse{Task: &krillv1.NextTaskResponse_Orient{Orient: &krillv1.OrientTask{Id: 12, KeypointCount: 4}}})
	task.orient = &orientBoxes{labelTypeID: f.labelType, pose: &pose, updatedAt: map[int64]pgtype.Timestamptz{}, check: check}
	for _, r := range rows {
		task.orient.updatedAt[r.ID] = r.UpdatedAt
	}
	d.add(task)
	v := krillv1.KeypointVisibility_KEYPOINT_VISIBILITY_VISIBLE
	turnedQuad := []*krillv1.Keypoint{{X: 0.4, Y: 0.5, Visibility: v}, {X: 0.4, Y: 0.4, Visibility: v}, {X: 0.5, Y: 0.4, Visibility: v}, {X: 0.5, Y: 0.5, Visibility: v}}
	_, err = s.ReportOrient(ctx, &krillv1.ReportOrientRequest{TaskId: 12, Done: true, Boxes: []*krillv1.OrientBox{
		{AnnotationId: near, Box: &krillv1.Box{Keypoints: quad(0.102, 0.1)}},
		{AnnotationId: off, Box: &krillv1.Box{Keypoints: quad(0.13, 0.14)}},
		{AnnotationId: turned, Box: &krillv1.Box{Keypoints: turnedQuad}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-task.done; err != nil {
		t.Fatal(err)
	}

	for _, r := range rows {
		if kps, status, source := f.keypoints(t, r.ID); !slices.Equal(kps, r.Keypoints) || status != "verified" || source != "human" {
			t.Errorf("box %d = %v %s %s, want the labeler's keypoints", r.ID, kps, status, source)
		}
	}

	found, nTurned, res := check.result()
	if found != 3 || nTurned != 1 || res.Compared != 3 {
		t.Errorf("found %d, turned %d, compared %d, want 3, 1, 3", found, nTurned, res.Compared)
	}
	// The video is 1000 pixels square: near is 2 px off at every corner,
	// off 50 px, and turned's corners are each a side, 100 px, from the
	// labeler's.
	if math.Abs(res.Median-50) > 1e-6 || math.Abs(res.P90-90) > 1e-6 {
		t.Errorf("median, p90 = %v, %v, want 50, 90", res.Median, res.P90)
	}
	if len(res.Keypoints) != 4 || res.Keypoints[0].Name != "tl" || res.Keypoints[0].Count != 3 {
		t.Errorf("keypoints = %+v, want all four compared on 3 boxes", res.Keypoints)
	}
	if len(res.Worst) != 3 || res.Worst[0].AnnotationID != turned || !res.Worst[0].Turned || res.Worst[0].ClipID != f.clip || res.Worst[0].FrameIdx != 1 {
		t.Errorf("worst = %+v, want the turned box first", res.Worst)
	}
}
