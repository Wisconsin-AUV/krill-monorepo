package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/uuid"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func fakeSlack(t *testing.T, tokenError, teamID string) *Slack {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/openid.connect.token":
			if tokenError != "" {
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": tokenError})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "access_token": "xoxp-1"})
		case "/api/openid.connect.userInfo":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "sub": "U1", "name": "Bucky",
				"https://slack.com/team_id": teamID, "https://slack.com/team_name": "WAUV",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	s := NewSlack(nil, func() SlackConfig {
		return SlackConfig{ClientID: "id", ClientSecret: "secret", TeamID: "T0123", PublicURL: "https://krill.test"}
	}, Cookies{Secure: true})
	s.baseURL = srv.URL
	return s
}

// A sign-in test must come back to the settings page without signing anyone
// in, and only the admin who started it may read how it went.
func TestSlackTestDoesNotSignIn(t *testing.T) {
	tests := []struct {
		name       string
		tokenError string
		teamID     string
		want       krillv1.SlackProblem
	}{
		{"works", "", "T0123", krillv1.SlackProblem_SLACK_PROBLEM_UNSPECIFIED},
		{"wrong workspace", "", "T9999", krillv1.SlackProblem_SLACK_PROBLEM_WRONG_TEAM},
		{"redirect not registered", "bad_redirect_uri", "", krillv1.SlackProblem_SLACK_PROBLEM_REDIRECT_URL},
		{"bad secret", "bad_client_secret", "", krillv1.SlackProblem_SLACK_PROBLEM_CLIENT_SECRET},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := fakeSlack(t, tt.tokenError, tt.teamID)
			owner := uuid.New()
			authorize, id, check := s.StartTest(owner)
			if check.Problem != krillv1.SlackProblem_SLACK_PROBLEM_UNSPECIFIED {
				t.Fatalf("start: %v", check.Problem)
			}
			loc, err := url.Parse(authorize)
			if err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			s.Callback(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
				"/auth/slack/callback?code=c&state="+url.QueryEscape(loc.Query().Get("state")), nil))

			if got := rec.Header().Get("Location"); got != "/admin/settings?slack_test="+url.QueryEscape(id) {
				t.Errorf("Location = %q", got)
			}
			for _, c := range rec.Result().Cookies() {
				if c.Name == CookieName {
					t.Errorf("set a session cookie")
				}
			}
			if got := s.TestResult(uuid.New(), id).Problem; got != krillv1.SlackProblem_SLACK_PROBLEM_EXPIRED {
				t.Errorf("another admin read the result: %v", got)
			}
			if got := s.TestResult(owner, id).Problem; got != tt.want {
				t.Errorf("problem = %v, want %v", got, tt.want)
			}
		})
	}
}
