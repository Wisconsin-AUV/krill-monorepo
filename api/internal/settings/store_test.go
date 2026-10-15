package settings

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
)

func testStore(t *testing.T, env map[string]string) (*Store, *pgxpool.Pool) {
	t.Helper()
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
	clear := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM settings`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_log WHERE target_type = 'setting'`)
	}
	clear()
	t.Cleanup(clear)
	if env["KRILL_DATABASE_URL"] == "" {
		env["KRILL_DATABASE_URL"] = url
	}
	s, err := Open(ctx, pool, func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, pool
}

func entry(s *Store, key string) Entry {
	for _, e := range s.List() {
		if e.Key == key {
			return e
		}
	}
	panic(key)
}

func TestSavedSettingAppliesAndSurvivesRestart(t *testing.T) {
	s, pool := testStore(t, map[string]string{})
	var seen []bool
	s.OnChange(func(c config.Config) { seen = append(seen, c.AllowSignup) })

	if err := s.Update(t.Context(), uuid.Nil, []Change{{Key: "KRILL_ALLOW_SIGNUP", Value: "false"}}); err != nil {
		t.Fatal(err)
	}
	if s.Config().AllowSignup || len(seen) != 2 || seen[1] {
		t.Fatalf("AllowSignup = %v, listener saw %v", s.Config().AllowSignup, seen)
	}

	reopened, err := Open(t.Context(), pool, s.env)
	if err != nil {
		t.Fatal(err)
	}
	if reopened.Config().AllowSignup {
		t.Error("saved setting was lost on restart")
	}

	if err := s.Update(t.Context(), uuid.Nil, []Change{{Key: "KRILL_ALLOW_SIGNUP", Reset: true}}); err != nil {
		t.Fatal(err)
	}
	if !s.Config().AllowSignup || entry(s, "KRILL_ALLOW_SIGNUP").Source != SourceDefault {
		t.Error("reset did not go back to the default")
	}
}

func TestEnvironmentWinsAndLocksTheSetting(t *testing.T) {
	s, pool := testStore(t, map[string]string{"KRILL_TEAM_NAME": "From env"})
	// A row saved before the variable was set must not apply.
	if _, err := pool.Exec(t.Context(), `INSERT INTO settings (key, value) VALUES ('KRILL_TEAM_NAME', 'Saved')`); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(t.Context(), pool, s.env)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Config().TeamName; got != "From env" {
		t.Errorf("TeamName = %q, want the environment's", got)
	}

	err = s.Update(t.Context(), uuid.Nil, []Change{{Key: "KRILL_TEAM_NAME", Value: "Panel"}})
	var fe *config.FieldError
	if !errors.As(err, &fe) || fe.Key != "KRILL_TEAM_NAME" {
		t.Errorf("err = %v, want a field error on KRILL_TEAM_NAME", err)
	}
}

func TestEnvOnlySettingsCantBeSaved(t *testing.T) {
	s, pool := testStore(t, map[string]string{})
	for _, key := range []string{"KRILL_DATABASE_URL", "KRILL_S3_SECRET_KEY", "KRILL_PUBLIC_URL"} {
		if err := s.Update(t.Context(), uuid.Nil, []Change{{Key: key, Value: "x"}}); err == nil {
			t.Errorf("%s was saved", key)
		}
	}
	// Nor does a row that got in some other way change where the API connects.
	if _, err := pool.Exec(t.Context(), `INSERT INTO settings (key, value) VALUES ('KRILL_S3_BUCKET', 'other')`); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(t.Context(), pool, s.env)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Config().S3Bucket; got != "krill" {
		t.Errorf("S3Bucket = %q, want krill", got)
	}
}

func TestInvalidChangesApplyNothing(t *testing.T) {
	s, pool := testStore(t, map[string]string{"KRILL_PUBLIC_URL": "https://krill.example.com", "KRILL_S3_PUBLIC_ENDPOINT": "https://s3.example.com"})

	// Verification codes need a way to send them, so the team name in the
	// same save must not stick either.
	err := s.Update(t.Context(), uuid.Nil, []Change{
		{Key: "KRILL_TEAM_NAME", Value: "New team"},
		{Key: "KRILL_EMAIL_VERIFICATION", Value: "verify"},
	})
	var fe *config.FieldError
	if !errors.As(err, &fe) || fe.Key != "KRILL_RESEND_API_KEY" {
		t.Fatalf("err = %v, want a field error on KRILL_RESEND_API_KEY", err)
	}
	var n int
	if err := pool.QueryRow(t.Context(), `SELECT count(*) FROM settings`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 || s.Config().TeamName == "New team" {
		t.Errorf("a rejected save left %d rows and team %q", n, s.Config().TeamName)
	}

	err = s.Update(t.Context(), uuid.Nil, []Change{{Key: "KRILL_GOLD_REVIEW_MIN_ATTEMPTS", Value: "three"}})
	if !errors.As(err, &fe) || fe.Key != "KRILL_GOLD_REVIEW_MIN_ATTEMPTS" {
		t.Errorf("err = %v, want a field error on KRILL_GOLD_REVIEW_MIN_ATTEMPTS", err)
	}
}

func TestSecretsAreNeverListed(t *testing.T) {
	s, _ := testStore(t, map[string]string{"KRILL_PUBLIC_URL": "https://krill.example.com", "KRILL_S3_PUBLIC_ENDPOINT": "https://s3.example.com"})
	err := s.Update(t.Context(), uuid.Nil, []Change{
		{Key: "KRILL_RESEND_API_KEY", Value: "re_secret"},
		{Key: "KRILL_EMAIL_FROM", Value: "Krill <noreply@example.com>"},
	})
	if err != nil {
		t.Fatal(err)
	}
	e := entry(s, "KRILL_RESEND_API_KEY")
	if e.Value != "" || !e.IsSet {
		t.Errorf("secret entry = %+v", e)
	}
	if s.Config().ResendAPIKey != "re_secret" {
		t.Error("secret was not applied")
	}
}

func TestSecretChangesAreAuditedWithoutTheirValue(t *testing.T) {
	s, pool := testStore(t, map[string]string{"KRILL_PUBLIC_URL": "https://krill.example.com", "KRILL_S3_PUBLIC_ENDPOINT": "https://s3.example.com"})
	for _, changes := range [][]Change{
		{{Key: "KRILL_RESEND_API_KEY", Value: "re_secret"}, {Key: "KRILL_EMAIL_FROM", Value: "Krill <noreply@example.com>"}},
		{{Key: "KRILL_RESEND_API_KEY", Reset: true}},
	} {
		if err := s.Update(t.Context(), uuid.Nil, changes); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := pool.Query(t.Context(), `SELECT target_id, details::text FROM audit_log WHERE action = 'setting.update' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var key, details string
		if err := rows.Scan(&key, &details); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(details, "re_secret") {
			t.Errorf("%s entry has the secret: %s", key, details)
		}
		if key == "KRILL_EMAIL_FROM" && !strings.Contains(details, "noreply@example.com") {
			t.Errorf("%s entry is missing the new value: %s", key, details)
		}
		got = append(got, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"KRILL_RESEND_API_KEY", "KRILL_EMAIL_FROM", "KRILL_RESEND_API_KEY"}; !slices.Equal(got, want) {
		t.Errorf("audited %v, want %v", got, want)
	}
}
