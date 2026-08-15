package web

import (
	"io/fs"
	"net/http"
)

// Handler serves the built frontend as a single-page app: static assets
// are served directly, and any path that doesn't match a real file (a
// client-side route like /admin/overview) falls back to index.html for
// react-router to take over. API and gateway routes are mounted at
// /api and /v1, so they never reach this handler.
func Handler() (http.Handler, error) {
	dist, err := fs.Sub(DistFS, "dist")
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(dist))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := dist.Open(trimLeadingSlash(r.URL.Path)); err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}

func trimLeadingSlash(p string) string {
	if len(p) > 0 && p[0] == '/' {
		return p[1:]
	}
	return p
}
