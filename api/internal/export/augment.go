package export

import (
	"image"
	"image/draw"
	"math"
	"math/rand/v2"

	xdraw "golang.org/x/image/draw"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

// Krill's own effects match the worker's augment.py, which trains with them.
// Its constants are BGR and these are RGB: water absorbs red first, so gains
// fall toward red and the haze is blue-green.
var (
	waterGainLow  = [3]float64{0.55, 0.95, 1.0}
	waterGainHigh = [3]float64{0.85, 1.1, 1.2}
	waterHaze     = [3]float64{35, 105, 120}
)

const (
	maxHaze      = 0.3
	maxPatchGain = 0.45
	maxPatches   = 3
	noiseSigmaLo = 2.0
	noiseSigmaHi = 10.0
	// feather is how many pixels around each pasted object blend into the
	// distractor, so its cut edge doesn't stand out as a hard line.
	feather = 4
	// Pasted objects keep at least minPasteSide pixels across, unless they
	// were smaller to begin with, and at most maxPasteShare of the frame.
	minPasteSide  = 12
	maxPasteShare = 0.9
	// A pasted object may overlap one already pasted by up to maxPasteIoU,
	// and cover up to maxPasteCover of either, so both stay recognizable.
	maxPasteIoU   = 0.1
	maxPasteCover = 0.3
	placeTries    = 20
	// shadowMapSide is the long side of the grid light and shadow are
	// computed on. The patches are blurred wider than they are, so nothing
	// finer survives anyway.
	shadowMapSide = 128

	// Ultralytics fills what a warp uncovers with this gray.
	borderGray = 114
	// Ultralytics' box_candidates: boxes this small, shrunk to this share of
	// their scaled area, or this elongated are dropped.
	minBoxPixels = 2
	minAreaRatio = 0.1
	maxBoxAspect = 100
)

// corners are a box's corners normalized to the frame, x1, y1, ..., x4, y4,
// clockwise on screen.
type corners [8]float64

type augBox struct {
	class int
	c     corners
}

func toAugBoxes(boxes []ItemBox, oriented bool) []augBox {
	out := make([]augBox, len(boxes))
	for i, b := range boxes {
		out[i].class = b.Class
		if oriented && len(b.Corners) == 8 {
			copy(out[i].c[:], b.Corners)
			continue
		}
		out[i].c = corners{b.X, b.Y, b.X + b.W, b.Y, b.X + b.W, b.Y + b.H, b.X, b.Y + b.H}
	}
	return out
}

func (c corners) bounds() (x1, y1, x2, y2 float64) {
	x1, y1 = math.Inf(1), math.Inf(1)
	x2, y2 = math.Inf(-1), math.Inf(-1)
	for k := range 4 {
		x, y := c[2*k], c[2*k+1]
		x1, x2 = min(x1, x), max(x2, x)
		y1, y2 = min(y1, y), max(y2, y)
	}
	return x1, y1, x2, y2
}

func (c corners) scaled(sx, sy float64) corners {
	var out corners
	for k := range 4 {
		out[2*k], out[2*k+1] = c[2*k]*sx, c[2*k+1]*sy
	}
	return out
}

// area is the shoelace sum, positive when the corners run clockwise on
// screen.
func (c corners) area() float64 {
	var a float64
	for k := range 4 {
		j := (k + 1) % 4
		a += c[2*k]*c[2*j+1] - c[2*j]*c[2*k+1]
	}
	return a / 2
}

// clockwise reverses corners that run counter-clockwise, keeping the first.
func (c *corners) clockwise() {
	if c.area() < 0 {
		c[2], c[3], c[6], c[7] = c[6], c[7], c[2], c[3]
	}
}

func fromAugBoxes(bs []augBox, oriented bool) []ItemBox {
	out := make([]ItemBox, len(bs))
	for i, b := range bs {
		x1, y1, x2, y2 := b.c.bounds()
		x1, y1, x2, y2 = clamp01(x1), clamp01(y1), clamp01(x2), clamp01(y2)
		out[i] = ItemBox{Class: b.class, X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
		if oriented {
			out[i].Corners = append([]float64(nil), b.c[:]...)
		}
	}
	return out
}

// Augment varies a frame and its boxes the way training would: Krill's own
// effects as the worker applies them on load, then Ultralytics' warp, color
// jitter, and flips. Boxes the warp pushes out of the frame, or that find no
// free spot on a distractor, are dropped.
// distractor returns an image to paste the objects onto, or nil when there
// are none.
func Augment(img *image.RGBA, boxes []ItemBox, oriented bool, a *krillv1.Augmentation, r *rand.Rand,
	distractor func(*rand.Rand) *image.RGBA,
) (*image.RGBA, []ItemBox) {
	bs := toAugBoxes(boxes, oriented)
	if distractor != nil && r.Float64() < float64(a.GetDistractors()) {
		if d := distractor(r); d != nil {
			img, bs = pasteObjects(img, bs, d, a, r, oriented)
		}
	}
	if a.GetAspect() > 1 {
		img = stretch(img, r, float64(a.GetAspect()))
	}
	if r.Float64() < float64(a.GetUnderwater()) {
		underwater(img, r)
	}
	if r.Float64() < float64(a.GetShadows()) {
		shadows(img, r)
	}
	if r.Float64() < float64(a.GetNoise()) {
		noise(img, r)
	}
	if a.GetDegrees() != 0 || a.GetTranslate() != 0 || a.GetScale() != 0 || a.GetShear() != 0 || a.GetPerspective() != 0 {
		m, s := randomPerspective(a, img.Rect.Dx(), img.Rect.Dy(), r)
		img, bs = transform(img, bs, m, s, oriented)
	}
	randomHSV(img, a, r)
	if r.Float64() < float64(a.GetFlipud()) {
		flipUD(img, bs)
	}
	if r.Float64() < float64(a.GetFliplr()) {
		flipLR(img, bs)
	}
	return img, fromAugBoxes(bs, oriented)
}

func uniform(r *rand.Rand, lo, hi float64) float64 {
	return lo + (hi-lo)*r.Float64()
}

func clampByte(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v)
}

func toRGBA(src image.Image) *image.RGBA {
	if m, ok := src.(*image.RGBA); ok && m.Rect.Min == (image.Point{}) {
		return m
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Rect, src, b.Min, draw.Src)
	return dst
}

// resize scales src to w by h. The approximate scaler is enough for the
// small changes in size augmentation makes, and several times faster.
func resize(src *image.RGBA, w, h int, s xdraw.Scaler) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	s.Scale(dst, dst.Rect, src, src.Rect, draw.Src, nil)
	return dst
}

// stretch squeezes one side by up to maxRatio. Labels are normalized, so
// they still fit.
func stretch(img *image.RGBA, r *rand.Rand, maxRatio float64) *image.RGBA {
	ratio := uniform(r, 1, maxRatio)
	w, h := img.Rect.Dx(), img.Rect.Dy()
	if r.Float64() < 0.5 {
		w = max(1, int(math.Round(float64(w)/ratio)))
	} else {
		h = max(1, int(math.Round(float64(h)/ratio)))
	}
	return resize(img, w, h, xdraw.ApproxBiLinear)
}

// cover scales src to cover w by h, keeping its aspect, and crops a random
// part.
func cover(src *image.RGBA, w, h int, r *rand.Rand) *image.RGBA {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	k := max(float64(w)/float64(sw), float64(h)/float64(sh))
	rw := max(w, int(math.Round(float64(sw)*k)))
	rh := max(h, int(math.Round(float64(sh)*k)))
	scaled := resize(src, rw, rh, xdraw.ApproxBiLinear)
	x, y := r.IntN(rw-w+1), r.IntN(rh-h+1)
	return scaled.SubImage(image.Rect(x, y, x+w, y+h)).(*image.RGBA)
}

// pasteObjects cuts each box out of img and pastes it, resized, rotated,
// stretched, and recolored on its own, at a random free spot on the
// distractor. Objects that find no free spot are left out.
func pasteObjects(img *image.RGBA, bs []augBox, distractor *image.RGBA, a *krillv1.Augmentation, r *rand.Rand, oriented bool) (*image.RGBA, []augBox) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	fw, fh := float64(w), float64(h)
	back := cover(distractor, w, h, r)
	out := image.NewRGBA(img.Rect)
	draw.Draw(out, out.Rect, back, back.Rect.Min, draw.Src)

	kept := make([]augBox, 0, len(bs))
	var placed []rect
	for _, i := range r.Perm(len(bs)) {
		b := bs[i]
		p := b.c.scaled(fw, fh)
		if math.Abs(p.area()) < 1 {
			continue
		}
		x1, y1, x2, y2 := p.bounds()
		cx, cy := (p[0]+p[2]+p[4]+p[6])/4, (p[1]+p[3]+p[5]+p[7])/4

		gain := uniform(r, 1-float64(a.GetScale()), 1+float64(a.GetScale()))
		sx, sy := gain, gain
		if a.GetAspect() > 1 {
			if ratio := uniform(r, 1, float64(a.GetAspect())); r.Float64() < 0.5 {
				sx /= ratio
			} else {
				sy /= ratio
			}
		}
		deg := float64(a.GetDegrees())
		rad := uniform(r, -deg, deg) * math.Pi / 180
		cos, sin := math.Cos(rad), math.Sin(rad)
		m := [4]float64{cos * sx, -sin * sy, sin * sx, cos * sy}

		var q corners
		for k := range 4 {
			dx, dy := p[2*k]-cx, p[2*k+1]-cy
			q[2*k], q[2*k+1] = m[0]*dx+m[1]*dy, m[2]*dx+m[3]*dy
		}
		qx1, qy1, qx2, qy2 := q.bounds()
		qw, qh := qx2-qx1, qy2-qy1
		k := 1.0
		if floor := min(minPasteSide, x2-x1, y2-y1); min(qw, qh) < floor {
			k = floor / min(qw, qh)
		}
		k = min(k, maxPasteShare*fw/qw, maxPasteShare*fh/qh)
		for j := range q {
			q[j] *= k
		}
		m = [4]float64{m[0] * k, m[1] * k, m[2] * k, m[3] * k}
		qx1, qy1, qw, qh = qx1*k, qy1*k, qw*k, qh*k

		var spot rect
		found := false
		for range placeTries {
			x, y := uniform(r, 0, fw-qw), uniform(r, 0, fh-qh)
			spot = rect{x, y, x + qw, y + qh}
			if !overlapsAny(spot, placed) {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		placed = append(placed, spot)
		dcx, dcy := spot.x1-qx1, spot.y1-qy1
		for j := range 4 {
			q[2*j] += dcx
			q[2*j+1] += dcy
		}

		det := m[0]*m[3] - m[1]*m[2]
		inv := [4]float64{m[3] / det, -m[1] / det, -m[2] / det, m[0] / det}
		vg := 1 + uniform(r, -1, 1)*float64(a.GetHsvV())
		sg := 1 + uniform(r, -1, 1)*float64(a.GetHsvS())
		paste(out, img, q, inv, [4]float64{cx, cy, dcx, dcy}, vg, sg)

		if !oriented {
			x1, y1, x2, y2 := q.bounds()
			q = corners{x1, y1, x2, y1, x2, y2, x1, y2}
		}
		q = q.scaled(1/fw, 1/fh)
		q.clockwise()
		kept = append(kept, augBox{class: b.class, c: q})
	}
	return out, kept
}

// paste blends the source under the destination quad q into dst, opaque
// inside q and fading over feather pixels outside it. inv maps a point,
// relative to the destination center, back to the source relative to the
// source center; at holds the source center, then the destination center.
func paste(dst, src *image.RGBA, q corners, inv, at [4]float64, vg, sg float64) {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	x1, y1, x2, y2 := q.bounds()
	area := image.Rect(int(math.Floor(x1))-feather, int(math.Floor(y1))-feather, int(math.Ceil(x2))+feather, int(math.Ceil(y2))+feather).Intersect(dst.Rect)

	// Each edge as a unit normal pointing out of q and an offset, so the
	// distance outside an edge is nx*x + ny*y + d.
	var edges [4][3]float64
	sign := 1.0
	if q.area() < 0 {
		sign = -1
	}
	for k := range 4 {
		ax, ay := q[2*k], q[2*k+1]
		bx, by := q[(2*k+2)%8], q[(2*k+3)%8]
		l := math.Hypot(bx-ax, by-ay)
		if l == 0 {
			edges[k] = [3]float64{0, 0, math.Inf(-1)}
			continue
		}
		nx, ny := sign*(by-ay)/l, -sign*(bx-ax)/l
		edges[k] = [3]float64{nx, ny, -(nx*ax + ny*ay)}
	}

	for y := area.Min.Y; y < area.Max.Y; y++ {
		py := float64(y) + 0.5
		row := dst.Pix[y*dst.Stride:]
		for x := area.Min.X; x < area.Max.X; x++ {
			px := float64(x) + 0.5
			d := math.Inf(-1)
			for _, e := range edges {
				d = max(d, e[0]*px+e[1]*py+e[2])
			}
			if d >= feather {
				continue
			}
			alpha := 1.0
			if d > 0 {
				alpha = 1 - d/feather
			}
			rx, ry := px-at[2], py-at[3]
			sx := at[0] + inv[0]*rx + inv[1]*ry - 0.5
			sy := at[1] + inv[2]*rx + inv[3]*ry - 0.5
			sx, sy = min(max(sx, 0), float64(w-1)), min(max(sy, 0), float64(h-1))
			x0, y0 := int(sx), int(sy)
			xa, ya := min(x0+1, w-1), min(y0+1, h-1)
			fx, fy := sx-float64(x0), sy-float64(y0)
			var c [3]float64
			for ch := range 3 {
				top := float64(src.Pix[y0*src.Stride+x0*4+ch])*(1-fx) + float64(src.Pix[y0*src.Stride+xa*4+ch])*fx
				bot := float64(src.Pix[ya*src.Stride+x0*4+ch])*(1-fx) + float64(src.Pix[ya*src.Stride+xa*4+ch])*fx
				c[ch] = top*(1-fy) + bot*fy
			}
			gray := 0.299*c[0] + 0.587*c[1] + 0.114*c[2]
			i := x * 4
			for ch := range 3 {
				v := (gray + (c[ch]-gray)*sg) * vg
				row[i+ch] = clampByte(v*alpha + float64(row[i+ch])*(1-alpha) + 0.5)
			}
		}
	}
}

type rect struct{ x1, y1, x2, y2 float64 }

func (a rect) area() float64 { return (a.x2 - a.x1) * (a.y2 - a.y1) }

// overlapsAny reports whether a covers too much of any of placed, or they of
// it, for both to stay recognizable.
func overlapsAny(a rect, placed []rect) bool {
	for _, b := range placed {
		iw := min(a.x2, b.x2) - max(a.x1, b.x1)
		ih := min(a.y2, b.y2) - max(a.y1, b.y1)
		if iw <= 0 || ih <= 0 {
			continue
		}
		inter := iw * ih
		if inter/(a.area()+b.area()-inter) > maxPasteIoU || inter > maxPasteCover*a.area() || inter > maxPasteCover*b.area() {
			return true
		}
	}
	return false
}

// applyLUT maps each color channel through its table.
func applyLUT(img *image.RGBA, lut *[3][256]uint8) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := range h {
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for i := 0; i < len(row); i += 4 {
			row[i] = lut[0][row[i]]
			row[i+1] = lut[1][row[i+1]]
			row[i+2] = lut[2][row[i+2]]
		}
	}
}

func underwater(img *image.RGBA, r *rand.Rand) {
	var gain [3]float64
	for c := range gain {
		gain[c] = uniform(r, waterGainLow[c], waterGainHigh[c])
	}
	haze := uniform(r, 0, maxHaze)
	var lut [3][256]uint8
	for c := range 3 {
		for v := range 256 {
			lut[c][v] = clampByte(float64(v)*gain[c]*(1-haze) + waterHaze[c]*haze)
		}
	}
	applyLUT(img, &lut)
}

// shadows adds soft blobs of light and shadow, like sun through the surface
// or the sub's own shadow.
func shadows(img *image.RGBA, r *rand.Rand) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	f := min(1, shadowMapSide/float64(max(w, h)))
	mw, mh := max(1, int(math.Ceil(float64(w)*f))), max(1, int(math.Ceil(float64(h)*f)))
	gain := make([]float32, mw*mh)
	for i := range gain {
		gain[i] = 1
	}
	patch := make([]float32, mw*mh)
	for range 1 + r.IntN(maxPatches) {
		cx, cy := uniform(r, 0, float64(w)), uniform(r, 0, float64(h))
		ax := math.Trunc(uniform(r, 0.1, 0.5) * float64(w))
		ay := math.Trunc(uniform(r, 0.1, 0.5) * float64(h))
		angle := uniform(r, 0, 180) * math.Pi / 180
		ksize := float64(int(max(ax, ay))/2*2 + 1)
		sigma := 0.3*((ksize-1)*0.5-1) + 0.8
		cos, sin := math.Cos(angle), math.Sin(angle)
		for y := range mh {
			for x := range mw {
				dx := (float64(x)+0.5)/f - cx
				dy := (float64(y)+0.5)/f - cy
				u, v := dx*cos+dy*sin, -dx*sin+dy*cos
				patch[y*mw+x] = 0
				if ax > 0 && ay > 0 && (u*u)/(ax*ax)+(v*v)/(ay*ay) <= 1 {
					patch[y*mw+x] = 1
				}
			}
		}
		// Three box blurs are close to a Gaussian.
		rad := int(math.Round((math.Sqrt(4*sigma*f*sigma*f+1) - 1) / 2))
		for range 3 {
			boxBlur(patch, mw, mh, rad)
		}
		g := float32(uniform(r, -maxPatchGain, maxPatchGain))
		for i, p := range patch {
			gain[i] += p * g
		}
	}

	row := make([]float32, mw)
	for y := range h {
		my := min(max((float64(y)+0.5)*f-0.5, 0), float64(mh-1))
		y0 := int(my)
		y1 := min(y0+1, mh-1)
		fy := float32(my - float64(y0))
		for x := range mw {
			row[x] = gain[y0*mw+x]*(1-fy) + gain[y1*mw+x]*fy
		}
		pix := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for x := range w {
			mx := min(max((float64(x)+0.5)*f-0.5, 0), float64(mw-1))
			x0 := int(mx)
			x1 := min(x0+1, mw-1)
			fx := float32(mx - float64(x0))
			g := float64(row[x0]*(1-fx) + row[x1]*fx)
			i := x * 4
			pix[i] = clampByte(float64(pix[i]) * g)
			pix[i+1] = clampByte(float64(pix[i+1]) * g)
			pix[i+2] = clampByte(float64(pix[i+2]) * g)
		}
	}
}

// boxBlur averages m over a square of radius rad, clamping at the edges.
func boxBlur(m []float32, w, h, rad int) {
	if rad < 1 {
		return
	}
	n := float32(2*rad + 1)
	tmp := make([]float32, len(m))
	for y := range h {
		var sum float32
		for k := -rad; k <= rad; k++ {
			sum += m[y*w+min(max(k, 0), w-1)]
		}
		for x := range w {
			tmp[y*w+x] = sum / n
			sum += m[y*w+min(x+rad+1, w-1)] - m[y*w+max(x-rad, 0)]
		}
	}
	for x := range w {
		var sum float32
		for k := -rad; k <= rad; k++ {
			sum += tmp[min(max(k, 0), h-1)*w+x]
		}
		for y := range h {
			m[y*w+x] = sum / n
			sum += tmp[min(y+rad+1, h-1)*w+x] - tmp[max(y-rad, 0)*w+x]
		}
	}
}

func noise(img *image.RGBA, r *rand.Rand) {
	sigma := uniform(r, noiseSigmaLo, noiseSigmaHi)
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := range h {
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for i := 0; i < len(row); i += 4 {
			for c := range 3 {
				row[i+c] = clampByte(float64(row[i+c]) + r.NormFloat64()*sigma)
			}
		}
	}
}

type mat3 [3][3]float64

func identity() mat3 {
	return mat3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
}

func (a mat3) mul(b mat3) mat3 {
	var out mat3
	for i := range 3 {
		for j := range 3 {
			for k := range 3 {
				out[i][j] += a[i][k] * b[k][j]
			}
		}
	}
	return out
}

func (a mat3) inverse() mat3 {
	det := a[0][0]*(a[1][1]*a[2][2]-a[1][2]*a[2][1]) -
		a[0][1]*(a[1][0]*a[2][2]-a[1][2]*a[2][0]) +
		a[0][2]*(a[1][0]*a[2][1]-a[1][1]*a[2][0])
	return mat3{
		{(a[1][1]*a[2][2] - a[1][2]*a[2][1]) / det, (a[0][2]*a[2][1] - a[0][1]*a[2][2]) / det, (a[0][1]*a[1][2] - a[0][2]*a[1][1]) / det},
		{(a[1][2]*a[2][0] - a[1][0]*a[2][2]) / det, (a[0][0]*a[2][2] - a[0][2]*a[2][0]) / det, (a[0][2]*a[1][0] - a[0][0]*a[1][2]) / det},
		{(a[1][0]*a[2][1] - a[1][1]*a[2][0]) / det, (a[0][1]*a[2][0] - a[0][0]*a[2][1]) / det, (a[0][0]*a[1][1] - a[0][1]*a[1][0]) / det},
	}
}

// rotation is cv2.getRotationMatrix2D about the origin.
func rotation(degrees, scale float64) mat3 {
	rad := degrees * math.Pi / 180
	a, b := scale*math.Cos(rad), scale*math.Sin(rad)
	return mat3{{a, b, 0}, {-b, a, 0}, {0, 0, 1}}
}

// randomPerspective is Ultralytics' RandomPerspective matrix for a w by h
// image, drawn in the same order, and its zoom.
func randomPerspective(a *krillv1.Augmentation, w, h int, r *rand.Rand) (mat3, float64) {
	c := identity()
	c[0][2], c[1][2] = -float64(w)/2, -float64(h)/2
	p := identity()
	persp := float64(a.GetPerspective())
	p[2][0] = uniform(r, -persp, persp)
	p[2][1] = uniform(r, -persp, persp)
	deg := float64(a.GetDegrees())
	angle := uniform(r, -deg, deg)
	scale := float64(a.GetScale())
	s := uniform(r, 1-scale, 1+scale)
	rot := rotation(angle, s)
	sh := identity()
	shear := float64(a.GetShear())
	sh[0][1] = math.Tan(uniform(r, -shear, shear) * math.Pi / 180)
	sh[1][0] = math.Tan(uniform(r, -shear, shear) * math.Pi / 180)
	t := identity()
	tr := float64(a.GetTranslate())
	t[0][2] = uniform(r, 0.5-tr, 0.5+tr) * float64(w)
	t[1][2] = uniform(r, 0.5-tr, 0.5+tr) * float64(h)
	return t.mul(sh).mul(rot).mul(p).mul(c), s
}

// transform warps the image by m onto the same size, and moves the boxes
// with it, dropping those Ultralytics would. s is the zoom in m.
func transform(img *image.RGBA, bs []augBox, m mat3, s float64, oriented bool) (*image.RGBA, []augBox) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	fw, fh := float64(w), float64(h)
	out := make([]augBox, 0, len(bs))
	for _, b := range bs {
		var p corners
		wmin, wmax := math.Inf(1), math.Inf(-1)
		for k := range 4 {
			x, y := b.c[2*k]*fw, b.c[2*k+1]*fh
			d := m[2][0]*x + m[2][1]*y + m[2][2]
			wmin, wmax = min(wmin, d), max(wmax, d)
			p[2*k] = (m[0][0]*x + m[0][1]*y + m[0][2]) / d
			p[2*k+1] = (m[1][0]*x + m[1][1]*y + m[1][2]) / d
		}
		// Corners on both sides of the projection plane flip into huge boxes.
		if wmin*wmax <= 0 {
			continue
		}
		ox1, oy1, ox2, oy2 := b.c.bounds()
		w1, h1 := (ox2-ox1)*fw*s, (oy2-oy1)*fh*s
		x1, y1, x2, y2 := p.bounds()
		x1, x2 = min(max(x1, 0), fw), min(max(x2, 0), fw)
		y1, y2 = min(max(y1, 0), fh), min(max(y2, 0), fh)
		w2, h2 := x2-x1, y2-y1
		ar := max(w2/(h2+1e-16), h2/(w2+1e-16))
		if w2 <= minBoxPixels || h2 <= minBoxPixels || w2*h2/(w1*h1+1e-16) <= minAreaRatio || ar >= maxBoxAspect {
			continue
		}
		if !oriented {
			x1, y1, x2, y2 = p.bounds()
			p = corners{x1, y1, x2, y1, x2, y2, x1, y2}
		}
		p = p.scaled(1/fw, 1/fh)
		p.clockwise()
		out = append(out, augBox{class: b.class, c: p})
	}
	return warp(img, m), out
}

// warp maps each output pixel back through m and samples bilinearly, with
// borderGray outside the source.
func warp(src *image.RGBA, m mat3) *image.RGBA {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	dst := image.NewRGBA(src.Rect)
	inv := m.inverse()
	pixel := func(x, y int) (r, g, b float64) {
		if x < 0 || y < 0 || x >= w || y >= h {
			return borderGray, borderGray, borderGray
		}
		i := y*src.Stride + x*4
		return float64(src.Pix[i]), float64(src.Pix[i+1]), float64(src.Pix[i+2])
	}
	for y := range h {
		fy := float64(y)
		row := dst.Pix[y*dst.Stride : y*dst.Stride+w*4]
		for x := range w {
			fx := float64(x)
			d := inv[2][0]*fx + inv[2][1]*fy + inv[2][2]
			sx := (inv[0][0]*fx + inv[0][1]*fy + inv[0][2]) / d
			sy := (inv[1][0]*fx + inv[1][1]*fy + inv[1][2]) / d
			i := x * 4
			row[i+3] = 255
			if sx <= -1 || sy <= -1 || sx >= float64(w) || sy >= float64(h) || math.IsNaN(sx) || math.IsNaN(sy) {
				row[i], row[i+1], row[i+2] = borderGray, borderGray, borderGray
				continue
			}
			x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
			ax, ay := sx-float64(x0), sy-float64(y0)
			if x0 >= 0 && y0 >= 0 && x0+1 < w && y0+1 < h {
				j := y0*src.Stride + x0*4
				p := src.Pix
				for c := range 3 {
					top := float64(p[j+c])*(1-ax) + float64(p[j+4+c])*ax
					bot := float64(p[j+src.Stride+c])*(1-ax) + float64(p[j+src.Stride+4+c])*ax
					row[i+c] = uint8(top*(1-ay) + bot*ay + 0.5)
				}
				continue
			}
			r00, g00, b00 := pixel(x0, y0)
			r10, g10, b10 := pixel(x0+1, y0)
			r01, g01, b01 := pixel(x0, y0+1)
			r11, g11, b11 := pixel(x0+1, y0+1)
			lerp := func(a, b, c, d float64) uint8 {
				return uint8((a*(1-ax)+b*ax)*(1-ay) + (c*(1-ax)+d*ax)*ay + 0.5)
			}
			row[i] = lerp(r00, r10, r01, r11)
			row[i+1] = lerp(g00, g10, g01, g11)
			row[i+2] = lerp(b00, b10, b01, b11)
		}
	}
	return dst
}

// randomHSV is Ultralytics' RandomHSV, with OpenCV's 8-bit HSV, where hue
// runs to 180.
func randomHSV(img *image.RGBA, a *krillv1.Augmentation, r *rand.Rand) {
	hg, sg, vg := float64(a.GetHsvH()), float64(a.GetHsvS()), float64(a.GetHsvV())
	if hg == 0 && sg == 0 && vg == 0 {
		return
	}
	rh, rs, rv := uniform(r, -1, 1)*hg, uniform(r, -1, 1)*sg, uniform(r, -1, 1)*vg
	var lutH, lutS, lutV [256]uint8
	for x := range 256 {
		fx := float64(x)
		hue := math.Mod(fx+rh*180, 180)
		if hue < 0 {
			hue += 180
		}
		lutH[x] = uint8(hue)
		lutS[x] = clampByte(fx * (rs + 1))
		lutV[x] = clampByte(fx * (rv + 1))
	}
	lutS[0] = 0

	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := range h {
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for i := 0; i < len(row); i += 4 {
			hh, ss, vv := rgbToHSV(row[i], row[i+1], row[i+2])
			row[i], row[i+1], row[i+2] = hsvToRGB(lutH[hh], lutS[ss], lutV[vv])
		}
	}
}

func rgbToHSV(r, g, b uint8) (h, s, v uint8) {
	fr, fg, fb := float64(r), float64(g), float64(b)
	hi := max(fr, fg, fb)
	diff := hi - min(fr, fg, fb)
	if hi == 0 {
		return 0, 0, 0
	}
	s = uint8(math.Round(diff * 255 / hi))
	if diff == 0 {
		return 0, s, uint8(hi)
	}
	var deg float64
	switch hi {
	case fr:
		deg = 60 * (fg - fb) / diff
	case fg:
		deg = 120 + 60*(fb-fr)/diff
	default:
		deg = 240 + 60*(fr-fg)/diff
	}
	if deg < 0 {
		deg += 360
	}
	return uint8(math.Round(deg/2)) % 180, s, uint8(hi)
}

func hsvToRGB(h, s, v uint8) (r, g, b uint8) {
	fs, fv := float64(s)/255, float64(v)
	if fs == 0 {
		return v, v, v
	}
	sector := float64(h) * 2 / 60
	i := int(sector) % 6
	f := sector - math.Floor(sector)
	p := fv * (1 - fs)
	q := fv * (1 - fs*f)
	t := fv * (1 - fs*(1-f))
	var fr, fg, fb float64
	switch i {
	case 0:
		fr, fg, fb = fv, t, p
	case 1:
		fr, fg, fb = q, fv, p
	case 2:
		fr, fg, fb = p, fv, t
	case 3:
		fr, fg, fb = p, q, fv
	case 4:
		fr, fg, fb = t, p, fv
	default:
		fr, fg, fb = fv, p, q
	}
	return clampByte(fr + 0.5), clampByte(fg + 0.5), clampByte(fb + 0.5)
}

// flipLR mirrors the image and boxes. Mirroring reverses the corners'
// winding, so they're put back in clockwise order.
func flipLR(img *image.RGBA, bs []augBox) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	for y := range h {
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for a, b := 0, (w-1)*4; a < b; a, b = a+4, b-4 {
			row[a], row[a+1], row[a+2], row[a+3], row[b], row[b+1], row[b+2], row[b+3] =
				row[b], row[b+1], row[b+2], row[b+3], row[a], row[a+1], row[a+2], row[a+3]
		}
	}
	for i := range bs {
		for j := 0; j < 8; j += 2 {
			bs[i].c[j] = 1 - bs[i].c[j]
		}
		bs[i].c.clockwise()
	}
}

func flipUD(img *image.RGBA, bs []augBox) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	tmp := make([]byte, w*4)
	for a, b := 0, h-1; a < b; a, b = a+1, b-1 {
		ra := img.Pix[a*img.Stride : a*img.Stride+w*4]
		rb := img.Pix[b*img.Stride : b*img.Stride+w*4]
		copy(tmp, ra)
		copy(ra, rb)
		copy(rb, tmp)
	}
	for i := range bs {
		for j := 1; j < 8; j += 2 {
			bs[i].c[j] = 1 - bs[i].c[j]
		}
		bs[i].c.clockwise()
	}
}
