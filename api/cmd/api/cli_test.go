package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wauv/krill/api/internal/db"
)

func TestSetRolePromotesAndAudits(t *testing.T) {
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	q := db.New(pool)
	name := "t" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	u, err := q.CreateUser(ctx, db.CreateUserParams{Name: name, Username: name, Email: name + "@example.com", Role: "labeler"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = q.DeleteUser(context.Background(), u.ID) })

	if _, _, err := setRole(ctx, pool, "nobody-"+name+"@example.com", "admin"); err == nil {
		t.Fatal("promoted a user that doesn't exist")
	}
	// Typed the way people type emails, not how they're stored.
	got, from, err := setRole(ctx, pool, "  "+strings.ToUpper(u.Email)+" ", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != "admin" || from != "labeler" {
		t.Fatalf("role = %q from %q, want admin from labeler", got.Role, from)
	}

	var details []byte
	err = pool.QueryRow(ctx, `SELECT details FROM audit_log WHERE action = 'user.role' AND target_id = $1`, u.ID.String()).Scan(&details)
	if err != nil {
		t.Fatalf("no audit entry: %v", err)
	}
	var change map[string]string
	if err := json.Unmarshal(details, &change); err != nil {
		t.Fatal(err)
	}
	if change["from"] != "labeler" || change["to"] != "admin" {
		t.Errorf("audit details = %v", change)
	}
}
