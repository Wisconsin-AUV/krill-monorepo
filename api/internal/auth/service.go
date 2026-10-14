package auth

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/email"
	"github.com/wauv/krill/api/internal/rpc"
)

var errBadLogin = connect.NewError(connect.CodeUnauthenticated, errors.New("incorrect username or password"))

type Options struct {
	Cookies     Cookies
	AllowSignup bool
	// SignupEmailDomain is empty when any email may be used.
	SignupEmailDomain string
	// RequireVerifiedEmail has accounts enter a code from their email before
	// using the app. It needs a Mailer.
	RequireVerifiedEmail bool
	// Mailer is nil when the API can't send email, which turns off password
	// resets and sign-in codes.
	Mailer         email.Sender
	TeamName       string
	SlackEnabled   bool
	ClientIPHeader string
}

type Service struct {
	krillv1connect.UnimplementedAuthServiceHandler
	pool *pgxpool.Pool
	q    *db.Queries
	opts func() Options
	// A per-account limit stops guessing one password; the looser per-IP
	// limit stops one client trying a few passwords across many accounts.
	accountLimit *failureLimiter
	ipLimit      *failureLimiter
	// Emailed codes are limited per IP for requests and wrong guesses, and per
	// account for emails sent, so spreading requests over many IPs still
	// can't flood one inbox.
	requestLimit  *failureLimiter
	codeFailLimit *failureLimiter
	sendLimit     *failureLimiter
	// signupLimit counts sign-ups that hit an existing account, so sign-up
	// can't be used to check many emails for accounts.
	signupLimit *failureLimiter
}

func NewService(pool *pgxpool.Pool, opts func() Options) *Service {
	return &Service{
		pool:          pool,
		q:             db.New(pool),
		opts:          opts,
		accountLimit:  newFailureLimiter(5, 15*time.Minute),
		ipLimit:       newFailureLimiter(30, 15*time.Minute),
		requestLimit:  newFailureLimiter(20, 15*time.Minute),
		codeFailLimit: newFailureLimiter(10, 15*time.Minute),
		sendLimit:     newFailureLimiter(5, time.Hour),
		signupLimit:   newFailureLimiter(10, 15*time.Minute),
	}
}

func setCookie(ctx context.Context, c *http.Cookie) {
	if info, ok := connect.CallInfoForHandlerContext(ctx); ok {
		info.ResponseHeader().Add("Set-Cookie", c.String())
	}
}

func (s *Service) GetSession(ctx context.Context, _ *krillv1.GetSessionRequest) (*krillv1.GetSessionResponse, error) {
	out := &krillv1.GetSessionResponse{
		SlackEnabled:   s.opts().SlackEnabled,
		SignupEnabled:  s.opts().AllowSignup,
		TeamName:       s.opts().TeamName,
		AllPermissions: PermissionInfos(),
		Roles:          RoleInfos(),

		EmailVerificationRequired: s.opts().RequireVerifiedEmail,
		EmailCodesEnabled:         s.opts().Mailer != nil,
	}
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
	login := strings.ToLower(strings.TrimSpace(req.GetLogin()))
	ip := s.clientIP(ctx)
	for _, l := range []struct {
		limiter *failureLimiter
		key     string
	}{{s.accountLimit, login}, {s.ipLimit, ip}} {
		if blocked, wait := l.limiter.blocked(l.key); blocked {
			return nil, connect.NewError(connect.CodeResourceExhausted,
				fmt.Errorf("too many failed sign-ins; try again in %d minutes", int(math.Ceil(wait.Minutes()))))
		}
	}
	fail := func() error {
		s.accountLimit.fail(login)
		s.ipLimit.fail(ip)
		return errBadLogin
	}

	user, err := s.q.GetUserByLogin(ctx, login)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, rpc.Internal(err, "load user")
		}
		_, _ = CheckPassword(dummyHash, req.GetPassword())
		return nil, fail()
	}
	if !user.PasswordHash.Valid {
		_, _ = CheckPassword(dummyHash, req.GetPassword())
		return nil, fail()
	}
	ok, err := CheckPassword(user.PasswordHash.String, req.GetPassword())
	if err != nil {
		return nil, rpc.Internal(err, "check password")
	}
	if !ok {
		return nil, fail()
	}
	s.accountLimit.clear(login)
	if user.Disabled {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("this account is disabled"))
	}
	cookie, err := s.opts().Cookies.StartSession(ctx, s.q, user.ID)
	if err != nil {
		return nil, rpc.Internal(err, "start session")
	}
	setCookie(ctx, cookie)
	setCookie(ctx, s.opts().Cookies.LastLogin("password"))
	return &krillv1.LoginResponse{User: ToProto(user)}, nil
}

func (s *Service) clientIP(ctx context.Context) string {
	info, ok := connect.CallInfoForHandlerContext(ctx)
	if !ok {
		return ""
	}
	return clientIP(info.Peer().Addr, info.RequestHeader(), s.opts().ClientIPHeader)
}

// clientIP reads the proxy header only when one is configured. Otherwise any
// client could send it and pick a new address to dodge the per-IP limit.
func clientIP(peer string, h http.Header, header string) string {
	if header != "" {
		if ip := strings.TrimSpace(h.Get(header)); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(peer)
	if err != nil {
		return peer
	}
	return host
}

func (s *Service) Register(ctx context.Context, req *krillv1.RegisterRequest) (*krillv1.RegisterResponse, error) {
	ip := s.clientIP(ctx)
	if blocked, wait := s.signupLimit.blocked(ip); blocked {
		return nil, tooMany("sign-ups with taken details", wait)
	}
	p, err := NormalizeProfile(req.GetName(), req.GetUsername(), req.GetEmail())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	if !InEmailDomain(p.Email, s.opts().SignupEmailDomain) {
		return nil, rpc.Invalid("sign up with your @%s email", s.opts().SignupEmailDomain)
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
	role, err := newUserRole(ctx, q, s.opts().AllowSignup)
	if err != nil {
		return nil, err
	}
	user, err := q.CreateUser(ctx, db.CreateUserParams{
		Name: p.Name, Username: p.Username, Email: p.Email, PasswordHash: text(hash), Role: role,
	})
	if err != nil {
		if dup := DuplicateError(err); dup != nil {
			s.signupLimit.fail(ip)
			return nil, dup
		}
		return nil, rpc.Internal(err, "create user")
	}
	// Sent before commit so the account isn't created when the email fails.
	if s.opts().RequireVerifiedEmail {
		if err := s.sendCode(ctx, q, user, user.Email, purposeVerify, true, s.origin(ctx)); err != nil {
			return nil, err
		}
	}
	cookie, err := s.opts().Cookies.StartSession(ctx, q, user.ID)
	if err != nil {
		return nil, rpc.Internal(err, "start session")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	setCookie(ctx, cookie)
	setCookie(ctx, s.opts().Cookies.LastLogin("password"))
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
	setCookie(ctx, s.opts().Cookies.Clear())
	return &krillv1.LogoutResponse{}, nil
}

func (s *Service) UpdateProfile(ctx context.Context, req *krillv1.UpdateProfileRequest) (*krillv1.UpdateProfileResponse, error) {
	sess, _ := SessionFrom(ctx)
	current := sess.User
	p, err := NormalizeProfile(req.GetName(), req.GetUsername(), req.GetEmail())
	if err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	newEmail := p.Email
	if newEmail != current.Email && !InEmailDomain(newEmail, s.opts().SignupEmailDomain) {
		return nil, rpc.Invalid("use your @%s email", s.opts().SignupEmailDomain)
	}
	// A verified email stays until the new one is verified too.
	keepEmail := s.opts().RequireVerifiedEmail && current.EmailVerifiedAt.Valid
	if keepEmail {
		p.Email = current.Email
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	user, err := q.UpdateUserProfile(ctx, db.UpdateUserProfileParams{
		ID: current.ID, Name: p.Name, Username: p.Username, Email: p.Email,
	})
	if err != nil {
		if dup := DuplicateError(err); dup != nil {
			return nil, dup
		}
		return nil, rpc.DBError(err, "user")
	}

	var sendTo string
	switch {
	case !s.opts().RequireVerifiedEmail:
	case keepEmail && newEmail != current.Email:
		taken, err := q.EmailInUse(ctx, db.EmailInUseParams{Email: newEmail, ID: current.ID})
		if err != nil {
			return nil, rpc.Internal(err, "check email")
		}
		if taken {
			return nil, connect.NewError(connect.CodeAlreadyExists, errors.New("an account with that email already exists"))
		}
		if user, err = q.SetPendingEmail(ctx, db.SetPendingEmailParams{ID: current.ID, PendingEmail: text(newEmail)}); err != nil {
			return nil, rpc.Internal(err, "set pending email")
		}
		sendTo = newEmail
	case keepEmail && current.PendingEmail.Valid:
		// Saving the current email again cancels the change.
		if user, err = q.SetPendingEmail(ctx, db.SetPendingEmailParams{ID: current.ID}); err != nil {
			return nil, rpc.Internal(err, "clear pending email")
		}
		if err := q.DeleteEmailCode(ctx, db.DeleteEmailCodeParams{UserID: current.ID, Purpose: purposeVerify}); err != nil {
			return nil, rpc.Internal(err, "delete code")
		}
	case newEmail != current.Email:
		sendTo = newEmail
	}
	if sendTo != "" {
		if err := s.sendCode(ctx, q, user, sendTo, purposeVerify, true, s.origin(ctx)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
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
	if err := q.DeleteEmailCode(ctx, db.DeleteEmailCodeParams{UserID: sess.User.ID, Purpose: purposeReset}); err != nil {
		return nil, rpc.Internal(err, "delete reset code")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.ChangePasswordResponse{}, nil
}
