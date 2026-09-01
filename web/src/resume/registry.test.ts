import { describe, expect, it } from 'vitest'
import {
  RESERVED,
  SEGMENT_RE,
  allNodes,
  ancestorsOf,
  bodyPaths,
  byPath,
  childrenOf,
  isVisible,
  metaPaths,
  normalizePath,
  resolve,
  siblingsOf,
} from './registry'
import { admin, stranger } from '../test/fixtures'

const label = (path: string) => path || '<root>'

/**
 * Convention-over-configuration costs a compile error: a typo makes a missing
 * node rather than a broken build. These invariants buy that back.
 */
describe('resume registry invariants', () => {
  it('pairs every meta.ts with a node.tsx', () => {
    expect(metaPaths).toEqual(bodyPaths)
  })

  it('gives every non-root node an existing parent', () => {
    for (const node of allNodes()) {
      if (node.parentPath === null) continue
      expect(byPath(node.parentPath), `${label(node.path)} has no parent`).toBeDefined()
    }
  })

  it('uses lowercase kebab-case segments', () => {
    for (const node of allNodes()) {
      for (const segment of node.segments) {
        expect(segment, `${label(node.path)} has a bad segment`).toMatch(SEGMENT_RE)
      }
    }
  })

  it('keeps depth-1 segments out of the reserved list', () => {
    for (const node of allNodes()) {
      if (node.depth !== 1) continue
      expect(RESERVED.has(node.segments[0]), `/${node.path} collides with a gateway route`).toBe(false)
    }
  })

  it('gives siblings unique order values', () => {
    for (const node of allNodes()) {
      const orders = node.childPaths.map((path) => byPath(path)!.meta.order)
      expect(new Set(orders).size, `${label(node.path)} has duplicate child order values`).toBe(orders.length)
    }
  })

  it('never nests deeper than four levels', () => {
    for (const node of allNodes()) {
      expect(node.depth, `/${node.path} is too deep`).toBeLessThanOrEqual(4)
    }
  })

  it('gives every node a teaser of at most 120 characters', () => {
    for (const node of allNodes()) {
      expect(node.meta.teaser.trim().length, `${label(node.path)} has an empty teaser`).toBeGreaterThan(0)
      expect(node.meta.teaser.length, `${label(node.path)} has a long teaser`).toBeLessThanOrEqual(120)
    }
  })

  it('has a root node at the empty path', () => {
    const root = byPath('')
    expect(root).toBeDefined()
    expect(root!.depth).toBe(0)
    expect(root!.parentPath).toBeNull()
    expect(root!.childPaths.length).toBeGreaterThan(0)
  })
})

describe('registry lookups', () => {
  it('normalises slashes and case', () => {
    expect(normalizePath('/Experience/Sber/')).toBe('experience/sber')
    expect(normalizePath(undefined)).toBe('')
    expect(normalizePath('/')).toBe('')
  })

  it('sorts children by order, then title', () => {
    const root = byPath('')!
    const orders = root.childPaths.map((path) => byPath(path)!.meta.order)
    expect(orders).toEqual([...orders].sort((a, b) => a - b))
  })

  it('walks the ancestor chain root first', () => {
    const chain = ancestorsOf(byPath('experience/sber')!).map((node) => node.path)
    expect(chain).toEqual(['', 'experience', 'experience/sber'])
  })

  it('lists siblings without the node itself', () => {
    const node = byPath('experience/sber')!
    const siblings = siblingsOf(node, null).map((sibling) => sibling.path)
    expect(siblings).not.toContain('experience/sber')
    expect(siblings).toContain('experience/aeroclub')
  })

  it('hides a signed-in node from a guest and shows it to a user', () => {
    const hermes = byPath('achievements/projects/hermes')!
    expect(hermes.meta.visibleWhen).toBe('signed-in')
    expect(isVisible(hermes.meta, null)).toBe(false)
    expect(isVisible(hermes.meta, stranger)).toBe(true)
    expect(isVisible(hermes.meta, admin)).toBe(true)

    const projects = byPath('achievements/projects')!
    expect(childrenOf(projects, null).map((child) => child.path)).not.toContain(hermes.path)
    expect(childrenOf(projects, stranger).map((child) => child.path)).toContain(hermes.path)
  })
})

describe('resolve', () => {
  it('resolves a real node', () => {
    const resolved = resolve('experience/sber', null)
    expect(resolved?.title).toBe('Sber')
    expect(resolved?.slug).toBeUndefined()
    expect(resolved?.kicker).toBe('Resume')
  })

  it('resolves the root', () => {
    expect(resolve('', null)?.node.path).toBe('')
  })

  it('resolves a dynamic architecture child to its parent node plus a slug', () => {
    const resolved = resolve('about-project/architecture/auth-gateway', null)
    expect(resolved?.node.path).toBe('about-project/architecture')
    expect(resolved?.slug).toBe('auth-gateway')
    expect(resolved?.title).toBe('Auth Gateway')
    expect(resolved?.kicker).toBe('Architecture')
  })

  it('returns null for an unknown slug under a dynamic parent', () => {
    expect(resolve('about-project/architecture/not-a-box', null)).toBeNull()
  })

  it('returns null for an unknown path and for a hidden node', () => {
    expect(resolve('nope', null)).toBeNull()
    expect(resolve('experience/nope', null)).toBeNull()
    expect(resolve('achievements/projects/hermes', null)).toBeNull()
    expect(resolve('achievements/projects/hermes', stranger)).not.toBeNull()
  })
})

describe('heading override', () => {
  it('uses the root heading for the h1 and the short title for navigation', () => {
    const root = byPath('')!
    expect(root.meta.title).toBe('Start')
    expect(resolve('', null)?.title).toBe('Alexey Filenkov')
  })
})

describe('document head', () => {
  it('uses a node seo title when it has one', () => {
    expect(resolve('', null)?.documentTitle).toBe('Alexey Filenkov · Senior AI Agents')
  })

  it('gives a dynamic child its own title, not its parent seo block', () => {
    const parent = resolve('about-project/architecture', null)!
    const child = resolve('about-project/architecture/auth-gateway', null)!
    expect(child.documentTitle).toBe('Auth Gateway · Architecture')
    expect(child.documentTitle).not.toBe(parent.documentTitle)
    expect(child.description).toBe(child.teaser)
  })
})
