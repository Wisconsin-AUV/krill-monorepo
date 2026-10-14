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

type clipStub struct {
	krillv1connect.UnimplementedClipServiceHandler
}

func (clipStub) DeleteClip(context.Context, *krillv1.DeleteClipRequest) (*krillv1.DeleteClipResponse, error) {
	return &krillv1.DeleteClipResponse{}, nil
}

// Deleting a clip throws away everyone's labels on it, so labelers can't.
func TestDeleteClipAccess(t *testing.T) {
	roles := map[string]string{"labeler-token": "labeler", "dev-token": "developer"}
	lookup := func(_ context.Context, hash []byte) (db.User, error) {
		for token, role := range roles {
			if string(hashToken(token)) == string(hash) {
				return db.User{Role: role}, nil
			}
		}
		return db.User{}, pgx.ErrNoRows
	}
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewClipServiceHandler(clipStub{},
		connect.WithInterceptors(NewInterceptor(lookup, "worker-token"))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		name  string
		token string
		want  connect.Code
	}{
		{"labeler", "labeler-token", connect.CodePermissionDenied},
		{"developer", "dev-token", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := krillv1connect.NewClipServiceClient(srv.Client(), srv.URL, withCookie(tt.token))
			_, err := client.DeleteClip(t.Context(), &krillv1.DeleteClipRequest{Id: 1})
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Errorf("err = %v, want code %v", err, tt.want)
			}
		})
	}
}
