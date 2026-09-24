package queue

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/video"
)

// claimAttempts bounds retries when another labeler claims the same clip
// between picking it and claiming it.
const claimAttempts = 3

type Service struct {
	krillv1connect.UnimplementedQueueServiceHandler
	pool    *pgxpool.Pool
	q       *db.Queries
	present *video.Presenter
}

func NewService(pool *pgxpool.Pool, store *storage.Store) *Service {
	return &Service{pool: pool, q: db.New(pool), present: video.NewPresenter(store)}
}

func (s *Service) GetQueue(ctx context.Context, _ *krillv1.GetQueueRequest) (*krillv1.GetQueueResponse, error) {
	open, err := s.q.ListOpenClips(ctx, auth.CallerID(ctx))
	if err != nil {
		return nil, rpc.Internal(err, "list open clips")
	}
	progress, err := s.q.ListClipProgress(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list clip progress")
	}
	ids := make([]int64, len(progress))
	for i, p := range progress {
		ids[i] = p.ID
	}
	claimRows, err := s.q.ListClipClaims(ctx, ids)
	if err != nil {
		return nil, rpc.Internal(err, "list claims")
	}
	claims := video.Claims(claimRows, time.Now())

	out := &krillv1.GetQueueResponse{OpenClips: make([]*krillv1.OpenClip, len(open))}
	for i, r := range open {
		c := s.present.Clip(ctx, r.Video, r.Clip, video.Stats{LabeledFrames: r.LabeledFrameCount, Boxes: r.BoxCount})
		c.Claim = claims[r.Clip.ID]
		out.OpenClips[i] = &krillv1.OpenClip{Clip: c, VideoName: r.Video.Name}
	}

	var current *krillv1.VideoProgress
	for _, p := range progress {
		if current == nil || current.Id != p.VideoID {
			current = &krillv1.VideoProgress{Id: p.VideoID, Name: p.VideoName}
			out.Videos = append(out.Videos, current)
		}
		claim := claims[p.ID]
		current.Clips = append(current.Clips, &krillv1.ClipProgress{
			Id:                p.ID,
			Index:             p.Idx,
			FrameCount:        p.FrameCount,
			LabeledFrameCount: p.LabeledFrameCount,
			Claim:             claim,
		})
		if p.LabeledFrameCount < p.FrameCount && (claim == nil || !claim.Active) {
			out.AvailableClips++
		}
	}
	return out, nil
}

func (s *Service) ClaimNextClip(ctx context.Context, req *krillv1.ClaimNextClipRequest) (*krillv1.ClaimNextClipResponse, error) {
	me := auth.CallerID(ctx)
	if id := req.GetReleaseClipId(); id != 0 {
		if err := s.q.ReleaseClipClaim(ctx, db.ReleaseClipClaimParams{ClipID: id, UserID: me}); err != nil {
			return nil, rpc.Internal(err, "release claim")
		}
	}

	open, err := s.q.ListOpenClips(ctx, me)
	if err != nil {
		return nil, rpc.Internal(err, "list open clips")
	}
	if len(open) > 0 {
		id := open[0].Clip.ID
		if _, err := s.q.ClaimClip(ctx, db.ClaimClipParams{
			ClipID: id, UserID: me, StaleBefore: video.StaleBefore(time.Now()),
		}); err != nil {
			return nil, rpc.Internal(err, "claim clip")
		}
		return &krillv1.ClaimNextClipResponse{ClipId: id}, nil
	}

	for range claimAttempts {
		id, taken, err := s.claimNew(ctx, me, req.GetReleaseClipId())
		if err != nil {
			return nil, err
		}
		if !taken {
			return &krillv1.ClaimNextClipResponse{ClipId: id}, nil
		}
	}
	return nil, rpc.Internal(errors.New("clip was claimed by someone else"), "claim clip")
}

// claimNew returns 0 when no clip is available, and taken when another
// labeler claimed the chosen clip first.
func (s *Service) claimNew(ctx context.Context, me uuid.UUID, skip int64) (id int64, taken bool, err error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, false, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	staleBefore := video.StaleBefore(time.Now())
	id, err = q.NextClip(ctx, db.NextClipParams{SkipClipID: skip, StaleBefore: staleBefore})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, rpc.Internal(err, "pick clip")
	}
	n, err := q.ClaimClip(ctx, db.ClaimClipParams{ClipID: id, UserID: me, StaleBefore: staleBefore})
	if err != nil {
		return 0, false, rpc.Internal(err, "claim clip")
	}
	if n == 0 {
		return 0, true, nil
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, false, rpc.Internal(err, "commit")
	}
	return id, false, nil
}
