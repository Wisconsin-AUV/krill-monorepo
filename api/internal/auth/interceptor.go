package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	"github.com/wauv/krill/api/internal/db"
)

// SessionLookup returns the active user for a session token hash, or
// pgx.ErrNoRows when the session is missing, expired, or the user is disabled.
type SessionLookup func(ctx context.Context, tokenHash []byte) (db.User, error)

// Interceptor loads the caller's session and enforces the role policy.
type Interceptor struct {
	lookup SessionLookup
}

func NewInterceptor(lookup SessionLookup) *Interceptor {
	return &Interceptor{lookup: lookup}
}

var _ connect.Interceptor = (*Interceptor)(nil)

func (i *Interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authorize(ctx, req.Spec().Procedure, req.Header())
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
		ctx, err := i.authorize(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *Interceptor) authorize(ctx context.Context, procedure string, h http.Header) (context.Context, error) {
	need, ok := policy[procedure]
	if !ok {
		return ctx, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("%s has no access policy", procedure))
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
	if ParseRole(s.User.Role) < need {
		name, _ := RoleName(need)
		return ctx, connect.NewError(connect.CodePermissionDenied, fmt.Errorf("this needs the %s role", name))
	}
	return ctx, nil
}
