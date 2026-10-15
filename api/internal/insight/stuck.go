package insight

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

const maxIdleDays = 365

func (s *Service) idleBefore(days int32) (int32, pgtype.Timestamptz, error) {
	if days == 0 {
		days = int32(min(s.settings.Config().StuckClaimDays, maxIdleDays)) //nolint:gosec // capped above
	}
	if days < 1 || days > maxIdleDays {
		return 0, pgtype.Timestamptz{}, rpc.Invalid("idle days must be between 1 and %d", maxIdleDays)
	}
	return days, pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -int(days)), Valid: true}, nil
}

func (s *Service) ListStuckClips(ctx context.Context, req *krillv1.ListStuckClipsRequest) (*krillv1.ListStuckClipsResponse, error) {
	days, before, err := s.idleBefore(req.GetIdleDays())
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListStuckClips(ctx, before)
	if err != nil {
		return nil, rpc.Internal(err, "list stuck clips")
	}
	out := &krillv1.ListStuckClipsResponse{IdleDays: days, Clips: make([]*krillv1.StuckClip, len(rows))}
	for i, r := range rows {
		u := r.User
		out.Clips[i] = &krillv1.StuckClip{
			ClipId:        r.ClipID,
			ClipIndex:     r.ClipIdx,
			VideoId:       r.VideoID,
			VideoName:     r.VideoName,
			Claimant:      auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt),
			ActiveAt:      timestamppb.New(r.ActiveAt.Time),
			FrameCount:    r.FrameCount,
			LabeledFrames: r.LabeledFrames,
			Boxes:         r.Boxes,
		}
	}
	return out, nil
}

func (s *Service) ReleaseStuckClips(ctx context.Context, req *krillv1.ReleaseStuckClipsRequest) (*krillv1.ReleaseStuckClipsResponse, error) {
	if len(req.GetClipIds()) == 0 {
		return nil, rpc.Invalid("no clips")
	}
	_, before, err := s.idleBefore(req.GetIdleDays())
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	released, err := q.ReleaseStuckClaims(ctx, db.ReleaseStuckClaimsParams{ClipIds: req.GetClipIds(), IdleBefore: before})
	if err != nil {
		return nil, rpc.Internal(err, "release claims")
	}
	for _, r := range released {
		name, err := audit.ClipName(ctx, q, r.ClipID)
		if err != nil {
			return nil, rpc.Internal(err, "name clip")
		}
		err = audit.Record(ctx, q, audit.Entry{
			Action:     audit.ClipRelease,
			TargetType: audit.TargetClip,
			TargetID:   strconv.FormatInt(r.ClipID, 10),
			TargetName: name,
			Details: map[string]any{
				"claimant":    r.UserName,
				"last_edited": r.ActiveAt.Time.UTC().Format(time.RFC3339),
			},
		})
		if err != nil {
			return nil, rpc.Internal(err, "record audit entry")
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.ReleaseStuckClipsResponse{Released: int32(len(released))}, nil //nolint:gosec // bounded by the request
}
