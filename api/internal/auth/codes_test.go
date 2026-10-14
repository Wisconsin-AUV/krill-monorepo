package auth

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/email"
)

type inbox chan email.Message

func (b inbox) Send(_ context.Context, m email.Message) error {
	b <- m
	return nil
}

// code waits for the next email and returns the code its subject leads with.
func (b inbox) code(t *testing.T, to string) string {
	t.Helper()
	select {
	case m := <-b:
		if m.To != to {
			t.Fatalf("email went to %s, want %s", m.To, to)
		}
		return m.Subject[:6]
	case <-time.After(5 * time.Second):
		t.Fatal("no email sent")
		return ""
	}
}

func (b inbox) empty(t *testing.T) {
	t.Helper()
	select {
	case m := <-b:
		t.Fatalf("unexpected email to %s", m.To)
	case <-time.After(100 * time.Millisecond):
	}
}

// otherCode is a well-formed code that isn't code.
func otherCode(code string) string {
	if code == "000000" {
		return "111111"
	}
	return "000000"
}

func testService(t *testing.T) (*Service, inbox) {
	t.Helper()
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(t.Context(), pool); err != nil {
		t.Fatal(err)
	}
	mail := make(inbox, 10)
	return NewService(pool, Options{RequireVerifiedEmail: true, Mailer: mail}), mail
}

func testUser(t *testing.T, s *Service, verified bool) db.User {
	t.Helper()
	name := "t" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	u, err := s.q.CreateUser(t.Context(), db.CreateUserParams{
		Name: name, Username: name, Email: name + "@wisc.edu", Role: "labeler",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.q.DeleteUser(context.Background(), u.ID) })
	if verified {
		if u, err = s.q.MarkEmailVerified(t.Context(), u.ID); err != nil {
			t.Fatal(err)
		}
	}
	return u
}

func sessionCtx(t *testing.T, s *Service, id uuid.UUID) context.Context {
	t.Helper()
	u, err := s.q.GetUser(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return withSession(t.Context(), Session{User: u})
}

func wantCode(t *testing.T, err error, code connect.Code) {
	t.Helper()
	if connect.CodeOf(err) != code {
		t.Fatalf("err = %v, want %v", err, code)
	}
}

func TestVerifyEmail(t *testing.T) {
	s, mail := testService(t)
	u := testUser(t, s, false)
	ctx := sessionCtx(t, s, u.ID)

	if _, err := s.SendVerificationCode(ctx, &krillv1.SendVerificationCodeRequest{}); err != nil {
		t.Fatal(err)
	}
	code := mail.code(t, u.Email)

	// Opening the page again keeps the code that is already in the inbox.
	if _, err := s.SendVerificationCode(ctx, &krillv1.SendVerificationCodeRequest{}); err != nil {
		t.Fatal(err)
	}
	mail.empty(t)
	_, err := s.SendVerificationCode(ctx, &krillv1.SendVerificationCodeRequest{Resend: true})
	wantCode(t, err, connect.CodeResourceExhausted)

	res, err := s.VerifyEmail(ctx, &krillv1.VerifyEmailRequest{Code: code[:3] + " " + code[3:]})
	if err != nil {
		t.Fatal(err)
	}
	if !res.GetUser().GetEmailVerified() {
		t.Error("email not verified")
	}
	_, err = s.VerifyEmail(ctx, &krillv1.VerifyEmailRequest{Code: code})
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestWrongCodesUseUpTheCode(t *testing.T) {
	s, mail := testService(t)
	u := testUser(t, s, false)
	ctx := sessionCtx(t, s, u.ID)
	if _, err := s.SendVerificationCode(ctx, &krillv1.SendVerificationCodeRequest{}); err != nil {
		t.Fatal(err)
	}
	code := mail.code(t, u.Email)

	// Each wrong guess returns an error, so this also checks the count
	// survives the handler's failed response.
	for range maxCodeAttempts {
		_, err := s.VerifyEmail(ctx, &krillv1.VerifyEmailRequest{Code: otherCode(code)})
		wantCode(t, err, connect.CodeInvalidArgument)
	}
	_, err := s.VerifyEmail(ctx, &krillv1.VerifyEmailRequest{Code: code})
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestChangeVerifiedEmail(t *testing.T) {
	s, mail := testService(t)
	u := testUser(t, s, true)
	ctx := sessionCtx(t, s, u.ID)
	next := "new-" + u.Email

	res, err := s.UpdateProfile(ctx, &krillv1.UpdateProfileRequest{Name: u.Name, Username: u.Username, Email: next})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetUser().GetEmail() != u.Email || res.GetUser().GetPendingEmail() != next {
		t.Fatalf("email = %q, pending = %q", res.GetUser().GetEmail(), res.GetUser().GetPendingEmail())
	}
	code := mail.code(t, next)

	ctx = sessionCtx(t, s, u.ID)
	verified, err := s.VerifyEmail(ctx, &krillv1.VerifyEmailRequest{Code: code})
	if err != nil {
		t.Fatal(err)
	}
	if verified.GetUser().GetEmail() != next || verified.GetUser().GetPendingEmail() != "" {
		t.Errorf("email = %q, pending = %q", verified.GetUser().GetEmail(), verified.GetUser().GetPendingEmail())
	}
}

func TestEmailChangeCodeOnlyVerifiesItsAddress(t *testing.T) {
	s, mail := testService(t)
	u := testUser(t, s, true)
	ctx := sessionCtx(t, s, u.ID)
	first := "first-" + u.Email
	if _, err := s.UpdateProfile(ctx, &krillv1.UpdateProfileRequest{Name: u.Name, Username: u.Username, Email: first}); err != nil {
		t.Fatal(err)
	}
	code := mail.code(t, first)

	// Pending moves on without a new code, as if the account changed it some
	// other way. The old code must not verify the new address.
	if _, err := s.q.SetPendingEmail(t.Context(), db.SetPendingEmailParams{ID: u.ID, PendingEmail: text("second-" + u.Email)}); err != nil {
		t.Fatal(err)
	}
	_, err := s.VerifyEmail(sessionCtx(t, s, u.ID), &krillv1.VerifyEmailRequest{Code: code})
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestResetPassword(t *testing.T) {
	s, mail := testService(t)
	u := testUser(t, s, false)

	if _, err := s.RequestPasswordReset(t.Context(), &krillv1.RequestPasswordResetRequest{Login: "nobody-" + u.Username}); err != nil {
		t.Fatal(err)
	}
	mail.empty(t)

	if _, err := s.RequestPasswordReset(t.Context(), &krillv1.RequestPasswordResetRequest{Login: u.Username}); err != nil {
		t.Fatal(err)
	}
	code := mail.code(t, u.Email)

	const password = "Correct-horse-9"
	_, err := s.ResetPassword(t.Context(), &krillv1.ResetPasswordRequest{Login: u.Email, Code: otherCode(code), NewPassword: password})
	wantCode(t, err, connect.CodeInvalidArgument)
	res, err := s.ResetPassword(t.Context(), &krillv1.ResetPasswordRequest{Login: u.Email, Code: code, NewPassword: password})
	if err != nil {
		t.Fatal(err)
	}
	if !res.GetUser().GetHasPassword() || !res.GetUser().GetEmailVerified() {
		t.Errorf("user = %v", res.GetUser())
	}
	got, err := s.q.GetUser(t.Context(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := CheckPassword(got.PasswordHash.String, password); err != nil || !ok {
		t.Error("password not set")
	}
}

func TestDescribeDevice(t *testing.T) {
	for ua, want := range map[string]string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:131.0) Gecko/20100101 Firefox/131.0":                                                    "Firefox on macOS",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1": "Safari on iOS",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Mobile Safari/537.36":                   "Chrome on Android",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36 Edg/129.0.0.0":           "Edge on Windows",
		"": "Unknown device",
	} {
		if got := describeDevice(ua); got != want {
			t.Errorf("describeDevice(%q) = %q, want %q", ua, got, want)
		}
	}
}
