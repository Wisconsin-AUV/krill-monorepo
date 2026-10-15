package email

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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
		"signin": SignInCode,
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

// html/template replaces a URL it doesn't trust in an href, which would leave
// the invite button going nowhere.
func TestInviteLinksToTheInvite(t *testing.T) {
	url := "https://krill.example.com/register?invite=3q2-7wE_AAAA"
	m, err := InviteEmail("bucky@wisc.edu", Invite{
		Inviter: "Bucky", Role: "labeler", URL: url, Expires: "on 21 October 2026", Team: "WAUV", Year: 2026,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.HTML, `href="`+url+`"`) {
		t.Error("HTML doesn't link to the invite")
	}
	if !strings.Contains(m.Text, url) || strings.Contains(m.HTML+m.Text, "{{") {
		t.Error("text is missing the invite link or has an unfilled action")
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

// A template without a preview would be missing from the admin panel.
func TestEveryTemplateHasAPreview(t *testing.T) {
	got, err := Previews(Sample{To: "bucky@wisc.edu", Username: "bucky", Team: "WAUV", Now: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	previewed := map[string]bool{}
	for _, p := range got {
		previewed[p.Name] = true
		if strings.Contains(p.Message.HTML+p.Message.Text, "{{") {
			t.Errorf("%s: unfilled template action", p.Name)
		}
	}
	for _, tmpl := range htmlTemplates.Templates() {
		if name := strings.TrimSuffix(tmpl.Name(), ".html"); !previewed[name] {
			t.Errorf("%s has no preview", name)
		}
	}
}
