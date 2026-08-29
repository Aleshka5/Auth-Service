import { Link, Navigate, useParams } from 'react-router-dom'
import { solutionBySlug } from '../resume/data'

export function Architecture() {
  const { slug } = useParams()
  const solution = solutionBySlug(slug)

  if (!solution) {
    return <Navigate to="/#architecture" replace />
  }

  const external = Boolean(solution.href && /^https?:\/\//.test(solution.href))

  return (
    <article className="arch-page">
      <p className="kicker">Architecture</p>
      <h1>{solution.title}</h1>
      <p className="lede">{solution.summary}</p>
      <p className="muted">
        A dedicated write-up for this box is not here yet. The map stays the source of truth until it is.
      </p>
      <div className="arch-page-actions">
        {solution.href ? (
          <a className="btn" href={solution.href} {...(external ? { rel: 'noreferrer' } : {})}>
            Open service
          </a>
        ) : null}
        <Link className="btn btn-ghost" to="/#architecture">
          Back to the map
        </Link>
      </div>
    </article>
  )
}
