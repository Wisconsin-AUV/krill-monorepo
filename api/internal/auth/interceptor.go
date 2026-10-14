package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/wauv/krill/api/internal/db"
)

// SessionLookup returns the active user for a session token hash, or
// pgx.ErrNoRows when the session is missing, expired, or the user is disabled.
type SessionLookup func(ctx context.Context, tokenHash []byte) (db.User, error)

// Interceptor loads the caller's session and enforces the role policy.
type Interceptor struct {
	lookup         SessionLookup
	workerToken    func() string
	verifiedEmail  func() bool
	clientIPHeader func() string
}

// NewInterceptor returns an interceptor that also accepts workerToken as a
// bearer token on worker procedures. An empty token turns them off.
func NewInterceptor(lookup SessionLookup, workerToken func() string) *Interceptor {
	return &Interceptor{
		lookup:         lookup,
		workerToken:    workerToken,
		verifiedEmail:  func() bool { return false },
		clientIPHeader: func() string { return "" },
	}
}

// RequireVerifiedEmail limits accounts with an unverified email to the
// procedures that verify it while required returns true.
func (i *Interceptor) RequireVerifiedEmail(required func() bool) *Interceptor {
	i.verifiedEmail = required
	return i
}

// TrustClientIPHeader reads ClientIP from the header that header names, when
// it names one.
func (i *Interceptor) TrustClientIPHeader(header func() string) *Interceptor {
	i.clientIPHeader = header
	return i
}

var _ connect.Interceptor = (*Interceptor)(nil)

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authorize(ctx, req.Spec().Procedure, req.Peer().Addr, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *Interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *Interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authorize(ctx, conn.Spec().Procedure, conn.Peer().Addr, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *Interceptor) authorize(ctx context.Context, procedure, peer string, h http.Header) (context.Context, error) {
	ctx = context.WithValue(ctx, clientIPKey{}, clientIP(peer, h, i.clientIPHeader()))
	need, ok := policy[procedure]
	if !ok {
		return ctx, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s has no access policy", procedure))
	}
	if need == worker {
		if !i.isWorker(h) {
			return ctx, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid worker token"))
		}
		return ctx, nil
	}
	if token := tokenFromHeader(h); token != "" {
		hash := hashToken(token)
		user, err := i.lookup(ctx, hash)
		switch {
		case err == nil:
			ctx = withSession(ctx, Session{User: user, TokenHash: hash})
		case !errors.Is(err, pgx.ErrNoRows):
			return ctx, connect.NewError(connect.CodeInternal, fmt.Errorf("load session: %w", err))
		}
	}
	if need == public {
		return ctx, nil
	}
	s, ok := SessionFrom(ctx)
	if !ok {
		return ctx, connect.NewError(connect.CodeUnauthenticated, errors.New("sign in to continue"))
	}
	if need == unverified {
		return ctx, nil
	}
	if i.verifiedEmail() && !s.User.EmailVerifiedAt.Valid {
		return ctx, connect.NewError(connect.CodePermissionDenied, errors.New("verify your email to continue"))
	}
	if need != signedIn && !Can(ParseRole(s.User.Role), need) {
		return ctx, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("you don't have permission to do this (%s)", describe(need)))
	}
	return ctx, nil
}

func (i *Interceptor) isWorker(h http.Header) bool {
	token, ok := strings.CutPrefix(h.Get("Authorization"), "Bearer ")
	want := i.workerToken()
	return ok && want != "" && subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1
}
