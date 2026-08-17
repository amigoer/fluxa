import { execSync } from 'child_process'
import path from 'path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

import pkg from './package.json' with { type: 'json' }

// The footer is where an admin reads the version off a self-hosted
// deployment to report a problem against, so it has to describe the
// build that is actually running rather than a literal somebody
// remembers to bump.
//
// The release number comes from package.json and the build it was cut
// from comes from git, because neither answers on its own: two
// deployments can both be v0.0.1 and be days of commits apart. The git
// half is optional -- a tarball or a shallow clone has no repository --
// and "-dirty" marks a build made from an uncommitted tree.
function buildVersion(): string {
  const version = `v${pkg.version}`
  try {
    const commit = execSync('git describe --always --dirty --abbrev=7', {
      stdio: ['ignore', 'pipe', 'ignore'],
    })
      .toString()
      .trim()
    return commit ? `${version} (${commit})` : version
  } catch {
    return version
  }
}

// https://vite.dev/config/
export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(buildVersion()),
  },
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
