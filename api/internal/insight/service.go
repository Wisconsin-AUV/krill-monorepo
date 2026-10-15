// Package insight backs the admin panel's labeling pages: labeler stats,
// dataset coverage, and stuck clips.
package insight

import (
	"cmp"
	"context"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/settings"
	"github.com/wauv/krill/api/internal/stats"
)

type Service struct {
	krillv1connect.UnimplementedInsightServiceHandler
	pool     *pgxpool.Pool
	q        *db.Queries
	settings *settings.Store
}

func NewService(pool *pgxpool.Pool, store *settings.Store) *Service {
	return &Service{pool: pool, q: db.New(pool), settings: store}
}

// activeGapCap bounds how much of the gap between two of a labeler's actions
// counts as labeling. Nothing records when someone starts or stops, so active
// time is pieced together from the timestamps their work leaves: boxes they
// drew or last changed, frames they marked, and proposals they decided. Each
// gap between consecutive actions counts in full up to the cap, and only the
// cap beyond it, so a lunch break adds two minutes rather than an hour while
// the thinking before an action after a short pause still counts. Moving the
// same box twice only keeps the last move, so the result undercounts slightly.
const activeGapCap = 2 * time.Minute

type period struct {
	loc   *time.Location
	start time.Time
	since pgtype.Timestamptz
}

func readPeriod(p krillv1.Period, zone string) (period, error) {
	loc, err := stats.Location(zone)
	if err != nil {
		return period{}, rpc.Invalid("%s", err)
	}
	start, bounded, err := stats.PeriodStart(p, time.Now().In(loc))
	if err != nil {
		return period{}, rpc.Invalid("%s", err)
	}
	return period{loc: loc, start: start, since: pgtype.Timestamptz{Time: start, Valid: bounded}}, nil
}

func (s *Service) GetLabelerStats(ctx context.Context, req *krillv1.GetLabelerStatsRequest) (*krillv1.GetLabelerStatsResponse, error) {
	p, err := readPeriod(req.GetPeriod(), req.GetTimeZone())
	if err != nil {
		return nil, err
	}
	by := map[uuid.UUID]*krillv1.LabelerStats{}
	get := func(id uuid.UUID) *krillv1.LabelerStats {
		if l, ok := by[id]; ok {
			return l
		}
		l := &krillv1.LabelerStats{}
		by[id] = l
		return l
	}

	frames, err := s.q.LabelerFrameCounts(ctx, p.since)
	if err != nil {
		return nil, rpc.Internal(err, "count frames")
	}
	for _, r := range frames {
		l := get(r.UserID)
		l.FramesLabeled, l.FramesEmpty = r.Labeled, r.Empty
		l.FramesMarked += r.Labeled + r.Empty
	}
	boxes, err := s.q.LabelerBoxesDrawn(ctx, p.since)
	if err != nil {
		return nil, rpc.Internal(err, "count boxes")
	}
	for _, r := range boxes {
		get(r.UserID).BoxesDrawn = r.Boxes
	}
	proposals, err := s.q.LabelerProposalCounts(ctx, p.since)
	if err != nil {
		return nil, rpc.Internal(err, "count proposals")
	}
	for _, r := range proposals {
		l := get(r.UserID)
		l.Proposals = append(l.Proposals, &krillv1.ProposalCounts{
			Source: r.Source, Accepted: r.Accepted, Edited: r.Edited, Rejected: r.Rejected,
		})
	}
	corrections, err := s.q.LabelerCorrections(ctx, p.since)
	if err != nil {
		return nil, rpc.Internal(err, "count corrections")
	}
	for _, r := range corrections {
		l := get(r.UserID)
		l.FramesCorrected, l.BoxesCorrected = r.Frames, r.Boxes
		l.FramesMarked += r.Reopened
	}
	active, err := s.q.LabelerActiveSeconds(ctx, db.LabelerActiveSecondsParams{Since: p.since, GapCap: activeGapCap.Seconds()})
	if err != nil {
		return nil, rpc.Internal(err, "measure active time")
	}
	for _, r := range active {
		if r.Seconds > 0 {
			get(r.UserID).ActiveSeconds = r.Seconds
		}
	}

	ids := make([]uuid.UUID, 0, len(by))
	for id := range by {
		ids = append(ids, id)
	}
	users, err := s.q.ListUsersByID(ctx, ids)
	if err != nil {
		return nil, rpc.Internal(err, "load users")
	}
	out := &krillv1.GetLabelerStatsResponse{Labelers: make([]*krillv1.LabelerStats, 0, len(users))}
	if p.since.Valid {
		out.Since = timestamppb.New(p.start)
	}
	for _, u := range users {
		l := by[u.ID]
		l.User = auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt)
		out.Labelers = append(out.Labelers, l)
	}
	slices.SortFunc(out.Labelers, func(a, b *krillv1.LabelerStats) int {
		return cmp.Or(cmp.Compare(b.GetFramesMarked(), a.GetFramesMarked()), cmp.Compare(a.GetUser().GetName(), b.GetUser().GetName()))
	})
	return out, nil
}

// bucketFor picks trend buckets that give a readable number of points.
func bucketFor(p krillv1.Period) (krillv1.TrendBucket, string) {
	switch p {
	case krillv1.Period_PERIOD_DAY:
		return krillv1.TrendBucket_TREND_BUCKET_HOUR, "hour"
	case krillv1.Period_PERIOD_ALL_TIME:
		return krillv1.TrendBucket_TREND_BUCKET_WEEK, "week"
	}
	return krillv1.TrendBucket_TREND_BUCKET_DAY, "day"
}

// truncate matches Postgres's date_trunc, whose weeks start on Monday.
func truncate(t time.Time, unit string) time.Time {
	y, m, d := t.Date()
	switch unit {
	case "hour":
		return time.Date(y, m, d, t.Hour(), 0, 0, 0, t.Location())
	case "week":
		day := time.Date(y, m, d, 0, 0, 0, 0, t.Location())
		return day.AddDate(0, 0, -((int(t.Weekday()) + 6) % 7))
	}
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func next(t time.Time, unit string) time.Time {
	switch unit {
	case "hour":
		return t.Add(time.Hour)
	case "week":
		return t.AddDate(0, 0, 7)
	}
	return t.AddDate(0, 0, 1)
}

// fillBuckets returns a point for every bucket from the first to the one
// containing now, with zeros where nothing happened.
func fillBuckets(rows []db.LabelerTrendRow, first, now time.Time, unit string) []*krillv1.TrendPoint {
	byStart := make(map[int64]db.LabelerTrendRow, len(rows))
	for _, r := range rows {
		byStart[r.Bucket.Time.Unix()] = r
	}
	var out []*krillv1.TrendPoint
	for b := truncate(first, unit); !b.After(now); b = next(b, unit) {
		r := byStart[b.Unix()]
		out = append(out, &krillv1.TrendPoint{
			Start:           timestamppb.New(b),
			Frames:          r.Frames,
			BoxesDrawn:      r.Boxes,
			ProposalsKept:   r.Kept,
			FramesCorrected: r.FramesCorrected,
			ActiveSeconds:   r.Seconds,
		})
	}
	return out
}

func (s *Service) GetLabelerTrend(ctx context.Context, req *krillv1.GetLabelerTrendRequest) (*krillv1.GetLabelerTrendResponse, error) {
	id, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, rpc.Invalid("bad user id")
	}
	p, err := readPeriod(req.GetPeriod(), req.GetTimeZone())
	if err != nil {
		return nil, err
	}
	u, err := s.q.GetUser(ctx, id)
	if err != nil {
		return nil, rpc.DBError(err, "user")
	}
	bucket, unit := bucketFor(req.GetPeriod())
	rows, err := s.q.LabelerTrend(ctx, db.LabelerTrendParams{
		UserID: id, Since: p.since, Unit: unit, TimeZone: p.loc.String(), GapCap: activeGapCap.Seconds(),
	})
	if err != nil {
		return nil, rpc.Internal(err, "load trend")
	}
	kinds, err := s.q.LabelerCorrectionKinds(ctx, db.LabelerCorrectionKindsParams{UserID: id, Since: p.since})
	if err != nil {
		return nil, rpc.Internal(err, "count corrections")
	}

	out := &krillv1.GetLabelerTrendResponse{
		User:   auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt),
		Bucket: bucket,
	}
	now := time.Now().In(p.loc)
	first := p.start
	if !p.since.Valid {
		first = now
		if len(rows) > 0 {
			first = rows[0].Bucket.Time.In(p.loc)
		}
	}
	out.Points = fillBuckets(rows, first, now, unit)
	for _, k := range kinds {
		out.Corrections = append(out.Corrections, &krillv1.CorrectionCount{Kind: k.Kind, Boxes: k.Boxes})
	}
	return out, nil
}
