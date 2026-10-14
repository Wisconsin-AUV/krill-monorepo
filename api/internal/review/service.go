package review

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
	"github.com/wauv/krill/api/internal/video"
)

// Reason is why a frame is in review, as stored in reviews.reason.
type Reason string

const (
	ReasonFlagged     Reason = "flagged"
	ReasonLowAccuracy Reason = "low_accuracy"
	ReasonDrift       Reason = "drift"
)

var reasons = map[Reason]krillv1.ReviewReason{
	ReasonFlagged:     krillv1.ReviewReason_REVIEW_REASON_FLAGGED,
	ReasonLowAccuracy: krillv1.ReviewReason_REVIEW_REASON_LOW_ACCURACY,
	ReasonDrift:       krillv1.ReviewReason_REVIEW_REASON_DRIFT,
}

var resolutions = map[krillv1.ReviewResolution]string{
	krillv1.ReviewResolution_REVIEW_RESOLUTION_ACCEPTED:  "accepted",
	krillv1.ReviewResolution_REVIEW_RESOLUTION_FIXED:     "fixed",
	krillv1.ReviewResolution_REVIEW_RESOLUTION_DISMISSED: "dismissed",
}

// Enqueue puts the frame in review unless it already has an open review, and
// returns the frame's open review. userID is who queued it, or uuid.Nil when
// it was queued automatically.
func Enqueue(ctx context.Context, q *db.Queries, frameID int64, reason Reason, userID uuid.UUID) (*krillv1.Review, error) {
	if _, ok := reasons[reason]; !ok {
		return nil, fmt.Errorf("unknown review reason %q", reason)
	}
	err := q.EnqueueReview(ctx, db.EnqueueReviewParams{
		FrameID:   frameID,
		Reason:    string(reason),
		CreatedBy: pgtype.UUID{Bytes: userID, Valid: userID != uuid.Nil},
	})
	if err != nil {
		return nil, fmt.Errorf("enqueue review of frame %d: %w", frameID, err)
	}
	r, err := q.GetOpenReview(ctx, frameID)
	if err != nil {
		return nil, fmt.Errorf("load review of frame %d: %w", frameID, err)
	}
	return toProto(db.ListClipOpenReviewsRow(r)), nil
}

// ClipReviews returns the open review on each frame of the clip, by frame id.
func ClipReviews(ctx context.Context, q *db.Queries, clipID int64) (map[int64]*krillv1.Review, error) {
	rows, err := q.ListClipOpenReviews(ctx, clipID)
	if err != nil {
		return nil, fmt.Errorf("list reviews of clip %d: %w", clipID, err)
	}
	out := make(map[int64]*krillv1.Review, len(rows))
	for _, r := range rows {
		out[r.FrameID] = toProto(r)
	}
	return out, nil
}

type Service struct {
	krillv1connect.UnimplementedReviewServiceHandler
	pool    *pgxpool.Pool
	q       *db.Queries
	present *video.Presenter
}

func NewService(pool *pgxpool.Pool, store *storage.Store) *Service {
	return &Service{pool: pool, q: db.New(pool), present: video.NewPresenter(store)}
}

func (s *Service) FlagFrame(ctx context.Context, req *krillv1.FlagFrameRequest) (*krillv1.FlagFrameResponse, error) {
	if _, err := s.q.GetFrame(ctx, req.GetFrameId()); err != nil {
		return nil, rpc.DBError(err, "frame")
	}
	if !req.GetFlagged() {
		err := s.q.DeleteOwnFlag(ctx, db.DeleteOwnFlagParams{FrameID: req.GetFrameId(), UserID: auth.CallerID(ctx)})
		if err != nil {
			return nil, rpc.Internal(err, "remove flag")
		}
		return &krillv1.FlagFrameResponse{}, nil
	}
	r, err := Enqueue(ctx, s.q, req.GetFrameId(), ReasonFlagged, auth.CallerID(ctx))
	if err != nil {
		return nil, rpc.Internal(err, "flag frame")
	}
	return &krillv1.FlagFrameResponse{Review: r}, nil
}

func (s *Service) ListReviews(ctx context.Context, req *krillv1.ListReviewsRequest) (*krillv1.ListReviewsResponse, error) {
	var filter pgtype.Text
	if req.GetReason() != krillv1.ReviewReason_REVIEW_REASON_UNSPECIFIED {
		name, ok := reasonName(req.GetReason())
		if !ok {
			return nil, rpc.Invalid("unknown reason")
		}
		filter = pgtype.Text{String: string(name), Valid: true}
	}
	rows, err := s.q.ListOpenReviews(ctx, filter)
	if err != nil {
		return nil, rpc.Internal(err, "list reviews")
	}
	out := make([]*krillv1.ReviewItem, len(rows))
	for i, r := range rows {
		url, err := s.present.FrameURL(ctx, r.VideoID, r.FrameIdx)
		if err != nil {
			return nil, rpc.Internal(err, "sign frame url")
		}
		out[i] = &krillv1.ReviewItem{
			Review: toProto(db.ListClipOpenReviewsRow{
				ID: r.ID, FrameID: r.FrameID, Reason: r.Reason, CreatedAt: r.CreatedAt,
				CreatorID: r.CreatorID, CreatorName: r.CreatorName, CreatorUsername: r.CreatorUsername,
				CreatorRole: r.CreatorRole, CreatorCreatedAt: r.CreatorCreatedAt,
			}),
			ClipId:       r.ClipID,
			ClipIndex:    r.ClipIdx,
			FrameIndex:   r.FrameIdx,
			VideoName:    r.VideoName,
			ThumbnailUrl: url,
			Labeler:      profile(r.LabelerID, r.LabelerName, r.LabelerUsername, r.LabelerRole, r.LabelerCreatedAt),
		}
	}
	return &krillv1.ListReviewsResponse{Reviews: out}, nil
}

func (s *Service) CountReviews(ctx context.Context, _ *krillv1.CountReviewsRequest) (*krillv1.CountReviewsResponse, error) {
	n, err := s.q.CountOpenReviews(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "count reviews")
	}
	return &krillv1.CountReviewsResponse{Count: n}, nil
}

func (s *Service) ResolveReview(ctx context.Context, req *krillv1.ResolveReviewRequest) (*krillv1.ResolveReviewResponse, error) {
	resolution, ok := resolutions[req.GetResolution()]
	if !ok {
		return nil, rpc.Invalid("unknown resolution")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	n, err := q.ResolveReview(ctx, db.ResolveReviewParams{
		ID:         req.GetId(),
		ResolvedBy: auth.CallerID(ctx),
		Resolution: resolution,
	})
	if err != nil {
		return nil, rpc.Internal(err, "resolve review")
	}
	if n > 0 {
		name, err := audit.ReviewName(ctx, q, req.GetId())
		if err != nil {
			return nil, rpc.Internal(err, "name review")
		}
		err = audit.Record(ctx, q, audit.Entry{
			Action:     audit.ReviewResolve,
			TargetType: audit.TargetReview,
			TargetID:   strconv.FormatInt(req.GetId(), 10),
			TargetName: name,
			Details:    map[string]any{"resolution": resolution},
		})
		if err != nil {
			return nil, rpc.Internal(err, "record audit entry")
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, rpc.Internal(err, "commit")
		}
		return &krillv1.ResolveReviewResponse{}, nil
	}
	exists, err := s.q.ReviewExists(ctx, req.GetId())
	if err != nil {
		return nil, rpc.Internal(err, "load review")
	}
	if !exists {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("review not found"))
	}
	return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("review is already resolved"))
}

func reasonName(r krillv1.ReviewReason) (Reason, bool) {
	for name, v := range reasons {
		if v == r {
			return name, true
		}
	}
	return "", false
}

func toProto(r db.ListClipOpenReviewsRow) *krillv1.Review {
	return &krillv1.Review{
		Id:        r.ID,
		FrameId:   r.FrameID,
		Reason:    reasons[Reason(r.Reason)],
		CreatedBy: profile(r.CreatorID, r.CreatorName, r.CreatorUsername, r.CreatorRole, r.CreatorCreatedAt),
		CreatedAt: timestamppb.New(r.CreatedAt.Time),
	}
}

func profile(id pgtype.UUID, name, username, role pgtype.Text, createdAt pgtype.Timestamptz) *krillv1.Profile {
	if !id.Valid {
		return nil
	}
	return auth.PublicProfile(id.Bytes, name.String, username.String, role.String, createdAt)
}
