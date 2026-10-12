package annotation

import "testing"

func TestDrifted(t *testing.T) {
	prev := Rect{X: 0.1, Y: 0.1, W: 0.2, H: 0.2}
	tests := []struct {
		name string
		next Rect
		want bool
	}{
		{"same box", prev, false},
		{"shifted, IoU 0.54", Rect{X: 0.16, Y: 0.1, W: 0.2, H: 0.2}, false},
		{"shifted, IoU 0.48", Rect{X: 0.17, Y: 0.1, W: 0.2, H: 0.2}, true},
		{"disjoint", Rect{X: 0.6, Y: 0.6, W: 0.2, H: 0.2}, true},
		{"grew 1.45x", Rect{X: 0.1, Y: 0.1, W: 0.29, H: 0.2}, false},
		{"grew 1.55x", Rect{X: 0.1, Y: 0.1, W: 0.31, H: 0.2}, true},
		{"shrank to 0.7x", Rect{X: 0.1, Y: 0.1, W: 0.14, H: 0.2}, false},
		{"shrank to 0.65x", Rect{X: 0.1, Y: 0.1, W: 0.13, H: 0.2}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Drifted(prev, tt.next); got != tt.want {
				t.Errorf("Drifted = %v, want %v", got, tt.want)
			}
		})
	}
}
