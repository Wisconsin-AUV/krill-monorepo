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

type reviewStub struct {
	krillv1connect.UnimplementedReviewServiceHandler
}

func (reviewStub) FlagFrame(context.Context, *krillv1.FlagFrameRequest) (*krillv1.FlagFrameResponse, error) {
	return &krillv1.FlagFrameResponse{}, nil
}

func (reviewStub) ResolveReview(context.Context, *krillv1.ResolveReviewRequest) (*krillv1.ResolveReviewResponse, error) {
	return &krillv1.ResolveReviewResponse{}, nil
}

func TestReviewAccess(t *testing.T) {
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
	mux.Handle(krillv1connect.NewReviewServiceHandler(reviewStub{},
		connect.WithInterceptors(NewInterceptor(lookup, "worker-token"))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		name    string
		token   string
		resolve bool
		want    connect.Code
	}{
		{"labeler flags", "labeler-token", false, 0},
		{"labeler resolves", "labeler-token", true, connect.CodePermissionDenied},
		{"developer resolves", "dev-token", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := krillv1connect.NewReviewServiceClient(srv.Client(), srv.URL, withCookie(tt.token))
			var err error
			if tt.resolve {
				_, err = client.ResolveReview(t.Context(), &krillv1.ResolveReviewRequest{})
			} else {
				_, err = client.FlagFrame(t.Context(), &krillv1.FlagFrameRequest{})
			}
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Errorf("err = %v, want code %v", err, tt.want)
			}
		})
	}
}
