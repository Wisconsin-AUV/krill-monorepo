package clip

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/video"
)

type Service struct {
	krillv1connect.UnimplementedClipServiceHandler
	q       *db.Queries
	present *video.Presenter
}

func NewService(pool *pgxpool.Pool, store *storage.Store) *Service {
	return &Service{q: db.New(pool), present: video.NewPresenter(store)}
}

func (s *Service) GetClip(ctx context.Context, req *krillv1.GetClipRequest) (*krillv1.GetClipResponse, error) {
	c, err := s.q.GetClip(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "clip")
	}
	v, err := s.q.GetVideo(ctx, c.VideoID)
	if err != nil {
		return nil, rpc.DBError(err, "video")
	}
	clipCount, err := s.q.CountClips(ctx, v.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count clips")
	}
	neighbours, err := s.q.GetNeighbourClips(ctx, c.ID)
	if err != nil {
		return nil, rpc.Internal(err, "find neighbouring clips")
	}
	rows, err := s.q.ListClipFrames(ctx, c.ID)
	if err != nil {
		return nil, rpc.Internal(err, "list frames")
	}

	frames := make([]*krillv1.Frame, len(rows))
	for i, f := range rows {
		url, err := s.present.FrameURL(ctx, v.ID, f.Idx)
		if err != nil {
			return nil, rpc.Internal(err, "sign frame url")
		}
		frames[i] = &krillv1.Frame{
			Id:          f.ID,
			Index:       f.Idx,
			TimestampMs: video.FrameMs(f.Idx, v.Fps),
			Url:         url,
		}
	}

	return &krillv1.GetClipResponse{
		Video:          s.present.Video(ctx, v, clipCount),
		Clip:           s.present.Clip(ctx, v, c),
		Frames:         frames,
		PreviousClipId: neighbours.PreviousID,
		NextClipId:     neighbours.NextID,
	}, nil
}
