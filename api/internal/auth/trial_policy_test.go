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

type pipelineStub struct {
	krillv1connect.UnimplementedPipelineServiceHandler
}

func (pipelineStub) StartModelTrial(context.Context, *krillv1.StartModelTrialRequest) (*krillv1.StartModelTrialResponse, error) {
	return &krillv1.StartModelTrialResponse{}, nil
}

func (pipelineStub) GetModelTrial(context.Context, *krillv1.GetModelTrialRequest) (*krillv1.GetModelTrialResponse, error) {
	return &krillv1.GetModelTrialResponse{}, nil
}

func (pipelineStub) StartTraining(context.Context, *krillv1.StartTrainingRequest) (*krillv1.StartTrainingResponse, error) {
	return &krillv1.StartTrainingResponse{}, nil
}

func TestModelTrialAccess(t *testing.T) {
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
	mux.Handle(krillv1connect.NewPipelineServiceHandler(pipelineStub{},
		connect.WithInterceptors(NewInterceptor(lookup, workerToken("worker-token")))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	type call func(context.Context, krillv1connect.PipelineServiceClient) error
	start := func(ctx context.Context, c krillv1connect.PipelineServiceClient) error {
		_, err := c.StartModelTrial(ctx, &krillv1.StartModelTrialRequest{})
		return err
	}
	get := func(ctx context.Context, c krillv1connect.PipelineServiceClient) error {
		_, err := c.GetModelTrial(ctx, &krillv1.GetModelTrialRequest{})
		return err
	}
	train := func(ctx context.Context, c krillv1connect.PipelineServiceClient) error {
		_, err := c.StartTraining(ctx, &krillv1.StartTrainingRequest{})
		return err
	}
	tests := []struct {
		name  string
		token string
		call  call
		want  connect.Code
	}{
		{"labeler sees a trial", "labeler-token", get, 0},
		{"labeler starts a trial", "labeler-token", start, connect.CodePermissionDenied},
		{"developer starts a trial", "dev-token", start, 0},
		{"developer trains", "dev-token", train, connect.CodePermissionDenied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := krillv1connect.NewPipelineServiceClient(srv.Client(), srv.URL, withCookie(tt.token))
			err := tt.call(t.Context(), client)
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Errorf("err = %v, want code %v", err, tt.want)
			}
		})
	}
}
