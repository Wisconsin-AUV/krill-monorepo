package auth

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func testSlack() *Slack {
	return NewSlack(nil, SlackConfig{ClientID: "id", ClientSecret: "secret", TeamID: "T1", PublicURL: "https://krill.test"}, Cookies{Secure: true})
}

func TestSlackLogin(t *testing.T) {
	rec := httptest.NewRecorder()
	testSlack().Login(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/slack/login", nil))

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
	for _, cookie := range []string{"", "other"} {
		req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/auth/slack/callback?code=good-code&state=abc", nil)
		if cookie != "" {
			req.Header.Set("Cookie", stateCookie+"="+cookie)
		}
		rec := httptest.NewRecorder()
		testSlack().Callback(rec, req)
		if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/login?error=") {
			t.Errorf("cookie %q: Location = %q, want the login page", cookie, loc)
		}
	}
}
