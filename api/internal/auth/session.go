package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/wauv/krill/api/internal/db"
)

const (
	CookieName = "krill_session"
	sessionTTL = 30 * 24 * time.Hour
)

type Session struct {
	User      db.User
	TokenHash []byte
}

type sessionKey struct{}

func withSession(ctx context.Context, s Session) context.Context {
	return context.WithValue(ctx, sessionKey{}, s)
}

// SessionFrom returns the caller's session. It is always set for procedures
// that require a role.
func SessionFrom(ctx context.Context) (Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(Session)
	return s, ok
}

func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

func tokenFromHeader(h http.Header) string {
	c, err := (&http.Request{Header: h}).Cookie(CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}

type Cookies struct {
	// Secure is set when the app is served over HTTPS.
	Secure bool
}

func (c Cookies) StartSession(ctx context.Context, q *db.Queries, userID uuid.UUID) (*http.Cookie, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("read session token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	expires := time.Now().Add(sessionTTL)
	if err := q.DeleteExpiredSessions(ctx); err != nil {
		return nil, fmt.Errorf("delete expired sessions: %w", err)
	}
	if err := q.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: hashToken(token),
		UserID:    userID,
		ExpiresAt: timestamptz(expires),
	}); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	if err := q.TouchLogin(ctx, userID); err != nil {
		return nil, fmt.Errorf("record login: %w", err)
	}
	return c.cookie(token, expires), nil
}

func (c Cookies) Clear() *http.Cookie {
	return c.cookie("", time.Unix(0, 0))
}

func (c Cookies) cookie(value string, expires time.Time) *http.Cookie {
	//nolint:gosec // Secure is off only when the app is served over plain HTTP
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   c.Secure,
		// Lax keeps the cookie off cross-site POSTs, which covers CSRF for RPCs.
		SameSite: http.SameSiteLaxMode,
	}
}
