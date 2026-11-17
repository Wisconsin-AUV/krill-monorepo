package annotation

import (
	"errors"
	"fmt"
	"math"
	"slices"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/taxonomy"
)

// keypointSigma is COCO's per-keypoint OKS falloff, the same for every
// keypoint since Krill's types have no measured spread.
const keypointSigma = 0.08

// MinCorrectOKS is the keypoint similarity a matched box needs for its
// keypoints to count as right.
const MinCorrectOKS = 0.75

// ClampKeypoints checks a box's keypoints against its label type's and
// flattens them to x, y, visibility triples, with coordinates clamped to
// where keypoints may lie, unlabeled ones at 0, 0, and quad centers placed
// where their corners' diagonals cross. It returns nil when none is labeled.
func ClampKeypoints(kps []*krillv1.Keypoint, pose taxonomy.Pose) ([]float64, error) {
	if len(kps) == 0 {
		return nil, nil
	}
	if pose.Count() == 0 {
		return nil, errors.New("label type has no keypoints")
	}
	if len(kps) != pose.Count() {
		return nil, fmt.Errorf("box must have %d keypoints", pose.Count())
	}
	out := make([]float64, 3*len(kps))
	for i, k := range kps {
		v := k.GetVisibility()
		switch v {
		case krillv1.KeypointVisibility_KEYPOINT_VISIBILITY_UNSPECIFIED:
			continue
		case krillv1.KeypointVisibility_KEYPOINT_VISIBILITY_HIDDEN, krillv1.KeypointVisibility_KEYPOINT_VISIBILITY_VISIBLE:
		default:
			return nil, fmt.Errorf("keypoint %d has an unknown visibility", i)
		}
		x, y := k.GetX(), k.GetY()
		if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
			return nil, errors.New("box has a non-finite keypoint")
		}
		out[3*i] = min(max(x, taxonomy.MinKeypoint), taxonomy.MaxKeypoint)
		out[3*i+1] = min(max(y, taxonomy.MinKeypoint), taxonomy.MaxKeypoint)
		out[3*i+2] = float64(v)
	}
	pose.SetCenters(out)
	if !AnyLabeled(out) {
		return nil, nil
	}
	return out, nil
}

// ResizeKeypoints drops keypoints past n and adds unlabeled ones up to it,
// for boxes from models that may find a different number than the type has.
func ResizeKeypoints(kps []*krillv1.Keypoint, n int) []*krillv1.Keypoint {
	if len(kps) == 0 || n == 0 {
		return nil
	}
	out := make([]*krillv1.Keypoint, n)
	copy(out, kps)
	for i := range out {
		if out[i] == nil {
			out[i] = &krillv1.Keypoint{}
		}
	}
	return out
}

// PadKeypoints fits stored keypoints to a label type with n keypoints, which
// may have gained some since they were labeled. It returns nil when none is
// labeled.
func PadKeypoints(kps []float64, n int) []float64 {
	if len(kps) == 0 || n == 0 {
		return nil
	}
	out := make([]float64, 3*n)
	copy(out, kps)
	if !AnyLabeled(out) {
		return nil
	}
	return out
}

func AnyLabeled(kps []float64) bool {
	for i := 2; i < len(kps); i += 3 {
		if kps[i] != taxonomy.Unlabeled {
			return true
		}
	}
	return false
}

// Keypoints converts flattened keypoints to messages, or nil when there are
// none.
func Keypoints(kps []float64) []*krillv1.Keypoint {
	if len(kps) == 0 {
		return nil
	}
	out := make([]*krillv1.Keypoint, len(kps)/3)
	for i := range out {
		out[i] = &krillv1.Keypoint{
			X: kps[3*i], Y: kps[3*i+1], Visibility: krillv1.KeypointVisibility(int32(kps[3*i+2])),
		}
	}
	return out
}

// OKS is the object keypoint similarity of an answer box's keypoints to a
// reference box's, over the keypoints the reference labeled, with distances
// in pixels of a frameW by frameH frame. ok is false when the reference
// labeled none, or the frame's size is unknown.
func OKS(ref, ans []float64, refBox Rect, frameW, frameH float64) (oks float64, ok bool) {
	area := refBox.W * frameW * refBox.H * frameH
	if area <= 0 {
		return 0, false
	}
	k := 2 * keypointSigma
	var sum float64
	n := 0
	for i := 0; i+2 < len(ref); i += 3 {
		if ref[i+2] == taxonomy.Unlabeled {
			continue
		}
		n++
		if i+2 >= len(ans) || ans[i+2] == taxonomy.Unlabeled {
			continue
		}
		dx := (ref[i] - ans[i]) * frameW
		dy := (ref[i+1] - ans[i+1]) * frameH
		sum += math.Exp(-(dx*dx + dy*dy) / (2 * area * k * k))
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// MergeKeypoints labels the keypoints old has unlabeled with found's, and
// replaces the ones in replace, then places quad centers again. It returns
// nil when that changes none.
func MergeKeypoints(old, found []float64, pose taxonomy.Pose, replace []int32) []float64 {
	n := pose.Count()
	merged := make([]float64, 3*n)
	copy(merged, old)
	for i := 0; i < n && 3*i+2 < len(found); i++ {
		if found[3*i+2] == taxonomy.Unlabeled {
			continue
		}
		if merged[3*i+2] == taxonomy.Unlabeled || slices.Contains(replace, int32(i)) { //nolint:gosec // keypoints per type are capped
			copy(merged[3*i:3*i+3], found[3*i:3*i+3])
		}
	}
	pose.SetCenters(merged)
	before := make([]float64, 3*n)
	copy(before, old)
	if slices.Equal(merged, before) {
		return nil
	}
	return merged
}
