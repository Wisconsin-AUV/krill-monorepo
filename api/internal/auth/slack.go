package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wauv/krill/api/internal/db"
)

const stateCookie = "krill_slack_state"

type SlackConfig struct {
	ClientID     string
	ClientSecret string
	// TeamID is the only workspace allowed to sign in.
	TeamID string
	// PublicURL is where browsers reach the app, used for the redirect URL.
	PublicURL string
}

// Slack implements Sign in with Slack, which is OpenID Connect and works on
// free workspaces. Unknown Slack users get a labeler account, and a Slack
// user whose verified email matches an account is linked to it.
type Slack struct {
	cfg     SlackConfig
	pool    *pgxpool.Pool
	q       *db.Queries
	cookies Cookies
	client  *http.Client
	baseURL string
}

func NewSlack(pool *pgxpool.Pool, cfg SlackConfig, cookies Cookies) *Slack {
	return &Slack{
		cfg:     cfg,
		pool:    pool,
		q:       db.New(pool),
		cookies: cookies,
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://slack.com",
	}
}

func (s *Slack) redirectURL() string {
	return s.cfg.PublicURL + "/auth/slack/callback"
}

func (s *Slack) Login(w http.ResponseWriter, r *http.Request) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		http.Error(w, "could not start sign-in", http.StatusInternalServerError)
		return
	}
	state := base64.RawURLEncoding.EncodeToString(raw)
	http.SetCookie(w, s.stateCookie(state, 600))
	q := url.Values{
		"response_type": {"code"},
		"scope":         {"openid profile email"},
		"client_id":     {s.cfg.ClientID},
		"state":         {state},
		"team":          {s.cfg.TeamID},
		"redirect_uri":  {s.redirectURL()},
	}
	http.Redirect(w, r, s.baseURL+"/openid/connect/authorize?"+q.Encode(), http.StatusFound)
}

// Callback finishes sign-in and redirects to the app, or to the login page
// with an error message.
func (s *Slack) Callback(w http.ResponseWriter, r *http.Request) {
	fail := func(msg string) {
		http.Redirect(w, r, "/login?error="+url.QueryEscape(msg), http.StatusFound)
	}
	http.SetCookie(w, s.stateCookie("", -1))

	query := r.URL.Query()
	if query.Get("error") != "" {
		fail("Slack sign-in was cancelled.")
		return
	}
	c, err := r.Cookie(stateCookie)
	if err != nil || query.Get("state") == "" ||
		subtle.ConstantTimeCompare([]byte(c.Value), []byte(query.Get("state"))) != 1 {
		fail("Slack sign-in expired. Try again.")
		return
	}

	id, err := s.identity(r.Context(), query.Get("code"))
	if err != nil {
		slog.Error("slack sign-in", "err", err)
		fail("Could not reach Slack. Try again.")
		return
	}
	if id.TeamID != s.cfg.TeamID {
		fail("That Slack workspace can't sign in here.")
		return
	}
	cookie, err := s.signIn(r.Context(), id)
	if err != nil {
		var msg userError
		if errors.As(err, &msg) {
			fail(string(msg))
			return
		}
		slog.Error("slack sign-in", "err", err)
		fail("Could not sign in with Slack.")
		return
	}
	http.SetCookie(w, cookie)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Slack) stateCookie(value string, maxAge int) *http.Cookie {
	//nolint:gosec // Secure is off only when the app is served over plain HTTP
	return &http.Cookie{
		Name:     stateCookie,
		Value:    value,
		Path:     "/auth/slack",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cookies.Secure,
		// Lax, not Strict, so the cookie survives the redirect back from Slack.
		SameSite: http.SameSiteLaxMode,
	}
}

type userError string

func (e userError) Error() string { return string(e) }

type slackIdentity struct {
	OK            bool   `json:"ok"`
	Error         string `json:"error"`
	Sub           string `json:"sub"`
	TeamID        string `json:"https://slack.com/team_id"`
	Name          string `json:"name"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}

// identity trades the authorization code for the user's Slack profile. The
// profile comes straight from Slack over TLS, so the ID token is not needed.
func (s *Slack) identity(ctx context.Context, code string) (slackIdentity, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {s.cfg.ClientID},
		"client_secret": {s.cfg.ClientSecret},
		"redirect_uri":  {s.redirectURL()},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/openid.connect.token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return slackIdentity{}, fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token struct {
		OK          bool   `json:"ok"`
		Error       string `json:"error"`
		AccessToken string `json:"access_token"`
	}
	if err := s.do(req, &token); err != nil {
		return slackIdentity{}, fmt.Errorf("exchange code: %w", err)
	}
	if !token.OK {
		return slackIdentity{}, fmt.Errorf("exchange code: %s", token.Error)
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+"/api/openid.connect.userInfo", nil)
	if err != nil {
		return slackIdentity{}, fmt.Errorf("build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	var id slackIdentity
	if err := s.do(req, &id); err != nil {
		return slackIdentity{}, fmt.Errorf("get user info: %w", err)
	}
	if !id.OK || id.Sub == "" {
		return slackIdentity{}, fmt.Errorf("get user info: %s", id.Error)
	}
	return id, nil
}

func (s *Slack) do(req *http.Request, out any) error {
	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned %s", res.Status)
	}
	return json.NewDecoder(res.Body).Decode(out)
}

func (s *Slack) signIn(ctx context.Context, id slackIdentity) (*http.Cookie, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)

	user, err := s.findOrCreate(ctx, q, id)
	if err != nil {
		return nil, err
	}
	if user.Disabled {
		return nil, userError("This account is disabled.")
	}
	cookie, err := s.cookies.StartSession(ctx, q, user.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return cookie, nil
}

func (s *Slack) findOrCreate(ctx context.Context, q *db.Queries, id slackIdentity) (db.User, error) {
	user, err := q.GetUserBySlackID(ctx, text(id.Sub))
	if !errors.Is(err, pgx.ErrNoRows) {
		return user, err
	}
	email := strings.ToLower(strings.TrimSpace(id.Email))
	if id.EmailVerified {
		user, err := q.GetUserByEmail(ctx, email)
		if err == nil {
			if user.SlackUserID.Valid {
				return db.User{}, userError("That email belongs to an account linked to a different Slack user.")
			}
			return q.LinkSlack(ctx, db.LinkSlackParams{ID: user.ID, SlackUserID: text(id.Sub)})
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return db.User{}, err
		}
	}

	username, err := freeUsername(ctx, q, email)
	if err != nil {
		return db.User{}, err
	}
	name := id.Name
	if strings.TrimSpace(name) == "" {
		name = username
	}
	p, err := NormalizeProfile(name, username, email)
	if err != nil {
		return db.User{}, userError("Your Slack profile has no usable email.")
	}
	role, err := newUserRole(ctx, q, true)
	if err != nil {
		return db.User{}, err
	}
	user, err = q.CreateUser(ctx, db.CreateUserParams{
		Name: p.Name, Username: p.Username, Email: p.Email, Role: role, SlackUserID: text(id.Sub),
	})
	if DuplicateError(err) != nil {
		return db.User{}, userError("An account with your Slack email already exists. Sign in with your password instead.")
	}
	return user, err
}

var usernameStrip = regexp.MustCompile(`[^a-z0-9_.-]`)

// freeUsername derives an unused username from the email's local part.
func freeUsername(ctx context.Context, q *db.Queries, email string) (string, error) {
	base := strings.TrimLeft(usernameStrip.ReplaceAllString(strings.ToLower(strings.Split(email, "@")[0]), ""), "_.-")
	if len(base) < 2 {
		base = "user"
	}
	base = base[:min(len(base), 28)]
	for i := 1; i < 1000; i++ {
		name := base
		if i > 1 {
			name = fmt.Sprintf("%s%d", base, i)
		}
		taken, err := q.UsernameExists(ctx, name)
		if err != nil {
			return "", fmt.Errorf("check username: %w", err)
		}
		if !taken {
			return name, nil
		}
	}
	return "", fmt.Errorf("no free username for %q", base)
}
