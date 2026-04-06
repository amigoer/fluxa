// Package web serves the embedded admin dashboard.
//
// The React bundle under web/src is built with Vite into web/dist and
// embedded directly into the Go binary so Fluxa keeps its single-file
// deployment story — operators download one executable and get both
// the gateway and the admin UI.
//
// A committed placeholder index.html ships in web/dist so the embed
// directive always matches at least one file, even on a fresh clone
// where `npm run build` has not run yet. The placeholder renders a
// short note telling the operator how to build the real dashboard.
//
// Routing: the handler falls back to index.html on any path that does
// not map to a file in dist/, which is the standard single-page-app
// pattern Vite-built bundles expect.

package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// Handler returns an http.Handler rooted at the given URL prefix
// (typically "/ui/"). It serves static assets for real files and
// falls through to index.html for SPA routes.
func Handler(prefix string) http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Only possible if the embed directive is broken at build time.
		panic("web: failed to locate embedded dist: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	index, _ := fs.ReadFile(sub, "index.html")

	return http.StripPrefix(strings.TrimRight(prefix, "/"), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/")
		if rel == "" {
			serveIndex(w, index)
			return
		}
		if _, err := fs.Stat(sub, rel); err != nil {
			serveIndex(w, index)
			return
		}
		fileServer.ServeHTTP(w, r)
	}))
}

// serveIndex writes the SPA shell. When the embedded index is empty
// (shouldn't happen, but defensive) we return 404 rather than a blank
// page so misconfiguration is loud.
func serveIndex(w http.ResponseWriter, index []byte) {
	if len(index) == 0 {
		http.NotFound(w, nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}
