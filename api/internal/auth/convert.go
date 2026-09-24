package auth

import (
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

func ToProto(u db.User) *krillv1.User {
	return &krillv1.User{
		Id:          u.ID.String(),
		Name:        u.Name,
		Username:    u.Username,
		Email:       u.Email,
		Role:        ParseRole(u.Role),
		Disabled:    u.Disabled,
		HasPassword: u.PasswordHash.Valid,
		SlackLinked: u.SlackUserID.Valid,
		CreatedAt:   timestamp(u.CreatedAt),
		LastLoginAt: timestamp(u.LastLoginAt),
		Permissions: Granted(ParseRole(u.Role)),
	}
}

// DuplicateError turns a unique violation on users into an AlreadyExists
// error naming the field. It returns nil for any other error.
func DuplicateError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
		return nil
	}
	msg := "that account already exists"
	switch pgErr.ConstraintName {
	case "users_username_key":
		msg = "that username is taken"
	case "users_email_key":
		msg = "an account with that email already exists"
	case "users_slack_user_id_key":
		msg = "that Slack account is linked to another user"
	}
	return connect.NewError(connect.CodeAlreadyExists, errors.New(msg))
}

func timestamp(t pgtype.Timestamptz) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}
