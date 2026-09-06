/**
 * Generates the owl chat's map of the site from the resume-tree node folders.
 *
 * The registry has no central list of nodes (AGENTS.md), so the Go API cannot
 * import one — this walks the same folders `vite.config.ts` walks for the
 * sitemap and writes `internal/llm/nodes.json`, which `internal/llm` embeds
 * into the system prompt. Run `npm run gen:owl-context` after adding a node.
 *
 * Nodes with a `visibleWhen` are behind sign-in and are left out: the chat is
 * public, so its prompt may only carry what a signed-out reader can open.
 */
import { readFileSync, readdirSync, writeFileSync } from 'node:fs'
import { join, relative } from 'node:path'

const NODES_DIR = new URL('../src/resume/nodes', import.meta.url).pathname
const OUT = new URL('../../internal/llm/nodes.json', import.meta.url).pathname

/** meta.ts is a hand-written object literal, so a field read is a regex read. */
function field(source, name) {
  const single = new RegExp(`^\\s*${name}:\\s*'((?:[^'\\\\]|\\\\.)*)'`, 'm').exec(source)
  if (single) return single[1].replace(/\\'/g, "'").replace(/\\\\/g, '\\')
  const double = new RegExp(`^\\s*${name}:\\s*"((?:[^"\\\\]|\\\\.)*)"`, 'm').exec(source)
  return double ? double[1].replace(/\\"/g, '"').replace(/\\\\/g, '\\') : null
}

function collect(dir, out) {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (!entry.isDirectory()) continue
    const child = join(dir, entry.name)
    try {
      const meta = readFileSync(join(child, 'meta.ts'), 'utf8')
      if (!meta.includes('visibleWhen')) {
        out.push({
          path: '/' + relative(NODES_DIR, child).split('\\').join('/'),
          title: field(meta, 'heading') ?? field(meta, 'title') ?? '',
          teaser: field(meta, 'teaser') ?? '',
        })
      }
    } catch {
      // not a node folder — keep walking, it may hold some
    }
    collect(child, out)
  }
  return out
}

const root = readFileSync(join(NODES_DIR, 'meta.ts'), 'utf8')
const nodes = [
  { path: '/', title: field(root, 'heading') ?? field(root, 'title') ?? '', teaser: field(root, 'teaser') ?? '' },
  ...collect(NODES_DIR, []),
].sort((a, b) => a.path.localeCompare(b.path))

writeFileSync(OUT, JSON.stringify(nodes, null, 2) + '\n')
console.log(`owl context: ${nodes.length} public nodes -> ${OUT}`)
