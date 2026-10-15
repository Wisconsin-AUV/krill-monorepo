package insight

import (
	"context"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/video"
)

const (
	// minBoxes is roughly where a YOLO class stops being guessed at.
	minBoxes = 200
	// minVideos lets a class land in both train and val, since splits are by
	// video, with one more so neither side rests on a single dive.
	minVideos = 3
)

func (s *Service) GetCoverage(ctx context.Context, _ *krillv1.GetCoverageRequest) (*krillv1.GetCoverageResponse, error) {
	types, err := s.q.ListLabelTypes(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list label types")
	}
	videos, err := s.q.CoverageVideos(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "count frames")
	}
	cells, err := s.q.CoverageCells(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "count boxes")
	}

	out := &krillv1.GetCoverageResponse{
		LabelTypes: make([]*krillv1.CoverageLabelType, len(types)),
		Videos:     make([]*krillv1.CoverageVideo, len(videos)),
		Cells:      make([]*krillv1.CoverageCell, len(cells)),
		MinBoxes:   minBoxes,
		MinVideos:  minVideos,
	}
	heldOut := map[int64]bool{}
	for i, v := range videos {
		heldOut[v.ID] = video.HeldOut(v.Split)
		out.Videos[i] = &krillv1.CoverageVideo{
			Id:        v.ID,
			Name:      v.Name,
			Split:     v.Split,
			HeldOut:   heldOut[v.ID],
			Frames:    v.Frames,
			Labeled:   v.Labeled,
			Empty:     v.Empty,
			Unlabeled: v.Frames - v.Labeled - v.Empty,
		}
	}
	byType := map[int64]*krillv1.CoverageLabelType{}
	for i, t := range types {
		lt := t.LabelType
		out.LabelTypes[i] = &krillv1.CoverageLabelType{Id: lt.ID, Name: lt.Name, Color: lt.Color}
		byType[lt.ID] = out.LabelTypes[i]
	}
	for i, c := range cells {
		out.Cells[i] = &krillv1.CoverageCell{
			VideoId:     c.VideoID,
			LabelTypeId: c.LabelTypeID,
			Verified:    c.Verified,
			Proposed:    c.Proposed,
			Frames:      c.Frames,
		}
		t, ok := byType[c.LabelTypeID]
		if !ok {
			continue
		}
		if heldOut[c.VideoID] {
			t.HeldOutVerified += c.Verified
			if c.Verified > 0 {
				t.HeldOutVideos++
			}
			continue
		}
		t.Verified += c.Verified
		t.Proposed += c.Proposed
		t.Frames += c.Frames
		if c.Verified > 0 {
			t.Videos++
		}
	}
	for _, t := range out.LabelTypes {
		t.FewBoxes = t.Verified < minBoxes
		t.FewVideos = t.Videos < minVideos
	}
	return out, nil
}
