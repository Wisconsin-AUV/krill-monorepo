package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
)

var errBadLogin = connect.NewError(connect.CodeUnauthenticated, errors.New("incorrect username or password"))

type Options struct {
	Cookies      Cookies
	AllowSignup  bool
	SlackEnabled bool
}

type Service struct {
	krillv1connect.UnimplementedAuthServiceHandler
	pool *pgxpool.Pool
	q    *db.Queries
	opts Options
}

func NewService(pool *pgxpool.Pool, opts Options) *Service {
	return &Service{pool: pool, q: db.New(pool), opts: opts}
}

func setCookie(ctx context.Context, c *http.Cookie) {
	if info, ok := connect.CallInfoForHandlerContext(ctx); ok {
		info.ResponseHeader().Add("Set-Cookie", c.String())
	}
}

func (s *Service) GetSession(ctx context.Context, _ *krillv1.GetSessionRequest) (*krillv1.GetSessionResponse, error) {
	out := &krillv1.GetSessionResponse{SlackEnabled: s.opts.SlackEnabled, SignupEnabled: s.opts.AllowSignup}
	if sess, ok := SessionFrom(ctx); ok {
		out.User = ToProto(sess.User)
	}
	if !out.SignupEnabled {
		n, err := s.q.CountUsers(ctx)
		if err != nil {
			return nil, rpc.Internal(err, "count users")
		}
		out.SignupEnabled = n == 0
	}
	return out, nil
}

func (s *Service) Login(ctx context.Context, req *krillv1.LoginRequest) (*krillv1.LoginResponse, error) {
	user, err := s.q.GetUserByLogin(ctx, strings.ToLower(strings.TrimSpace(req.GetLogin())))
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, rpc.Internal(err, "load user")
		}
		_, _ = CheckPassword(dummyHash, req.GetPassword())
		return nil, errBadLogin
	}
	if !user.PasswordHash.Valid {
		_, _ = CheckPassword(dummyHash, req.GetPassword())
		return nil, errBadLogin
	}
	ok, err := CheckPassword(user.PasswordHash.String, req.GetPassword())
	if err != nil {
		return nil, rpc.Internal(err, "check password")
	}
	if !ok {
		return nil, errBadLogin
	}
	if user.Disabled {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("this account is disabled"))
	}
	cookie, err := s.opts.Cookies.StartSession(ctx, s.q, user.ID)
	if err != nil {
		return nil, rpc.Internal(err, "start session")
	}
	setCookie(ctx, cookie)
	return &krillv1.LoginResponse{User: ToProto(user)}, nil
}

func (s *Service) Register(ctx context.Context, req *krillv1.RegisterRequest) (*krillv1.RegisterResponse, error) {
	p, err := NormalizeProfile(req.GetName(), req.GetUsername(), req.GetEmail())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	if err := ValidatePassword(req.GetPassword()); err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	hash, err := HashPassword(req.GetPassword())
	if err != nil {
		return nil, rpc.Internal(err, "hash password")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	role, err := newUserRole(ctx, q, s.opts.AllowSignup)
	if err != nil {
		return nil, err
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Name: p.Name, Username: p.Username, Email: p.Email, PasswordHash: text(hash), Role: role,
	})
	if err != nil {
		if dup := DuplicateError(err); dup != nil {
			return nil, dup
		}
		return nil, rpc.Internal(err, "create user")
	}
	cookie, err := s.opts.Cookies.StartSession(ctx, q, user.ID)
	if err != nil {
		return nil, rpc.Internal(err, "start session")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	setCookie(ctx, cookie)
	return &krillv1.RegisterResponse{User: ToProto(user)}, nil
}

// newUserRole makes the first account an admin so a fresh install can be set
// up without touching the database. The table lock stops two first sign-ups
// from both becoming admin.
func newUserRole(ctx context.Context, q *db.Queries, allowSignup bool) (string, error) {
	if err := q.LockUsers(ctx); err != nil {
		return "", rpc.Internal(err, "lock users")
	}
	n, err := q.CountUsers(ctx)
	if err != nil {
		return "", rpc.Internal(err, "count users")
	}
	if n == 0 {
		return "admin", nil
	}
	if !allowSignup {
		return "", connect.NewError(connect.CodePermissionDenied, errors.New("sign-up is closed; ask an admin for an account"))
	}
	return "labeler", nil
}

func (s *Service) Logout(ctx context.Context, _ *krillv1.LogoutRequest) (*krillv1.LogoutResponse, error) {
	if sess, ok := SessionFrom(ctx); ok {
		if err := s.q.DeleteSession(ctx, sess.TokenHash); err != nil {
			return nil, rpc.Internal(err, "delete session")
		}
	}
	setCookie(ctx, s.opts.Cookies.Clear())
	return &krillv1.LogoutResponse{}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, req *krillv1.UpdateProfileRequest) (*krillv1.UpdateProfileResponse, error) {
	sess, _ := SessionFrom(ctx)
	p, err := NormalizeProfile(req.GetName(), req.GetUsername(), req.GetEmail())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	user, err := s.q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID: sess.User.ID, Name: p.Name, Username: p.Username, Email: p.Email,
	})
	if err != nil {
		if dup := DuplicateError(err); dup != nil {
			return nil, dup
		}
		return nil, rpc.DBError(err, "user")
	}
	return &krillv1.UpdateProfileResponse{User: ToProto(user)}, nil
}

func (s *Service) ChangePassword(ctx context.Context, req *krillv1.ChangePasswordRequest) (*krillv1.ChangePasswordResponse, error) {
	sess, _ := SessionFrom(ctx)
	if sess.User.PasswordHash.Valid {
		ok, err := CheckPassword(sess.User.PasswordHash.String, req.GetCurrentPassword())
		if err != nil {
			return nil, rpc.Internal(err, "check password")
		}
		if !ok {
			return nil, rpc.Invalid("current password is incorrect")
		}
	}
	if err := ValidatePassword(req.GetNewPassword()); err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	hash, err := HashPassword(req.GetNewPassword())
	if err != nil {
		return nil, rpc.Internal(err, "hash password")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if _, err := q.SetUserPassword(ctx, db.SetUserPasswordParams{ID: sess.User.ID, PasswordHash: text(hash)}); err != nil {
		return nil, rpc.Internal(err, "set password")
	}
	if err := q.DeleteOtherSessions(ctx, db.DeleteOtherSessionsParams{UserID: sess.User.ID, TokenHash: sess.TokenHash}); err != nil {
		return nil, rpc.Internal(err, "delete sessions")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.ChangePasswordResponse{}, nil
}
