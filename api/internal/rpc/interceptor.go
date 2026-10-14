package rpc

import (
	"context"
	"errors"
	"log/slog"

	"connectrpc.com/connect"
)

// ErrorInterceptor logs internal errors and hides their detail from the
// client, since they carry database and storage messages.
type ErrorInterceptor struct{}

var _ connect.Interceptor = ErrorInterceptor{}

func (ErrorInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		res, err := next(ctx, req)
		return res, redact(ctx, req.Spec().Procedure, err)
	}
}

func (ErrorInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (ErrorInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		return redact(ctx, conn.Spec().Procedure, next(ctx, conn))
	}
}

func redact(ctx context.Context, procedure string, err error) error {
	if err == nil {
		return nil
	}
	// Handlers that return a plain error reach the client as Unknown.
	code := connect.CodeOf(err)
	if code != connect.CodeInternal && code != connect.CodeUnknown {
		return err
	}
	if !errors.Is(err, context.Canceled) {
		slog.ErrorContext(ctx, "rpc failed", "procedure", procedure, "error", err)
	}
	return connect.NewError(connect.CodeInternal, errors.New("something went wrong"))
}
