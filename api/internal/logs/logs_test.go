package logs

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		key, value, want string
	}{
		{"password", "hunter2", redacted},
		{"worker_token", "abc", redacted},
		{"KRILL_RESEND_API_KEY", "re_123", redacted},
		{"req.Authorization", "Bearer abc", redacted},
		{"err", "connect postgres://krill:hunter2@db:5432/krill failed", "connect postgres://krill:[redacted]@db:5432/krill failed"},
		{"url", "http://minio/f.jpg?X-Amz-Credential=AK%2F1&X-Amz-Signature=deadbeef", "http://minio/f.jpg?X-Amz-Credential=[redacted]&X-Amz-Signature=[redacted]"},
		{"err", `header "Authorization: Bearer s3cr3t"`, `header "Authorization: Bearer [redacted]"`},
		{"msg", `config {"api_key": "re_123", "team": "x"}`, `config {"api_key": "[redacted]", "team": "x"}`},
		// Logged emails have to stay readable in development.
		{"subject", "123456 is your Krill sign-in code", "123456 is your Krill sign-in code"},
		{"text", "Enter this code in Krill to sign in:\n\n123456", "Enter this code in Krill to sign in:\n\n123456"},
		{"key", "videos/1/source", "videos/1/source"},
	}
	for _, tt := range tests {
		if got := redactAttr(tt.key, tt.value); got != tt.want {
			t.Errorf("redactAttr(%q, %q) = %q, want %q", tt.key, tt.value, got, tt.want)
		}
	}
}

func TestHandlerKeepsStdoutAndBuffers(t *testing.T) {
	var out bytes.Buffer
	buf := NewBuffer(10)
	inner := slog.NewJSONHandler(&out, nil)
	log := slog.New(NewHandler(inner, buf)).With("job", 7).WithGroup("req").With("token", "abc")
	log.Debug("dropped")
	log.Info("hello", "err", errors.New("boom"), slog.Group("db", "password", "pw", "rows", 3))

	var plain bytes.Buffer
	slog.New(slog.NewJSONHandler(&plain, nil)).With("job", 7).WithGroup("req").With("token", "abc").
		Info("hello", "err", errors.New("boom"), slog.Group("db", "password", "pw", "rows", 3))
	// Strip the times, which differ.
	trim := func(s string) string {
		_, after, _ := strings.Cut(s, `"level"`)
		return after
	}
	if trim(out.String()) != trim(plain.String()) {
		t.Errorf("stdout changed:\n got %s\nwant %s", out.String(), plain.String())
	}

	records, _ := buf.List(0, func(*krillv1.LogRecord) bool { return true }, 10)
	if len(records) != 1 {
		t.Fatalf("%d records, want 1", len(records))
	}
	got := map[string]string{}
	for _, a := range records[0].GetAttrs() {
		got[a.GetKey()] = a.GetValue()
	}
	want := map[string]string{"job": "7", "req.token": redacted, "req.err": "boom", "req.db.password": redacted, "req.db.rows": "3"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("attr %s = %q, want %q", k, got[k], v)
		}
	}
}

func TestBufferKeepsNewestAndSkipsResent(t *testing.T) {
	b := NewBuffer(3)
	rec := func(seq int64, msg string) *krillv1.LogRecord {
		return &krillv1.LogRecord{Seq: seq, Message: msg, Level: krillv1.LogLevel_LOG_LEVEL_INFO}
	}
	b.Ingest([]*krillv1.LogRecord{rec(1, "a"), rec(2, "b")})
	// A heartbeat whose response was lost sends 2 again.
	b.Ingest([]*krillv1.LogRecord{rec(2, "b"), rec(3, "c"), rec(4, "d"), rec(5, "e")})
	all := func(*krillv1.LogRecord) bool { return true }
	messages := func(rs []*krillv1.LogRecord) string {
		var s []string
		for _, r := range rs {
			s = append(s, r.GetMessage())
		}
		return strings.Join(s, "")
	}
	if got, last := b.List(0, all, 10); messages(got) != "cde" || last != 5 {
		t.Errorf("List = %q, %d", messages(got), last)
	}
	if got, _ := b.List(3, all, 10); messages(got) != "de" {
		t.Errorf("after 3 = %q", messages(got))
	}
	if got, _ := b.List(0, all, 2); messages(got) != "de" {
		t.Errorf("limit 2 = %q", messages(got))
	}
	match := matcher(krillv1.LogLevel_LOG_LEVEL_UNSPECIFIED, "C")
	if got, _ := b.List(0, match, 10); messages(got) != "c" {
		t.Errorf("query = %q", messages(got))
	}
}
