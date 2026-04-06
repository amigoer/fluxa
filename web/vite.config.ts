import path from "path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Vite config for the Fluxa admin dashboard.
//
// - `base: "./"` makes the built bundle path-relative so it works when
//   the Go binary mounts the SPA at /ui/ (the go:embed filesystem does
//   not know the URL prefix at build time).
// - dev mode proxies /admin and /v1 to the Go backend so you can run
//   `npm run dev` side-by-side with `go run ./cmd/fluxa`.
export default defineConfig({
  base: "./",
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/admin": "http://localhost:8080",
      "/v1": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
