// Package web embeds the compiled frontend assets (produced by `npm run build`
// in web/) into the Go binary so the server ships as a single self-contained
// executable with no external static files.
package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// distFS holds the contents of ./dist, populated at compile time via go:embed.
// The directive requires that the dist/ directory exists when `go build` runs;
// the Makefile / release workflow run `npm run build` before the Go build to
// ensure this.
//
//go:embed all:dist
var distFS embed.FS

// DistFS returns a filesystem rooted at the dist/ directory.
func DistFS() fs.FS {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}

// Handler returns an http.Handler that serves the embedded frontend with
// client-side routing support: any path that is not a real asset falls back
// to index.html so the SPA router can take over.
func Handler() http.Handler {
	fsys := DistFS()
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(fsys, p); err != nil {
			// SPA fallback: serve index.html for any unknown route.
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}