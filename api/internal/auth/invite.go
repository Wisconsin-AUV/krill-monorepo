package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/wauv/krill/api/internal/db"
)

var (
	errInviteUsed = errors.New("this invite link has expired or been used up")
	errClosed     = errors.New("sign-up is closed; ask an admin for an account")
)

// NewInviteToken returns a token for an invite link and the hash to store.
func NewInviteToken() (token string, hash []byte, err error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("read invite token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(raw)
	return token, hashToken(token), nil
}

// HashInviteToken is how an invite token is stored.
func HashInviteToken(token string) []byte {
	return hashToken(token)
}

// signupRules are what an account needs to sign up without an invite or a
// pre-approved email.
type signupRules struct {
	allowSignup bool
	// emailDomain is empty when any email may be used.
	emailDomain string
}

// grant is the role a new account gets, and what to record once it exists.
type grant struct {
	role   string
	redeem func(ctx context.Context, q *db.Queries, userID uuid.UUID) error
}

// grantSignup decides whether an account with email may sign up and with
// which role. The first account is always an admin. Otherwise an invite
// token, then a pre-approved email, sets the role and skips the rules;
// failing both, the rules apply and the account is a labeler. Without
// checkApproved a pre-approval is ignored, for emails Slack hasn't verified.
//
// It runs in the sign-up's transaction. A used invite or pre-approval stays
// locked until it commits, so it can't be used twice at once.
func grantSignup(ctx context.Context, q *db.Queries, rules signupRules, token, email string, checkApproved bool) (grant, error) {
	// The table lock stops two first sign-ups from both becoming admin.
	if err := q.LockUsers(ctx); err != nil {
		return grant{}, fmt.Errorf("lock users: %w", err)
	}
	n, err := q.CountUsers(ctx)
	if err != nil {
		return grant{}, fmt.Errorf("count users: %w", err)
	}
	if n == 0 {
		if !InEmailDomain(email, rules.emailDomain) {
			return grant{}, errDomain{rules.emailDomain}
		}
		return grant{role: "admin"}, nil
	}

	if token != "" {
		inv, err := q.UseInvite(ctx, hashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return grant{}, errInviteUsed
		}
		if err != nil {
			return grant{}, fmt.Errorf("use invite: %w", err)
		}
		return grant{
			role: inv.Role,
			redeem: func(ctx context.Context, q *db.Queries, userID uuid.UUID) error {
				return q.AddInviteUse(ctx, db.AddInviteUseParams{InviteID: inv.ID, UserID: userID})
			},
		}, nil
	}

	if checkApproved {
		approved, err := q.LockApprovedEmail(ctx, email)
		if err == nil {
			return grant{
				role: approved.Role,
				redeem: func(ctx context.Context, q *db.Queries, userID uuid.UUID) error {
					return q.UseApprovedEmail(ctx, db.UseApprovedEmailParams{Email: email, UsedBy: pgtype.UUID{Bytes: userID, Valid: true}})
				},
			}, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return grant{}, fmt.Errorf("check approved email: %w", err)
		}
	}

	if !InEmailDomain(email, rules.emailDomain) {
		return grant{}, errDomain{rules.emailDomain}
	}
	if !rules.allowSignup {
		return grant{}, errClosed
	}
	return grant{role: "labeler"}, nil
}

type errDomain struct{ domain string }

func (e errDomain) Error() string { return "sign up with your @" + e.domain + " email" }

func (g grant) record(ctx context.Context, q *db.Queries, userID uuid.UUID) error {
	if g.redeem == nil {
		return nil
	}
	if err := g.redeem(ctx, q, userID); err != nil {
		return fmt.Errorf("record invite use: %w", err)
	}
	return nil
}
