// Package web serves the embedded admin console.
//
// The console is a React + Vite single-page app under web/src. Vite
// builds it into web/dist and it is embedded directly into the Go binary
// so Fluxa keeps its single-file deployment story — operators download
// one executable and get both the gateway and the admin UI. `make build`
// and `make run` compile the console automatically.
//
// The repository keeps an empty web/dist/.gitkeep so the embed directive
// matches at least one file even on a fresh clone that has not built the
// console yet. In that case the handler serves an inline HTML stub that
// tells the operator how to build the real bundle — the gateway and the
// /admin REST API run normally regardless.
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

// assetsDir is the subdirectory Vite writes content-hashed bundles into
// (build.assetsDir in web/vite.config.ts). Requests below it are treated
// as static assets rather than SPA routes.
const assetsDir = "assets"

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
			// A miss under assets/ is a stale or mistyped bundle URL, not
			// a client-side route. Falling through to index.html there
			// would answer a script request with HTML, which surfaces as
			// a confusing MIME type error in the browser instead of an
			// honest 404.
			if strings.HasPrefix(rel, assetsDir+"/") {
				http.NotFound(w, r)
				return
			}
			serveIndex(w, index)
			return
		}
		// Hashed bundle files are immutable by construction, so let
		// browsers and proxies keep them for a year. index.html is
		// deliberately excluded: it is the file that points at the new
		// hashes after a deploy.
		if strings.HasPrefix(rel, assetsDir+"/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
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

// fallbackIndex is served when web/dist contains no index.html, which
// only happens on a fresh clone that has not run `make build` yet. The
// page is intentionally self-contained (no external assets) so it
// renders correctly even when embedded into a stripped binary.
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
    <h1>Fluxa admin console</h1>
    <p class="hint">No compiled bundle found in the embedded filesystem.</p>
    <p>Rebuild the binary with the console included:</p>
    <pre><code>make build</code></pre>
    <p>
      This runs <code>npm install &amp;&amp; npm run build</code> inside
      <code>web/</code> and recompiles the Go binary so the SPA ends up
      embedded at the root URL. The admin REST API under
      <code>/admin/*</code> is already live on this port — the console is
      just a client on top of it.
    </p>
  </body>
</html>`
