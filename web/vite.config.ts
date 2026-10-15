/// <reference types="vitest/config" />
import tailwindcss from '@tailwindcss/vite'
import react from '@vitejs/plugin-react'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'

// Bundles the repository's CHANGELOG.md for the changelog dialog. Vite serves
// files outside web/ as plain text, so a ?raw import of it breaks in dev.
function changelog(): Plugin {
  const id = 'virtual:changelog'
  const file = fileURLToPath(new URL('../CHANGELOG.md', import.meta.url))
  return {
    name: 'krill-changelog',
    resolveId: (source) => (source === id ? `\0${id}` : undefined),
    load(source) {
      if (source !== `\0${id}`) return
      this.addWatchFile(file)
      return `export default ${JSON.stringify(readFileSync(file, 'utf8'))}`
    },
  }
}

export default defineConfig({
  plugins: [react(), tailwindcss(), changelog()],
  resolve: {
    alias: { '@': new URL('./src', import.meta.url).pathname },
  },
  server: {
    port: 5173,
    proxy: {
      '/krill.v1.': process.env.KRILL_API_URL ?? 'http://localhost:8080',
      '/auth/': process.env.KRILL_API_URL ?? 'http://localhost:8080',
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
  },
})
