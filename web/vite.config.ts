import path from "node:path";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// The console is a pure client of the Go gateway's /admin REST API. In
// production the built bundle is embedded into the binary by web/embed.go
// and served from the same origin, so no CORS or base-path handling is
// needed. In development Vite serves the UI on :5173 and proxies the two
// API surfaces to the gateway on :8080, which keeps the origin identical
// in both modes — the app never has to know where it is running.
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/admin": { target: "http://localhost:8080", changeOrigin: true },
      "/v1": { target: "http://localhost:8080", changeOrigin: true },
      "/health": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
  build: {
    // web/embed.go embeds this directory; keep the name in sync with the
    // //go:embed all:dist directive.
    outDir: "dist",
    // The gateway serves these with far-future caching semantics in mind:
    // every asset name carries a content hash.
    assetsDir: "assets",
    sourcemap: false,
  },
});
