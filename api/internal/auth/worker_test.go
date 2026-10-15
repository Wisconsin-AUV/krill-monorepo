package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
)

type heartbeatStub struct {
	krillv1connect.UnimplementedWorkerServiceHandler
	got *Worker
}

func (s heartbeatStub) Heartbeat(ctx context.Context, _ *krillv1.HeartbeatRequest) (*krillv1.HeartbeatResponse, error) {
	*s.got, _ = WorkerFrom(ctx)
	return &krillv1.HeartbeatResponse{}, nil
}

func TestSavedWorkerTokens(t *testing.T) {
	active, activeHash, err := NewWorkerToken()
	if err != nil {
		t.Fatal(err)
	}
	revoked, _, err := NewWorkerToken()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(active, workerTokenPrefix) || active == revoked {
		t.Fatalf("tokens %q and %q", active, revoked)
	}
	tokens := func(_ context.Context, hash []byte) (db.UseWorkerTokenRow, error) {
		if string(hash) == string(activeHash) {
			return db.UseWorkerTokenRow{ID: 3, Name: "rig"}, nil
		}
		if string(hash) == string(hashToken("broken-db-case")) {
			return db.UseWorkerTokenRow{}, errors.New("db down")
		}
		// Revoked tokens are left out by the query, like unknown ones.
		return db.UseWorkerTokenRow{}, pgx.ErrNoRows
	}
	sessions := func(_ context.Context, hash []byte) (db.User, error) {
		if string(hash) == string(hashToken("admin-session")) {
			return db.User{Role: "admin"}, nil
		}
		return db.User{}, pgx.ErrNoRows
	}

	var got Worker
	interceptor := connect.WithInterceptors(NewInterceptor(sessions, workerToken("env-token")).AcceptWorkerTokens(tokens))
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewWorkerServiceHandler(heartbeatStub{got: &got}, interceptor))
	mux.Handle(krillv1connect.NewAdminServiceHandler(adminStub{}, interceptor))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		name string
		opt  connect.ClientOption
		want connect.Code
		as   Worker
	}{
		{"saved token", withHeader("Authorization", "Bearer "+active), 0, Worker{TokenID: 3, TokenName: "rig"}},
		{"environment token", withHeader("Authorization", "Bearer env-token"), 0, Worker{TokenName: EnvWorkerToken}},
		{"revoked token", withHeader("Authorization", "Bearer "+revoked), connect.CodeUnauthenticated, Worker{}},
		{"empty bearer", withHeader("Authorization", "Bearer "), connect.CodeUnauthenticated, Worker{}},
		{"session token as bearer", withHeader("Authorization", "Bearer admin-session"), connect.CodeUnauthenticated, Worker{}},
		{"admin session", withCookie("admin-session"), connect.CodeUnauthenticated, Worker{}},
		{"lookup fails", withHeader("Authorization", "Bearer broken-db-case"), connect.CodeInternal, Worker{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got = Worker{}
			client := krillv1connect.NewWorkerServiceClient(srv.Client(), srv.URL, tt.opt)
			_, err := client.Heartbeat(t.Context(), &krillv1.HeartbeatRequest{})
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Fatalf("err = %v, want code %v", err, tt.want)
			}
			if got != tt.as {
				t.Errorf("worker = %+v, want %+v", got, tt.as)
			}
		})
	}

	// A worker token must not open anything but worker procedures.
	for _, token := range []string{active, "env-token"} {
		client := krillv1connect.NewAdminServiceClient(srv.Client(), srv.URL, withHeader("Authorization", "Bearer "+token))
		if _, err := client.ListSettings(t.Context(), &krillv1.ListSettingsRequest{}); connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Errorf("ListSettings with a worker token: err = %v, want unauthenticated", err)
		}
	}
}
