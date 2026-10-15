package content

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/ingest"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/video"
)

const (
	maxListedClips = 1000
	maxExtractFPS  = 120
)

func (s *Service) ListContentVideos(ctx context.Context, _ *krillv1.ListContentVideosRequest) (*krillv1.ListContentVideosResponse, error) {
	model := s.currentModel()
	rows, err := s.q.ListContentVideos(ctx, model)
	if err != nil {
		return nil, rpc.Internal(err, "list videos")
	}
	out := &krillv1.ListContentVideosResponse{Videos: make([]*krillv1.ContentVideo, len(rows)), CurrentModel: model}
	for i, r := range rows {
		out.Videos[i] = &krillv1.ContentVideo{
			Id:                r.ID,
			Name:              r.Name,
			Status:            video.StatusProto(r.Status),
			Error:             r.Error,
			IngestProgress:    r.IngestProgress,
			ExtractFps:        r.ExtractFps,
			Fps:               r.Fps,
			FrameCount:        r.FrameCount,
			ClipCount:         r.ClipCount,
			EmbeddedClipCount: r.EmbeddedClipCount,
			CurrentClipCount:  r.CurrentClipCount,
			LabeledFrameCount: r.LabeledFrameCount,
			BoxCount:          r.BoxCount,
			CreatedAt:         ts(r.CreatedAt),
		}
	}
	return out, nil
}

func (s *Service) ListContentClips(ctx context.Context, req *krillv1.ListContentClipsRequest) (*krillv1.ListContentClipsResponse, error) {
	params := db.ListContentClipsParams{Lim: maxListedClips + 1}
	if id := req.GetVideoId(); id != 0 {
		params.VideoID = pgtype.Int8{Int64: id, Valid: true}
	}
	if id := req.GetClaimantId(); id != "" {
		u, err := uuid.Parse(id)
		if err != nil {
			return nil, rpc.Invalid("bad claimant id")
		}
		params.Claimant = pgtype.UUID{Bytes: u, Valid: true}
	}
	rows, err := s.q.ListContentClips(ctx, params)
	if err != nil {
		return nil, rpc.Internal(err, "list clips")
	}
	out := &krillv1.ListContentClipsResponse{CurrentModel: s.currentModel()}
	if len(rows) > maxListedClips {
		rows = rows[:maxListedClips]
		out.Truncated = true
	}
	stale := video.StaleBefore(time.Now()).Time
	out.Clips = make([]*krillv1.ContentClip, len(rows))
	for i, r := range rows {
		c := &krillv1.ContentClip{
			Id:                r.ID,
			VideoId:           r.VideoID,
			VideoName:         r.VideoName,
			Index:             r.Idx,
			FrameCount:        r.FrameCount,
			LabeledFrameCount: r.LabeledFrameCount,
			BoxCount:          r.BoxCount,
		}
		if r.EmbeddedAt.Valid {
			c.EmbedModel = r.EmbedModel
		}
		if r.ClaimantID.Valid {
			c.Claimant = auth.PublicProfile(r.ClaimantID.Bytes, r.ClaimantName.String, r.ClaimantUsername.String, r.ClaimantRole.String, r.ClaimantCreatedAt)
			c.ClaimActiveAt = ts(r.ClaimActiveAt)
			c.ClaimActive = !r.ClaimActiveAt.Time.Before(stale)
		}
		out.Clips[i] = c
	}
	return out, nil
}

func impact(r db.ListReextractStatsRow) *krillv1.ReextractImpact {
	out := &krillv1.ReextractImpact{
		VideoId:           r.ID,
		Name:              r.Name,
		Status:            video.StatusProto(r.Status),
		ExtractFps:        r.ExtractFps,
		LabeledFrameCount: r.LabeledFrameCount,
		TrackCount:        r.TrackCount,
		BoxCount:          r.BoxCount,
		GoldFrameCount:    r.GoldFrameCount,
		OpenReviewCount:   r.OpenReviewCount,
		ClaimCount:        r.ClaimCount,
	}
	switch r.Status {
	case "uploading":
		out.Blocked = "Not uploaded"
	case "queued", "processing":
		out.Blocked = "Already extracting"
	}
	return out
}

func hasLabels(r db.ListReextractStatsRow) bool {
	return r.LabeledFrameCount > 0 || r.TrackCount > 0 || r.GoldFrameCount > 0
}

func (s *Service) reextractStats(ctx context.Context, q *db.Queries, ids []int64) ([]db.ListReextractStatsRow, error) {
	if err := checkSelection(len(ids)); err != nil {
		return nil, err
	}
	rows, err := q.ListReextractStats(ctx, ids)
	if err != nil {
		return nil, rpc.Internal(err, "count labels")
	}
	if len(rows) != len(ids) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("video not found"))
	}
	return rows, nil
}

func (s *Service) PreviewReextract(ctx context.Context, req *krillv1.PreviewReextractRequest) (*krillv1.PreviewReextractResponse, error) {
	rows, err := s.reextractStats(ctx, s.q, unique(req.GetVideoIds()))
	if err != nil {
		return nil, err
	}
	out := &krillv1.PreviewReextractResponse{Videos: make([]*krillv1.ReextractImpact, len(rows))}
	for i, r := range rows {
		out.Videos[i] = impact(r)
	}
	return out, nil
}

var errHasLabels = errors.New("re-extracting replaces every frame, which deletes their labels")

func (s *Service) ReextractVideos(ctx context.Context, req *krillv1.ReextractVideosRequest) (*krillv1.ReextractVideosResponse, error) {
	if req.ExtractFps != nil {
		if fps := req.GetExtractFps(); fps < 0 || fps > maxExtractFPS {
			return nil, rpc.Invalid("extract_fps must be between 0 and %d", maxExtractFPS)
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	rows, err := s.reextractStats(ctx, q, unique(req.GetVideoIds()))
	if err != nil {
		return nil, err
	}

	jobs := make([]river.InsertManyParams, len(rows))
	videoNames := make([]string, len(rows))
	for i, r := range rows {
		if b := impact(r).GetBlocked(); b != "" {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s: %s", r.Name, b))
		}
		if hasLabels(r) && !req.GetDiscardLabels() {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s has labels: %w", r.Name, errHasLabels))
		}
		fps := r.ExtractFps
		if req.ExtractFps != nil {
			fps = req.GetExtractFps()
		}
		_, err := q.QueueReextract(ctx, db.QueueReextractParams{ID: r.ID, ExtractFps: fps})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s changed; try again", r.Name))
		}
		if err != nil {
			return nil, rpc.Internal(err, "queue video")
		}
		details := map[string]any{
			"labeled_frames": r.LabeledFrameCount,
			"tracks":         r.TrackCount,
			"boxes":          r.BoxCount,
		}
		if fps != r.ExtractFps {
			details["extract_fps"] = audit.Change(r.ExtractFps, fps)
		}
		err = audit.Record(ctx, q, audit.Entry{
			Action:     audit.VideoReextract,
			TargetType: audit.TargetVideo,
			TargetID:   strconv.FormatInt(r.ID, 10),
			TargetName: r.Name,
			Details:    details,
		})
		if err != nil {
			return nil, rpc.Internal(err, "record audit entry")
		}
		jobs[i] = river.InsertManyParams{Args: ingest.Args{VideoID: r.ID}}
		videoNames[i] = r.Name
	}
	res, err := s.jobs.InsertManyTx(ctx, tx, jobs)
	if err != nil {
		return nil, rpc.Internal(err, "queue ingest")
	}
	job, err := s.createJob(ctx, q, kindReextract, names(videoNames, "videos"), jobIDs(res))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.ReextractVideosResponse{Job: job}, nil
}

func (s *Service) RegenerateEmbeddings(ctx context.Context, req *krillv1.RegenerateEmbeddingsRequest) (*krillv1.RegenerateEmbeddingsResponse, error) {
	videoIDs, clipIDs := unique(req.GetVideoIds()), unique(req.GetClipIds())
	if err := checkSelection(len(videoIDs) + len(clipIDs)); err != nil {
		return nil, err
	}
	model := s.currentModel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	ids, err := q.ListEmbedClips(ctx, db.ListEmbedClipsParams{
		VideoIds: videoIDs, ClipIds: clipIDs, OnlyStale: req.GetOnlyStale(), Model: model,
	})
	if err != nil {
		return nil, rpc.Internal(err, "list clips")
	}
	if len(ids) == 0 {
		return &krillv1.RegenerateEmbeddingsResponse{}, nil
	}
	if len(ids) > riverListMax {
		return nil, rpc.Invalid("at most %d clips at once", riverListMax)
	}
	jobs := make([]river.InsertManyParams, len(ids))
	for i, id := range ids {
		jobs[i] = river.InsertManyParams{Args: ingest.EmbedArgs{ClipID: id}}
	}
	res, err := s.jobs.InsertManyTx(ctx, tx, jobs)
	if err != nil {
		return nil, rpc.Internal(err, "queue embeddings")
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action: audit.EmbedRegenerate,
		Details: map[string]any{
			"clips":      len(ids),
			"only_stale": req.GetOnlyStale(),
			"model":      model,
			"video_ids":  videoIDs,
			"clip_ids":   clipIDs,
		},
	})
	if err != nil {
		return nil, rpc.Internal(err, "record audit entry")
	}
	summary := fmt.Sprintf("%d clips", len(ids))
	if len(ids) == 1 {
		summary = "1 clip"
	}
	job, err := s.createJob(ctx, q, kindEmbed, summary, jobIDs(res))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.RegenerateEmbeddingsResponse{Clips: int32(len(ids)), Job: job}, nil //nolint:gosec // bounded by riverListMax
}

func (s *Service) ReassignClips(ctx context.Context, req *krillv1.ReassignClipsRequest) (*krillv1.ReassignClipsResponse, error) {
	ids := unique(req.GetClipIds())
	if err := checkSelection(len(ids)); err != nil {
		return nil, err
	}
	var to *db.User
	if id := req.GetUserId(); id != "" {
		uid, err := uuid.Parse(id)
		if err != nil {
			return nil, rpc.Invalid("bad user id")
		}
		u, err := s.q.GetUser(ctx, uid)
		if err != nil {
			return nil, rpc.DBError(err, "user")
		}
		if u.Disabled {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s is disabled", u.Name))
		}
		to = &u
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	clips, err := q.LockLiveClips(ctx, ids)
	if err != nil {
		return nil, rpc.Internal(err, "lock clips")
	}
	if len(clips) != len(ids) {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("clip not found"))
	}
	if to == nil {
		if _, err := q.ReleaseClipClaims(ctx, ids); err != nil {
			return nil, rpc.Internal(err, "release clips")
		}
	} else if _, err := q.AssignClips(ctx, db.AssignClipsParams{ClipIds: ids, UserID: to.ID}); err != nil {
		return nil, rpc.Internal(err, "assign clips")
	}
	changed := 0
	for _, c := range clips {
		e := audit.Entry{
			TargetType: audit.TargetClip,
			TargetID:   strconv.FormatInt(c.ID, 10),
			TargetName: fmt.Sprintf("%s · Clip %d", c.VideoName, c.Idx+1),
		}
		switch {
		case to == nil && !c.ClaimantID.Valid:
			continue
		case to == nil:
			e.Action = audit.ClipRelease
			e.Details = map[string]any{"from": c.ClaimantName.String}
		default:
			e.Action = audit.ClipAssign
			var from any
			if c.ClaimantID.Valid {
				from = c.ClaimantName.String
			}
			e.Details = audit.Change(from, to.Name)
		}
		if err := audit.Record(ctx, q, e); err != nil {
			return nil, rpc.Internal(err, "record audit entry")
		}
		changed++
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.ReassignClipsResponse{Clips: int32(changed)}, nil //nolint:gosec // bounded by maxSelection
}

func jobIDs(res []*rivertype.JobInsertResult) []int64 {
	out := make([]int64, len(res))
	for i, r := range res {
		out[i] = r.Job.ID
	}
	return out
}
