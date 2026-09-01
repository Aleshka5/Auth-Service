import { Link } from 'react-router-dom'
import { prefetch } from '../../hooks/useNodePrefetch'
import { byPath, childrenOf, siblingsOf } from '../../resume/registry'
import type { ResumeNode } from '../../resume/types'
import type { User } from '../../types'

function warm(path: string) {
  prefetch(byPath(path))
}

function Card({ node, index }: { node: ResumeNode; index: number }) {
  return (
    <li className="branch-card">
      <Link
        to={`/${node.path}`}
        onPointerEnter={() => warm(node.path)}
        onPointerDown={() => warm(node.path)}
        onFocus={() => warm(node.path)}
      >
        <span className="branch-num" aria-hidden="true">
          {String(index + 1).padStart(2, '0')}
        </span>
        <span className="branch-body">
          <span className="branch-title">{node.meta.title}</span>
          <span className="branch-teaser">{node.meta.teaser}</span>
        </span>
        {node.meta.meta ? <span className="branch-meta">{node.meta.meta}</span> : null}
        <span className="branch-arrow" aria-hidden="true">
          →
        </span>
      </Link>
    </li>
  )
}

type Props = {
  node: ResumeNode
  /** Set on a dynamic child; such a page is always a leaf whose only exit is up. */
  slug?: string
  user: User | null
}

/**
 * The three ways out of a node: down into its children, sideways to its
 * siblings, and back up to its parent. Each group renders only when non-empty.
 */
export function BranchCards({ node, slug, user }: Props) {
  const deeper = slug ? [] : childrenOf(node, user)
  const sideways = slug ? [] : siblingsOf(node, user)
  const up = slug ? node : node.parentPath === null ? null : byPath(node.parentPath)

  if (deeper.length === 0 && sideways.length === 0 && !up) return null

  return (
    <nav className="branches" aria-label="Where next">
      {deeper.length > 0 ? (
        <section className="branch-group">
          <h2 className="branch-heading">{node.parentPath === null ? 'Where next?' : 'Deeper'}</h2>
          <ul className="branch-cards">
            {deeper.map((child, index) => (
              <Card key={child.path} node={child} index={index} />
            ))}
          </ul>
        </section>
      ) : null}

      {sideways.length > 0 ? (
        <section className="branch-group">
          <h2 className="branch-heading">Or sideways</h2>
          <ul className="branch-siblings">
            {sideways.map((sibling) => (
              <li key={sibling.path}>
                <Link to={`/${sibling.path}`} onPointerEnter={() => warm(sibling.path)}>
                  {sibling.meta.title}
                </Link>
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      {up ? (
        <Link className="branch-up" to={`/${up.path}`} onPointerEnter={() => warm(up.path)}>
          <span aria-hidden="true">←</span> Up to {up.path === '' ? 'Start' : up.meta.title}
        </Link>
      ) : null}
    </nav>
  )
}
