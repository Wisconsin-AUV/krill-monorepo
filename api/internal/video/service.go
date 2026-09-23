package video

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/ingest"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
)

const (
	uploadExpiry  = time.Hour
	maxExtractFPS = 120
	maxNameLen    = 200
)

type Service struct {
	krillv1connect.UnimplementedVideoServiceHandler
	pool  *pgxpool.Pool
	q     *db.Queries
	store *storage.Store
	jobs  *river.Client[pgx.Tx]
	*Presenter
}

func NewService(pool *pgxpool.Pool, store *storage.Store, jobs *river.Client[pgx.Tx]) *Service {
	return &Service{pool: pool, q: db.New(pool), store: store, jobs: jobs, Presenter: NewPresenter(store)}
}

func (s *Service) CreateVideo(ctx context.Context, req *krillv1.CreateVideoRequest) (*krillv1.CreateVideoResponse, error) {
	filename := filepath.Base(strings.TrimSpace(req.GetFilename()))
	if filename == "" || filename == "." {
		return nil, rpc.Invalid("filename is required")
	}
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		name = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	if len(name) > maxNameLen {
		return nil, rpc.Invalid("name must be at most %d characters", maxNameLen)
	}
	if fps := req.GetExtractFps(); fps < 0 || fps > maxExtractFPS {
		return nil, rpc.Invalid("extract_fps must be between 0 and %d", maxExtractFPS)
	}

	v, err := s.q.CreateVideo(ctx, db.CreateVideoParams{Name: name, Filename: filename, ExtractFps: req.GetExtractFps()})
	if err != nil {
		return nil, rpc.Internal(err, "create video")
	}
	uploadURL, err := s.store.PresignPut(ctx, storage.VideoSourceKey(v.ID), uploadExpiry)
	if err != nil {
		return nil, rpc.Internal(err, "presign upload")
	}
	return &krillv1.CreateVideoResponse{Video: s.Video(ctx, v, Stats{}), UploadUrl: uploadURL}, nil
}

func (s *Service) StartIngest(ctx context.Context, req *krillv1.StartIngestRequest) (*krillv1.StartIngestResponse, error) {
	id := req.GetVideoId()
	exists, err := s.store.Exists(ctx, storage.VideoSourceKey(id))
	if err != nil {
		return nil, rpc.Internal(err, "check upload")
	}
	if !exists {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("video file has not been uploaded"))
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()

	v, err := s.q.WithTx(tx).QueueIngest(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("video is not waiting for ingest"))
	}
	if err != nil {
		return nil, rpc.DBError(err, "video")
	}
	if _, err := s.jobs.InsertTx(ctx, tx, ingest.Args{VideoID: id}, nil); err != nil {
		return nil, rpc.Internal(err, "queue ingest")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.StartIngestResponse{Video: s.Video(ctx, v, Stats{})}, nil
}

func (s *Service) ListVideos(ctx context.Context, _ *krillv1.ListVideosRequest) (*krillv1.ListVideosResponse, error) {
	rows, err := s.q.ListVideos(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list videos")
	}
	out := make([]*krillv1.Video, len(rows))
	for i, r := range rows {
		out[i] = s.Video(ctx, r.Video, Stats{Clips: r.ClipCount, LabeledFrames: r.LabeledFrameCount, Boxes: r.BoxCount})
	}
	return &krillv1.ListVideosResponse{Videos: out}, nil
}

func (s *Service) GetVideo(ctx context.Context, req *krillv1.GetVideoRequest) (*krillv1.GetVideoResponse, error) {
	v, err := s.q.GetVideo(ctx, req.GetId())
	if err != nil {
		return nil, rpc.DBError(err, "video")
	}
	clips, err := s.q.ListClips(ctx, v.ID)
	if err != nil {
		return nil, rpc.Internal(err, "list clips")
	}
	stats, err := s.q.GetVideoLabelStats(ctx, v.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count labels")
	}
	out := make([]*krillv1.Clip, len(clips))
	for i, c := range clips {
		out[i] = s.Clip(ctx, v, c.Clip, Stats{LabeledFrames: c.LabeledFrameCount, Boxes: c.BoxCount})
	}
	return &krillv1.GetVideoResponse{
		Video: s.Video(ctx, v, Stats{
			Clips:         int32(len(clips)), //nolint:gosec // clip counts fit in int32
			LabeledFrames: stats.LabeledFrameCount,
			Boxes:         stats.BoxCount,
		}),
		Clips: out,
	}, nil
}

func (s *Service) UpdateVideo(ctx context.Context, req *krillv1.UpdateVideoRequest) (*krillv1.UpdateVideoResponse, error) {
	params := db.UpdateVideoParams{ID: req.GetId()}
	if req.Name != nil {
		name := strings.TrimSpace(req.GetName())
		if name == "" || len(name) > maxNameLen {
			return nil, rpc.Invalid("name must be 1 to %d characters", maxNameLen)
		}
		params.Name = pgtype.Text{String: name, Valid: true}
	}
	if req.Notes != nil {
		params.Notes = pgtype.Text{String: strings.TrimSpace(req.GetNotes()), Valid: true}
	}
	if req.Split != nil {
		split, ok := splitFromProto(req.GetSplit())
		if !ok {
			return nil, rpc.Invalid("unknown split %v", req.GetSplit())
		}
		params.Split = pgtype.Text{String: split, Valid: true}
	}

	v, err := s.q.UpdateVideo(ctx, params)
	if err != nil {
		return nil, rpc.DBError(err, "video")
	}
	clipCount, err := s.q.CountClips(ctx, v.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count clips")
	}
	stats, err := s.q.GetVideoLabelStats(ctx, v.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count labels")
	}
	return &krillv1.UpdateVideoResponse{Video: s.Video(ctx, v, Stats{
		Clips: clipCount, LabeledFrames: stats.LabeledFrameCount, Boxes: stats.BoxCount,
	})}, nil
}

func (s *Service) DeleteVideo(ctx context.Context, req *krillv1.DeleteVideoRequest) (*krillv1.DeleteVideoResponse, error) {
	id := req.GetId()
	n, err := s.q.DeleteVideo(ctx, id)
	if err != nil {
		return nil, rpc.Internal(err, "delete video")
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("video %d not found", id))
	}
	if err := s.store.RemovePrefix(ctx, storage.VideoPrefix(id)); err != nil {
		// The rows are gone, so the objects are unreachable. Log instead of
		// failing a delete that already happened.
		slog.Error("remove video objects", "video_id", id, "err", err)
	}
	return &krillv1.DeleteVideoResponse{}, nil
}
