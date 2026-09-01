import { readFileSync, readdirSync } from 'node:fs'
import { join, relative } from 'node:path'
import { gzipSync } from 'node:zlib'
import type { Plugin } from 'vite'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

const ORIGIN = 'https://filenkov.store'
const NODES_DIR = new URL('./src/resume/nodes', import.meta.url).pathname

/** Every folder under nodes/ that holds a meta.ts is a public URL. */
function nodePaths(dir = NODES_DIR): string[] {
  const found: string[] = []
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue
    const child = join(dir, entry.name)
    const meta = join(child, 'meta.ts')
    try {
      // A node behind sign-in has nothing to offer a crawler.
      if (!readFileSync(meta, 'utf8').includes('visibleWhen')) {
        found.push(relative(NODES_DIR, child).split('\\').join('/'))
      }
    } catch {
      // not a node folder — keep walking, it may hold some
    }
    found.push(...nodePaths(child))
  }
  return found
}

/**
 * The registry knows every URL at build time, so the sitemap is free (§13.1).
 * Full prerendering is deliberately not done here — it needs headless Chromium.
 */
function sitemap(): Plugin {
  return {
    name: 'resume-sitemap',
    apply: 'build',
    generateBundle() {
      const paths = ['', ...nodePaths()].sort()
      const urls = paths
        .map((path) => `  <url><loc>${ORIGIN}/${path}</loc></url>`)
        .join('\n')
      this.emitFile({
        type: 'asset',
        fileName: 'sitemap.xml',
        source: `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${urls}\n</urlset>\n`,
      })
      this.emitFile({
        type: 'asset',
        fileName: 'robots.txt',
        source: `User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /hermes\n\nSitemap: ${ORIGIN}/sitemap.xml\n`,
      })
    },
  }
}

/** Fails the build when the shell outgrows the §10 budget. */
const INITIAL_JS_BUDGET_KB = 95

function bundleGuard(): Plugin {
  return {
    name: 'resume-bundle-guard',
    apply: 'build',
    closeBundle() {
      const dist = new URL('./dist/assets', import.meta.url).pathname
      let entry: string | undefined
      try {
        entry = readdirSync(dist).find((name) => /^index-.*\.js$/.test(name))
      } catch {
        return
      }
      if (!entry) return
      const gzipped = gzipSync(readFileSync(join(dist, entry))).length / 1024
      const size = gzipped.toFixed(1)
      if (gzipped > INITIAL_JS_BUDGET_KB) {
        throw new Error(`Initial JS is ${size} KB gzipped, over the ${INITIAL_JS_BUDGET_KB} KB budget`)
      }
      console.log(`  initial JS ${size} KB gzipped (budget ${INITIAL_JS_BUDGET_KB} KB)`)
    },
  }
}

export default defineConfig({
  base: '/',
  plugins: [react(), sitemap(), bundleGuard()],
  build: {
    rollupOptions: {
      output: {
        // Name each node chunk after its URL path, so the network tab reads
        // "experience-sber-cheaper-tokens" rather than thirty files called "node".
        chunkFileNames(chunk) {
          const match = /resume\/nodes\/(.*?)\/?node\.tsx$/.exec(chunk.facadeModuleId ?? '')
          if (!match) return 'assets/[name]-[hash].js'
          const name = match[1].replace(/\//g, '-') || 'start'
          return `assets/node-${name}-[hash].js`
        },
      },
    },
  },
  server: {
    proxy: {
      '/api': { target: 'http://127.0.0.1:8083', changeOrigin: true },
      '/oauth': { target: 'http://127.0.0.1:8083', changeOrigin: true },
      '/users': { target: 'http://127.0.0.1:8083', changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
  },
})
