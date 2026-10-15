package audit

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

const (
	defaultPage = 50
	maxPage     = 200
)

type Service struct {
	krillv1connect.UnimplementedAuditServiceHandler
	q *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{q: db.New(pool)}
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func (s *Service) ListAuditLog(ctx context.Context, req *krillv1.ListAuditLogRequest) (*krillv1.ListAuditLogResponse, error) {
	size := req.GetPageSize()
	switch {
	case size < 0:
		return nil, rpc.Invalid("page size must not be negative")
	case size == 0:
		size = defaultPage
	case size > maxPage:
		size = maxPage
	}
	params := db.ListAuditLogParams{Lim: size + 1}
	if req.GetActorId() != "" {
		id, err := uuid.Parse(req.GetActorId())
		if err != nil {
			return nil, rpc.Invalid("bad actor id")
		}
		params.Actor = pgtype.UUID{Bytes: id, Valid: true}
	}
	if p := req.GetActionPrefix(); p != "" {
		params.ActionPattern = pgtype.Text{String: likeEscaper.Replace(p) + "%", Valid: true}
	}
	if (req.GetTargetType() == "") != (req.GetTargetId() == "") {
		return nil, rpc.Invalid("set both target type and target id")
	}
	if req.GetTargetType() != "" {
		params.TargetType = pgtype.Text{String: req.GetTargetType(), Valid: true}
		params.TargetID = pgtype.Text{String: req.GetTargetId(), Valid: true}
	}
	if t := req.GetPageToken(); t != "" {
		id, err := strconv.ParseInt(t, 10, 64)
		if err != nil {
			return nil, rpc.Invalid("bad page token")
		}
		params.BeforeID = pgtype.Int8{Int64: id, Valid: true}
	}

	rows, err := s.q.ListAuditLog(ctx, params)
	if err != nil {
		return nil, rpc.Internal(err, "list audit log")
	}
	out := &krillv1.ListAuditLogResponse{}
	if len(rows) > int(size) {
		rows = rows[:size]
		out.NextPageToken = strconv.FormatInt(rows[len(rows)-1].ID, 10)
	}
	out.Entries = make([]*krillv1.AuditEntry, len(rows))
	for i, r := range rows {
		var details map[string]any
		if err := json.Unmarshal(r.Details, &details); err != nil {
			return nil, rpc.Internal(err, "decode audit details")
		}
		st, err := structpb.NewStruct(details)
		if err != nil {
			return nil, rpc.Internal(err, "convert audit details")
		}
		e := &krillv1.AuditEntry{
			Id:         r.ID,
			At:         timestamppb.New(r.At.Time),
			ActorName:  r.ActorName,
			Action:     r.Action,
			TargetType: r.TargetType,
			TargetId:   r.TargetID,
			TargetName: r.TargetName,
			Details:    st,
			Ip:         r.Ip,
		}
		if r.ActorID.Valid {
			e.Actor = auth.PublicProfile(r.ActorID.Bytes, r.ActorUserName.String, r.ActorUsername.String, r.ActorRole.String, r.ActorCreatedAt)
		}
		out.Entries[i] = e
	}
	return out, nil
}

func (s *Service) ListAuditFilters(ctx context.Context, _ *krillv1.ListAuditFiltersRequest) (*krillv1.ListAuditFiltersResponse, error) {
	actions, err := s.q.ListAuditActions(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list audit actions")
	}
	users, err := s.q.ListAuditActors(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list audit actors")
	}
	actors := make([]*krillv1.Profile, len(users))
	for i, u := range users {
		actors[i] = auth.PublicProfile(u.ID, u.Name, u.Username, u.Role, u.CreatedAt)
	}
	return &krillv1.ListAuditFiltersResponse{Actions: actions, Actors: actors}, nil
}
