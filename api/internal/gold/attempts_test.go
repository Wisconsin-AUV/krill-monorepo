package gold

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/db"
)

// noRows is a database with no gold attempts, so a caller who gets past the
// access check sees NotFound or Internal rather than PermissionDenied.
type noRows struct{}

var errNoDB = errors.New("no database")

func (noRows) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errNoDB
}

func (noRows) Query(context.Context, string, ...any) (pgx.Rows, error) { return nil, errNoDB }

func (noRows) QueryRow(context.Context, string, ...any) pgx.Row { return errRow{} }

func (noRows) CopyFrom(context.Context, pgx.Identifier, []string, pgx.CopyFromSource) (int64, error) {
	return 0, errNoDB
}

type errRow struct{}

func (errRow) Scan(...any) error { return pgx.ErrNoRows }

func TestGoldAttemptAccess(t *testing.T) {
	owner, other, dev := uuid.New(), uuid.New(), uuid.New()
	users := map[string]db.User{
		"owner": {ID: owner, Role: "labeler"},
		"other": {ID: other, Role: "labeler"},
		"dev":   {ID: dev, Role: "developer"},
	}
	lookup := func(_ context.Context, hash []byte) (db.User, error) {
		for token, u := range users {
			if sum := sha256.Sum256([]byte(token)); bytes.Equal(sum[:], hash) {
				return u, nil
			}
		}
		return db.User{}, pgx.ErrNoRows
	}
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewGoldServiceHandler(&Service{q: db.New(noRows{})},
		connect.WithInterceptors(auth.NewInterceptor(lookup, func() string { return "" }))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		name   string
		caller string
		list   bool
		userID string
		want   connect.Code
	}{
		{"another labeler reads it", "other", false, owner.String(), connect.CodePermissionDenied},
		{"owner reads it", "owner", false, owner.String(), connect.CodeNotFound},
		{"developer reads it", "dev", false, owner.String(), connect.CodeNotFound},
		{"another labeler lists them", "other", true, owner.String(), connect.CodePermissionDenied},
		{"labeler lists everyone's", "owner", true, "", connect.CodePermissionDenied},
		{"owner lists them", "owner", true, owner.String(), connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := krillv1connect.NewGoldServiceClient(srv.Client(), srv.URL, withCookie(tt.caller))
			var err error
			if tt.list {
				_, err = client.ListGoldAttempts(t.Context(), &krillv1.ListGoldAttemptsRequest{UserId: tt.userID})
			} else {
				_, err = client.GetGoldAttempt(t.Context(), &krillv1.GetGoldAttemptRequest{FrameId: 1, UserId: tt.userID})
			}
			if got := connect.CodeOf(err); got != tt.want {
				t.Errorf("code = %v, want %v (err %v)", got, tt.want, err)
			}
		})
	}
}

func withCookie(token string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			req.Header().Set("Cookie", auth.CookieName+"="+token)
			return next(ctx, req)
		}
	}))
}
