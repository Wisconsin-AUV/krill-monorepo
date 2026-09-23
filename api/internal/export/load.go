package export

import (
	"context"
	"fmt"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/taxonomy"
)

const (
	maxStride            = 100
	maxDedupDistance     = 16
	maxValFraction       = 0.5
	defaultDedupDistance = 4
)

// Normalize fills defaults and rejects out-of-range options.
func Normalize(o *krillv1.ExportOptions) (*krillv1.ExportOptions, error) {
	out := &krillv1.ExportOptions{
		ValFraction:   o.GetValFraction(),
		Stride:        o.GetStride(),
		Dedup:         o.GetDedup(),
		DedupDistance: o.GetDedupDistance(),
		VideoIds:      o.GetVideoIds(),
	}
	if out.GetStride() == 0 {
		out.Stride = 1
	}
	if out.GetDedup() && out.GetDedupDistance() == 0 {
		out.DedupDistance = defaultDedupDistance
	}
	if out.GetValFraction() < 0 || out.GetValFraction() > maxValFraction {
		return nil, fmt.Errorf("val_fraction must be between 0 and %.1f", maxValFraction)
	}
	if out.GetStride() < 1 || out.GetStride() > maxStride {
		return nil, fmt.Errorf("stride must be between 1 and %d", maxStride)
	}
	if out.GetDedupDistance() < 0 || out.GetDedupDistance() > maxDedupDistance {
		return nil, fmt.Errorf("dedup_distance must be between 0 and %d", maxDedupDistance)
	}
	return out, nil
}

func Load(ctx context.Context, q *db.Queries, o *krillv1.ExportOptions) (Plan, error) {
	ids := o.GetVideoIds()
	if ids == nil {
		ids = []int64{}
	}

	typeRows, err := q.ListAllLabelTypes(ctx)
	if err != nil {
		return Plan{}, fmt.Errorf("list label types: %w", err)
	}
	types := make([]LabelType, len(typeRows))
	for i, t := range typeRows {
		attrs, err := taxonomy.ParseAttributes(t.Attributes)
		if err != nil {
			return Plan{}, fmt.Errorf("label type %d: %w", t.ID, err)
		}
		types[i] = LabelType{ID: t.ID, Name: t.Name, Attributes: attrs}
	}

	videoRows, err := q.ListExportVideos(ctx, ids)
	if err != nil {
		return Plan{}, fmt.Errorf("list videos: %w", err)
	}
	videos := make([]Video, len(videoRows))
	for i, v := range videoRows {
		videos[i] = Video{ID: v.ID, Name: v.Name, Split: v.Split}
	}

	frameRows, err := q.ListExportFrames(ctx, ids)
	if err != nil {
		return Plan{}, fmt.Errorf("list frames: %w", err)
	}
	frames := make([]Frame, len(frameRows))
	for i, f := range frameRows {
		frames[i] = Frame{
			ID: f.ID, VideoID: f.VideoID, ClipID: f.ClipID, Idx: f.Idx,
			PHash:  uint64(f.Phash), //nolint:gosec // stored bit-for-bit in a bigint
			Status: f.Status,
		}
	}

	boxRows, err := q.ListExportAnnotations(ctx, ids)
	if err != nil {
		return Plan{}, fmt.Errorf("list boxes: %w", err)
	}
	boxes := make([]Box, len(boxRows))
	for i, b := range boxRows {
		values, err := taxonomy.ParseValues(b.Attributes)
		if err != nil {
			return Plan{}, fmt.Errorf("box on frame %d: %w", b.FrameID, err)
		}
		boxes[i] = Box{
			FrameID: b.FrameID, X: b.X, Y: b.Y, W: b.Width, H: b.Height,
			Status: b.Status, TypeID: b.LabelTypeID, Attributes: values,
		}
	}

	return Build(Options{
		ValFraction:   float64(o.GetValFraction()),
		Stride:        int(o.GetStride()),
		Dedup:         o.GetDedup(),
		DedupDistance: int(o.GetDedupDistance()),
	}, types, videos, frames, boxes), nil
}
