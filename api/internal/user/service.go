package user

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

var errSelf = connect.NewError(connect.CodeFailedPrecondition,
	errors.New("you can't change your own role, disable, or delete yourself; ask another admin"))

type Service struct {
	krillv1connect.UnimplementedUserServiceHandler
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, q: db.New(pool)}
}

// target parses the user ID and rejects the caller acting on themselves,
// which keeps at least one admin around.
func target(ctx context.Context, id string) (uuid.UUID, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, rpc.Invalid("invalid user id")
	}
	if sess, ok := auth.SessionFrom(ctx); ok && sess.User.ID == uid {
		return uuid.Nil, errSelf
	}
	return uid, nil
}

func (s *Service) ListUsers(ctx context.Context, _ *krillv1.ListUsersRequest) (*krillv1.ListUsersResponse, error) {
	rows, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list users")
	}
	out := make([]*krillv1.User, len(rows))
	for i, u := range rows {
		out[i] = auth.ToProto(u)
	}
	return &krillv1.ListUsersResponse{Users: out}, nil
}

func (s *Service) UpdateUser(ctx context.Context, req *krillv1.UpdateUserRequest) (*krillv1.UpdateUserResponse, error) {
	id, err := target(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	u, err := q.GetUser(ctx, id)
	if err != nil {
		return nil, rpc.DBError(err, "user")
	}
	if req.Role != nil {
		name, ok := auth.RoleName(req.GetRole())
		if !ok {
			return nil, rpc.Invalid("unknown role")
		}
		if u, err = q.SetUserRole(ctx, db.SetUserRoleParams{ID: id, Role: name}); err != nil {
			return nil, rpc.Internal(err, "set role")
		}
	}
	if req.Disabled != nil {
		if u, err = q.SetUserDisabled(ctx, db.SetUserDisabledParams{ID: id, Disabled: req.GetDisabled()}); err != nil {
			return nil, rpc.Internal(err, "set disabled")
		}
		if req.GetDisabled() {
			if err := q.DeleteUserSessions(ctx, id); err != nil {
				return nil, rpc.Internal(err, "delete sessions")
			}
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.UpdateUserResponse{User: auth.ToProto(u)}, nil
}

func (s *Service) SetUserPassword(ctx context.Context, req *krillv1.SetUserPasswordRequest) (*krillv1.SetUserPasswordResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, rpc.Invalid("invalid user id")
	}
	if err := auth.ValidatePassword(req.GetPassword()); err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	hash, err := auth.HashPassword(req.GetPassword())
	if err != nil {
		return nil, rpc.Internal(err, "hash password")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	n, err := q.SetUserPassword(ctx, db.SetUserPasswordParams{ID: id, PasswordHash: pgtype.Text{String: hash, Valid: true}})
	if err != nil {
		return nil, rpc.Internal(err, "set password")
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	if err := q.DeleteUserSessions(ctx, id); err != nil {
		return nil, rpc.Internal(err, "delete sessions")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.SetUserPasswordResponse{}, nil
}

func (s *Service) DeleteUser(ctx context.Context, req *krillv1.DeleteUserRequest) (*krillv1.DeleteUserResponse, error) {
	id, err := target(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	n, err := s.q.DeleteUser(ctx, id)
	if err != nil {
		return nil, rpc.Internal(err, "delete user")
	}
	if n == 0 {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
	}
	return &krillv1.DeleteUserResponse{}, nil
}
