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
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

var errSelf = connect.NewError(connect.CodeFailedPrecondition,
	errors.New("you can't change your own role, disable, or delete yourself; ask another admin"))

// PasswordResetter emails a user a password reset code with the caller's
// transaction.
type PasswordResetter interface {
	SendPasswordReset(ctx context.Context, q *db.Queries, u db.User) error
}

type Service struct {
	krillv1connect.UnimplementedUserServiceHandler
	pool   *pgxpool.Pool
	q      *db.Queries
	resets PasswordResetter
}

func NewService(pool *pgxpool.Pool, resets PasswordResetter) *Service {
	return &Service{pool: pool, q: db.New(pool), resets: resets}
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

func record(ctx context.Context, q *db.Queries, action string, u db.User, details any) error {
	err := audit.Record(ctx, q, audit.Entry{
		Action:     action,
		TargetType: audit.TargetUser,
		TargetID:   u.ID.String(),
		TargetName: u.Name,
		Details:    details,
	})
	if err != nil {
		return rpc.Internal(err, "record audit entry")
	}
	return nil
}

func (s *Service) ListUsers(ctx context.Context, _ *krillv1.ListUsersRequest) (*krillv1.ListUsersResponse, error) {
	rows, err := s.q.ListUsers(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list users")
	}
	counts, err := s.q.CountActiveSessions(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "count sessions")
	}
	sessions := make(map[uuid.UUID]int32, len(counts))
	for _, c := range counts {
		sessions[c.UserID] = c.Sessions
	}
	out := make([]*krillv1.User, len(rows))
	for i, u := range rows {
		out[i] = auth.ToProto(u)
		out[i].ActiveSessions = sessions[u.ID]
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
		if name != u.Role {
			from := u.Role
			if u, err = q.SetUserRole(ctx, db.SetUserRoleParams{ID: id, Role: name}); err != nil {
				return nil, rpc.Internal(err, "set role")
			}
			if err := record(ctx, q, audit.UserRole, u, audit.Change(from, name)); err != nil {
				return nil, err
			}
		}
	}
	if req.Disabled != nil && req.GetDisabled() != u.Disabled {
		if req.GetDisabled() {
			u, _, err = deactivate(ctx, q, u)
		} else {
			u, err = reactivate(ctx, q, u)
		}
		if err != nil {
			return nil, err
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

	err = s.inTx(ctx, id, func(q *db.Queries, u db.User) error {
		if _, err := q.SetUserPassword(ctx, db.SetUserPasswordParams{ID: id, PasswordHash: pgtype.Text{String: hash, Valid: true}}); err != nil {
			return rpc.Internal(err, "set password")
		}
		if err := q.DeleteUserSessions(ctx, id); err != nil {
			return rpc.Internal(err, "delete sessions")
		}
		return record(ctx, q, audit.UserPassword, u, nil)
	})
	if err != nil {
		return nil, err
	}
	return &krillv1.SetUserPasswordResponse{}, nil
}

func (s *Service) DeleteUser(ctx context.Context, req *krillv1.DeleteUserRequest) (*krillv1.DeleteUserResponse, error) {
	id, err := target(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	err = s.inTx(ctx, id, func(q *db.Queries, u db.User) error {
		if _, err := q.DeleteUser(ctx, id); err != nil {
			return rpc.Internal(err, "delete user")
		}
		return record(ctx, q, audit.UserDelete, u, nil)
	})
	if err != nil {
		return nil, err
	}
	return &krillv1.DeleteUserResponse{}, nil
}

func (s *Service) SignOutUser(ctx context.Context, req *krillv1.SignOutUserRequest) (*krillv1.SignOutUserResponse, error) {
	id, err := target(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	err = s.inTx(ctx, id, func(q *db.Queries, u db.User) error {
		if err := q.DeleteUserSessions(ctx, id); err != nil {
			return rpc.Internal(err, "delete sessions")
		}
		return record(ctx, q, audit.UserSignOut, u, nil)
	})
	if err != nil {
		return nil, err
	}
	return &krillv1.SignOutUserResponse{}, nil
}

func (s *Service) MarkEmailVerified(ctx context.Context, req *krillv1.MarkEmailVerifiedRequest) (*krillv1.MarkEmailVerifiedResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, rpc.Invalid("invalid user id")
	}
	var out db.User
	err = s.inTx(ctx, id, func(q *db.Queries, u db.User) error {
		v, err := q.MarkEmailVerified(ctx, id)
		if err != nil {
			return rpc.Internal(err, "mark email verified")
		}
		out = v
		if u.EmailVerifiedAt.Valid {
			return nil
		}
		return record(ctx, q, audit.UserVerifyEmail, u, map[string]any{"email": u.Email})
	})
	if err != nil {
		return nil, err
	}
	return &krillv1.MarkEmailVerifiedResponse{User: auth.ToProto(out)}, nil
}

type released struct {
	Sessions int64 `json:"sessions_ended"`
	Claims   int64 `json:"claims_released"`
}

// deactivate signs u out everywhere and frees the clips they were working on,
// so nobody waits on a claim that will never be finished. Their labels keep
// their name.
func deactivate(ctx context.Context, q *db.Queries, u db.User) (db.User, released, error) {
	var r released
	u, err := q.SetUserDisabled(ctx, db.SetUserDisabledParams{ID: u.ID, Disabled: true})
	if err != nil {
		return u, r, rpc.Internal(err, "deactivate")
	}
	if r.Sessions, err = q.EndUserSessions(ctx, u.ID); err != nil {
		return u, r, rpc.Internal(err, "delete sessions")
	}
	if r.Claims, err = q.ReleaseUserClaims(ctx, u.ID); err != nil {
		return u, r, rpc.Internal(err, "release claims")
	}
	if err := q.DeleteUserEmailCodes(ctx, u.ID); err != nil {
		return u, r, rpc.Internal(err, "delete codes")
	}
	return u, r, record(ctx, q, audit.UserDisable, u, r)
}

func reactivate(ctx context.Context, q *db.Queries, u db.User) (db.User, error) {
	u, err := q.SetUserDisabled(ctx, db.SetUserDisabledParams{ID: u.ID, Disabled: false})
	if err != nil {
		return u, rpc.Internal(err, "reactivate")
	}
	return u, record(ctx, q, audit.UserEnable, u, nil)
}

func (s *Service) DeactivateUser(ctx context.Context, req *krillv1.DeactivateUserRequest) (*krillv1.DeactivateUserResponse, error) {
	id, err := target(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	out := &krillv1.DeactivateUserResponse{}
	err = s.inTx(ctx, id, func(q *db.Queries, u db.User) error {
		var r released
		if !u.Disabled {
			var err error
			if u, r, err = deactivate(ctx, q, u); err != nil {
				return err
			}
		}
		out.User = auth.ToProto(u)
		out.SessionsEnded = r.Sessions
		out.ClaimsReleased = r.Claims
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Service) ReactivateUser(ctx context.Context, req *krillv1.ReactivateUserRequest) (*krillv1.ReactivateUserResponse, error) {
	id, err := target(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	var out db.User
	err = s.inTx(ctx, id, func(q *db.Queries, u db.User) error {
		out = u
		if !u.Disabled {
			return nil
		}
		var err error
		out, err = reactivate(ctx, q, u)
		return err
	})
	if err != nil {
		return nil, err
	}
	return &krillv1.ReactivateUserResponse{User: auth.ToProto(out)}, nil
}

func (s *Service) SendPasswordReset(ctx context.Context, req *krillv1.SendPasswordResetRequest) (*krillv1.SendPasswordResetResponse, error) {
	id, err := uuid.Parse(req.GetId())
	if err != nil {
		return nil, rpc.Invalid("invalid user id")
	}
	err = s.inTx(ctx, id, func(q *db.Queries, u db.User) error {
		if u.Disabled {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("reactivate this account first"))
		}
		// Recorded before sending, so an email never goes out without an entry.
		if err := record(ctx, q, audit.UserPasswordReset, u, map[string]any{"email": u.Email}); err != nil {
			return err
		}
		return s.resets.SendPasswordReset(ctx, q, u)
	})
	if err != nil {
		return nil, err
	}
	return &krillv1.SendPasswordResetResponse{}, nil
}

// inTx runs fn in a transaction with the user it acts on.
func (s *Service) inTx(ctx context.Context, id uuid.UUID, fn func(q *db.Queries, u db.User) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	u, err := q.GetUser(ctx, id)
	if err != nil {
		return rpc.DBError(err, "user")
	}
	if err := fn(q, u); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return rpc.Internal(err, "commit")
	}
	return nil
}
