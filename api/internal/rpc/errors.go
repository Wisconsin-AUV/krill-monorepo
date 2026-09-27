package rpc

import (
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5"
)

// DBError maps pgx.ErrNoRows to NotFound and wraps anything else as Internal.
func DBError(err error, entity string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("%s not found", entity))
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("%s: %w", entity, err))
}

func Invalid(format string, args ...any) error {
	return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf(format, args...))
}

func Internal(err error, action string) error {
	return connect.NewError(connect.CodeInternal, fmt.Errorf("%s: %w", action, err))
}
