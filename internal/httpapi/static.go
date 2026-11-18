package httpapi

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
)

func init() {
	// Minimal container images ship no mime.types, and Go's built-in table lacks this one.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// spaHandler serves the embedded single-page app. Unknown paths fall back to
// index.html so client-side routes survive a page refresh.
func spaHandler(dist fs.FS) http.Handler {
	files := http.FileServer(http.FS(dist))
	_, err := fs.Stat(dist, "index.html")
	hasIndex := err == nil

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hasIndex {
			http.Error(w, "frontend not built: run `make web`", http.StatusNotFound)
			return
		}

		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if info, err := fs.Stat(dist, name); err != nil || info.IsDir() {
			name = "index.html"
		}

		if strings.HasPrefix(name, "assets/") {
			// Vite fingerprints everything under assets/, so it is safe to cache forever.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}

		if name == "index.html" {
			http.ServeFileFS(w, r, dist, "index.html")
			return
		}
		files.ServeHTTP(w, r)
	})
}
