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
	workerTokens   WorkerTokenLookup
	verifiedEmail  func() bool
	clientIPHeader func() string
}

// NewInterceptor returns an interceptor that also accepts workerToken as a
// bearer token on worker procedures. An empty token accepts none, unless
// AcceptWorkerTokens adds saved ones.
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

// AcceptWorkerTokens also accepts the saved worker tokens lookup finds on
// worker procedures.
func (i *Interceptor) AcceptWorkerTokens(lookup WorkerTokenLookup) *Interceptor {
	i.workerTokens = lookup
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
	ctx = withClient(ctx, clientIP(peer, h, i.clientIPHeader()), h.Get("User-Agent"))
	need, ok := policy[procedure]
	if !ok {
		return ctx, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s has no access policy", procedure))
	}
	if need == worker {
		w, err := i.worker(ctx, h)
		if err != nil {
			return ctx, err
		}
		return context.WithValue(ctx, workerKey{}, w), nil
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

var errBadWorkerToken = connect.NewError(connect.CodeUnauthenticated, errors.New("invalid worker token"))

func (i *Interceptor) worker(ctx context.Context, h http.Header) (Worker, error) {
	token, ok := strings.CutPrefix(h.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return Worker{}, errBadWorkerToken
	}
	if want := i.workerToken(); want != "" && subtle.ConstantTimeCompare([]byte(token), []byte(want)) == 1 {
		return Worker{TokenName: EnvWorkerToken}, nil
	}
	if i.workerTokens == nil {
		return Worker{}, errBadWorkerToken
	}
	// Saved tokens are found by hash, so lookup timing says nothing about
	// the token itself.
	t, err := i.workerTokens(ctx, hashToken(token))
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Worker{}, errBadWorkerToken
	case err != nil:
		return Worker{}, connect.NewError(connect.CodeInternal, fmt.Errorf("load worker token: %w", err))
	}
	return Worker{TokenID: t.ID, TokenName: t.Name}, nil
}
