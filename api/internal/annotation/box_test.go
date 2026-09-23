package annotation

import (
	"math"
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func TestClampBox(t *testing.T) {
	tests := []struct {
		name    string
		in      *krillv1.Box
		want    [4]float64
		wantErr bool
	}{
		{"inside", &krillv1.Box{X: 0.1, Y: 0.2, Width: 0.3, Height: 0.4}, [4]float64{0.1, 0.2, 0.3, 0.4}, false},
		{"past right edge", &krillv1.Box{X: 0.8, Y: 0, Width: 0.5, Height: 0.5}, [4]float64{0.8, 0, 0.2, 0.5}, false},
		{"negative origin", &krillv1.Box{X: -0.1, Y: -0.2, Width: 0.3, Height: 0.4}, [4]float64{0, 0, 0.2, 0.2}, false},
		{"fully outside", &krillv1.Box{X: 1.2, Y: 0, Width: 0.1, Height: 0.1}, [4]float64{}, true},
		{"zero width", &krillv1.Box{X: 0.5, Y: 0.5, Width: 0, Height: 0.1}, [4]float64{}, true},
		{"nan", &krillv1.Box{X: math.NaN(), Y: 0, Width: 0.1, Height: 0.1}, [4]float64{}, true},
		{"nil", nil, [4]float64{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, w, h, err := clampBox(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			got := [4]float64{x, y, w, h}
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > 1e-9 {
					t.Fatalf("clampBox = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
