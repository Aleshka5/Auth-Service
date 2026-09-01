import { lazy, type ComponentType, type LazyExoticComponent } from 'react'
import { isAdmin, type User } from '../types'
import type { NodeMeta, NodeProps, ResolvedNode, ResumeNode } from './types'

/**
 * A node is a folder under ./nodes, and its folder path is its URL path.
 * There is no central list to keep in sync — Vite discovers the folders.
 */
const metaModules = import.meta.glob<{ default: NodeMeta }>('./nodes/**/meta.ts', { eager: true })
const bodyModules = import.meta.glob<{ default: ComponentType<NodeProps> }>('./nodes/**/node.tsx')

/** Segments that would collide with a gateway route or an SPA page (§6.2). */
export const RESERVED = new Set([
  'auth',
  'admin',
  'api',
  'users',
  'oauth',
  'health',
  'hermes',
  'chat',
  'sessions',
  'assets',
  'architecture',
])

export const SEGMENT_RE = /^[a-z0-9]+(-[a-z0-9]+)*$/

const toPath = (key: string) => key.replace(/^\.\/nodes\//, '').replace(/\/?(meta\.ts|node\.tsx)$/, '')

/** Glob keys as paths, exported so registry.test.ts can pair them up. */
export const metaPaths: string[] = Object.keys(metaModules).map(toPath).sort()
export const bodyPaths: string[] = Object.keys(bodyModules).map(toPath).sort()

const bodyByPath = new Map<string, () => Promise<{ default: ComponentType<NodeProps> }>>()
for (const [key, load] of Object.entries(bodyModules)) bodyByPath.set(toPath(key), load)

const missingBody = (path: string) => async () => {
  throw new Error(`Resume node "${path || '/'}" has no node.tsx`)
}

function build(): Map<string, ResumeNode> {
  const nodes = new Map<string, ResumeNode>()

  for (const [key, mod] of Object.entries(metaModules)) {
    const path = toPath(key)
    const segments = path === '' ? [] : path.split('/')
    nodes.set(path, {
      path,
      segments,
      depth: segments.length,
      parentPath: segments.length === 0 ? null : segments.slice(0, -1).join('/'),
      childPaths: [],
      meta: mod.default,
      load: bodyByPath.get(path) ?? missingBody(path),
    })
  }

  for (const node of nodes.values()) {
    if (node.parentPath === null) continue
    nodes.get(node.parentPath)?.childPaths.push(node.path)
  }

  for (const node of nodes.values()) {
    node.childPaths.sort((a, b) => {
      const left = nodes.get(a)!.meta
      const right = nodes.get(b)!.meta
      return left.order - right.order || left.title.localeCompare(right.title)
    })
  }

  return nodes
}

const registry = build()

export const allNodes = (): ResumeNode[] => [...registry.values()]

export function byPath(path: string): ResumeNode | undefined {
  return registry.get(path)
}

/** Strips the leading and trailing slash and lowercases. '/Experience/Sber/' → 'experience/sber'. */
export function normalizePath(raw: string | undefined | null): string {
  return (raw ?? '').trim().replace(/^\/+|\/+$/g, '').toLowerCase()
}

export function isVisible(meta: NodeMeta, user: User | null): boolean {
  switch (meta.visibleWhen ?? 'always') {
    case 'signed-in':
      return Boolean(user)
    case 'admin':
      return isAdmin(user)
    default:
      return true
  }
}

export function childrenOf(node: ResumeNode, user: User | null): ResumeNode[] {
  return node.childPaths
    .map((path) => registry.get(path)!)
    .filter((child) => isVisible(child.meta, user))
}

/** Siblings, current node excluded, in sibling order. */
export function siblingsOf(node: ResumeNode, user: User | null): ResumeNode[] {
  if (node.parentPath === null) return []
  const parent = registry.get(node.parentPath)
  if (!parent) return []
  return childrenOf(parent, user).filter((sibling) => sibling.path !== node.path)
}

/** Root first, current node last. */
export function ancestorsOf(node: ResumeNode): ResumeNode[] {
  const chain: ResumeNode[] = []
  let cursor: ResumeNode | undefined = node
  while (cursor) {
    chain.unshift(cursor)
    cursor = cursor.parentPath === null ? undefined : registry.get(cursor.parentPath)
  }
  return chain
}

/**
 * Turns a URL path into the node to render. Handles dynamic children (§9.4):
 * `about-project/architecture/auth-gateway` resolves to the architecture node
 * with `slug: 'auth-gateway'`.
 */
export function resolve(path: string, user: User | null): ResolvedNode | null {
  const exact = registry.get(path)
  if (exact) {
    if (!isVisible(exact.meta, user)) return null
    return {
      node: exact,
      title: exact.meta.heading ?? exact.meta.title,
      teaser: exact.meta.teaser,
      metaLine: exact.meta.meta,
      kicker: exact.meta.kicker ?? 'Resume',
      path,
      documentTitle: exact.meta.seo?.title ?? `${exact.meta.heading ?? exact.meta.title} · Alexey Filenkov`,
      description: exact.meta.seo?.description ?? exact.meta.teaser,
    }
  }

  const cut = path.lastIndexOf('/')
  if (cut < 0) return null
  const parent = registry.get(path.slice(0, cut))
  const slug = path.slice(cut + 1)
  const dynamic = parent?.meta.dynamicChild
  if (!parent || !dynamic || !isVisible(parent.meta, user)) return null

  const head = dynamic.resolve(slug)
  if (!head) return null
  return {
    node: parent,
    slug,
    title: head.title,
    teaser: head.teaser,
    metaLine: head.meta,
    kicker: parent.meta.kicker ?? parent.meta.title,
    path,
    // The parent's seo block describes the parent, not this slug.
    documentTitle: `${head.title} · ${parent.meta.title}`,
    description: head.teaser,
  }
}

/** One lazy wrapper per node, so re-visiting does not re-create the chunk boundary. */
const components = new Map<string, LazyExoticComponent<ComponentType<NodeProps>>>()

export function nodeComponent(node: ResumeNode): LazyExoticComponent<ComponentType<NodeProps>> {
  let component = components.get(node.path)
  if (!component) {
    component = lazy(node.load)
    components.set(node.path, component)
  }
  return component
}
