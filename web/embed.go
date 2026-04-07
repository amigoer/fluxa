// Package web serves the embedded admin dashboard.
//
// The React bundle under web/src is built with Vite into web/dist and
// embedded directly into the Go binary so Fluxa keeps its single-file
// deployment story — operators download one executable and get both
// the gateway and the admin UI. `make build` and `make run` compile
// the dashboard automatically, so normal workflows produce a binary
// that already serves the real SPA at the root URL.
//
// The repository keeps an empty web/dist/.gitkeep so the embed
// directive matches at least one file even on a brand-new clone that
// has not built the dashboard yet. In that case the handler falls
// back to an inline HTML stub that tells the operator how to build
// the real bundle — the gateway itself still starts normally.
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

// fallbackIndex is served when web/dist contains no index.html — which
// only happens on a fresh clone that has not run `make build` yet.
// The page is intentionally self-contained (no external assets) so it
// renders correctly even when embedded into a stripped binary.
const fallbackIndex = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <title>Fluxa admin — dashboard not built</title>
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
    <p>Rebuild the binary with the dashboard included:</p>
    <pre><code>make build</code></pre>
    <p>
      This will run <code>npm install &amp;&amp; npm run build</code> inside
      <code>web/</code> and then recompile the Go binary so the React SPA
      ends up embedded at the root URL. The admin REST API under
      <code>/admin/*</code> is already live on this port — the React UI
      is just a client on top of it.
    </p>
  </body>
</html>`

