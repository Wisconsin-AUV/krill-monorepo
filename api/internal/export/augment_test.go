package export

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"slices"
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func testRand() *rand.Rand {
	return rand.New(rand.NewPCG(1, 2)) //nolint:gosec // repeatable, not secret
}

func solid(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
	}
	return img
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

var (
	black = color.RGBA{A: 255}
	white = color.RGBA{255, 255, 255, 255}
	red   = color.RGBA{200, 0, 0, 255}
	blue  = color.RGBA{0, 0, 200, 255}
)

func near(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if math.Abs(a[i]-b[i]) > 1e-9 {
			return false
		}
	}
	return true
}

func clockwiseOnScreen(c []float64) bool {
	var a float64
	for i := 0; i < 8; i += 2 {
		j := (i + 2) % 8
		a += c[i]*c[j+1] - c[j]*c[i+1]
	}
	return a > 0
}

// rotate90 turns an image a quarter counter-clockwise about its center, as a
// positive Ultralytics angle does.
func rotate90(w, h int) mat3 {
	c := identity()
	c[0][2], c[1][2] = -float64(w)/2, -float64(h)/2
	t := identity()
	t[0][2], t[1][2] = float64(w)/2, float64(h)/2
	return t.mul(rotation(90, 1)).mul(c)
}

func TestTransformRotatesBoxes(t *testing.T) {
	img := solid(100, 100, black)
	fill(img, image.Rect(10, 20, 30, 40), white)
	box := ItemBox{Class: 3, X: 0.1, Y: 0.2, W: 0.2, H: 0.2}

	for _, oriented := range []bool{false, true} {
		out, bs := transform(img, toAugBoxes([]ItemBox{box}, oriented), rotate90(100, 100), 1, oriented)
		got := fromAugBoxes(bs, oriented)
		if len(got) != 1 {
			t.Fatalf("oriented %v: %d boxes, want 1", oriented, len(got))
		}
		b := got[0]
		if b.Class != 3 || !near([]float64{b.X, b.Y, b.W, b.H}, []float64{0.2, 0.7, 0.2, 0.2}) {
			t.Errorf("oriented %v: box = %+v, want x 0.2 y 0.7 w 0.2 h 0.2", oriented, b)
		}
		if oriented {
			// Top left, top right, bottom right, and bottom left before, so
			// bottom left, top left, top right, and bottom right after.
			want := []float64{0.2, 0.9, 0.2, 0.7, 0.4, 0.7, 0.4, 0.9}
			if !near(b.Corners, want) || !clockwiseOnScreen(b.Corners) {
				t.Errorf("corners = %v, want %v", b.Corners, want)
			}
		}
		// The pixel at (20, 30) lands at (30, 80).
		if c := out.RGBAAt(30, 80); c != white {
			t.Errorf("oriented %v: rotated object pixel = %v", oriented, c)
		}
		if c := out.RGBAAt(20, 30); c != black {
			t.Errorf("oriented %v: old object pixel = %v", oriented, c)
		}
	}
}

func TestTransformDropsBoxesOffFrame(t *testing.T) {
	img := solid(100, 100, black)
	shift := identity()
	shift[0][2] = 80
	boxes := []ItemBox{
		{Class: 0, X: 0.2, Y: 0.4, W: 0.2, H: 0.2},  // to x 100-120, gone
		{Class: 1, X: 0.18, Y: 0.4, W: 0.2, H: 0.2}, // to x 98-118, 2 pixels left
		{Class: 2, X: 0.1, Y: 0.4, W: 0.2, H: 0.2},  // to x 90-110, half left
	}
	out, bs := transform(img, toAugBoxes(boxes, false), shift, 1, false)
	got := fromAugBoxes(bs, false)
	if len(got) != 1 || got[0].Class != 2 {
		t.Fatalf("kept %+v, want only class 2", got)
	}
	if !near([]float64{got[0].X, got[0].W}, []float64{0.9, 0.1}) {
		t.Errorf("kept box = %+v, want clipped to x 0.9 w 0.1", got[0])
	}
	if c := out.RGBAAt(10, 50); c != (color.RGBA{borderGray, borderGray, borderGray, 255}) {
		t.Errorf("uncovered pixel = %v, want gray", c)
	}
}

func TestAugmentFlipLR(t *testing.T) {
	img := solid(100, 100, black)
	fill(img, image.Rect(10, 20, 30, 40), white)
	boxes := []ItemBox{{Class: 0, X: 0.1, Y: 0.2, W: 0.2, H: 0.2, Corners: []float64{0.1, 0.2, 0.3, 0.2, 0.3, 0.4, 0.1, 0.4}}}
	a := &krillv1.Augmentation{Fliplr: 1, Aspect: 1}
	out, got := Augment(img, boxes, true, nil, a, testRand(), nil)
	if len(got) != 1 {
		t.Fatalf("%d boxes, want 1", len(got))
	}
	b := got[0]
	if !near([]float64{b.X, b.Y, b.W, b.H}, []float64{0.7, 0.2, 0.2, 0.2}) {
		t.Errorf("box = %+v, want x 0.7", b)
	}
	if want := []float64{0.9, 0.2, 0.9, 0.4, 0.7, 0.4, 0.7, 0.2}; !near(b.Corners, want) || !clockwiseOnScreen(b.Corners) {
		t.Errorf("corners = %v, want %v", b.Corners, want)
	}
	if out.RGBAAt(84, 25) != white || out.RGBAAt(15, 25) != black {
		t.Errorf("image not mirrored")
	}
}

// inQuad reports whether (x, y) is inside the convex quad c, scaled to w by h.
func inQuad(c []float64, w, h int, x, y float64) bool {
	pos, neg := false, false
	for k := range 4 {
		ax, ay := c[2*k]*float64(w), c[2*k+1]*float64(h)
		bx, by := c[(2*k+2)%8]*float64(w), c[(2*k+3)%8]*float64(h)
		cross := (bx-ax)*(y-ay) - (by-ay)*(x-ax)
		pos = pos || cross > 0
		neg = neg || cross < 0
	}
	return !pos || !neg
}

// toward is the pixel a share of the way from corner k of c to its center.
func toward(c []float64, w, h, k int, share float64) image.Point {
	cx, cy := (c[0]+c[2]+c[4]+c[6])/4, (c[1]+c[3]+c[5]+c[7])/4
	x := c[2*k] + (cx-c[2*k])*share
	y := c[2*k+1] + (cy-c[2*k+1])*share
	return image.Pt(int(x*float64(w)), int(y*float64(h)))
}

// pasteBoxes pastes like Augment, without the whole-frame edits that the same
// settings would also turn on.
func pasteBoxes(img *image.RGBA, boxes []ItemBox, oriented bool, r *rand.Rand, distractor *image.RGBA) (*image.RGBA, []ItemBox) {
	out, bs := pasteObjects(img, toAugBoxes(boxes, oriented), distractor, r, oriented)
	return out, fromAugBoxes(bs, oriented)
}

func TestPastesOrientedObjects(t *testing.T) {
	const w, h = 240, 180
	sq := []float64{0.30, 0.20, 0.45, 0.30, 0.37, 0.50, 0.22, 0.40}
	img := solid(w, h, blue)
	for y := range h {
		for x := range w {
			px, py := float64(x)+0.5, float64(y)+0.5
			if inQuad(sq, w, h, px, py) {
				img.SetRGBA(x, y, red)
				// Marks corner 1, which a mirror would swap with corner 3.
				if math.Hypot(px-sq[2]*w, py-sq[3]*h) < 20 {
					img.SetRGBA(x, y, white)
				}
			}
		}
	}
	green := color.RGBA{0, 200, 0, 255}
	r := testRand()
	for range 30 {
		out, got := pasteBoxes(img, []ItemBox{{Class: 2, Corners: sq}}, true, r, solid(64, 64, green))
		if len(got) != 1 {
			t.Fatalf("%d boxes, want 1", len(got))
		}
		c := got[0].Corners
		if got[0].Class != 2 || !clockwiseOnScreen(c) {
			t.Fatalf("box = %+v, want class 2 clockwise", got[0])
		}
		// The warp after pasting resizes and rotates the whole frame, so
		// pasting only moves objects.
		for k := 1; k < 4; k++ {
			if !near([]float64{c[2*k] - c[0], c[2*k+1] - c[1]}, []float64{sq[2*k] - sq[0], sq[2*k+1] - sq[1]}) {
				t.Fatalf("pasted %v, want %v moved", c, sq)
			}
		}
		for k, want := range map[int]color.RGBA{0: red, 1: white, 2: red, 3: red} {
			p := toward(c, w, h, k, 0.25)
			if got := out.RGBAAt(p.X, p.Y); got != want {
				t.Errorf("near corner %d of %v: %v, want %v", k, c, got, want)
			}
		}
		// Everything well outside the label is the distractor.
		for y := 0; y < h; y += 7 {
			for x := 0; x < w; x += 7 {
				px, py := float64(x)+0.5, float64(y)+0.5
				near := false
				for dy := -8.0; dy <= 8 && !near; dy += 4 {
					for dx := -8.0; dx <= 8 && !near; dx += 4 {
						near = inQuad(c, w, h, px+dx, py+dy)
					}
				}
				if !near && out.RGBAAt(x, y) != green {
					t.Fatalf("pixel (%d, %d) outside %v = %v, want green", x, y, c, out.RGBAAt(x, y))
				}
			}
		}
	}
}

func TestAugmentPasteSkipsOverlaps(t *testing.T) {
	img := solid(200, 200, red)
	var boxes []ItemBox
	for i := range 12 {
		boxes = append(boxes, ItemBox{Class: i, X: 0.05 * float64(i%4), Y: 0.05 * float64(i/4), W: 0.35, H: 0.35})
	}
	distractor := func(*rand.Rand) *image.RGBA { return solid(64, 64, blue) }
	a := &krillv1.Augmentation{Distractors: 1, Aspect: 1}
	r := testRand()
	for range 10 {
		out, got := Augment(img, boxes, false, nil, a, r, distractor)
		if len(got) == 0 || len(got) == len(boxes) {
			t.Fatalf("%d of %d boxes placed", len(got), len(boxes))
		}
		for i, b := range got {
			if c := out.RGBAAt(int((b.X+b.W/2)*200), int((b.Y+b.H/2)*200)); c != red {
				t.Errorf("center of %+v = %v, want the pasted object", b, c)
			}
			for _, o := range got[i+1:] {
				iw := min(b.X+b.W, o.X+o.W) - max(b.X, o.X)
				ih := min(b.Y+b.H, o.Y+o.H) - max(b.Y, o.Y)
				if iw <= 0 || ih <= 0 {
					continue
				}
				inter, ab, ao := iw*ih, b.W*b.H, o.W*o.H
				if inter/(ab+ao-inter) > maxPasteIoU+1e-9 || inter > maxPasteCover*min(ab, ao)+1e-9 {
					t.Errorf("%+v and %+v overlap by %.3f", b, o, inter)
				}
			}
		}
	}
}

func TestAugmentPasteLeavesInputs(t *testing.T) {
	img := solid(100, 80, red)
	fill(img, image.Rect(30, 20, 70, 60), white)
	boxes := []ItemBox{{Class: 1, X: 0.3, Y: 0.25, W: 0.4, H: 0.5, Corners: []float64{0.3, 0.25, 0.7, 0.25, 0.7, 0.75, 0.3, 0.75}}}
	back := solid(50, 50, blue)
	imgPix, backPix := slices.Clone(img.Pix), slices.Clone(back.Pix)
	corners := slices.Clone(boxes[0].Corners)

	if _, got := pasteBoxes(img, boxes, true, testRand(), back); len(got) != 1 {
		t.Errorf("%d boxes, want 1", len(got))
	}
	if !slices.Equal(img.Pix, imgPix) || !slices.Equal(back.Pix, backPix) || !slices.Equal(boxes[0].Corners, corners) {
		t.Error("inputs changed")
	}

	distractor := func(*rand.Rand) *image.RGBA { return back }
	out, got := Augment(img, nil, false, nil, &krillv1.Augmentation{Distractors: 1, Aspect: 1}, testRand(), distractor)
	if len(got) != 0 || out.Rect.Dx() != 100 || out.Rect.Dy() != 80 {
		t.Fatalf("frame without boxes: %d boxes, size %v", len(got), out.Rect)
	}
	for y := range 80 {
		for x := range 100 {
			if c := out.RGBAAt(x, y); c != blue {
				t.Fatalf("frame without boxes: (%d, %d) = %v, want the distractor", x, y, c)
			}
		}
	}
}

func TestOverlappingShadowsStayWithinThePatchGain(t *testing.T) {
	const v = 200
	lo, hi := uint8(v*(1-maxPatchGain)), uint8(min(255, math.Round(v*(1+maxPatchGain))))
	r := testRand()
	for range 200 {
		img := solid(60, 40, color.RGBA{v, v, v, 255})
		shadows(img, r)
		for i, c := range img.Pix {
			if i%4 != 3 && (c < lo || c > hi) {
				t.Fatalf("pixel channel %d = %d, want between %d and %d", i, c, lo, hi)
			}
		}
	}
}

func TestAugmentFlipLRSwapsKeypoints(t *testing.T) {
	img := solid(100, 100, black)
	// Left and right corners of a sheet, and its center, which mirrors onto
	// itself.
	boxes := []ItemBox{{Class: 0, X: 0.1, Y: 0.2, W: 0.4, H: 0.2, Keypoints: []float64{0.1, 0.3, 2, 0.5, 0.3, 1, 0, 0, 0}}}
	a := &krillv1.Augmentation{Fliplr: 1, Aspect: 1}
	_, got := Augment(img, boxes, false, []int{1, 0, 2}, a, testRand(), nil)
	if len(got) != 1 {
		t.Fatalf("%d boxes, want 1", len(got))
	}
	if want := []float64{0.5, 0.3, 1, 0.9, 0.3, 2, 0, 0, 0}; !near(got[0].Keypoints, want) {
		t.Errorf("keypoints = %v, want %v", got[0].Keypoints, want)
	}
}

func TestTransformMovesKeypoints(t *testing.T) {
	img := solid(100, 100, black)
	box := ItemBox{Class: 0, X: 0.1, Y: 0.2, W: 0.2, H: 0.2, Keypoints: []float64{0.1, 0.2, 2, 0.3, 0.4, 1}}
	_, bs := transform(img, toAugBoxes([]ItemBox{box}, false), rotate90(100, 100), 1, false)
	got := fromAugBoxes(bs, false)
	if want := []float64{0.2, 0.9, 2, 0.4, 0.7, 1}; len(got) != 1 || !near(got[0].Keypoints, want) {
		t.Errorf("rotated keypoints = %+v, want %v", got, want)
	}

	// Half the box and one keypoint leave the frame.
	shift := identity()
	shift[0][2] = 80
	box = ItemBox{Class: 0, X: 0.1, Y: 0.4, W: 0.2, H: 0.2, Keypoints: []float64{0.15, 0.5, 2, 0.25, 0.5, 2}}
	_, bs = transform(img, toAugBoxes([]ItemBox{box}, false), shift, 1, false)
	got = fromAugBoxes(bs, false)
	if want := []float64{0.95, 0.5, 2, 0, 0, 0}; len(got) != 1 || !near(got[0].Keypoints, want) {
		t.Errorf("shifted keypoints = %+v, want %v", got, want)
	}
}
