package stats

import (
	"context"
	"strings"
	"time"

	// The API image has no system time zone database.
	_ "time/tzdata"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

type Service struct {
	krillv1connect.UnimplementedStatsServiceHandler
	q *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: db.New(pool)}
}

func contributions(boxes, frames int64) *krillv1.Contributions {
	return &krillv1.Contributions{Boxes: boxes, Frames: frames, Total: boxes + frames}
}

func (s *Service) leaderboard(ctx context.Context, p krillv1.Period, loc *time.Location) (*krillv1.GetLeaderboardResponse, error) {
	start, bounded, err := since(p, time.Now().In(loc))
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	rows, err := s.q.Leaderboard(ctx, pgtype.Timestamptz{Time: start, Valid: bounded})
	if err != nil {
		return nil, rpc.Internal(err, "load leaderboard")
	}

	out := &krillv1.GetLeaderboardResponse{Entries: make([]*krillv1.LeaderboardEntry, len(rows))}
	if bounded {
		out.Since = timestamppb.New(start)
	}
	var boxes, frames int64
	var rank int32
	for i, r := range rows {
		c := contributions(r.Boxes, r.Frames)
		if i == 0 || c.Total != out.Entries[i-1].Contributions.Total {
			rank = int32(i + 1) //nolint:gosec // bounded by the number of users
		}
		out.Entries[i] = &krillv1.LeaderboardEntry{Rank: rank, User: auth.PublicProfile(r.ID, r.Name, r.Username, r.Role, r.CreatedAt), Contributions: c}
		boxes += r.Boxes
		frames += r.Frames
	}
	out.Team = contributions(boxes, frames)
	return out, nil
}

func (s *Service) GetLeaderboard(ctx context.Context, req *krillv1.GetLeaderboardRequest) (*krillv1.GetLeaderboardResponse, error) {
	loc, err := location(req.GetTimeZone())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	return s.leaderboard(ctx, req.GetPeriod(), loc)
}

func (s *Service) GetProfile(ctx context.Context, req *krillv1.GetProfileRequest) (*krillv1.GetProfileResponse, error) {
	loc, err := location(req.GetTimeZone())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	u, err := s.q.GetUserByUsername(ctx, strings.ToLower(req.GetUsername()))
	if err != nil {
		return nil, rpc.DBError(err, "user")
	}
	out := &krillv1.GetProfileResponse{User: auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt)}

	for _, p := range periods {
		board, err := s.leaderboard(ctx, p, loc)
		if err != nil {
			return nil, err
		}
		ps := &krillv1.PeriodStats{
			Period:        p,
			Contributions: contributions(0, 0),
			Contributors:  int32(len(board.Entries)), //nolint:gosec // bounded by the number of users
		}
		for _, e := range board.Entries {
			if e.User.Id == out.User.Id {
				ps.Contributions = e.Contributions
				ps.Rank = e.Rank
				break
			}
		}
		out.Periods = append(out.Periods, ps)
	}

	rows, err := s.q.UserContributionDays(ctx, db.UserContributionDaysParams{UserID: u.ID, TimeZone: loc.String()})
	if err != nil {
		return nil, rpc.Internal(err, "load contribution days")
	}
	now := time.Now().In(loc)
	today, err := time.Parse(dateLayout, now.Format(dateLayout))
	if err != nil {
		return nil, rpc.Internal(err, "parse today")
	}
	gridStart := startOfWeek(today).AddDate(0, 0, -52*7)
	days := make([]time.Time, len(rows))
	for i, r := range rows {
		d, err := time.Parse(dateLayout, r.Day)
		if err != nil {
			return nil, rpc.Internal(err, "parse contribution day")
		}
		days[i] = d
		if out.BestDay == nil || r.Count > out.BestDay.Count {
			out.BestDay = &krillv1.ContributionDay{Date: r.Day, Count: r.Count}
		}
		if !d.Before(gridStart) {
			out.Days = append(out.Days, &krillv1.ContributionDay{Date: r.Day, Count: r.Count})
		}
	}
	st := countStreaks(days, today)
	out.CurrentStreak = st.current
	out.LongestStreak = st.longest
	out.ActiveDays = int32(len(days)) //nolint:gosec // at most one per calendar day

	if out.Clips, err = s.q.UserClipCount(ctx, u.ID); err != nil {
		return nil, rpc.Internal(err, "count clips")
	}
	types, err := s.q.UserLabelTypeCounts(ctx, u.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count label types")
	}
	for _, t := range types {
		out.LabelTypes = append(out.LabelTypes, &krillv1.LabelTypeCount{
			LabelTypeId: t.ID, Name: t.Name, Color: t.Color, Boxes: t.Boxes,
		})
	}
	return out, nil
}
