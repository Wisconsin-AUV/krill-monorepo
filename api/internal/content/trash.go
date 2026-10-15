package content

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/trash"
)

// sizeWorkers bounds how many storage listings run at once for the trash.
const sizeWorkers = 8

func (s *Service) ListTrash(ctx context.Context, _ *krillv1.ListTrashRequest) (*krillv1.ListTrashResponse, error) {
	videos, err := s.q.ListTrashedVideos(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list trashed videos")
	}
	clips, err := s.q.ListTrashedClips(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list trashed clips")
	}
	days := s.retention()
	purgeAt := func(t pgtype.Timestamptz) *timestamppb.Timestamp {
		if days <= 0 {
			return nil
		}
		return timestamppb.New(t.Time.AddDate(0, 0, days))
	}

	items := make([]*krillv1.TrashItem, 0, len(videos)+len(clips))
	sizes := make([]func(context.Context) (int64, error), 0, cap(items))
	for _, v := range videos {
		item := &krillv1.TrashItem{
			Kind:              krillv1.TrashKind_TRASH_KIND_VIDEO,
			Id:                v.ID,
			VideoId:           v.ID,
			VideoName:         v.Name,
			DeletedAt:         timestamppb.New(v.DeletedAt.Time),
			PurgeAt:           purgeAt(v.DeletedAt),
			FrameCount:        v.FrameCount,
			ClipCount:         v.ClipCount,
			LabeledFrameCount: v.LabeledFrameCount,
			TrackCount:        v.TrackCount,
			BoxCount:          v.BoxCount,
		}
		if v.DeletedByID.Valid {
			item.DeletedBy = auth.PublicProfile(v.DeletedByID.Bytes, v.DeletedByName.String, v.DeletedByUsername.String, v.DeletedByRole.String, v.DeletedByCreatedAt)
		}
		if v.Status == "ready" && v.ThumbnailFrame >= 0 {
			item.ThumbnailUrl = s.thumbnail(ctx, v.ID, v.ThumbnailFrame)
		}
		items = append(items, item)
		sizes = append(sizes, func(ctx context.Context) (int64, error) {
			return s.store.Size(ctx, storage.VideoPrefix(v.ID), "", "")
		})
	}
	for _, c := range clips {
		item := &krillv1.TrashItem{
			Kind:              krillv1.TrashKind_TRASH_KIND_CLIP,
			Id:                c.ID,
			VideoId:           c.VideoID,
			VideoName:         c.VideoName,
			ClipIndex:         c.Idx,
			DeletedAt:         timestamppb.New(c.DeletedAt.Time),
			PurgeAt:           purgeAt(c.DeletedAt),
			FrameCount:        c.FrameCount,
			LabeledFrameCount: c.LabeledFrameCount,
			TrackCount:        c.TrackCount,
			BoxCount:          c.BoxCount,
			ThumbnailUrl:      s.thumbnail(ctx, c.VideoID, c.StartFrame+c.FrameCount/2),
		}
		if c.DeletedByID.Valid {
			item.DeletedBy = auth.PublicProfile(c.DeletedByID.Bytes, c.DeletedByName.String, c.DeletedByUsername.String, c.DeletedByRole.String, c.DeletedByCreatedAt)
		}
		items = append(items, item)
		sizes = append(sizes, func(ctx context.Context) (int64, error) {
			return clipSize(ctx, s.store, c.VideoID, c.StartFrame, c.FrameCount)
		})
	}

	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(sizeWorkers)
	for i, size := range sizes {
		g.Go(func() error {
			n, err := size(gctx)
			if err != nil {
				slog.WarnContext(ctx, "measure trashed item", "video_id", items[i].GetVideoId(), "err", err)
				n = -1
			}
			items[i].SizeBytes = n
			return nil
		})
	}
	_ = g.Wait()

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].GetDeletedAt().AsTime().After(items[j].GetDeletedAt().AsTime())
	})
	return &krillv1.ListTrashResponse{Items: items, RetentionDays: int32(days)}, nil //nolint:gosec // a setting in days
}

// clipSize adds up the clip's frames and embeddings, whose keys sort by frame
// index within the video.
func clipSize(ctx context.Context, store *storage.Store, videoID int64, start, count int32) (int64, error) {
	var total int64
	for _, r := range []struct {
		prefix string
		key    func(int64, int32) string
	}{
		{storage.FramesPrefix(videoID), storage.FrameKey},
		{storage.EmbeddingsPrefix(videoID), storage.EmbeddingKey},
	} {
		after := ""
		if start > 0 {
			after = r.key(videoID, start-1)
		}
		n, err := store.Size(ctx, r.prefix, after, r.key(videoID, start+count-1))
		if err != nil {
			return 0, err
		}
		total += n
	}
	return total, nil
}

func (s *Service) thumbnail(ctx context.Context, videoID int64, idx int32) string {
	url, err := s.present.FrameURL(ctx, videoID, idx)
	if err != nil {
		slog.WarnContext(ctx, "presign trash thumbnail", "video_id", videoID, "err", err)
		return ""
	}
	return url
}

func (s *Service) RestoreTrash(ctx context.Context, req *krillv1.RestoreTrashRequest) (*krillv1.RestoreTrashResponse, error) {
	videoIDs, clipIDs := unique(req.GetVideoIds()), unique(req.GetClipIds())
	if err := checkSelection(len(videoIDs) + len(clipIDs)); err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	// Videos first, so a clip picked along with its video can come back too.
	for _, id := range videoIDs {
		if _, err := trash.RestoreVideo(ctx, q, id); err != nil {
			return nil, err
		}
	}
	for _, id := range clipIDs {
		if err := trash.RestoreClip(ctx, q, id); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.RestoreTrashResponse{
		Videos: int32(len(videoIDs)), //nolint:gosec // bounded by maxSelection
		Clips:  int32(len(clipIDs)),  //nolint:gosec // bounded by maxSelection
	}, nil
}

func (s *Service) PurgeTrash(ctx context.Context, req *krillv1.PurgeTrashRequest) (*krillv1.PurgeTrashResponse, error) {
	videoIDs, clipIDs := unique(req.GetVideoIds()), unique(req.GetClipIds())
	if err := checkSelection(len(videoIDs) + len(clipIDs)); err != nil {
		return nil, err
	}
	p, job, err := s.purge(ctx, videoIDs, clipIDs, false)
	if err != nil {
		return nil, err
	}
	return &krillv1.PurgeTrashResponse{Videos: int32(p.Videos), Clips: int32(p.Clips), Job: job}, nil //nolint:gosec // bounded by maxSelection
}

func (s *Service) EmptyTrash(ctx context.Context, _ *krillv1.EmptyTrashRequest) (*krillv1.EmptyTrashResponse, error) {
	rows, err := s.q.ListTrashIDs(ctx, pgtype.Timestamptz{Time: time.Now(), Valid: true, InfinityModifier: pgtype.Infinity})
	if err != nil {
		return nil, rpc.Internal(err, "list trash")
	}
	var videoIDs, clipIDs []int64
	for _, r := range rows {
		if r.Kind == "video" {
			videoIDs = append(videoIDs, r.ID)
		} else {
			clipIDs = append(clipIDs, r.ID)
		}
	}
	// Videos still processing stay until their ingest finishes.
	p, job, err := s.purge(ctx, videoIDs, clipIDs, true)
	if err != nil {
		return nil, err
	}
	return &krillv1.EmptyTrashResponse{Videos: int32(p.Videos), Clips: int32(p.Clips), Job: job}, nil //nolint:gosec // counted from the trash
}

func (s *Service) purge(ctx context.Context, videoIDs, clipIDs []int64, skipBusy bool) (trash.Purged, *krillv1.ContentJob, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return trash.Purged{}, nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	p, err := trash.Purge(ctx, q, tx, s.jobs, videoIDs, clipIDs, skipBusy, nil)
	if err != nil {
		return trash.Purged{}, nil, err
	}
	var job *krillv1.ContentJob
	if len(p.Jobs) > 0 {
		if job, err = s.createJob(ctx, q, trash.JobKind, p.Summary(), p.Jobs); err != nil {
			return trash.Purged{}, nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return trash.Purged{}, nil, rpc.Internal(err, "commit")
	}
	return p, job, nil
}
