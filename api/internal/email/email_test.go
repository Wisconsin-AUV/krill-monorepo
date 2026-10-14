package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTemplatesFillEveryField(t *testing.T) {
	c := Code{
		Code: "042137", Minutes: 10, IP: "128.104.24.71", Device: "Firefox on macOS",
		Time: "14 October 2026, 05:04 UTC", Team: "<b>WAUV</b>", Year: 2026,
		Email: "bucky@wisc.edu", Username: "bucky",
	}
	for name, build := range map[string]func(string, Code) (Message, error){
		"verify": VerifyEmail,
		"reset":  ResetPassword,
	} {
		m, err := build("bucky@wisc.edu", c)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, body := range []string{m.HTML, m.Text} {
			if strings.Contains(body, "{{") {
				t.Errorf("%s: unfilled template action", name)
			}
			for _, want := range []string{c.Code, c.IP, c.Device, c.Time} {
				if !strings.Contains(body, want) {
					t.Errorf("%s: missing %q", name, want)
				}
			}
		}
		if strings.Contains(m.HTML, "<b>WAUV</b>") || !strings.Contains(m.HTML, "&lt;b&gt;WAUV&lt;/b&gt;") {
			t.Errorf("%s: team name not escaped in HTML", name)
		}
		if !strings.HasPrefix(m.Subject, c.Code) {
			t.Errorf("%s: subject %q doesn't lead with the code", name, m.Subject)
		}
	}
}

func TestResendSend(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer re_key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	r := NewResend("re_key", "Krill <noreply@krill.camdenrush.com>")
	r.baseURL = srv.URL
	if err := r.Send(context.Background(), Message{To: "bucky@wisc.edu", Subject: "s", HTML: "h", Text: "t"}); err != nil {
		t.Fatal(err)
	}
	if got["from"] != "Krill <noreply@krill.camdenrush.com>" || got["to"].([]any)[0] != "bucky@wisc.edu" {
		t.Errorf("sent %v", got)
	}

	r.apiKey = "wrong"
	if err := r.Send(context.Background(), Message{To: "bucky@wisc.edu"}); err == nil {
		t.Error("expected an error when Resend rejects the request")
	}
}
