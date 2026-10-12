package gold

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

const (
	defaultAttemptPage = 20
	maxAttemptPage     = 100
)

var errNotYourAttempt = connect.NewError(connect.CodePermissionDenied, errors.New("you can only see your own gold checks"))

// canSee reports whether the caller may see attempts made by userID.
func canSee(ctx context.Context, userID uuid.UUID) bool {
	return userID == auth.CallerID(ctx) || auth.CallerCan(ctx, krillv1.Permission_PERMISSION_REVIEW_LABELS)
}

func meanIoU(iouSum float64, matched int32) float64 {
	if matched == 0 {
		return 0
	}
	return iouSum / float64(matched)
}

func (s *Service) GetGoldAttempt(ctx context.Context, req *krillv1.GetGoldAttemptRequest) (*krillv1.GetGoldAttemptResponse, error) {
	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, rpc.Invalid("bad user id")
	}
	if !canSee(ctx, userID) {
		return nil, errNotYourAttempt
	}
	a, err := s.q.GetGoldAttempt(ctx, db.GetGoldAttemptParams{FrameID: req.GetFrameId(), UserID: userID})
	if err != nil {
		return nil, rpc.DBError(err, "gold attempt")
	}
	var reference, answer []Box
	if err := json.Unmarshal(a.Reference, &reference); err != nil {
		return nil, rpc.Internal(err, "decode gold boxes")
	}
	if err := json.Unmarshal(a.Boxes, &answer); err != nil {
		return nil, rpc.Internal(err, "decode attempt boxes")
	}
	url, err := s.present.FrameURL(ctx, a.VideoID, a.Idx)
	if err != nil {
		return nil, rpc.Internal(err, "sign frame url")
	}
	// Matches are recomputed because only their totals are stored. The answer
	// key is frozen when the frame is made gold, so they come out the same.
	result := resultToProto(reference, answer, Score(reference, answer))
	result.Score = a.Score
	return &krillv1.GetGoldAttemptResponse{
		Attempt: &krillv1.GoldAttempt{
			FrameId:   a.FrameID,
			User:      auth.PublicProfile(a.UserID, a.Name, a.Username, a.Role, a.UserCreatedAt),
			ClipId:    a.ClipID,
			Index:     a.Idx,
			VideoName: a.VideoName,
			Score:     a.Score,
			MeanIou:   meanIoU(a.IouSum, a.Matched),
			CreatedAt: timestamppb.New(a.CreatedAt.Time),
		},
		Result: result,
		Url:    url,
		Width:  a.Width,
		Height: a.Height,
	}, nil
}

// attemptCursor is the last attempt of a page, in list order.
type attemptCursor struct {
	at      time.Time
	frameID int64
	userID  uuid.UUID
}

func (c attemptCursor) String() string {
	return fmt.Sprintf("%d.%d.%s", c.at.UnixMicro(), c.frameID, c.userID)
}

func parseAttemptCursor(token string) (attemptCursor, error) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return attemptCursor{}, errors.New("bad page token")
	}
	micros, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return attemptCursor{}, errors.New("bad page token")
	}
	frameID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return attemptCursor{}, errors.New("bad page token")
	}
	userID, err := uuid.Parse(parts[2])
	if err != nil {
		return attemptCursor{}, errors.New("bad page token")
	}
	return attemptCursor{at: time.UnixMicro(micros), frameID: frameID, userID: userID}, nil
}

func (s *Service) ListGoldAttempts(ctx context.Context, req *krillv1.ListGoldAttemptsRequest) (*krillv1.ListGoldAttemptsResponse, error) {
	size := req.GetPageSize()
	switch {
	case size < 0:
		return nil, rpc.Invalid("page size must not be negative")
	case size == 0:
		size = defaultAttemptPage
	case size > maxAttemptPage:
		size = maxAttemptPage
	}
	params := db.ListGoldAttemptsParams{Lim: size + 1}
	if req.GetUserId() == "" {
		if !auth.CallerCan(ctx, krillv1.Permission_PERMISSION_REVIEW_LABELS) {
			return nil, errNotYourAttempt
		}
	} else {
		userID, err := uuid.Parse(req.GetUserId())
		if err != nil {
			return nil, rpc.Invalid("bad user id")
		}
		if !canSee(ctx, userID) {
			return nil, errNotYourAttempt
		}
		params.UserID = pgtype.UUID{Bytes: userID, Valid: true}
	}
	if req.GetFrameId() != 0 {
		params.FrameID = pgtype.Int8{Int64: req.GetFrameId(), Valid: true}
	}
	if req.GetPageToken() != "" {
		c, err := parseAttemptCursor(req.GetPageToken())
		if err != nil {
			return nil, rpc.Invalid("%s", err)
		}
		params.BeforeAt = pgtype.Timestamptz{Time: c.at, Valid: true}
		params.BeforeFrameID = pgtype.Int8{Int64: c.frameID, Valid: true}
		params.BeforeUserID = pgtype.UUID{Bytes: c.userID, Valid: true}
	}

	rows, err := s.q.ListGoldAttempts(ctx, params)
	if err != nil {
		return nil, rpc.Internal(err, "list gold attempts")
	}
	out := &krillv1.ListGoldAttemptsResponse{}
	if len(rows) > int(size) {
		rows = rows[:size]
		last := rows[len(rows)-1]
		out.NextPageToken = attemptCursor{at: last.CreatedAt.Time, frameID: last.FrameID, userID: last.UserID}.String()
	}
	out.Attempts = make([]*krillv1.GoldAttempt, len(rows))
	for i, r := range rows {
		out.Attempts[i] = &krillv1.GoldAttempt{
			FrameId:   r.FrameID,
			User:      auth.PublicProfile(r.UserID, r.Name, r.Username, r.Role, r.UserCreatedAt),
			ClipId:    r.ClipID,
			Index:     r.Idx,
			VideoName: r.VideoName,
			Score:     r.Score,
			MeanIou:   meanIoU(r.IouSum, r.Matched),
			CreatedAt: timestamppb.New(r.CreatedAt.Time),
		}
	}
	return out, nil
}
