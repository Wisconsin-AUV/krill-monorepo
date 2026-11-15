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

// maxCornerReach bounds how far a rotated box's corners may lie outside the
// frame, since a box clipped by the edge still has its far corners past it.
const maxCornerReach = 1

// ClampCorners is ClampBox for a box of a label type that may be oriented. A
// box of an oriented type with corners gets the corners' clipped bounds, and
// its corners flattened to x1, y1, ..., x4, y4 in clockwise order. Corners
// are dropped for other types.
func ClampCorners(b *krillv1.Box, oriented bool) (x, y, w, h float64, corners []float64, err error) {
	if b == nil || !oriented || len(b.GetCorners()) == 0 {
		x, y, w, h, err = ClampBox(b)
		return x, y, w, h, nil, err
	}
	if len(b.GetCorners()) != 4 {
		return 0, 0, 0, 0, nil, errors.New("box must have 4 corners")
	}
	corners = make([]float64, 0, 8)
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, p := range b.GetCorners() {
		px, py := p.GetX(), p.GetY()
		if math.IsNaN(px) || math.IsNaN(py) || math.IsInf(px, 0) || math.IsInf(py, 0) {
			return 0, 0, 0, 0, nil, errors.New("box has a non-finite corner")
		}
		if px < -maxCornerReach || px > 1+maxCornerReach || py < -maxCornerReach || py > 1+maxCornerReach {
			return 0, 0, 0, 0, nil, errors.New("box has a corner too far outside the frame")
		}
		corners = append(corners, px, py)
		minX, minY = math.Min(minX, px), math.Min(minY, py)
		maxX, maxY = math.Max(maxX, px), math.Max(maxY, py)
	}
	if signedArea(corners) < 0 {
		corners = []float64{corners[0], corners[1], corners[6], corners[7], corners[4], corners[5], corners[2], corners[3]}
	}
	x, y, w, h, err = ClampBox(&krillv1.Box{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY})
	if err != nil {
		return 0, 0, 0, 0, nil, err
	}
	return x, y, w, h, corners, nil
}

// signedArea is positive for corners that run clockwise on screen, where y
// points down.
func signedArea(c []float64) float64 {
	var a float64
	for i := 0; i < len(c); i += 2 {
		j := (i + 2) % len(c)
		a += c[i]*c[j+1] - c[j]*c[i+1]
	}
	return a / 2
}

// Corners converts flattened corners to points, or nil when there are none.
func Corners(c []float64) []*krillv1.Point {
	if len(c) != 8 {
		return nil
	}
	out := make([]*krillv1.Point, 4)
	for i := range out {
		out[i] = &krillv1.Point{X: c[2*i], Y: c[2*i+1]}
	}
	return out
}
