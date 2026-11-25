package tracker

import (
	"cmp"
	"math"
	"slices"
	"sync"

	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/taxonomy"
)

// checkWorst is how many of the boxes farthest off a check keeps.
const checkWorst = 10

// keypointCheck collects the keypoints the worker fits to boxes labelers
// posed, across all of a check's tasks.
type keypointCheck struct {
	pose taxonomy.Pose
	mu   sync.Mutex
	// boxes are the posed boxes sent to the worker.
	boxes []db.ListHandPosedBoxesRow
	// fitted are the keypoints the worker found, by annotation.
	fitted map[int64][]float64
}

func newKeypointCheck(pose taxonomy.Pose, boxes []db.ListHandPosedBoxesRow) *keypointCheck {
	return &keypointCheck{pose: pose, boxes: boxes, fitted: map[int64][]float64{}}
}

func (c *keypointCheck) add(annotationID int64, keypoints []float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fitted[annotationID] = keypoints
}

// result compares the fitted keypoints with the labelers'. Boxes the worker
// didn't report are ones it didn't find the design in.
func (c *keypointCheck) result() (found, turned int, res taxonomy.CheckResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	var boxes []scoredBox
	for _, b := range c.boxes {
		fitted, ok := c.fitted[b.ID]
		if !ok {
			continue
		}
		found++
		w, h := float64(b.VideoWidth), float64(b.VideoHeight)
		box := taxonomy.CheckedBox{AnnotationID: b.ID, ClipID: b.ClipID, FrameIdx: b.FrameIdx}
		for _, q := range c.pose.Quads {
			if q.Reference && turnedQuad(b.Keypoints, fitted, q.Corners, w, h) {
				box.Turned = true
			}
		}
		if box.Turned {
			turned++
		}
		boxes = append(boxes, scoredBox{box: box, hand: b.Keypoints, placed: fitted, w: w, h: h})
	}
	return found, turned, score(c.pose, boxes)
}

// scoredBox is a box posed by hand with the keypoints a check placed on it,
// normalized to a w by h frame.
type scoredBox struct {
	box          taxonomy.CheckedBox
	hand, placed []float64
	w, h         float64
}

// score is how far the placed keypoints are from the labelers', over boxes
// with a keypoint labeled in both.
func score(pose taxonomy.Pose, boxes []scoredBox) (res taxonomy.CheckResult) {
	perKeypoint := make([][]float64, pose.Count())
	var boxErrors []float64
	var compared []taxonomy.CheckedBox
	for _, b := range boxes {
		box := b.box
		n := 0
		for i, e := range keypointErrors(b.hand, b.placed, pose.Count(), b.w, b.h) {
			if math.IsNaN(e) {
				continue
			}
			perKeypoint[i] = append(perKeypoint[i], e)
			box.Error = max(box.Error, e)
			n++
		}
		if n == 0 {
			continue
		}
		boxErrors = append(boxErrors, box.Error)
		compared = append(compared, box)
	}
	res.Compared = len(compared)
	res.Median, res.P90 = medianP90(boxErrors)
	res.Keypoints = []taxonomy.KeypointError{}
	for i, errs := range perKeypoint {
		if len(errs) == 0 {
			continue
		}
		e := taxonomy.KeypointError{Name: pose.Keypoints[i].Name, Count: len(errs)}
		e.Median, e.P90 = medianP90(errs)
		res.Keypoints = append(res.Keypoints, e)
	}
	slices.SortStableFunc(compared, func(a, b taxonomy.CheckedBox) int { return cmp.Compare(b.Error, a.Error) })
	res.Worst = compared[:min(len(compared), checkWorst)]
	return res
}

// keypointErrors is the distance in pixels between each of count keypoints
// in hand and fitted, which are x, y, visibility triples normalized to a w
// by h frame. It's NaN for keypoints either leaves unlabeled.
func keypointErrors(hand, fitted []float64, count int, w, h float64) []float64 {
	out := make([]float64, count)
	for i := range out {
		out[i] = math.NaN()
		if labeled(hand, i) && labeled(fitted, i) {
			out[i] = pixelDistance(hand, i, fitted, i, w, h)
		}
	}
	return out
}

// turnedQuad reports whether turning the fitted quad's corners a quarter or
// half turn brings them clearly closer to the labeled ones, which happens
// when the fit takes the design for upside down or sideways.
func turnedQuad(hand, fitted []float64, corners []int32, w, h float64) bool {
	if len(corners) != 4 {
		return false
	}
	// worst is the largest distance with fitted turned by k corners, and n
	// how many corners it covers.
	worst := func(k int) (worst float64, n int) {
		for i, c := range corners {
			f := corners[(i+k)%4]
			if labeled(hand, int(c)) && labeled(fitted, int(f)) {
				worst = max(worst, pixelDistance(hand, int(c), fitted, int(f), w, h))
				n++
			}
		}
		return worst, n
	}
	// Two corners can match a turned quad by chance.
	as, n := worst(0)
	if n < 3 {
		return false
	}
	for k := 1; k < 4; k++ {
		if turned, n := worst(k); n >= 3 && 2*turned < as {
			return true
		}
	}
	return false
}

func labeled(kps []float64, i int) bool {
	return 3*i+2 < len(kps) && kps[3*i+2] > taxonomy.Unlabeled
}

func pixelDistance(a []float64, i int, b []float64, j int, w, h float64) float64 {
	return math.Hypot((a[3*i]-b[3*j])*w, (a[3*i+1]-b[3*j+1])*h)
}

// medianP90 returns the median and 90th percentile of vals, interpolating
// between neighbors.
func medianP90(vals []float64) (median, p90 float64) {
	if len(vals) == 0 {
		return 0, 0
	}
	sorted := slices.Sorted(slices.Values(vals))
	return percentile(sorted, 0.5), percentile(sorted, 0.9)
}

func percentile(sorted []float64, p float64) float64 {
	r := p * float64(len(sorted)-1)
	lo := int(r)
	if lo+1 >= len(sorted) {
		return sorted[lo]
	}
	return sorted[lo] + (r-float64(lo))*(sorted[lo+1]-sorted[lo])
}

// sampleEvenly picks n of rows evenly, keeping their order.
func sampleEvenly[T any](rows []T, n int) []T {
	if len(rows) <= n {
		return rows
	}
	out := make([]T, n)
	for i := range out {
		out[i] = rows[i*len(rows)/n]
	}
	return out
}
