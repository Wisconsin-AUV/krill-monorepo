package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func fakeSlack(t *testing.T) *Slack {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/openid.connect.token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.PostForm.Get("code") != "good-code" || r.PostForm.Get("client_secret") != "secret" {
			_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_code"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"access_token":"xoxp-1"}`))
	})
	mux.HandleFunc("GET /api/openid.connect.userInfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer xoxp-1" {
			_, _ = w.Write([]byte(`{"ok":false,"error":"invalid_auth"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"sub":"U1","https://slack.com/team_id":"T1","name":"Ada","email":"Ada@wisc.edu","email_verified":true}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	s := NewSlack(nil, SlackConfig{ClientID: "id", ClientSecret: "secret", TeamID: "T1", PublicURL: "https://krill.test"}, Cookies{Secure: true})
	s.baseURL = srv.URL
	return s
}

func TestSlackLogin(t *testing.T) {
	s := fakeSlack(t)
	rec := httptest.NewRecorder()
	s.Login(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/slack/login", nil))

	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	q := loc.Query()
	if q.Get("team") != "T1" || q.Get("redirect_uri") != "https://krill.test/auth/slack/callback" {
		t.Errorf("redirect = %s", loc)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != q.Get("state") || !cookies[0].Secure {
		t.Errorf("state cookie = %+v, state = %q", cookies, q.Get("state"))
	}
}

func TestSlackCallbackRejectsBadState(t *testing.T) {
	s := fakeSlack(t)
	for _, cookie := range []string{"", "other"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/slack/callback?code=good-code&state=abc", nil)
		if cookie != "" {
			req.Header.Set("Cookie", stateCookie+"="+cookie)
		}
		rec := httptest.NewRecorder()
		s.Callback(rec, req)
		if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/login?error=") {
			t.Errorf("cookie %q: Location = %q, want the login page", cookie, loc)
		}
	}
}

func TestSlackIdentity(t *testing.T) {
	s := fakeSlack(t)
	id, err := s.identity(t.Context(), "good-code")
	if err != nil {
		t.Fatal(err)
	}
	if id.Sub != "U1" || id.TeamID != "T1" || id.Email != "Ada@wisc.edu" || !id.EmailVerified {
		t.Errorf("identity = %+v", id)
	}
	if _, err := s.identity(t.Context(), "bad-code"); err == nil {
		t.Error("expected error for a bad code")
	}
}
