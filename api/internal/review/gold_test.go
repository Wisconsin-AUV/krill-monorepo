package review

import "testing"

func TestGoldRoutingBelow(t *testing.T) {
	g := GoldRouting{Threshold: 0.7, MinAttempts: 3}
	tests := []struct {
		name     string
		attempts int32
		score    float64
		want     bool
	}{
		{"below threshold", 5, 0.5, true},
		{"at threshold", 5, 0.7, false},
		{"above threshold", 5, 0.9, false},
		{"too few attempts", 2, 0, false},
		{"exactly the minimum", 3, 0.6, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := g.Below(tt.attempts, tt.score); got != tt.want {
				t.Errorf("Below(%d, %v) = %v, want %v", tt.attempts, tt.score, got, tt.want)
			}
		})
	}
}
