package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
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
		connect.WithInterceptors(NewInterceptor(lookup, "worker-token"))))
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

type workerStub struct {
	krillv1connect.UnimplementedWorkerServiceHandler
}

func (workerStub) NextTask(context.Context, *krillv1.NextTaskRequest) (*krillv1.NextTaskResponse, error) {
	return &krillv1.NextTaskResponse{}, nil
}

func (workerStub) ReportEmbed(context.Context, *krillv1.ReportEmbedRequest) (*krillv1.ReportEmbedResponse, error) {
	return &krillv1.ReportEmbedResponse{}, nil
}

func TestInterceptorWorker(t *testing.T) {
	lookup := func(context.Context, []byte) (db.User, error) { return db.User{Role: "admin"}, nil }
	tests := []struct {
		name       string
		configured string
		header     string
		want       connect.Code
	}{
		{"valid token", "worker-token", "Bearer worker-token", 0},
		{"wrong token", "worker-token", "Bearer nope", connect.CodeUnauthenticated},
		{"no token", "worker-token", "", connect.CodeUnauthenticated},
		{"user session", "worker-token", "", connect.CodeUnauthenticated},
		{"token not configured", "", "Bearer ", connect.CodeUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.Handle(krillv1connect.NewWorkerServiceHandler(workerStub{},
				connect.WithInterceptors(NewInterceptor(lookup, tt.configured))))
			srv := httptest.NewServer(mux)
			defer srv.Close()

			opts := []connect.ClientOption{withHeader("Authorization", tt.header)}
			if tt.name == "user session" {
				opts = append(opts, withCookie("admin-token"))
			}
			client := krillv1connect.NewWorkerServiceClient(srv.Client(), srv.URL, opts...)
			_, err := client.NextTask(t.Context(), &krillv1.NextTaskRequest{})
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Errorf("NextTask: err = %v, want code %v", err, tt.want)
			}
			// Embeddings land in MinIO, so a forged report would mark a clip
			// embedded with nothing behind it.
			_, err = client.ReportEmbed(t.Context(), &krillv1.ReportEmbedRequest{})
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Errorf("ReportEmbed: err = %v, want code %v", err, tt.want)
			}
		})
	}
}

type annotationStub struct {
	krillv1connect.UnimplementedAnnotationServiceHandler
}

func (annotationStub) ListTracksByType(context.Context, *krillv1.ListTracksByTypeRequest) (*krillv1.ListTracksByTypeResponse, error) {
	return &krillv1.ListTracksByTypeResponse{}, nil
}

func TestListTracksByTypeAccess(t *testing.T) {
	lookup := func(_ context.Context, hash []byte) (db.User, error) {
		if string(hash) == string(hashToken("labeler-token")) {
			return db.User{Role: "labeler"}, nil
		}
		return db.User{}, pgx.ErrNoRows
	}
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewAnnotationServiceHandler(annotationStub{},
		connect.WithInterceptors(NewInterceptor(lookup, "worker-token"))))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	tests := []struct {
		name string
		opt  connect.ClientOption
		want connect.Code
	}{
		{"labeler", withCookie("labeler-token"), 0},
		{"no session", withCookie(""), connect.CodeUnauthenticated},
		{"worker token", withHeader("Authorization", "Bearer worker-token"), connect.CodeUnauthenticated},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := krillv1connect.NewAnnotationServiceClient(srv.Client(), srv.URL, tt.opt)
			_, err := client.ListTracksByType(t.Context(), &krillv1.ListTracksByTypeRequest{})
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Errorf("err = %v, want code %v", err, tt.want)
			}
		})
	}
}

type authStub struct {
	krillv1connect.UnimplementedAuthServiceHandler
}

func (authStub) VerifyEmail(context.Context, *krillv1.VerifyEmailRequest) (*krillv1.VerifyEmailResponse, error) {
	return &krillv1.VerifyEmailResponse{}, nil
}

func TestInterceptorRequiresVerifiedEmail(t *testing.T) {
	verified := pgtype.Timestamptz{Time: time.Now(), Valid: true}
	users := map[string]db.User{
		"verified":   {Role: "admin", EmailVerifiedAt: verified},
		"unverified": {Role: "admin"},
	}
	lookup := func(_ context.Context, hash []byte) (db.User, error) {
		for token, u := range users {
			if string(hashToken(token)) == string(hash) {
				return u, nil
			}
		}
		return db.User{}, pgx.ErrNoRows
	}

	for _, required := range []bool{true, false} {
		mux := http.NewServeMux()
		opts := connect.WithInterceptors(NewInterceptor(lookup, "").RequireVerifiedEmail(required))
		mux.Handle(krillv1connect.NewLabelServiceHandler(labelStub{}, opts))
		mux.Handle(krillv1connect.NewAuthServiceHandler(authStub{}, opts))
		srv := httptest.NewServer(mux)

		for _, tt := range []struct {
			token string
			want  connect.Code
		}{
			{"verified", 0},
			{"unverified", connect.CodePermissionDenied},
		} {
			if !required {
				tt.want = 0
			}
			labels := krillv1connect.NewLabelServiceClient(srv.Client(), srv.URL, withCookie(tt.token))
			_, err := labels.ListLabelTypes(t.Context(), &krillv1.ListLabelTypesRequest{})
			if tt.want == 0 && err != nil || tt.want != 0 && connect.CodeOf(err) != tt.want {
				t.Errorf("required=%v %s lists labels: err = %v, want code %v", required, tt.token, err, tt.want)
			}
			auth := krillv1connect.NewAuthServiceClient(srv.Client(), srv.URL, withCookie(tt.token))
			if _, err := auth.VerifyEmail(t.Context(), &krillv1.VerifyEmailRequest{}); err != nil {
				t.Errorf("required=%v %s verifies: %v", required, tt.token, err)
			}
		}
		srv.Close()
	}
}

func withHeader(key, value string) connect.ClientOption {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if value != "" {
				req.Header().Set(key, value)
			}
			return next(ctx, req)
		}
	}))
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

func TestEveryPermissionIsDefined(t *testing.T) {
	for v, name := range krillv1.Permission_name {
		p := krillv1.Permission(v)
		if p == krillv1.Permission_PERMISSION_UNSPECIFIED {
			continue
		}
		if !slices.ContainsFunc(Permissions, func(d PermissionDef) bool { return d.Permission == p }) {
			t.Errorf("%s has no entry in Permissions", name)
		}
	}
}
