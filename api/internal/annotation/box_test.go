package annotation

import (
	"math"
	"slices"
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
			x, y, w, h, err := ClampBox(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			got := [4]float64{x, y, w, h}
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > 1e-9 {
					t.Fatalf("ClampBox = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestClampCorners(t *testing.T) {
	pts := func(c ...float64) []*krillv1.Point {
		out := make([]*krillv1.Point, 0, len(c)/2)
		for i := 0; i+1 < len(c); i += 2 {
			out = append(out, &krillv1.Point{X: c[i], Y: c[i+1]})
		}
		return out
	}
	// A diamond poking past the left edge, with x/y/w/h the client got wrong.
	diamond := pts(0.1, -0.1, 0.3, 0.1, 0.1, 0.3, -0.1, 0.1)
	tests := []struct {
		name        string
		in          *krillv1.Box
		oriented    bool
		wantBox     [4]float64
		wantCorners []float64
		wantErr     bool
	}{
		{
			"bounds come from corners", &krillv1.Box{X: 0.5, Y: 0.5, Width: 0.1, Height: 0.1, Corners: diamond}, true,
			[4]float64{0, 0, 0.3, 0.3},
			[]float64{0.1, -0.1, 0.3, 0.1, 0.1, 0.3, -0.1, 0.1},
			false,
		},
		{
			"counter-clockwise is reversed", &krillv1.Box{Corners: pts(0.1, 0.1, 0.1, 0.3, 0.3, 0.3, 0.3, 0.1)}, true,
			[4]float64{0.1, 0.1, 0.2, 0.2},
			[]float64{0.1, 0.1, 0.3, 0.1, 0.3, 0.3, 0.1, 0.3},
			false,
		},
		{
			"dropped for other types", &krillv1.Box{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2, Corners: diamond}, false,
			[4]float64{0.1, 0.1, 0.2, 0.2},
			nil, false,
		},
		{"unrotated", &krillv1.Box{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2}, true, [4]float64{0.1, 0.1, 0.2, 0.2}, nil, false},
		{"three corners", &krillv1.Box{Corners: pts(0, 0, 0.2, 0, 0.2, 0.2)}, true, [4]float64{}, nil, true},
		{"nan corner", &krillv1.Box{Corners: pts(0, 0, math.NaN(), 0, 0.2, 0.2, 0, 0.2)}, true, [4]float64{}, nil, true},
		{"far outside", &krillv1.Box{Corners: pts(0, 0, 2.5, 0, 2.5, 0.2, 0, 0.2)}, true, [4]float64{}, nil, true},
		{"entirely off the frame", &krillv1.Box{Corners: pts(1.1, 0, 1.3, 0, 1.3, 0.2, 1.1, 0.2)}, true, [4]float64{}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			x, y, w, h, corners, err := ClampCorners(tt.in, tt.oriented)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			got := [4]float64{x, y, w, h}
			for i := range got {
				if math.Abs(got[i]-tt.wantBox[i]) > 1e-9 {
					t.Fatalf("box = %v, want %v", got, tt.wantBox)
				}
			}
			if !slices.Equal(corners, tt.wantCorners) {
				t.Errorf("corners = %v, want %v", corners, tt.wantCorners)
			}
		})
	}
}
