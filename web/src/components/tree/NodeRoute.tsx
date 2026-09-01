import { useEffect, useRef } from 'react'
import { Navigate, useNavigationType, useParams } from 'react-router-dom'
import { useAuth } from '../../auth'
import { normalizePath, resolve } from '../../resume/registry'
import { redirectFor } from '../../resume/redirects'
import { NodeSkeletonHold } from './NodeSkeleton'
import { NodeView } from './NodeView'

/** Back/Forward should land where the reader left, not at the top. */
const scrollPositions = new Map<string, number>()

function scrollTo(top: number) {
  if (typeof window.scrollTo !== 'function') return
  window.scrollTo({ top, behavior: 'auto' })
}

/** Serves every path the tree owns — which is everything not claimed by a real route. */
export function NodeRoute() {
  const params = useParams()
  const path = normalizePath(params['*'])
  const { user, loading } = useAuth()
  const navigationType = useNavigationType()
  const redirect = redirectFor(path)
  // The first node of a visit must not pull focus; every later one must (§11).
  const seen = useRef<string | null>(null)
  const isFirstNode = seen.current === null

  useEffect(() => {
    if (redirect) return
    seen.current = path
    scrollTo(navigationType === 'POP' ? (scrollPositions.get(path) ?? 0) : 0)
    return () => {
      scrollPositions.set(path, window.scrollY)
    }
    // navigationType is read once per navigation; re-running on it alone would re-scroll.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path, redirect])

  if (redirect) return <Navigate to={redirect} replace />

  const resolved = resolve(path, user)
  if (!resolved) {
    // Sign-in may still unlock this node — do not bounce until auth has settled.
    if (loading) return <NodeSkeletonHold />
    return <Navigate to="/" replace />
  }

  return <NodeView key={resolved.path} resolved={resolved} user={user} focusOnReady={!isFirstNode} />
}

/** `/architecture/:slug` predates the tree and is linked from elsewhere. */
export function LegacyArchitectureRedirect() {
  const { slug } = useParams()
  return <Navigate to={`/about-project/architecture/${slug ?? ''}`} replace />
}
