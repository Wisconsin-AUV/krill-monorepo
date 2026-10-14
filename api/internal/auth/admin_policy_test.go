package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
)

type adminStub struct {
	krillv1connect.UnimplementedAdminServiceHandler
}

func (adminStub) ListSettings(context.Context, *krillv1.ListSettingsRequest) (*krillv1.ListSettingsResponse, error) {
	return &krillv1.ListSettingsResponse{}, nil
}

// Settings hold secrets like the worker token, so only admins may read them.
func TestSettingsAreAdminOnly(t *testing.T) {
	roles := map[string]string{"dev-token": "developer", "admin-token": "admin"}
	lookup := func(_ context.Context, hash []byte) (db.User, error) {
		for token, role := range roles {
			if string(hashToken(token)) == string(hash) {
				return db.User{Role: role}, nil
			}
		}
		return db.User{}, pgx.ErrNoRows
	}
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewAdminServiceHandler(adminStub{},
		connect.WithInterceptors(NewInterceptor(lookup, workerToken("worker-token")))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for token, want := range map[string]connect.Code{
		"":            connect.CodeUnauthenticated,
		"dev-token":   connect.CodePermissionDenied,
		"admin-token": 0,
	} {
		client := krillv1connect.NewAdminServiceClient(srv.Client(), srv.URL, withCookie(token))
		_, err := client.ListSettings(t.Context(), &krillv1.ListSettingsRequest{})
		if want == 0 && err != nil || want != 0 && connect.CodeOf(err) != want {
			t.Errorf("token %q: err = %v, want code %v", token, err, want)
		}
	}
	// The worker token opens worker procedures only.
	client := krillv1connect.NewAdminServiceClient(srv.Client(), srv.URL, withHeader("Authorization", "Bearer worker-token"))
	if _, err := client.ListSettings(t.Context(), &krillv1.ListSettingsRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Errorf("worker token: err = %v", err)
	}
}
