// Package export plans and writes YOLO datasets.
package export

import (
	"cmp"
	"math/bits"
	"slices"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/taxonomy"
)

const (
	splitTrain = "train"
	splitVal   = "val"
	splitAuto  = "auto"
)

type Options struct {
	ValFraction   float64
	Stride        int
	Dedup         bool
	DedupDistance int
}

type LabelType struct {
	ID         int64
	Name       string
	Attributes []taxonomy.Attribute
}

type Video struct {
	ID    int64
	Name  string
	Split string
}

type Frame struct {
	ID      int64
	VideoID int64
	ClipID  int64
	Idx     int32
	PHash   uint64
	Status  string
}

type Box struct {
	FrameID    int64
	X, Y, W, H float64
	Status     string
	TypeID     int64
	Attributes map[string]string
}

type ItemBox struct {
	Class      int
	X, Y, W, H float64
}

type Item struct {
	Frame Frame
	Split string
	Boxes []ItemBox
}

type Plan struct {
	Classes []string
	Items   []Item
	Stats   *krillv1.ExportStats
}

type videoPlan struct {
	video Video
	items []Item
	boxes int
}

// Build decides which frames go into the dataset and in which split.
// Frames arrive ordered by video and index.
func Build(opts Options, types []LabelType, videos []Video, frames []Frame, boxes []Box) Plan {
	stats := &krillv1.ExportStats{}
	classIDs := map[string]int{}
	var classes []string
	typesByID := map[int64]LabelType{}
	for _, t := range types {
		typesByID[t.ID] = t
		for _, c := range taxonomy.Classes(t.Name, t.Attributes) {
			classIDs[c] = len(classes)
			classes = append(classes, c)
		}
	}

	boxesByFrame := map[int64][]Box{}
	for _, b := range boxes {
		boxesByFrame[b.FrameID] = append(boxesByFrame[b.FrameID], b)
	}
	framesByVideo := map[int64][]Frame{}
	for _, f := range frames {
		framesByVideo[f.VideoID] = append(framesByVideo[f.VideoID], f)
	}

	stride := max(opts.Stride, 1)
	plans := make([]videoPlan, 0, len(videos))
	for _, v := range videos {
		vp := videoPlan{video: v}
		eligible := 0
		var lastHash uint64
		haveLast := false

	frameLoop:
		for _, f := range framesByVideo[v.ID] {
			if f.Status == "unlabeled" {
				stats.UnlabeledFrames++
				continue
			}
			item := Item{Frame: f}
			for _, b := range boxesByFrame[f.ID] {
				if b.Status == "proposed" {
					stats.UnverifiedFrames++
					continue frameLoop
				}
				t, ok := typesByID[b.TypeID]
				if !ok || !taxonomy.Complete(t.Attributes, b.Attributes) {
					stats.IncompleteFrames++
					continue frameLoop
				}
				item.Boxes = append(item.Boxes, ItemBox{
					Class: classIDs[taxonomy.ClassName(t.Name, t.Attributes, b.Attributes)],
					X:     b.X, Y: b.Y, W: b.W, H: b.H,
				})
			}

			eligible++
			if (eligible-1)%stride != 0 {
				stats.StrideSkipped++
				continue
			}
			if opts.Dedup && haveLast && bits.OnesCount64(f.PHash^lastHash) <= opts.DedupDistance {
				stats.DuplicateSkipped++
				continue
			}
			lastHash, haveLast = f.PHash, true
			vp.items = append(vp.items, item)
			vp.boxes += len(item.Boxes)
		}
		plans = append(plans, vp)
	}

	splits := assignSplits(plans, opts.ValFraction)

	type coverage struct{ clips, videos map[int64]bool }
	cover := make([]coverage, len(classes))
	classStats := make([]*krillv1.ExportClassStats, len(classes))
	for i, name := range classes {
		classStats[i] = &krillv1.ExportClassStats{Id: int32(i), Name: name} //nolint:gosec // class counts are capped by taxonomy
		cover[i] = coverage{clips: map[int64]bool{}, videos: map[int64]bool{}}
	}

	var items []Item
	for _, vp := range plans {
		split := splits[vp.video.ID]
		stats.Videos = append(stats.Videos, &krillv1.ExportVideoStats{
			VideoId: vp.video.ID,
			Name:    vp.video.Name,
			Split:   split,
			Pinned:  vp.video.Split != splitAuto,
			Images:  int32(len(vp.items)), //nolint:gosec // frame counts fit in int32
			Boxes:   int32(vp.boxes),      //nolint:gosec // box counts fit in int32
		})
		for _, it := range vp.items {
			it.Split = split
			items = append(items, it)
			if len(it.Boxes) == 0 {
				stats.NegativeImages++
			}
			if split == splitVal {
				stats.ValImages++
			} else {
				stats.TrainImages++
			}
			for _, b := range it.Boxes {
				cs := classStats[b.Class]
				if split == splitVal {
					stats.ValBoxes++
					cs.ValBoxes++
				} else {
					stats.TrainBoxes++
					cs.TrainBoxes++
				}
				cover[b.Class].clips[it.Frame.ClipID] = true
				cover[b.Class].videos[it.Frame.VideoID] = true
			}
		}
	}
	for i, cs := range classStats {
		cs.Clips = int32(len(cover[i].clips))   //nolint:gosec // clip counts fit in int32
		cs.Videos = int32(len(cover[i].videos)) //nolint:gosec // video counts fit in int32
	}
	stats.Classes = classStats

	return Plan{Classes: classes, Items: items, Stats: stats}
}

// assignSplits keeps pinned videos where they are, then fills val with
// unpinned videos, largest first, while they fit under the target share. If
// that leaves val empty, the smallest unpinned video goes to val so there is
// always something to validate on when there are at least two videos.
func assignSplits(plans []videoPlan, valFraction float64) map[int64]string {
	out := map[int64]string{}
	total, val, withImages := 0, 0, 0
	var auto []videoPlan
	for _, vp := range plans {
		n := len(vp.items)
		total += n
		if n > 0 {
			withImages++
		}
		switch vp.video.Split {
		case splitVal:
			out[vp.video.ID] = splitVal
			val += n
		case splitTrain:
			out[vp.video.ID] = splitTrain
		default:
			out[vp.video.ID] = splitTrain
			auto = append(auto, vp)
		}
	}
	if valFraction <= 0 {
		return out
	}

	slices.SortStableFunc(auto, func(a, b videoPlan) int {
		if c := cmp.Compare(len(b.items), len(a.items)); c != 0 {
			return c
		}
		return cmp.Compare(a.video.ID, b.video.ID)
	})
	target := valFraction * float64(total)
	for _, vp := range auto {
		if n := len(vp.items); n > 0 && float64(val+n) <= target {
			out[vp.video.ID] = splitVal
			val += n
		}
	}
	if val == 0 && withImages >= 2 {
		for i := len(auto) - 1; i >= 0; i-- {
			if len(auto[i].items) > 0 {
				out[auto[i].video.ID] = splitVal
				break
			}
		}
	}
	return out
}
