package video

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/storage"
)

const urlExpiry = 12 * time.Hour

var statusToProto = map[string]krillv1.VideoStatus{
	"uploading":  krillv1.VideoStatus_VIDEO_STATUS_UPLOADING,
	"queued":     krillv1.VideoStatus_VIDEO_STATUS_QUEUED,
	"processing": krillv1.VideoStatus_VIDEO_STATUS_PROCESSING,
	"ready":      krillv1.VideoStatus_VIDEO_STATUS_READY,
	"failed":     krillv1.VideoStatus_VIDEO_STATUS_FAILED,
}

var splitToProto = map[string]krillv1.SplitAssignment{
	"auto":  krillv1.SplitAssignment_SPLIT_ASSIGNMENT_AUTO,
	"train": krillv1.SplitAssignment_SPLIT_ASSIGNMENT_TRAIN,
	"val":   krillv1.SplitAssignment_SPLIT_ASSIGNMENT_VAL,
}

func splitFromProto(s krillv1.SplitAssignment) (string, bool) {
	for k, v := range splitToProto {
		if v == s {
			return k, true
		}
	}
	return "", false
}

func (s *Service) toProto(ctx context.Context, v db.Video, clipCount int32) *krillv1.Video {
	out := &krillv1.Video{
		Id:             v.ID,
		Name:           v.Name,
		Filename:       v.Filename,
		Notes:          v.Notes,
		Status:         statusToProto[v.Status],
		Error:          v.Error,
		IngestProgress: v.IngestProgress,
		Split:          splitToProto[v.Split],
		ExtractFps:     v.ExtractFps,
		Width:          v.Width,
		Height:         v.Height,
		Fps:            v.Fps,
		DurationMs:     v.DurationMs,
		FrameCount:     v.FrameCount,
		ClipCount:      clipCount,
		CreatedAt:      timestamppb.New(v.CreatedAt.Time),
	}
	if v.Status == "ready" && v.FrameCount > 0 {
		out.ThumbnailUrl = s.frameURL(ctx, v.ID, v.FrameCount/2)
	}
	return out
}

func (s *Service) clipToProto(ctx context.Context, v db.Video, c db.Clip) *krillv1.Clip {
	return &krillv1.Clip{
		Id:           c.ID,
		VideoId:      c.VideoID,
		Index:        c.Idx,
		StartFrame:   c.StartFrame,
		FrameCount:   c.FrameCount,
		StartMs:      frameMs(c.StartFrame, v.Fps),
		DurationMs:   frameMs(c.FrameCount, v.Fps),
		ThumbnailUrl: s.frameURL(ctx, v.ID, c.StartFrame+c.FrameCount/2),
	}
}

func frameMs(frames int32, fps float64) int64 {
	if fps <= 0 {
		return 0
	}
	return int64(float64(frames) * 1000 / fps)
}

// frameURL returns "" on failure. A missing thumbnail should not fail the
// whole listing.
func (s *Service) frameURL(ctx context.Context, videoID int64, idx int32) string {
	u, err := s.store.PresignGet(ctx, storage.FrameKey(videoID, idx), urlExpiry, "")
	if err != nil {
		slog.Warn("presign thumbnail", "video_id", videoID, "err", err)
		return ""
	}
	return u
}
