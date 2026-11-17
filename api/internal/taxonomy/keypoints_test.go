package taxonomy

import (
	"context"
	"math"
	"os"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

func idx(i int32) *int32 { return &i }

func keypoints(names ...string) []Keypoint {
	out := make([]Keypoint, len(names))
	for i, n := range names {
		out[i] = Keypoint{Name: n}
	}
	return out
}

func TestValidatePose(t *testing.T) {
	sheet := func() Pose {
		return Pose{
			Keypoints: keypoints("tl", "tr", "br", "bl", "center"),
			Quads:     []Quad{{Corners: []int32{0, 1, 2, 3}, Reference: true, Center: idx(4)}},
		}
	}
	with := func(f func(*Pose)) Pose {
		p := sheet()
		f(&p)
		return p
	}
	tests := []struct {
		name     string
		pose     Pose
		oriented bool
		wantErr  bool
	}{
		{"sheet", sheet(), false, false},
		{"none on an oriented type", Pose{}, true, false},
		{"keypoints on an oriented type", sheet(), true, true},
		{"unnamed", with(func(p *Pose) { p.Keypoints[1].Name = "" }), false, true},
		{"same name twice", with(func(p *Pose) { p.Keypoints[1].Name = "tl" }), false, true},
		{"mirrors", with(func(p *Pose) { p.Keypoints[0].Mirror, p.Keypoints[1].Mirror = idx(1), idx(0) }), false, false},
		{"mirror one way", with(func(p *Pose) { p.Keypoints[0].Mirror = idx(1) }), false, true},
		{"mirror out of range", with(func(p *Pose) { p.Keypoints[0].Mirror = idx(5) }), false, true},
		{"three corners", with(func(p *Pose) { p.Quads[0].Corners = []int32{0, 1, 2} }), false, true},
		{"repeated corner", with(func(p *Pose) { p.Quads[0].Corners = []int32{0, 1, 2, 0} }), false, true},
		{"corner out of range", with(func(p *Pose) { p.Quads[0].Corners[3] = 9 }), false, true},
		{"center is a corner", with(func(p *Pose) { p.Quads[0].Center = idx(2) }), false, true},
		{"two reference quads", with(func(p *Pose) { p.Quads = append(p.Quads, Quad{Corners: []int32{4, 3, 2, 1}, Reference: true}) }), false, true},
		{"edge to itself", with(func(p *Pose) { p.Edges = []Edge{{From: 1, To: 1}} }), false, true},
		{"edge out of range", with(func(p *Pose) { p.Edges = []Edge{{From: 1, To: 7}} }), false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.pose.validate(tt.oriented); (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	self := Pose{Keypoints: []Keypoint{{Name: "a", Mirror: idx(0)}}}
	if err := self.validate(false); err != nil || self.Keypoints[0].Mirror != nil {
		t.Errorf("mirror onto itself: err = %v, mirror = %v, want it unset", err, self.Keypoints[0].Mirror)
	}
}

func TestRekeyed(t *testing.T) {
	old := Pose{Keypoints: keypoints("a", "b", "c")}
	tests := []struct {
		name string
		new  []string
		want bool
	}{
		{"unchanged", []string{"a", "b", "c"}, false},
		{"renamed", []string{"a", "x", "c"}, false},
		{"appended", []string{"a", "b", "c", "d"}, false},
		{"removed last", []string{"a", "b"}, true},
		{"removed middle", []string{"a", "c"}, true},
		{"swapped", []string{"b", "a", "c"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (Pose{Keypoints: keypoints(tt.new...)}).Rekeyed(old); got != tt.want {
				t.Errorf("Rekeyed = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetCenters(t *testing.T) {
	p := Pose{Keypoints: keypoints("tl", "tr", "br", "bl", "c"), Quads: []Quad{{Corners: []int32{0, 1, 2, 3}, Center: idx(4)}}}
	// A sheet seen in perspective, wider at the bottom, so its center is
	// above the middle of its corners' bounds.
	kps := []float64{0.4, 0.2, 2, 0.6, 0.2, 2, 0.8, 0.6, 1, 0.2, 0.6, 2, 0.9, 0.9, 2}
	p.SetCenters(kps)
	// The diagonals (0.4, 0.2)-(0.8, 0.6) and (0.6, 0.2)-(0.2, 0.6) cross
	// at (0.5, 0.3).
	if got := kps[12:]; math.Abs(got[0]-0.5) > 1e-9 || math.Abs(got[1]-0.3) > 1e-9 || got[2] != Visible {
		t.Errorf("center = %v, want 0.5 0.3 visible", got)
	}

	for i := range 4 {
		kps[3*i+2] = Hidden
	}
	p.SetCenters(kps)
	if kps[14] != Hidden {
		t.Errorf("center of hidden corners = %v, want hidden", kps[14])
	}

	kps[2] = Unlabeled
	p.SetCenters(kps)
	if got := kps[12:]; !slices.Equal(got, []float64{0, 0, Unlabeled}) {
		t.Errorf("center with an unlabeled corner = %v, want unlabeled", got)
	}
}

func TestKeypointEditsKeepLabeledBoxes(t *testing.T) {
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
	s := NewService(pool, nil, nil)
	scalar := func(sql string, args ...any) int64 {
		t.Helper()
		var v int64
		if err := pool.QueryRow(ctx, sql, args...).Scan(&v); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		return v
	}
	labelType := func(kps string) (int64, string) {
		name := "kp_test_" + uuid.NewString()[:8]
		id := scalar(`INSERT INTO label_types (name, color, position, keypoints) VALUES ($1, '#ffffff', 1000, $2) RETURNING id`, name, kps)
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM tracks WHERE label_type_id = $1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, id)
		})
		return id, name
	}
	bin, binName := labelType(`[{"name": "left"}, {"name": "right"}]`)
	other, _ := labelType(`[{"name": "top"}, {"name": "bottom"}]`)
	videoID := scalar(`INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'ready') RETURNING id`)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	clipID := scalar(`INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 1) RETURNING id`, videoID)
	frameID := scalar(`INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, 0, 0) RETURNING id`, videoID, clipID)
	trackID := scalar(`INSERT INTO tracks (clip_id, label_type_id) VALUES ($1, $2) RETURNING id`, clipID, bin)
	scalar(`INSERT INTO annotations (track_id, frame_id, x, y, width, height, keypoints)
		VALUES ($1, $2, 0.1, 0.1, 0.2, 0.2, '{0.1, 0.1, 2, 0, 0, 0}') RETURNING id`, trackID, frameID)

	update := func(names ...string) error {
		req := &krillv1.UpdateLabelTypeRequest{Id: bin, Name: binName, Color: "#ffffff"}
		for _, n := range names {
			req.Keypoints = append(req.Keypoints, &krillv1.KeypointDef{Name: n})
		}
		_, err := s.UpdateLabelType(ctx, req)
		return err
	}
	if err := update("right", "left"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("reorder: err = %v, want failed precondition", err)
	}
	if err := update("left"); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("remove: err = %v, want failed precondition", err)
	}
	if err := update("port", "starboard", "handle"); err != nil {
		t.Errorf("rename and append: err = %v", err)
	}

	_, err = s.MergeLabelTypes(ctx, &krillv1.MergeLabelTypesRequest{SourceId: bin, TargetId: other})
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("merge into different keypoints: err = %v, want failed precondition", err)
	}
	if _, err := s.MergeLabelTypes(ctx, &krillv1.MergeLabelTypesRequest{SourceId: other, TargetId: bin}); err != nil {
		t.Errorf("merge a type with none labeled: err = %v", err)
	}
}
