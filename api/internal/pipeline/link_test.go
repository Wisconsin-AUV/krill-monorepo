package pipeline

import (
	"slices"
	"testing"

	"github.com/wauv/krill/api/internal/annotation"
)

func det(frame int32, target int, x float64) detection {
	return detection{
		frameID:  int64(frame) + 100,
		frameIdx: frame,
		box:      annotation.Box{X: x, Y: 0.2, Width: 0.2, Height: 0.2},
		target:   target,
	}
}

func TestLinkTracks(t *testing.T) {
	tests := []struct {
		name string
		dets []detection
		want [][]int
	}{
		{
			name: "same object on consecutive frames",
			dets: []detection{det(0, 0, 0.1), det(1, 0, 0.12), det(2, 0, 0.14)},
			want: [][]int{{0, 1, 2}},
		},
		{
			name: "another class in the same place",
			dets: []detection{det(0, 0, 0.1), det(1, 1, 0.1)},
			want: [][]int{{0}, {1}},
		},
		{
			name: "a missed frame or two",
			dets: []detection{det(0, 0, 0.1), det(3, 0, 0.1)},
			want: [][]int{{0, 1}},
		},
		{
			name: "gone too long",
			dets: []detection{det(0, 0, 0.1), det(4, 0, 0.1)},
			want: [][]int{{0}, {1}},
		},
		{
			name: "two objects keep their own tracks",
			dets: []detection{det(0, 0, 0.1), det(0, 0, 0.6), det(1, 0, 0.62), det(1, 0, 0.11)},
			want: [][]int{{0, 3}, {1, 2}},
		},
		{
			name: "unsorted input",
			dets: []detection{det(2, 0, 0.14), det(0, 0, 0.1), det(1, 0, 0.12)},
			want: [][]int{{1, 2, 0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := linkTracks(tt.dets)
			if !slices.EqualFunc(got, tt.want, slices.Equal) {
				t.Errorf("linkTracks = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDropCoveredKeepsBoxesOffExistingOnes(t *testing.T) {
	existing := map[int64][]annotation.Box{100: {{X: 0.1, Y: 0.2, Width: 0.2, Height: 0.2}}}
	got := dropCovered([]detection{det(0, 0, 0.11), det(0, 0, 0.6), det(1, 0, 0.11)}, existing)
	if len(got) != 2 || got[0].box.X != 0.6 || got[1].frameIdx != 1 {
		t.Errorf("dropCovered = %+v, want the far box and the other frame's", got)
	}
}
