import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(import.meta.dirname, './src'),
    },
  },
  build: {
    // go:embed can't reach outside its own directory tree, so the
    // build has to land directly in web/dist for web/embed.go to embed
    // it (DESIGN.md section 12).
    outDir: path.resolve(import.meta.dirname, '../web/dist'),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/v1': 'http://localhost:8080',
    },
  },
})
