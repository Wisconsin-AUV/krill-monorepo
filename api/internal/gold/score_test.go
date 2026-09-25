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
			got := Score(tt.reference, tt.answer)
			if got.Correct != tt.wantCorrect || len(got.Matches) != tt.wantMatches || math.Abs(got.Score-tt.wantScore) > 1e-9 {
				t.Errorf("correct %d, matches %d, score %v; want %d, %d, %v",
					got.Correct, len(got.Matches), got.Score, tt.wantCorrect, tt.wantMatches, tt.wantScore)
			}
		})
	}
}
