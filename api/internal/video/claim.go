package video

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
)

// ClaimTimeout is how long a claim holds without edits before the queue can
// give the clip to someone else.
const ClaimTimeout = 30 * time.Minute

func StaleBefore(now time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: now.Add(-ClaimTimeout), Valid: true}
}

// Claims maps clip IDs to their claims.
func Claims(rows []db.ListClipClaimsRow, now time.Time) map[int64]*krillv1.ClipClaim {
	stale := StaleBefore(now).Time
	out := make(map[int64]*krillv1.ClipClaim, len(rows))
	for _, r := range rows {
		u := r.User
		out[r.ClipID] = &krillv1.ClipClaim{
			User:     auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt),
			ActiveAt: timestamppb.New(r.ActiveAt.Time),
			Active:   !r.ActiveAt.Time.Before(stale),
		}
	}
	return out
}
