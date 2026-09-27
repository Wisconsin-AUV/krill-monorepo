package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	dir := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html>app</html>"), 0o600))
	must(os.Mkdir(filepath.Join(dir, "assets"), 0o750))
	must(os.WriteFile(filepath.Join(dir, "assets", "app-abc.js"), []byte("js"), 0o600))

	h := Handler(dir)
	tests := []struct {
		path, body, cache string
	}{
		{"/", "app", "no-cache"},
		{"/clips/12", "app", "no-cache"},
		{"/assets/app-abc.js", "js", "immutable"},
		{"/../../etc/passwd", "app", "no-cache"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tt.body) {
				t.Errorf("body = %q, want %q", rec.Body.String(), tt.body)
			}
			if !strings.Contains(rec.Header().Get("Cache-Control"), tt.cache) {
				t.Errorf("Cache-Control = %q, want %q", rec.Header().Get("Cache-Control"), tt.cache)
			}
		})
	}
}
