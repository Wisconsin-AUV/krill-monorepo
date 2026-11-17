package gold

import (
	"math"
	"testing"
)

func box(typ int64, x, y, w, h float64, attrs map[string]string) Box {
	return Box{LabelTypeID: typ, Attributes: attrs, X: x, Y: y, Width: w, Height: h}
}

func TestScore(t *testing.T) {
	red := map[string]string{"color": "red"}
	white := map[string]string{"color": "white"}
	tests := []struct {
		name        string
		reference   []Box
		answer      []Box
		wantCorrect int
		wantMatches int
		wantScore   float64
	}{
		{
			name:      "empty frame answered empty",
			wantScore: 1,
		},
		{
			name:        "exact box",
			reference:   []Box{box(1, 0.1, 0.1, 0.2, 0.2, nil)},
			answer:      []Box{box(1, 0.1, 0.1, 0.2, 0.2, map[string]string{})},
			wantCorrect: 1, wantMatches: 1, wantScore: 1,
		},
		{
			name:      "same place but different type",
			reference: []Box{box(1, 0.1, 0.1, 0.2, 0.2, nil)},
			answer:    []Box{box(2, 0.1, 0.1, 0.2, 0.2, nil)},
			wantScore: 0,
		},
		{
			// IoU 0.6: the same object, but too loose to count.
			name:        "loose box",
			reference:   []Box{box(1, 0, 0, 0.3, 0.2, nil)},
			answer:      []Box{box(1, 0, 0, 0.5, 0.2, nil)},
			wantMatches: 1, wantScore: 0,
		},
		{
			name:        "wrong attribute",
			reference:   []Box{box(1, 0, 0, 0.2, 0.2, red)},
			answer:      []Box{box(1, 0, 0, 0.2, 0.2, white)},
			wantMatches: 1, wantScore: 0,
		},
		{
			name:        "missed and extra boxes both count against the score",
			reference:   []Box{box(1, 0, 0, 0.1, 0.1, nil), box(1, 0.5, 0.5, 0.1, 0.1, nil)},
			answer:      []Box{box(1, 0, 0, 0.1, 0.1, nil), box(1, 0.8, 0.8, 0.1, 0.1, nil)},
			wantCorrect: 1, wantMatches: 1, wantScore: 1.0 / 3,
		},
		{
			// A box overlapping two references goes to the closer one, so
			// the other reference can still match its own box.
			name: "each box matches its best pair",
			reference: []Box{
				box(1, 0, 0, 0.2, 0.2, nil),
				box(1, 0.04, 0, 0.2, 0.2, nil),
			},
			answer: []Box{
				box(1, 0.05, 0, 0.2, 0.2, nil),
				box(1, 0, 0, 0.2, 0.2, nil),
			},
			wantCorrect: 2, wantMatches: 2, wantScore: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score(tt.reference, tt.answer, Frame{Width: 1000, Height: 500})
			if got.Correct != tt.wantCorrect || len(got.Matches) != tt.wantMatches || math.Abs(got.Score-tt.wantScore) > 1e-9 {
				t.Errorf("correct %d, matches %d, score %v; want %d, %d, %v",
					got.Correct, len(got.Matches), got.Score, tt.wantCorrect, tt.wantMatches, tt.wantScore)
			}
		})
	}
}

func TestScoreKeypoints(t *testing.T) {
	// A 200 by 100 pixel box on a 1000 by 500 frame, whose OKS falloff is
	// exp(-d^2 / 1024) for d pixels.
	ref := Box{LabelTypeID: 1, X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2, Keypoints: []float64{0.15, 0.15, 2, 0.25, 0.15, 1, 0, 0, 0}}
	withKeypoints := func(kps ...float64) Box {
		b := ref
		b.Keypoints = kps
		return b
	}
	frame := Frame{Width: 1000, Height: 500}
	tests := []struct {
		name        string
		answer      Box
		wantOKS     float64
		wantCorrect bool
	}{
		{"exact", withKeypoints(0.15, 0.15, 2, 0.25, 0.15, 2, 0.3, 0.3, 2), 1, true},
		// 10 pixels off on one, which is exp(-100/1024) = 0.907, and the
		// reference's unlabeled third is ignored.
		{"close", withKeypoints(0.16, 0.15, 2, 0.25, 0.15, 1, 0, 0, 0), (math.Exp(-100.0/1024) + 1) / 2, true},
		{"one missing", withKeypoints(0.15, 0.15, 2), 0.5, false},
		{"none", withKeypoints(), 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Score([]Box{ref}, []Box{tt.answer}, frame)
			if len(got.Matches) != 1 {
				t.Fatalf("matches = %d, want 1", len(got.Matches))
			}
			m := got.Matches[0]
			if m.OKS == nil || math.Abs(*m.OKS-tt.wantOKS) > 1e-9 || m.Correct != tt.wantCorrect {
				t.Errorf("oks %v, correct %v; want %v, %v", m.OKS, m.Correct, tt.wantOKS, tt.wantCorrect)
			}
		})
	}

	got := Score([]Box{withKeypoints()}, []Box{ref}, frame)
	if m := got.Matches[0]; m.OKS != nil || !m.Correct {
		t.Errorf("reference without keypoints: oks %v, correct %v; want unset and correct", m.OKS, m.Correct)
	}
}
