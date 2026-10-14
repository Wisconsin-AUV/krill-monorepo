package user

import (
	"bytes"
	"context"
	"errors"

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
)

type SessionService struct {
	krillv1connect.UnimplementedSessionServiceHandler
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewSessionService(pool *pgxpool.Pool) *SessionService {
	return &SessionService{pool: pool, q: db.New(pool)}
}

func sessionProto(r db.ListSessionsRow) *krillv1.Session {
	return &krillv1.Session{
		Id:         r.ID.String(),
		User:       auth.PublicProfile(r.UserID, r.UserName, r.Username, r.Role, r.UserCreatedAt),
		Ip:         r.Ip,
		Device:     auth.DescribeDevice(r.UserAgent),
		UserAgent:  r.UserAgent,
		CreatedAt:  timestamppb.New(r.CreatedAt.Time),
		LastSeenAt: timestamppb.New(r.LastSeenAt.Time),
		ExpiresAt:  timestamppb.New(r.ExpiresAt.Time),
		Current:    r.Current,
	}
}

func (s *SessionService) list(ctx context.Context, userID pgtype.UUID) ([]*krillv1.Session, error) {
	sess, _ := auth.SessionFrom(ctx)
	// Not nil, which would be NULL and make current NULL too.
	current := append([]byte{}, sess.TokenHash...)
	rows, err := s.q.ListSessions(ctx, db.ListSessionsParams{Current: current, UserID: userID})
	if err != nil {
		return nil, rpc.Internal(err, "list sessions")
	}
	out := make([]*krillv1.Session, len(rows))
	for i, r := range rows {
		out[i] = sessionProto(r)
	}
	return out, nil
}

func (s *SessionService) ListSessions(ctx context.Context, req *krillv1.ListSessionsRequest) (*krillv1.ListSessionsResponse, error) {
	var userID pgtype.UUID
	if req.GetUserId() != "" {
		id, err := uuid.Parse(req.GetUserId())
		if err != nil {
			return nil, rpc.Invalid("invalid user id")
		}
		userID = pgtype.UUID{Bytes: id, Valid: true}
	}
	out, err := s.list(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &krillv1.ListSessionsResponse{Sessions: out}, nil
}

func (s *SessionService) ListMySessions(ctx context.Context, _ *krillv1.ListMySessionsRequest) (*krillv1.ListMySessionsResponse, error) {
	out, err := s.list(ctx, pgtype.UUID{Bytes: auth.CallerID(ctx), Valid: true})
	if err != nil {
		return nil, err
	}
	return &krillv1.ListMySessionsResponse{Sessions: out}, nil
}

func (s *SessionService) RevokeSession(ctx context.Context, req *krillv1.RevokeSessionRequest) (*krillv1.RevokeSessionResponse, error) {
	if err := s.revoke(ctx, req.GetId(), pgtype.UUID{}); err != nil {
		return nil, err
	}
	return &krillv1.RevokeSessionResponse{}, nil
}

func (s *SessionService) RevokeMySession(ctx context.Context, req *krillv1.RevokeMySessionRequest) (*krillv1.RevokeMySessionResponse, error) {
	if err := s.revoke(ctx, req.GetId(), pgtype.UUID{Bytes: auth.CallerID(ctx), Valid: true}); err != nil {
		return nil, err
	}
	return &krillv1.RevokeMySessionResponse{}, nil
}

var errCurrentSession = connect.NewError(connect.CodeFailedPrecondition, errors.New("sign out to end this session"))

// revoke deletes a session, only one of owner's when owner is set. The
// caller's own session is left alone, so nobody cuts off the request that
// shows them the result.
func (s *SessionService) revoke(ctx context.Context, rawID string, owner pgtype.UUID) error {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return rpc.Invalid("invalid session id")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	row, err := q.DeleteSessionByID(ctx, db.DeleteSessionByIDParams{ID: id, UserID: owner})
	if err != nil {
		return rpc.DBError(err, "session")
	}
	if sess, _ := auth.SessionFrom(ctx); len(sess.TokenHash) > 0 && bytes.Equal(row.TokenHash, sess.TokenHash) {
		return errCurrentSession
	}
	u, err := q.GetUser(ctx, row.UserID)
	if err != nil {
		return rpc.Internal(err, "load user")
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.SessionRevoke,
		TargetType: audit.TargetUser,
		TargetID:   u.ID.String(),
		TargetName: u.Name,
		Details:    map[string]any{"device": auth.DescribeDevice(row.UserAgent), "ip": row.Ip},
	})
	if err != nil {
		return rpc.Internal(err, "record audit entry")
	}
	if err := tx.Commit(ctx); err != nil {
		return rpc.Internal(err, "commit")
	}
	return nil
}
