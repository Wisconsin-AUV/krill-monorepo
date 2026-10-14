package pipeline

import (
	"cmp"
	"math"
	"slices"

	"github.com/wauv/krill/api/internal/annotation"
)

const (
	// linkMinIoU is the overlap above which boxes of one class on nearby
	// frames are taken to be the same object.
	linkMinIoU = 0.3
	// linkMaxGap lets a track survive a frame or two the detector missed.
	linkMaxGap = 2
	// duplicateIoU drops a proposal over a box that is already there.
	duplicateIoU = 0.5
)

// detection is one box a pre-labeler found.
type detection struct {
	frameID  int64
	frameIdx int32
	box      annotation.Box
	conf     float32
	// target groups detections that may share a track: one class, or one
	// plugin output.
	target int
	model  string
}

func iou(a, b annotation.Box) float64 {
	w := math.Min(a.X+a.Width, b.X+b.Width) - math.Max(a.X, b.X)
	h := math.Min(a.Y+a.Height, b.Y+b.Height) - math.Max(a.Y, b.Y)
	if w <= 0 || h <= 0 {
		return 0
	}
	inter := w * h
	return inter / (a.Width*a.Height + b.Width*b.Height - inter)
}

// dropCovered removes detections that overlap a box already on their frame,
// so nobody's box gets a duplicate proposal on top of it.
func dropCovered(dets []detection, existing map[int64][]annotation.Box) []detection {
	out := dets[:0:0]
	for _, d := range dets {
		covered := slices.ContainsFunc(existing[d.frameID], func(b annotation.Box) bool {
			return iou(d.box, b) > duplicateIoU
		})
		if !covered {
			out = append(out, d)
		}
	}
	return out
}

type openTrack struct {
	group   int
	target  int
	lastIdx int32
	last    annotation.Box
}

// linkTracks groups detections into tracks. A detection continues the track
// of the same target whose last box, at most linkMaxGap frames back, overlaps
// it most. It returns the detection indexes of each track in frame order.
func linkTracks(dets []detection) [][]int {
	order := make([]int, len(dets))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(a, b int) int { return cmp.Compare(dets[a].frameIdx, dets[b].frameIdx) })

	var groups [][]int
	var open []*openTrack
	for start := 0; start < len(order); {
		idx := dets[order[start]].frameIdx
		end := start
		for end < len(order) && dets[order[end]].frameIdx == idx {
			end++
		}
		frame := order[start:end]
		start = end

		live := open[:0]
		for _, t := range open {
			if idx-t.lastIdx <= linkMaxGap+1 {
				live = append(live, t)
			}
		}
		open = live

		type pair struct {
			det   int
			track *openTrack
			iou   float64
		}
		var pairs []pair
		for _, d := range frame {
			for _, t := range open {
				if t.target != dets[d].target || t.lastIdx == idx {
					continue
				}
				if v := iou(dets[d].box, t.last); v >= linkMinIoU {
					pairs = append(pairs, pair{d, t, v})
				}
			}
		}
		slices.SortStableFunc(pairs, func(a, b pair) int { return cmp.Compare(b.iou, a.iou) })
		matched := map[int]bool{}
		for _, p := range pairs {
			if matched[p.det] || p.track.lastIdx == idx {
				continue
			}
			matched[p.det] = true
			p.track.lastIdx, p.track.last = idx, dets[p.det].box
			groups[p.track.group] = append(groups[p.track.group], p.det)
		}
		for _, d := range frame {
			if matched[d] {
				continue
			}
			groups = append(groups, []int{d})
			open = append(open, &openTrack{group: len(groups) - 1, target: dets[d].target, lastIdx: idx, last: dets[d].box})
		}
	}
	return groups
}
