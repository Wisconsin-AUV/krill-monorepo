// Package web serves the built React app with a single-page-app fallback.
package web

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Handler serves files from dir. Unknown paths get index.html so client-side
// routes like /clips/12 survive a page reload.
func Handler(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean("/" + r.URL.Path)
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(clean)))
		if err != nil || info.IsDir() {
			serveIndex(w, r, dir)
			return
		}
		// Vite puts a content hash in every asset name.
		if strings.HasPrefix(clean, "/assets/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

func serveIndex(w http.ResponseWriter, r *http.Request, dir string) {
	f, err := os.Open(filepath.Join(dir, "index.html")) //nolint:gosec // dir is operator config
	if err != nil {
		http.Error(w, "web app not built", http.StatusNotFound)
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, "web app not readable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, "index.html", info.ModTime(), f)
}
