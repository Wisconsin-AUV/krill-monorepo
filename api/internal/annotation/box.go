package annotation

import (
	"errors"
	"math"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

// minSide is the smallest box side kept after clamping, about one pixel on a
// 1080p frame. Anything thinner is almost certainly a stray click.
const minSide = 0.001

// ClampBox clips a normalized box to the frame. Boxes dragged partly off the
// edge are common, so they are clipped rather than rejected.
func ClampBox(b *krillv1.Box) (x, y, w, h float64, err error) {
	if b == nil {
		return 0, 0, 0, 0, errors.New("box is required")
	}
	for _, v := range []float64{b.GetX(), b.GetY(), b.GetWidth(), b.GetHeight()} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return 0, 0, 0, 0, errors.New("box has a non-finite coordinate")
		}
	}
	x0 := math.Max(0, b.GetX())
	y0 := math.Max(0, b.GetY())
	x1 := math.Min(1, b.GetX()+b.GetWidth())
	y1 := math.Min(1, b.GetY()+b.GetHeight())
	if x1-x0 < minSide || y1-y0 < minSide {
		return 0, 0, 0, 0, errors.New("box is too small or outside the frame")
	}
	return x0, y0, x1 - x0, y1 - y0, nil
}
