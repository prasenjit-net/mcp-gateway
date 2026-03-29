package main

import (
	"embed"
	"io/fs"
	"net/http"
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
		path := strings.TrimPrefix(r.URL.Path, "/_ui")
		if path == "" || path == "/" {
			path = "/index.html"
		}
		// Check if asset file exists; if not, serve index.html for SPA routing
		if !strings.Contains(path, ".") {
			path = "/index.html"
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = path
		fileServer.ServeHTTP(w, r2)
	})
}
