package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/auth"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
)

const usage = `usage:
  krill                                       start the server
  krill user role <email or username> <role>  set a user's role: labeler, developer, or admin`

// runCommand runs a command typed on the server. Setting a role here is the
// only way to make the first admin, since sign-up never grants admin without
// an invite.
func runCommand(ctx context.Context, args []string) error {
	if len(args) != 4 || args[0] != "user" || args[1] != "role" {
		return errors.New(usage)
	}
	if err := checkRole(args[3]); err != nil {
		return err
	}
	databaseURL, ok := config.Env("KRILL_DATABASE_URL")
	if !ok {
		return errors.New("KRILL_DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	u, from, err := setRole(ctx, pool, args[2], args[3])
	if err != nil {
		return err
	}
	if from == u.Role {
		fmt.Printf("%s <%s> is already %s\n", u.Username, u.Email, u.Role)
	} else {
		fmt.Printf("%s <%s> is now %s, was %s\n", u.Username, u.Email, u.Role, from)
	}
	return nil
}

func checkRole(role string) error {
	if auth.ParseRole(role) != krillv1.Role_ROLE_UNSPECIFIED {
		return nil
	}
	names := make([]string, len(auth.Roles))
	for i, r := range auth.Roles {
		names[i] = r.Name
	}
	return fmt.Errorf("unknown role %q; use %s", role, strings.Join(names, ", "))
}

// setRole gives the user with login a valid role and returns the role they
// had. The change is audited with no actor, which the audit log shows as
// System.
func setRole(ctx context.Context, pool *pgxpool.Pool, login, role string) (db.User, string, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return db.User{}, "", fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)

	u, err := q.GetUserByLogin(ctx, strings.ToLower(strings.TrimSpace(login)))
	if errors.Is(err, pgx.ErrNoRows) {
		return db.User{}, "", fmt.Errorf("no user has the email or username %q; sign up first", login)
	}
	if err != nil {
		return db.User{}, "", fmt.Errorf("load user: %w", err)
	}
	from := u.Role
	if from == role {
		return u, from, nil
	}
	if u, err = q.SetUserRole(ctx, db.SetUserRoleParams{ID: u.ID, Role: role}); err != nil {
		return db.User{}, "", fmt.Errorf("set role: %w", err)
	}
	err = audit.Record(ctx, q, audit.Entry{
		Action:     audit.UserRole,
		TargetType: audit.TargetUser,
		TargetID:   u.ID.String(),
		TargetName: u.Name,
		Details:    audit.Change(from, role),
	})
	if err != nil {
		return db.User{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return db.User{}, "", fmt.Errorf("commit: %w", err)
	}
	return u, from, nil
}
