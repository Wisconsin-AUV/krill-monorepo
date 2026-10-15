package auth

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

// closedService has sign-up closed to everyone outside wisc.edu, and to them
// too.
func closedService(t *testing.T) *Service {
	t.Helper()
	open, _ := testService(t)
	// Makes sure the next account isn't the first, which is always allowed.
	testUser(t, open, true)
	return NewService(open.pool, func() Options {
		return Options{AllowSignup: false, SignupEmailDomain: "wisc.edu"}
	})
}

type inviteSpec struct {
	role     string
	maxUses  int32
	expires  time.Duration
	revoked  bool
	usesLeft int32
}

func testInvite(t *testing.T, s *Service, spec inviteSpec) string {
	t.Helper()
	token, hash, err := NewInviteToken()
	if err != nil {
		t.Fatal(err)
	}
	inv, err := s.q.CreateInvite(t.Context(), db.CreateInviteParams{
		TokenHash: hash, Role: spec.role, MaxUses: spec.maxUses,
		ExpiresAt: timestamptz(time.Now().Add(spec.expires)),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.pool.Exec(context.Background(), `DELETE FROM invites WHERE id = $1`, inv.ID) })
	if spec.revoked {
		if _, err := s.q.RevokeInvite(t.Context(), inv.ID); err != nil {
			t.Fatal(err)
		}
	}
	if spec.usesLeft < spec.maxUses {
		if _, err := s.pool.Exec(t.Context(), `UPDATE invites SET uses = max_uses - $2 WHERE id = $1`, inv.ID, spec.usesLeft); err != nil {
			t.Fatal(err)
		}
	}
	return token
}

func registration(email, token string) *krillv1.RegisterRequest {
	name := "t" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	if email == "" {
		email = name + "@example.com"
	}
	return &krillv1.RegisterRequest{
		Name: name, Username: name, Email: email, Password: "Correct-Horse-9", InviteToken: token,
	}
}

func cleanupUser(t *testing.T, s *Service, res *krillv1.RegisterResponse) {
	t.Helper()
	if res == nil {
		return
	}
	t.Cleanup(func() { _, _ = s.q.DeleteUser(context.Background(), uuid.MustParse(res.GetUser().GetId())) })
}

func TestInviteSignsUpPastClosedSignup(t *testing.T) {
	s := closedService(t)

	res, err := s.Register(t.Context(), registration("", ""))
	cleanupUser(t, s, res)
	wantCode(t, err, connect.CodeInvalidArgument)

	res, err = s.Register(t.Context(), registration("someone@wisc.edu", ""))
	cleanupUser(t, s, res)
	wantCode(t, err, connect.CodePermissionDenied)

	token := testInvite(t, s, inviteSpec{role: "developer", maxUses: 2, expires: time.Hour, usesLeft: 2})
	res, err = s.Register(t.Context(), registration("", token))
	cleanupUser(t, s, res)
	if err != nil {
		t.Fatal(err)
	}
	if res.GetUser().GetRole() != krillv1.Role_ROLE_DEVELOPER {
		t.Errorf("role = %v, want the invite's", res.GetUser().GetRole())
	}
	var uses, recorded int
	err = s.pool.QueryRow(t.Context(), `
		SELECT i.uses, (SELECT count(*) FROM invite_uses u WHERE u.invite_id = i.id)
		FROM invites i WHERE token_hash = $1`, HashInviteToken(token)).Scan(&uses, &recorded)
	if err != nil {
		t.Fatal(err)
	}
	if uses != 1 || recorded != 1 {
		t.Errorf("uses = %d, recorded = %d, want 1 and 1", uses, recorded)
	}
}

func TestDeadInvitesAreRejected(t *testing.T) {
	s := closedService(t)
	for name, spec := range map[string]inviteSpec{
		"expired":   {role: "labeler", maxUses: 1, expires: -time.Minute, usesLeft: 1},
		"revoked":   {role: "labeler", maxUses: 1, expires: time.Hour, usesLeft: 1, revoked: true},
		"used up":   {role: "labeler", maxUses: 3, expires: time.Hour, usesLeft: 0},
		"made up":   {},
		"truncated": {role: "labeler", maxUses: 1, expires: time.Hour, usesLeft: 1},
	} {
		t.Run(name, func(t *testing.T) {
			token := "not-a-real-invite-token"
			if spec.maxUses > 0 {
				token = testInvite(t, s, spec)
			}
			if name == "truncated" {
				token = token[:len(token)-1]
			}
			res, err := s.Register(t.Context(), registration("", token))
			cleanupUser(t, s, res)
			wantCode(t, err, connect.CodeFailedPrecondition)
		})
	}
}

// Two people racing for the last use of an invite: only one gets in.
func TestInviteLastUseCantBeShared(t *testing.T) {
	s := closedService(t)
	token := testInvite(t, s, inviteSpec{role: "labeler", maxUses: 1, expires: time.Hour, usesLeft: 1})

	const racers = 8
	var wg sync.WaitGroup
	errs := make([]error, racers)
	for i := range racers {
		wg.Go(func() {
			res, err := s.Register(t.Context(), registration("", token))
			cleanupUser(t, s, res)
			errs[i] = err
		})
	}
	wg.Wait()
	var ok int
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case connect.CodeOf(err) != connect.CodeFailedPrecondition:
			t.Errorf("err = %v", err)
		}
	}
	if ok != 1 {
		t.Errorf("%d sign-ups used a one-use invite", ok)
	}
}

func TestApprovedEmailSignsUpOnce(t *testing.T) {
	s := closedService(t)
	email := "t" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12] + "@example.com"
	if _, err := s.q.AddApprovedEmail(t.Context(), db.AddApprovedEmailParams{Email: email, Role: "developer"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.q.RemoveApprovedEmail(context.Background(), email) })

	session, err := s.GetSession(t.Context(), &krillv1.GetSessionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if !session.GetApprovedSignupOpen() {
		t.Error("sign-up form hidden with an approved email waiting")
	}

	res, err := s.Register(t.Context(), registration(strings.ToUpper(email), ""))
	cleanupUser(t, s, res)
	if err != nil {
		t.Fatal(err)
	}
	if res.GetUser().GetRole() != krillv1.Role_ROLE_DEVELOPER {
		t.Errorf("role = %v, want the approval's", res.GetUser().GetRole())
	}
	if _, err := s.q.DeleteUser(t.Context(), uuid.MustParse(res.GetUser().GetId())); err != nil {
		t.Fatal(err)
	}
	// Used up, even though its account is gone.
	res, err = s.Register(t.Context(), registration(email, ""))
	cleanupUser(t, s, res)
	wantCode(t, err, connect.CodeInvalidArgument)
}

func TestSlackSignUpTakesInviteRole(t *testing.T) {
	s := closedService(t)
	slack := NewSlack(s.pool, func() SlackConfig { return SlackConfig{TeamID: "T1"} }, Cookies{})
	token := testInvite(t, s, inviteSpec{role: "admin", maxUses: 1, expires: time.Hour, usesLeft: 1})
	name := "t" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	id := slackIdentity{Sub: "U" + name, TeamID: "T1", Name: name, Email: name + "@example.com", EmailVerified: true}

	if _, err := slack.signIn(t.Context(), id, token); err != nil {
		t.Fatal(err)
	}
	u, err := s.q.GetUserByEmail(t.Context(), id.Email)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.q.DeleteUser(context.Background(), u.ID) })
	if u.Role != "admin" {
		t.Errorf("role = %s, want admin", u.Role)
	}

	other := id
	other.Sub, other.Email = id.Sub+"x", "x"+id.Email
	_, err = slack.signIn(t.Context(), other, token)
	if !errors.As(err, new(userError)) {
		t.Errorf("used invite: err = %v, want a message for the user", err)
	}
	if _, err := s.q.GetUserByEmail(t.Context(), other.Email); err == nil {
		t.Error("used invite still created an account")
	}
}
