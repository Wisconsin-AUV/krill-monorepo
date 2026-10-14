package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"strings"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/email"
	"github.com/wauv/krill/api/internal/rpc"
)

const (
	codeTTL      = 10 * time.Minute
	codeCooldown = time.Minute
	// Five guesses per code, and a new code at most once a minute, leaves a
	// six-digit code out of reach of guessing.
	maxCodeAttempts = 5

	purposeVerify = "verify_email"
	purposeReset  = "reset_password"
)

var (
	errBadCode  = connect.NewError(connect.CodeInvalidArgument, errors.New("that code is incorrect or has expired"))
	errCooldown = connect.NewError(connect.CodeResourceExhausted, errors.New("wait a minute before sending another code"))
	errNoEmail  = connect.NewError(connect.CodeFailedPrecondition, errors.New("this server can't send email"))
)

func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("read code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashCode(userID uuid.UUID, purpose, code string) []byte {
	sum := sha256.Sum256([]byte(purpose + ":" + userID.String() + ":" + code))
	return sum[:]
}

// normalizeCode accepts a code typed with spaces or a dash.
func normalizeCode(code string) (string, bool) {
	code = strings.NewReplacer(" ", "", "-", "").Replace(code)
	if len(code) != 6 {
		return "", false
	}
	for _, r := range code {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return code, true
}

// verifyTarget is the address a verification code goes to, or "" when there
// is nothing to verify.
func verifyTarget(u db.User) string {
	switch {
	case u.PendingEmail.Valid:
		return u.PendingEmail.String
	case !u.EmailVerifiedAt.Valid:
		return u.Email
	}
	return ""
}

// origin is where and when a code was asked for, shown in the email.
type origin struct {
	device, ip string
	at         time.Time
}

func (s *Service) origin(ctx context.Context) origin {
	var ua string
	if info, ok := connect.CallInfoForHandlerContext(ctx); ok {
		ua = info.RequestHeader().Get("User-Agent")
	}
	return origin{device: describeDevice(ua), ip: s.clientIP(ctx), at: time.Now().UTC()}
}

// sendCode issues a code and emails it to the address. The caller commits q's
// transaction, so a failed send leaves no code behind. Without resend, a live
// code is kept and nothing is sent.
func (s *Service) sendCode(ctx context.Context, q *db.Queries, u db.User, to, purpose string, resend bool, from origin) error {
	if s.opts.Mailer == nil {
		return errNoEmail
	}
	code, err := newCode()
	if err != nil {
		return rpc.Internal(err, "issue code")
	}
	_, err = q.IssueEmailCode(ctx, db.IssueEmailCodeParams{
		UserID:          u.ID,
		Purpose:         purpose,
		Email:           to,
		CodeHash:        hashCode(u.ID, purpose, code),
		TtlSeconds:      int32(codeTTL.Seconds()),
		CooldownSeconds: int32(codeCooldown.Seconds()),
		KeepLive:        !resend,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if resend {
			return errCooldown
		}
		return nil
	}
	if err != nil {
		return rpc.Internal(err, "issue code")
	}

	data := email.Code{
		Code:     code,
		Minutes:  int(codeTTL.Minutes()),
		Device:   from.device,
		IP:       from.ip,
		Time:     from.at.Format("2 January 2006, 15:04 UTC"),
		Team:     s.opts.TeamName,
		Year:     from.at.Year(),
		Email:    to,
		Username: u.Username,
	}
	render := email.VerifyEmail
	if purpose == purposeReset {
		render = email.ResetPassword
	}
	msg, err := render(to, data)
	if err != nil {
		return rpc.Internal(err, "render email")
	}
	if err := s.opts.Mailer.Send(ctx, msg); err != nil {
		slog.Error("send email", "purpose", purpose, "err", err)
		return connect.NewError(connect.CodeAborted, errors.New("could not send the email; try again"))
	}
	return nil
}

// checkCode uses up the code if it matches and returns the address it was
// sent to. A wrong guess counts against the code, so the caller must commit
// q's transaction even when ok is false.
func checkCode(ctx context.Context, q *db.Queries, userID uuid.UUID, purpose, code string) (sentTo string, ok bool, err error) {
	key := db.DeleteEmailCodeParams{UserID: userID, Purpose: purpose}
	row, err := q.LockEmailCode(ctx, db.LockEmailCodeParams{UserID: userID, Purpose: purpose})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("load code: %w", err)
	}
	if row.Expired {
		return "", false, q.DeleteEmailCode(ctx, key)
	}
	if subtle.ConstantTimeCompare(row.CodeHash, hashCode(userID, purpose, code)) != 1 {
		if row.Attempts+1 >= maxCodeAttempts {
			return "", false, q.DeleteEmailCode(ctx, key)
		}
		return "", false, q.CountEmailCodeAttempt(ctx, db.CountEmailCodeAttemptParams(key))
	}
	return row.Email, true, q.DeleteEmailCode(ctx, key)
}

func (s *Service) SendVerificationCode(ctx context.Context, req *krillv1.SendVerificationCodeRequest) (*krillv1.SendVerificationCodeResponse, error) {
	sess, _ := SessionFrom(ctx)
	to := verifyTarget(sess.User)
	if to == "" {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("your email is already verified"))
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.sendCode(ctx, s.q.WithTx(tx), sess.User, to, purposeVerify, req.GetResend(), s.origin(ctx)); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.SendVerificationCodeResponse{}, nil
}

func (s *Service) VerifyEmail(ctx context.Context, req *krillv1.VerifyEmailRequest) (*krillv1.VerifyEmailResponse, error) {
	sess, _ := SessionFrom(ctx)
	code, ok := normalizeCode(req.GetCode())
	if !ok {
		return nil, errBadCode
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	sentTo, ok, err := checkCode(ctx, q, sess.User.ID, purposeVerify, code)
	if err != nil {
		return nil, rpc.Internal(err, "check code")
	}
	// A code sent to an earlier pending email can't verify a later one.
	if !ok || sentTo != verifyTarget(sess.User) {
		if err := tx.Commit(ctx); err != nil {
			return nil, rpc.Internal(err, "commit")
		}
		return nil, errBadCode
	}
	user, err := q.VerifyUserEmail(ctx, db.VerifyUserEmailParams{ID: sess.User.ID, Email: sentTo})
	if err != nil {
		if dup := DuplicateError(err); dup != nil {
			return nil, dup
		}
		return nil, rpc.Internal(err, "verify email")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	return &krillv1.VerifyEmailResponse{User: ToProto(user)}, nil
}

func (s *Service) RequestPasswordReset(ctx context.Context, req *krillv1.RequestPasswordResetRequest) (*krillv1.RequestPasswordResetResponse, error) {
	if s.opts.Mailer == nil {
		return nil, errNoEmail
	}
	if err := s.checkResetLimit(ctx); err != nil {
		return nil, err
	}
	s.resetLimit.fail(s.clientIP(ctx))

	login := strings.ToLower(strings.TrimSpace(req.GetLogin()))
	user, err := s.q.GetUserByLogin(ctx, login)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && user.Disabled {
		return &krillv1.RequestPasswordResetResponse{}, nil
	}
	if err != nil {
		return nil, rpc.Internal(err, "load user")
	}
	// Sending in the background keeps the response time the same whether or
	// not the account exists.
	from := s.origin(ctx)
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	go func() {
		defer cancel()
		if err := s.sendResetCode(ctx, user, from); err != nil && !errors.Is(err, errCooldown) {
			slog.Error("send password reset", "err", err)
		}
	}()
	return &krillv1.RequestPasswordResetResponse{}, nil
}

func (s *Service) sendResetCode(ctx context.Context, user db.User, from origin) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := s.sendCode(ctx, s.q.WithTx(tx), user, user.Email, purposeReset, true, from); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) ResetPassword(ctx context.Context, req *krillv1.ResetPasswordRequest) (*krillv1.ResetPasswordResponse, error) {
	if err := s.checkResetLimit(ctx); err != nil {
		return nil, err
	}
	// Checked first so a weak password doesn't use up a guess.
	if err := ValidatePassword(req.GetNewPassword()); err != nil {
		return nil, rpc.Invalid("%s", err)
	}
	fail := func() error {
		s.resetLimit.fail(s.clientIP(ctx))
		return errBadCode
	}
	code, ok := normalizeCode(req.GetCode())
	if !ok {
		return nil, fail()
	}
	login := strings.ToLower(strings.TrimSpace(req.GetLogin()))
	user, err := s.q.GetUserByLogin(ctx, login)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && user.Disabled {
		return nil, fail()
	}
	if err != nil {
		return nil, rpc.Internal(err, "load user")
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
	sentTo, ok, err := checkCode(ctx, q, user.ID, purposeReset, code)
	if err != nil {
		return nil, rpc.Internal(err, "check code")
	}
	if !ok || sentTo != user.Email {
		if err := tx.Commit(ctx); err != nil {
			return nil, rpc.Internal(err, "commit")
		}
		return nil, fail()
	}
	if _, err := q.SetUserPassword(ctx, db.SetUserPasswordParams{ID: user.ID, PasswordHash: text(hash)}); err != nil {
		return nil, rpc.Internal(err, "set password")
	}
	// The code proves the email is theirs.
	if user, err = q.MarkEmailVerified(ctx, user.ID); err != nil {
		return nil, rpc.Internal(err, "verify email")
	}
	if err := q.DeleteUserSessions(ctx, user.ID); err != nil {
		return nil, rpc.Internal(err, "delete sessions")
	}
	cookie, err := s.opts.Cookies.StartSession(ctx, q, user.ID)
	if err != nil {
		return nil, rpc.Internal(err, "start session")
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	s.accountLimit.clear(login)
	setCookie(ctx, cookie)
	setCookie(ctx, s.opts.Cookies.LastLogin("password"))
	return &krillv1.ResetPasswordResponse{User: ToProto(user)}, nil
}

func (s *Service) checkResetLimit(ctx context.Context) error {
	if blocked, wait := s.resetLimit.blocked(s.clientIP(ctx)); blocked {
		return connect.NewError(connect.CodeResourceExhausted,
			fmt.Errorf("too many password reset attempts; try again in %d minutes", int(math.Ceil(wait.Minutes()))))
	}
	return nil
}

// describeDevice names the browser and OS in a User-Agent, such as "Firefox
// on macOS".
func describeDevice(ua string) string {
	has := func(s ...string) bool {
		for _, v := range s {
			if strings.Contains(ua, v) {
				return true
			}
		}
		return false
	}
	var browser, os string
	// Order matters: Edge and Opera also claim Chrome, and Chrome claims Safari.
	switch {
	case has("Edg/", "EdgA/", "EdgiOS/"):
		browser = "Edge"
	case has("OPR/"):
		browser = "Opera"
	case has("Firefox/", "FxiOS/"):
		browser = "Firefox"
	case has("Chrome/", "CriOS/"):
		browser = "Chrome"
	case has("Safari/"):
		browser = "Safari"
	}
	// iOS claims Mac OS X, and Android claims Linux.
	switch {
	case has("iPhone", "iPad"):
		os = "iOS"
	case has("Android"):
		os = "Android"
	case has("Windows"):
		os = "Windows"
	case has("CrOS"):
		os = "ChromeOS"
	case has("Macintosh", "Mac OS X"):
		os = "macOS"
	case has("Linux"):
		os = "Linux"
	}
	switch {
	case browser != "" && os != "":
		return browser + " on " + os
	case browser != "":
		return browser
	case os != "":
		return os
	}
	return "Unknown device"
}
