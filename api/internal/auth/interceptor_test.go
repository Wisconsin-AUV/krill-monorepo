package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
	"github.com/wauv/krill/api/internal/db"
)

func TestEveryProcedureHasPolicy(t *testing.T) {
	var count int
	protoregistry.GlobalFiles.RangeFilesByPackage("krill.v1", func(f protoreflect.FileDescriptor) bool {
		for i := range f.Services().Len() {
			svc := f.Services().Get(i)
			for j := range svc.Methods().Len() {
				count++
				procedure := fmt.Sprintf("/%s/%s", svc.FullName(), svc.Methods().Get(j).Name())
				if _, ok := policy[procedure]; !ok {
					t.Errorf("%s has no entry in policy", procedure)
				}
			}
		}
		return true
	})
	if count == 0 {
		t.Fatal("found no procedures")
	}
}

type labelStub struct {
	krillv1connect.UnimplementedLabelServiceHandler
}

func (labelStub) ListLabelTypes(context.Context, *krillv1.ListLabelTypesRequest) (*krillv1.ListLabelTypesResponse, error) {
	return &krillv1.ListLabelTypesResponse{}, nil
}

func (labelStub) DeleteLabelType(context.Context, *krillv1.DeleteLabelTypeRequest) (*krillv1.DeleteLabelTypeResponse, error) {
	return &krillv1.DeleteLabelTypeResponse{}, nil
}

func TestInterceptor(t *testing.T) {
	users := map[string]db.User{
		"labeler-token":  {Role: "labeler"},
		"dev-token":      {Role: "developer"},
		"broken-db-case": {},
	}
	lookup := func(_ context.Context, hash []byte) (db.User, error) {
		if string(hash) == string(hashToken("broken-db-case")) {
			return db.User{}, errors.New("db down")
		}
		for token, u := range users {
			if string(hashToken(token)) == string(hash) {
				return u, nil
			}
		}
		return db.User{}, pgx.ErrNoRows
	}

	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewLabelServiceHandler(labelStub{},
		connect.WithInterceptors(NewInterceptor(lookup))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		name   string
		token  string
		delete bool
		want   connect.Code
	}{
		{"no session", "", false, connect.CodeUnauthenticated},
		{"unknown session", "stale", false, connect.CodeUnauthenticated},
		{"labeler reads", "labeler-token", false, 0},
		{"labeler deletes", "labeler-token", true, connect.CodePermissionDenied},
		{"developer deletes", "dev-token", true, 0},
		{"lookup fails", "broken-db-case", false, connect.CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := krillv1connect.NewLabelServiceClient(srv.Client(), srv.URL, withCookie(tt.token))
			var err error
			if tt.delete {
				_, err = client.DeleteLabelType(t.Context(), &krillv1.DeleteLabelTypeRequest{})
			} else {
				_, err = client.ListLabelTypes(t.Context(), &krillv1.ListLabelTypesRequest{})
			}
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Errorf("err = %v, want code %v", err, tt.want)
			}
		})
	}
}

func withCookie(token string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Cookie", CookieName+"="+token)
			}
			return next(ctx, req)
		}
	}))
}
