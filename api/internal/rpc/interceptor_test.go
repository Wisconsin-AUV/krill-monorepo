package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestErrorInterceptor(t *testing.T) {
	secret := errors.New(`pq: relation "users" at 10.0.0.4`)
	tests := []struct {
		name     string
		err      error
		wantCode connect.Code
		wantKept bool
	}{
		{"internal", Internal(secret, "create video"), connect.CodeInternal, false},
		{"db error", DBError(secret, "video"), connect.CodeInternal, false},
		{"plain error", secret, connect.CodeInternal, false},
		{"invalid", Invalid("name is required"), connect.CodeInvalidArgument, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			call := ErrorInterceptor{}.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
				return nil, tt.err
			})
			_, err := call(context.Background(), connect.NewRequest(&emptypb.Empty{}))
			if got := connect.CodeOf(err); got != tt.wantCode {
				t.Errorf("code = %v, want %v", got, tt.wantCode)
			}
			if kept := err.Error() == tt.err.Error(); kept != tt.wantKept {
				t.Errorf("error = %q, kept = %v, want %v", err, kept, tt.wantKept)
			}
			if !tt.wantKept && strings.Contains(err.Error(), "10.0.0.4") {
				t.Errorf("error leaks detail: %q", err)
			}
		})
	}
}
