package annotation

import "math"

// A tracked box that overlaps the track's previous box less than this, or
// grows or shrinks by more than this factor, has likely slid off its object.
const (
	minDriftIoU       = 0.5
	maxDriftAreaRatio = 1.5
)

// DuplicateIoU is the overlap above which two boxes of a frame are taken to
// be the same object.
const DuplicateIoU = 0.5

type Rect struct {
	X, Y, W, H float64
}

func (r Rect) area() float64 { return r.W * r.H }

func iou(a, b Rect) float64 {
	w := math.Min(a.X+a.W, b.X+b.W) - math.Max(a.X, b.X)
	h := math.Min(a.Y+a.H, b.Y+b.H) - math.Max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	overlap := w * h
	return overlap / (a.area() + b.area() - overlap)
}

// Drifted reports whether next jumped too far from prev, the track's box on
// the frame before it.
func Drifted(prev, next Rect) bool {
	ratio := next.area() / prev.area()
	return iou(prev, next) < minDriftIoU || ratio > maxDriftAreaRatio || ratio < 1/maxDriftAreaRatio
}
