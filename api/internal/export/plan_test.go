package export

import (
	"testing"

	"github.com/wauv/krill/api/internal/taxonomy"
)

func frames(videoID, clipID int64, n int, status string, hash func(int) uint64) []Frame {
	out := make([]Frame, n)
	for i := range out {
		out[i] = Frame{
			ID:      videoID*1000 + int64(i),
			VideoID: videoID,
			ClipID:  clipID,
			Idx:     int32(i), //nolint:gosec // test sizes are small
			PHash:   hash(i),
			Status:  status,
		}
	}
	return out
}

func distinct(i int) uint64 { return uint64(i) * 0x5555_5555_5555_5555 } //nolint:gosec // test indexes are non-negative

func TestBuildFiltersFrames(t *testing.T) {
	types := []LabelType{
		{ID: 1, Name: "gate"},
		{ID: 2, Name: "hole", Attributes: []taxonomy.Attribute{{Name: "size", Options: []string{"big", "small"}}}},
	}
	videos := []Video{{ID: 1, Name: "a", Split: "train"}}
	fs := frames(1, 10, 6, "labeled", distinct)
	fs[5].Status = "unlabeled"
	boxes := []Box{
		{FrameID: fs[0].ID, TypeID: 1, X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Status: "verified"},
		{FrameID: fs[1].ID, TypeID: 2, Attributes: map[string]string{"size": "small"}, W: 0.1, H: 0.1, Status: "verified"},
		{FrameID: fs[2].ID, TypeID: 2, Attributes: map[string]string{}, W: 0.1, H: 0.1, Status: "verified"},
		{FrameID: fs[3].ID, TypeID: 1, W: 0.1, H: 0.1, Status: "proposed"},
	}

	p := Build(Options{Stride: 1}, types, videos, fs, boxes)

	if got, want := p.Classes, []string{"gate", "hole-big", "hole-small"}; len(got) != len(want) || got[2] != want[2] {
		t.Fatalf("classes = %v, want %v", got, want)
	}
	s := p.Stats
	if s.TrainImages != 3 || s.UnlabeledFrames != 1 || s.IncompleteFrames != 1 || s.UnverifiedFrames != 1 {
		t.Errorf("stats = %+v", s)
	}
	if s.NegativeImages != 1 {
		t.Errorf("negatives = %d, want 1", s.NegativeImages)
	}
	if p.Items[1].Boxes[0].Class != 2 {
		t.Errorf("hole-small class id = %d, want 2", p.Items[1].Boxes[0].Class)
	}
	if s.Classes[0].Clips != 1 || s.Classes[0].Videos != 1 || s.Classes[1].TrainBoxes != 0 {
		t.Errorf("class stats = %+v", s.Classes)
	}
}

func TestBuildStrideAndDedup(t *testing.T) {
	videos := []Video{{ID: 1, Split: "train"}}

	p := Build(Options{Stride: 3}, nil, videos, frames(1, 1, 10, "empty", distinct), nil)
	if p.Stats.TrainImages != 4 || p.Stats.StrideSkipped != 6 {
		t.Errorf("stride: images %d skipped %d, want 4 and 6", p.Stats.TrainImages, p.Stats.StrideSkipped)
	}

	same := func(i int) uint64 {
		if i < 5 {
			return 0
		}
		return ^uint64(0)
	}
	p = Build(Options{Stride: 1, Dedup: true, DedupDistance: 4}, nil, videos, frames(1, 1, 10, "empty", same), nil)
	if p.Stats.TrainImages != 2 || p.Stats.DuplicateSkipped != 8 {
		t.Errorf("dedup: images %d skipped %d, want 2 and 8", p.Stats.TrainImages, p.Stats.DuplicateSkipped)
	}
}

func TestAssignSplits(t *testing.T) {
	plan := func(id int64, split string, n int) videoPlan {
		return videoPlan{video: Video{ID: id, Split: split}, items: make([]Item, n)}
	}
	tests := []struct {
		name  string
		plans []videoPlan
		frac  float64
		want  map[int64]string
	}{
		{
			"balances toward target",
			[]videoPlan{plan(1, "auto", 50), plan(2, "auto", 30), plan(3, "auto", 20)},
			0.2,
			map[int64]string{1: "train", 2: "train", 3: "val"},
		},
		{
			"pinned val counts toward target",
			[]videoPlan{plan(1, "val", 20), plan(2, "auto", 50), plan(3, "auto", 30)},
			0.2,
			map[int64]string{1: "val", 2: "train", 3: "train"},
		},
		{
			"zero fraction keeps everything in train",
			[]videoPlan{plan(1, "auto", 10), plan(2, "auto", 10)},
			0,
			map[int64]string{1: "train", 2: "train"},
		},
		{
			"every video larger than target still fills val",
			[]videoPlan{plan(1, "auto", 60), plan(2, "auto", 40)},
			0.2,
			map[int64]string{1: "train", 2: "val"},
		},
		{
			"single video goes to train",
			[]videoPlan{plan(1, "auto", 100)},
			0.2,
			map[int64]string{1: "train"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assignSplits(tt.plans, tt.frac)
			for id, want := range tt.want {
				if got[id] != want {
					t.Errorf("video %d = %s, want %s (all %v)", id, got[id], want, got)
				}
			}
		})
	}
}
