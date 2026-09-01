import { Link } from 'react-router-dom'

export type SpineItem = {
  /** URL path without the leading slash; '' is the root. */
  path: string
  title: string
}

/**
 * Not a diagram — a hairline with one dot per node on the path from the root to
 * the current node. On narrow screens the same markup becomes a breadcrumb strip.
 */
export function Spine({ items }: { items: readonly SpineItem[] }) {
  if (items.length === 0) return null
  const lastIndex = items.length - 1

  return (
    <nav className="spine" aria-label="Resume path">
      <ol>
        {items.map((item, index) => {
          const current = index === lastIndex
          return (
            <li key={item.path} className="spine-item" data-current={current ? '' : undefined}>
              <span className="spine-dot" aria-hidden="true" />
              {current ? (
                <span className="spine-label" aria-current="page">
                  {item.title}
                </span>
              ) : (
                <Link className="spine-label" to={`/${item.path}`}>
                  {item.title}
                </Link>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
