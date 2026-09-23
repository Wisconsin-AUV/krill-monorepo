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

// URLExpiry covers a long labeling session without re-fetching the clip.
const URLExpiry = 12 * time.Hour

// Presenter converts rows to protos and signs their image URLs.
type Presenter struct {
	store *storage.Store
}

func NewPresenter(store *storage.Store) *Presenter {
	return &Presenter{store: store}
}

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

// Stats are counts computed by the caller's query, not stored on the row.
type Stats struct {
	Clips         int32
	LabeledFrames int32
	Boxes         int32
}

func (s *Presenter) Video(ctx context.Context, v db.Video, st Stats) *krillv1.Video {
	out := &krillv1.Video{
		Id:                v.ID,
		Name:              v.Name,
		Filename:          v.Filename,
		Notes:             v.Notes,
		Status:            statusToProto[v.Status],
		Error:             v.Error,
		IngestProgress:    v.IngestProgress,
		Split:             splitToProto[v.Split],
		ExtractFps:        v.ExtractFps,
		Width:             v.Width,
		Height:            v.Height,
		Fps:               v.Fps,
		DurationMs:        v.DurationMs,
		FrameCount:        v.FrameCount,
		ClipCount:         st.Clips,
		LabeledFrameCount: st.LabeledFrames,
		BoxCount:          st.Boxes,
		CreatedAt:         timestamppb.New(v.CreatedAt.Time),
	}
	if v.Status == "ready" && v.FrameCount > 0 {
		out.ThumbnailUrl = s.frameURL(ctx, v.ID, v.FrameCount/2)
	}
	return out
}

func (s *Presenter) Clip(ctx context.Context, v db.Video, c db.Clip, st Stats) *krillv1.Clip {
	return &krillv1.Clip{
		Id:                c.ID,
		VideoId:           c.VideoID,
		Index:             c.Idx,
		StartFrame:        c.StartFrame,
		FrameCount:        c.FrameCount,
		StartMs:           FrameMs(c.StartFrame, v.Fps),
		DurationMs:        FrameMs(c.FrameCount, v.Fps),
		ThumbnailUrl:      s.frameURL(ctx, v.ID, c.StartFrame+c.FrameCount/2),
		LabeledFrameCount: st.LabeledFrames,
		BoxCount:          st.Boxes,
	}
}

func FrameMs(frames int32, fps float64) int64 {
	if fps <= 0 {
		return 0
	}
	return int64(float64(frames) * 1000 / fps)
}

// frameURL returns "" on failure. A missing thumbnail should not fail the
// whole listing.
func (s *Presenter) frameURL(ctx context.Context, videoID int64, idx int32) string {
	u, err := s.FrameURL(ctx, videoID, idx)
	if err != nil {
		slog.Warn("presign thumbnail", "video_id", videoID, "err", err)
		return ""
	}
	return u
}

func (s *Presenter) FrameURL(ctx context.Context, videoID int64, idx int32) (string, error) {
	return s.store.PresignGet(ctx, storage.FrameKey(videoID, idx), URLExpiry, "")
}
