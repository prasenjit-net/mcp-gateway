package main

import (
	"embed"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed ui/dist
var uiFiles embed.FS

func uiHandler() http.Handler {
	sub, err := fs.Sub(uiFiles, "ui/dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Strip /_ui prefix so file server sees paths rooted at ui/dist/
		path := strings.TrimPrefix(r.URL.Path, "/_ui")
		if path == "" {
			path = "/"
		}

		// Assets (anything with a file extension) are served directly.
		// FileServer handles caching headers and correct MIME types.
		if ext := filepath.Ext(path); ext != "" {
			r2 := r.Clone(r.Context())
			r2.URL.Path = path
			fileServer.ServeHTTP(w, r2)
			return
		}

		// All other paths (SPA client-side routes) get index.html directly.
		// We do NOT pass these through FileServer to avoid its
		// index.html → "/" redirect which causes ERR_TOO_MANY_REDIRECTS.
		index, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.Error(w, "UI not built — run: make build-ui", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(index) //nolint:errcheck
	})
}
