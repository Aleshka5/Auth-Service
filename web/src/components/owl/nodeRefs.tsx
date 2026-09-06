import type { ReactNode } from 'react'
import { Link } from 'react-router-dom'
import { normalizePath, resolve } from '../../resume/registry'
import type { User } from '../../types'

/**
 * A path-shaped run of kebab-case segments. Deliberately loose: whether it is a
 * real page is decided by the registry, not by this pattern. It stops at the
 * first character a segment cannot contain, so "the /experience/sber branch."
 * links the path and leaves the full stop alone.
 */
const CANDIDATE = /\/[a-z0-9]+(?:-[a-z0-9]+)*(?:\/[a-z0-9]+(?:-[a-z0-9]+)*)*/g

/**
 * Turns the page paths the owl writes into links into the tree.
 *
 * Every candidate is resolved through the same registry the router uses, so a
 * path the model invented stays plain text, and a node the reader may not see
 * (`visibleWhen`) is never linked to them — `resolve` already applies that.
 * Dynamic children such as `/about-project/architecture/auth-gateway` resolve
 * too, with no extra bookkeeping here.
 */
export function withNodeLinks(text: string, user: User | null): ReactNode[] {
  const out: ReactNode[] = []
  let cursor = 0

  for (const match of text.matchAll(CANDIDATE)) {
    const raw = match[0]
    const start = match.index
    const path = normalizePath(raw)
    // The root is a bare "/" and never appears here; an empty path would link
    // every stray slash in a sentence.
    const resolved = path === '' ? null : resolve(path, user)
    if (!resolved) continue

    if (start > cursor) out.push(text.slice(cursor, start))
    out.push(
      <Link key={`${start}-${path}`} className="owl-chat-ref" to={`/${path}`} title={resolved.title}>
        {raw}
      </Link>,
    )
    cursor = start + raw.length
  }

  if (cursor < text.length) out.push(text.slice(cursor))
  return out
}
