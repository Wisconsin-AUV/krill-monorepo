package gold

import (
	"maps"
	"math"
	"sort"

	"github.com/wauv/krill/api/internal/annotation"
)

const (
	// matchIoU pairs a reference box with an answer box of the same type.
	// Below it the labeler most likely boxed a different object.
	matchIoU = 0.5
	// correctIoU is how tight a matched box must be to count as correct.
	// Loose boxes are the error gold frames exist to catch, so this is well
	// above matchIoU.
	correctIoU = 0.75
)

// Box is normalized to the frame, like krillv1.Box. It is also the JSON
// stored in gold_frames.boxes and gold_attempts.boxes.
type Box struct {
	LabelTypeID int64             `json:"label_type_id"`
	Attributes  map[string]string `json:"attributes"`
	X           float64           `json:"x"`
	Y           float64           `json:"y"`
	Width       float64           `json:"width"`
	Height      float64           `json:"height"`
	// Keypoints are x, y, visibility triples, or nil when none is labeled.
	Keypoints []float64 `json:"keypoints,omitempty"`
}

// Frame is the frame's size in pixels, which keypoint distances are measured
// in.
type Frame struct {
	Width, Height float64
}

type Match struct {
	Reference int
	Answer    int
	IoU       float64
	// OKS is set when the reference has keypoints labeled.
	OKS     *float64
	Correct bool
}

type Result struct {
	Matches []Match
	Correct int
	IoUSum  float64
	// Correct boxes over reference plus extra boxes.
	Score float64
}

func iou(a, b Box) float64 {
	w := math.Min(a.X+a.Width, b.X+b.Width) - math.Max(a.X, b.X)
	h := math.Min(a.Y+a.Height, b.Y+b.Height) - math.Max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	inter := w * h
	return inter / (a.Width*a.Height + b.Width*b.Height - inter)
}

// Score pairs boxes greedily, highest IoU first.
func Score(reference, answer []Box, frame Frame) Result {
	return score(reference, answer, frame, maps.Equal[map[string]string])
}

// ScoreSetAttributes is Score for answers that always carry every attribute,
// like a model's: a match only has to agree on the attributes the reference
// sets.
func ScoreSetAttributes(reference, answer []Box, frame Frame) Result {
	return score(reference, answer, frame, func(ref, ans map[string]string) bool {
		for k, v := range ref {
			if v != "" && ans[k] != v {
				return false
			}
		}
		return true
	})
}

func score(reference, answer []Box, frame Frame, sameAttributes func(ref, ans map[string]string) bool) Result {
	var pairs []Match
	for i, r := range reference {
		for j, a := range answer {
			if r.LabelTypeID != a.LabelTypeID {
				continue
			}
			if v := iou(r, a); v >= matchIoU {
				pairs = append(pairs, Match{Reference: i, Answer: j, IoU: v})
			}
		}
	}
	sort.SliceStable(pairs, func(i, j int) bool { return pairs[i].IoU > pairs[j].IoU })

	var out Result
	usedRef := make([]bool, len(reference))
	usedAns := make([]bool, len(answer))
	for _, p := range pairs {
		if usedRef[p.Reference] || usedAns[p.Answer] {
			continue
		}
		usedRef[p.Reference], usedAns[p.Answer] = true, true
		ref, ans := reference[p.Reference], answer[p.Answer]
		p.Correct = p.IoU >= correctIoU && sameAttributes(ref.Attributes, ans.Attributes)
		refBox := annotation.Rect{X: ref.X, Y: ref.Y, W: ref.Width, H: ref.Height}
		if oks, ok := annotation.OKS(ref.Keypoints, ans.Keypoints, refBox, frame.Width, frame.Height); ok {
			p.OKS = &oks
			p.Correct = p.Correct && oks >= annotation.MinCorrectOKS
		}
		if p.Correct {
			out.Correct++
		}
		out.IoUSum += p.IoU
		out.Matches = append(out.Matches, p)
	}

	scored := len(reference) + len(answer) - len(out.Matches)
	if scored == 0 {
		out.Score = 1
	} else {
		out.Score = float64(out.Correct) / float64(scored)
	}
	return out
}
