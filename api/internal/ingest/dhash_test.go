package ingest

import (
	"image"
	"image/color"
	"math/bits"
	"testing"
)

func gradient(w, h int, reverse bool, noise uint8) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8(x * 255 / (w - 1)) //nolint:gosec // x < w, so v <= 255
			if reverse {
				v = 255 - v
			}
			if (x+y)%7 == 0 && v < 255-noise {
				v += noise
			}
			img.SetGray(x, y, color.Gray{Y: v})
		}
	}
	return img
}

func TestDHash(t *testing.T) {
	base := DHash(gradient(320, 240, false, 0))

	tests := []struct {
		name    string
		img     image.Image
		maxDist int
		minDist int
	}{
		{"identical", gradient(320, 240, false, 0), 0, 0},
		{"slight noise", gradient(320, 240, false, 3), 4, 0},
		{"different size", gradient(640, 480, false, 0), 4, 0},
		{"reversed", gradient(320, 240, true, 0), 64, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := bits.OnesCount64(base ^ DHash(tt.img))
			if d > tt.maxDist || d < tt.minDist {
				t.Errorf("distance = %d, want between %d and %d", d, tt.minDist, tt.maxDist)
			}
		})
	}
}
