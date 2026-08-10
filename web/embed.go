// Package web serves the embedded admin dashboard.
//
// The dashboard front-end is built into web/dist and embedded directly
// into the Go binary so Fluxa keeps its single-file deployment story —
// operators download one executable and get both the gateway and the
// admin UI. `make build` and `make run` compile the dashboard
// automatically when a front-end is present under web/.
//
// The previous React dashboard was removed pending a rewrite, so right
// now web/dist holds only the tracked .gitkeep placeholder that keeps
// the embed directive matching at least one file. With no index.html to
// serve, the handler falls back to an inline HTML stub — the gateway
// and the /admin REST API run normally regardless.
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
// (typically "/"). It serves static assets for real files and falls
// through to index.html for SPA routes so client-side navigation
// works on a hard reload.
func Handler(prefix string) http.Handler {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		// Only possible if the embed directive is broken at build time.
		panic("web: failed to locate embedded dist: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	index, _ := fs.ReadFile(sub, "index.html")
	if len(index) == 0 {
		index = []byte(fallbackIndex)
	}

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

// serveIndex writes the SPA shell (or the fallback stub when no real
// bundle has been embedded yet).
func serveIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(index)
}

// fallbackIndex is served when web/dist contains no index.html — the
// case while the dashboard front-end is being rewritten, and on a fresh
// clone that has not run `make build` yet. The page is intentionally
// self-contained (no external assets) so it renders correctly even when
// embedded into a stripped binary.
const fallbackIndex = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <title>Fluxa admin — dashboard unavailable</title>
    <style>
      body { font-family: ui-sans-serif, system-ui, -apple-system, sans-serif;
             max-width: 40rem; margin: 4rem auto; padding: 0 1rem;
             line-height: 1.5; color: #1f2937; }
      code { background: #f3f4f6; padding: 0.1rem 0.3rem; border-radius: 4px; }
      h1 { margin-bottom: 0.25rem; }
      .hint { color: #6b7280; font-size: 0.9rem; }
      pre { background: #f3f4f6; padding: 1rem; border-radius: 6px; overflow-x: auto; }
    </style>
  </head>
  <body>
    <h1>Fluxa admin dashboard</h1>
    <p class="hint">No compiled bundle found in the embedded filesystem.</p>
    <p>
      The dashboard front-end has been removed and is being rewritten, so
      no UI is embedded in this binary. The admin REST API under
      <code>/admin/*</code> is live on this port and fully usable in the
      meantime — the dashboard is only a client on top of it.
    </p>
    <p>
      Once a front-end exists under <code>web/</code>, rebuild with
      <code>make build</code> to embed it at the root URL:
    </p>
    <pre><code>make build</code></pre>
  </body>
</html>`
