package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

// slackError is the error code in a Slack Web API response.
type slackError string

func (e slackError) Error() string { return string(e) }

// SlackCheck is how a Slack settings test went.
type SlackCheck struct {
	Problem krillv1.SlackProblem
	// Slack's error code.
	Detail      string
	RedirectURL string
	TeamID      string
	TeamName    string
	// The saved workspace ID.
	ExpectedTeamID string
	UserName       string
}

func (c SlackConfig) RedirectURL() string {
	return c.redirectURL()
}

var teamIDPattern = regexp.MustCompile(`^[TE][A-Z0-9]{4,}$`)

// precheck finds settings Slack would refuse without asking it.
func (c SlackConfig) precheck() krillv1.SlackProblem {
	switch {
	case c.ClientID == "":
		return krillv1.SlackProblem_SLACK_PROBLEM_OFF
	case !strings.HasPrefix(c.PublicURL, "https://"):
		return krillv1.SlackProblem_SLACK_PROBLEM_REDIRECT_URL
	case !teamIDPattern.MatchString(c.TeamID):
		return krillv1.SlackProblem_SLACK_PROBLEM_TEAM_ID
	}
	return krillv1.SlackProblem_SLACK_PROBLEM_UNSPECIFIED
}

// credentialProblem maps the error codes Slack uses for a bad client ID or
// secret on its OAuth methods.
func credentialProblem(code string) (krillv1.SlackProblem, bool) {
	switch code {
	case "invalid_client_id":
		return krillv1.SlackProblem_SLACK_PROBLEM_CLIENT_ID, true
	case "bad_client_secret", "invalid_client":
		return krillv1.SlackProblem_SLACK_PROBLEM_CLIENT_SECRET, true
	case "bad_redirect_uri":
		return krillv1.SlackProblem_SLACK_PROBLEM_REDIRECT_URL, true
	}
	return 0, false
}

// CheckCredentials asks Slack whether the client ID and secret are right
// without anyone signing in. The token exchange methods all check the code
// or token before the client, so a made-up code only ever gets invalid_code.
// oauth.v2.exchange checks the client first, so a made-up legacy token gets
// invalid_client_id or bad_client_secret when those are wrong, and a token
// error when they are right.
func (s *Slack) CheckCredentials(ctx context.Context) SlackCheck {
	cfg := s.cfg()
	out := SlackCheck{RedirectURL: cfg.redirectURL(), ExpectedTeamID: cfg.TeamID}
	if out.Problem = cfg.precheck(); out.Problem != krillv1.SlackProblem_SLACK_PROBLEM_UNSPECIFIED {
		return out
	}
	form := url.Values{
		"client_id":     {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"token":         {"xoxp-0-0-0-0"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/oauth.v2.exchange",
		strings.NewReader(form.Encode()))
	if err != nil {
		out.Problem, out.Detail = krillv1.SlackProblem_SLACK_PROBLEM_OTHER, err.Error()
		return out
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var res struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := s.do(req, &res); err != nil {
		slog.Warn("check slack credentials", "err", err)
		out.Problem, out.Detail = krillv1.SlackProblem_SLACK_PROBLEM_UNREACHABLE, err.Error()
		return out
	}
	if p, ok := credentialProblem(res.Error); ok {
		out.Problem, out.Detail = p, res.Error
		return out
	}
	switch res.Error {
	case "invalid_token", "invalid_auth", "not_authed", "token_revoked", "token_expired",
		"not_allowed_token_type", "token_rotation_not_enabled", "token_already_exchanged",
		"authorization_not_found":
		return out
	}
	out.Problem, out.Detail = krillv1.SlackProblem_SLACK_PROBLEM_OTHER, res.Error
	return out
}

// slackTestTTL is how long a sign-in test can take, long enough to sign in
// to Slack first.
const slackTestTTL = 10 * time.Minute

type slackTest struct {
	owner   uuid.UUID
	state   string
	started time.Time
	done    bool
	result  SlackCheck
}

// slackTests are sign-in tests in flight. They are kept in memory because a
// lost test only means starting another.
type slackTests struct {
	mu   sync.Mutex
	byID map[string]*slackTest
}

func newSlackTests() *slackTests {
	return &slackTests{byID: map[string]*slackTest{}}
}

func (t *slackTests) prune(now time.Time) {
	for id, test := range t.byID {
		if now.Sub(test.started) > slackTestTTL {
			delete(t.byID, id)
		}
	}
}

// take returns the unfinished test the state belongs to, once.
func (t *slackTests) take(state string) (string, bool) {
	if state == "" {
		return "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.prune(time.Now())
	for id, test := range t.byID {
		if !test.done && test.state == state {
			test.state = ""
			return id, true
		}
	}
	return "", false
}

func (t *slackTests) finish(id string, result SlackCheck) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if test, ok := t.byID[id]; ok {
		test.done, test.result = true, result
	}
}

// StartTest begins a Slack sign-in that ends on the admin panel's settings
// page with a result, without signing anyone in.
func (s *Slack) StartTest(owner uuid.UUID) (authorizeURL, id string, check SlackCheck) {
	cfg := s.cfg()
	check = SlackCheck{RedirectURL: cfg.redirectURL(), ExpectedTeamID: cfg.TeamID}
	if check.Problem = cfg.precheck(); check.Problem != krillv1.SlackProblem_SLACK_PROBLEM_UNSPECIFIED {
		return "", "", check
	}
	id, state := rand.Text(), rand.Text()
	s.tests.mu.Lock()
	defer s.tests.mu.Unlock()
	s.tests.prune(time.Now())
	s.tests.byID[id] = &slackTest{owner: owner, state: state, started: time.Now()}
	return s.authorizeURL(cfg, state), id, check
}

// TestResult is the result of the owner's test, or an expired one when the
// test is unknown, someone else's, or never came back from Slack.
func (s *Slack) TestResult(owner uuid.UUID, id string) SlackCheck {
	s.tests.mu.Lock()
	defer s.tests.mu.Unlock()
	s.tests.prune(time.Now())
	test, ok := s.tests.byID[id]
	if !ok || test.owner != owner || !test.done {
		cfg := s.cfg()
		return SlackCheck{
			Problem:        krillv1.SlackProblem_SLACK_PROBLEM_EXPIRED,
			RedirectURL:    cfg.redirectURL(),
			ExpectedTeamID: cfg.TeamID,
		}
	}
	return test.result
}

func (s *Slack) finishTest(w http.ResponseWriter, r *http.Request, id string) {
	s.tests.finish(id, s.runTest(r.Context(), r.URL.Query()))
	http.Redirect(w, r, "/admin/settings?slack_test="+url.QueryEscape(id), http.StatusFound)
}

func (s *Slack) runTest(ctx context.Context, query url.Values) SlackCheck {
	cfg := s.cfg()
	out := SlackCheck{RedirectURL: cfg.redirectURL(), ExpectedTeamID: cfg.TeamID}
	if out.Problem = cfg.precheck(); out.Problem != krillv1.SlackProblem_SLACK_PROBLEM_UNSPECIFIED {
		return out
	}
	if e := query.Get("error"); e != "" {
		out.Problem, out.Detail = krillv1.SlackProblem_SLACK_PROBLEM_CANCELLED, e
		return out
	}
	id, err := s.identity(ctx, cfg, query.Get("code"))
	if err != nil {
		var code slackError
		if !errors.As(err, &code) {
			slog.Warn("slack sign-in test", "err", err)
			out.Problem, out.Detail = krillv1.SlackProblem_SLACK_PROBLEM_UNREACHABLE, err.Error()
			return out
		}
		out.Detail = string(code)
		if p, ok := credentialProblem(string(code)); ok {
			out.Problem = p
		} else {
			out.Problem = krillv1.SlackProblem_SLACK_PROBLEM_OTHER
		}
		return out
	}
	out.TeamID, out.TeamName, out.UserName = id.TeamID, id.TeamName, id.Name
	if id.TeamID != cfg.TeamID {
		out.Problem = krillv1.SlackProblem_SLACK_PROBLEM_WRONG_TEAM
	}
	return out
}
