import type { ComponentType } from 'react'

export type NodeVisibility = 'always' | 'signed-in' | 'admin'

/** Props every node body receives. `slug` is set only for dynamic children (§9.4). */
export type NodeProps = {
  slug?: string
}

/** A dynamic child stands in for a URL segment that has no folder of its own. */
export type DynamicChild = {
  /** Returns the synthetic node's head data, or null when the slug is unknown. */
  resolve: (slug: string) => { title: string; teaser: string; meta?: string } | null
}

export type NodeMeta = {
  /** Branch-card and spine label. Keep under ~28 chars. */
  title: string
  /** The article's h1, when it should differ from the short navigation label. */
  heading?: string
  /** One line under the title on the branch card. Loaded eagerly — keep it short. */
  teaser: string
  /** Small right-aligned label: "2023 — 2026 · 3 years", "filenkov.store". */
  meta?: string
  /** Sibling sort key. Sparse (10, 20, 30) so inserting never renumbers. */
  order: number
  /** Small uppercase line above the h1. Defaults to "Resume". */
  kicker?: string
  /** Overrides <title> and the description meta; falls back to title/teaser. */
  seo?: { title?: string; description?: string }
  visibleWhen?: NodeVisibility
  /** A link out of the tree, rendered under the article. */
  outbound?: { label: string; href: string }
  /** Serves `<path>/:slug` from this node's body (§9.4). */
  dynamicChild?: DynamicChild
}

export type ResumeNode = {
  /** '' for root, else 'experience/sber'. Never leading or trailing '/'. */
  path: string
  segments: string[]
  depth: number
  parentPath: string | null
  /** Already sorted by order, then title. */
  childPaths: string[]
  meta: NodeMeta
  load: () => Promise<{ default: ComponentType<NodeProps> }>
}

/** The current node as the view needs it — a real node, or a dynamic child of one. */
export type ResolvedNode = {
  node: ResumeNode
  /** Set when the URL addressed a dynamic child; the body renders with it. */
  slug?: string
  /** Head data: the node's own meta, or the resolved synthetic one. */
  title: string
  teaser: string
  metaLine?: string
  kicker: string
  path: string
  /** <title> for this URL. A dynamic child gets its own, not its parent's. */
  documentTitle: string
  /** meta description for this URL. */
  description: string
}
