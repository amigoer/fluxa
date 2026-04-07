import path from "path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Vite config for the Fluxa admin dashboard.
//
// - `base: "/"` keeps asset URLs absolute (e.g. /assets/index-xyz.js)
//   so client-side routes like /providers or /keys still load the
//   bundle correctly on a hard refresh — relative paths would resolve
//   against the deepest URL segment and 404 the JS chunk.
// - dev mode proxies /admin and /v1 to the Go backend so you can run
//   `npm run dev` side-by-side with `go run ./cmd/fluxa`.
export default defineConfig({
  base: "/",
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
