package clip

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/annotation"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/review"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/video"
)

type Service struct {
	krillv1connect.UnimplementedClipServiceHandler
	pool    *pgxpool.Pool
	q       *db.Queries
	present *video.Presenter
	jobs    *river.Client[pgx.Tx]
}

func NewService(pool *pgxpool.Pool, store *storage.Store, jobs *river.Client[pgx.Tx]) *Service {
	return &Service{pool: pool, q: db.New(pool), present: video.NewPresenter(store), jobs: jobs}
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
	stats, err := s.q.GetClipLabelStats(ctx, c.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count labels")
	}
	videoStats, err := s.q.GetVideoLabelStats(ctx, v.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count labels")
	}
	trackRows, err := s.q.ListClipTracks(ctx, c.ID)
	if err != nil {
		return nil, rpc.Internal(err, "list tracks")
	}
	annRows, err := s.q.ListClipAnnotations(ctx, c.ID)
	if err != nil {
		return nil, rpc.Internal(err, "list boxes")
	}
	claimRows, err := s.q.ListClipClaims(ctx, []int64{c.ID})
	if err != nil {
		return nil, rpc.Internal(err, "load claim")
	}

	tracks := make([]*krillv1.Track, len(trackRows))
	for i, t := range trackRows {
		if tracks[i], err = annotation.Track(t); err != nil {
			return nil, rpc.Internal(err, "decode track")
		}
	}
	annotations := make([]*krillv1.Annotation, len(annRows))
	for i, a := range annRows {
		annotations[i] = annotation.Annotation(a)
	}

	gold := map[int64]bool{}
	if auth.CallerCan(ctx, krillv1.Permission_PERMISSION_REVIEW_LABELS) {
		ids, err := s.q.ListClipGoldFrames(ctx, c.ID)
		if err != nil {
			return nil, rpc.Internal(err, "list gold frames")
		}
		for _, id := range ids {
			gold[id] = true
		}
	}

	reviews, err := review.ClipReviews(ctx, s.q, c.ID)
	if err != nil {
		return nil, rpc.Internal(err, "list reviews")
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
			Status:      annotation.FrameStatus(f.Status),
			Gold:        gold[f.ID],
			Review:      reviews[f.ID],
		}
	}

	out := s.present.Clip(ctx, v, c, video.Stats{LabeledFrames: stats.LabeledFrameCount, Boxes: stats.BoxCount})
	out.Claim = video.Claims(claimRows, time.Now())[c.ID]

	trackIDs := make([]int64, len(trackRows))
	for i, t := range trackRows {
		trackIDs[i] = t.ID
	}
	tracking, err := annotation.Tracking(ctx, s.jobs, trackIDs)
	if err != nil {
		return nil, rpc.Internal(err, "load tracking")
	}

	return &krillv1.GetClipResponse{
		Video: s.present.Video(ctx, v, video.Stats{
			Clips: clipCount, LabeledFrames: videoStats.LabeledFrameCount, Boxes: videoStats.BoxCount,
		}),
		Clip:             out,
		Tracks:           tracks,
		Annotations:      annotations,
		Frames:           frames,
		PreviousClipId:   neighbours.PreviousID,
		NextClipId:       neighbours.NextID,
		TrackingTrackIds: tracking,
	}, nil
}

// DeleteClip leaves the frame images in storage. The video's thumbnail is
// picked by frame index, so it could point at one of them, and they are
// removed with the video.
func (s *Service) DeleteClip(ctx context.Context, req *krillv1.DeleteClipRequest) (*krillv1.DeleteClipResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	// Locks the video row before touching its clips, so two deletes in one
	// video renumber one after the other instead of deadlocking.
	videoID, err := q.SubtractClipFrames(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "clip")
	}
	name, err := audit.ClipName(ctx, q, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "name clip")
	}
	idx, err := q.DeleteClip(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "delete clip")
	}
	if err := q.ParkClipsAfter(ctx, db.ParkClipsAfterParams{VideoID: videoID, Idx: idx}); err != nil {
		return nil, rpc.Internal(err, "renumber clips")
	}
	if err := q.UnparkClips(ctx, videoID); err != nil {
		return nil, rpc.Internal(err, "renumber clips")
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.ClipDelete,
		TargetType: audit.TargetClip,
		TargetID:   strconv.FormatInt(req.GetId(), 10),
		TargetName: name,
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.DeleteClipResponse{}, nil
}
