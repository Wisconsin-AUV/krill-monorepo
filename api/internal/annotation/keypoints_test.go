package annotation

import (
	"math"
	"slices"
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/taxonomy"
)

const (
	unlabeled = krillv1.KeypointVisibility_KEYPOINT_VISIBILITY_UNSPECIFIED
	hidden    = krillv1.KeypointVisibility_KEYPOINT_VISIBILITY_HIDDEN
	visible   = krillv1.KeypointVisibility_KEYPOINT_VISIBILITY_VISIBLE
)

// sheet is a quad of keypoints 0 to 3 centered on keypoint 4.
func sheet() taxonomy.Pose {
	center := int32(4)
	return taxonomy.Pose{
		Keypoints: make([]taxonomy.Keypoint, 5),
		Quads:     []taxonomy.Quad{{Corners: []int32{0, 1, 2, 3}, Center: &center}},
	}
}

func TestClampKeypoints(t *testing.T) {
	kp := func(x, y float64, v krillv1.KeypointVisibility) *krillv1.Keypoint {
		return &krillv1.Keypoint{X: x, Y: y, Visibility: v}
	}
	square := []*krillv1.Keypoint{kp(0.1, 0.1, visible), kp(0.3, 0.1, visible), kp(0.3, 0.3, hidden), kp(0.1, 0.3, visible)}
	tests := []struct {
		name    string
		in      []*krillv1.Keypoint
		pose    taxonomy.Pose
		want    []float64
		wantErr bool
	}{
		{"none", nil, sheet(), nil, false},
		{
			"center is placed, whatever the client sent", append(slices.Clone(square), kp(0.9, 0.9, hidden)), sheet(),
			[]float64{0.1, 0.1, 2, 0.3, 0.1, 2, 0.3, 0.3, 1, 0.1, 0.3, 2, 0.2, 0.2, 2},
			false,
		},
		{
			"far outside is clamped and unlabeled is zeroed",
			[]*krillv1.Keypoint{kp(-3, 0.5, hidden), kp(0.2, 2.5, visible), kp(0.4, 0.4, unlabeled), kp(0.1, 0.1, unlabeled), kp(0, 0, unlabeled)},
			sheet(),
			[]float64{-1, 0.5, 1, 0.2, 2, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			false,
		},
		{"all unlabeled", []*krillv1.Keypoint{kp(0.4, 0.4, unlabeled), {}, {}, {}, {}}, sheet(), nil, false},
		{"wrong count", square, sheet(), nil, true},
		{"type without keypoints", square, taxonomy.Pose{}, nil, true},
		{"nan", []*krillv1.Keypoint{kp(math.NaN(), 0, visible), {}, {}, {}, {}}, sheet(), nil, true},
		{"unknown visibility", []*krillv1.Keypoint{kp(0.1, 0.1, 3), {}, {}, {}, {}}, sheet(), nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClampKeypoints(tt.in, tt.pose)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("keypoints = %v, want %v", got, tt.want)
			}
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > 1e-9 {
					t.Fatalf("keypoints = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestMergeKeypoints(t *testing.T) {
	pose := sheet()
	// A labeler placed the top left corner where SAM put it elsewhere.
	old := []float64{0.12, 0.1, 2}
	found := []float64{0.1, 0.1, 2, 0.3, 0.1, 2, 0.3, 0.3, 2, 0.1, 0.3, 2, 0.2, 0.2, 2}
	want := []float64{0.12, 0.1, 2, 0.3, 0.1, 2, 0.3, 0.3, 2, 0.1, 0.3, 2, 0, 0, 2}
	got := MergeKeypoints(old, found, pose, nil)
	if len(got) != len(want) {
		t.Fatalf("merged = %v, want %v", got, want)
	}
	for i := range 12 {
		if got[i] != want[i] {
			t.Fatalf("merged = %v, want the labeler's corner and SAM's others", got)
		}
	}
	if got[14] != taxonomy.Visible {
		t.Errorf("center = %v, want placed", got[12:])
	}
	if again := MergeKeypoints(got, found, pose, nil); again != nil {
		t.Errorf("merging nothing new = %v, want nil", again)
	}
	// Filling the quad again replaces the corner the labeler had placed.
	refilled := MergeKeypoints(got, found, pose, []int32{0, 1, 2, 3})
	if refilled == nil || refilled[0] != found[0] {
		t.Errorf("refilled = %v, want SAM's top left corner", refilled)
	}
}

func TestKeypointsDrifted(t *testing.T) {
	// 100 by 100 pixels on a 1000 by 1000 frame, which OKS forgives about
	// 30 pixels of.
	box := Rect{X: 0.1, Y: 0.1, W: 0.1, H: 0.1}
	prev := []float64{0.12, 0.12, 2, 0.18, 0.12, 2, 0, 0, 0}
	if KeypointsDrifted(prev, []float64{0.125, 0.12, 2, 0.18, 0.125, 2, 0.5, 0.5, 2}, box, 1000, 1000) {
		t.Error("keypoints a few pixels off drifted")
	}
	if !KeypointsDrifted(prev, []float64{0.18, 0.12, 2, 0.12, 0.12, 2}, box, 1000, 1000) {
		t.Error("swapped keypoints didn't drift")
	}
	if KeypointsDrifted(nil, []float64{0.5, 0.5, 2}, box, 1000, 1000) {
		t.Error("a box with no keypoints before drifted")
	}
}
