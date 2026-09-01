import { Suspense, useEffect, useRef, useState } from 'react'
import { useDelayedFlag } from '../../hooks/useDelayedFlag'
import { useDocumentHead } from '../../hooks/useDocumentHead'
import { useChildPrefetch } from '../../hooks/useNodePrefetch'
import { ancestorsOf, nodeComponent } from '../../resume/registry'
import type { ResolvedNode } from '../../resume/types'
import type { User } from '../../types'
import { BranchCards } from './BranchCards'
import { NodeBoundary } from './NodeBoundary'
import { NodeSkeleton, NodeSkeletonHold } from './NodeSkeleton'
import { Spine, type SpineItem } from './Spine'

function DelayedSkeleton() {
  const show = useDelayedFlag(true)
  return show ? <NodeSkeleton /> : <NodeSkeletonHold />
}

function NodeBody({
  resolved,
  onReady,
}: {
  resolved: ResolvedNode
  onReady: () => void
}) {
  const Body = nodeComponent(resolved.node)
  return (
    <>
      <Body slug={resolved.slug} />
      <ReadySignal onReady={onReady} />
    </>
  )
}

function ReadySignal({ onReady }: { onReady: () => void }) {
  const ref = useRef(onReady)
  ref.current = onReady
  useEffect(() => {
    ref.current()
  }, [])
  return null
}

function spineItems(resolved: ResolvedNode): SpineItem[] {
  const chain = ancestorsOf(resolved.node).map((node) => ({
    path: node.path,
    title: node.path === '' ? 'Start' : node.meta.title,
  }))
  if (resolved.slug) chain.push({ path: resolved.path, title: resolved.title })
  return chain
}

/**
 * The chrome around one node: spine, kicker, h1, meta line, the lazily loaded
 * body, and the ways out. A node author never writes any of this.
 */
export function NodeView({
  resolved,
  user,
  focusOnReady = false,
}: {
  resolved: ResolvedNode
  user: User | null
  /** False on the reader's first node, so a fresh page load does not steal focus. */
  focusOnReady?: boolean
}) {
  const [announced, setAnnounced] = useState('')
  const headingRef = useRef<HTMLHeadingElement>(null)

  useDocumentHead({
    path: resolved.path,
    title: resolved.documentTitle,
    description: resolved.description,
  })
  useChildPrefetch(resolved.slug ? [] : resolved.node.childPaths)

  // The child's effect fires before ours, so the announcement must not be reset
  // from an effect here — it would wipe what the body just announced.
  const onReady = () => {
    setAnnounced(resolved.title)
    if (focusOnReady) headingRef.current?.focus()
  }

  return (
    <div className="node-layout">
      <Spine items={spineItems(resolved)} />
      <article className="node-article" id="node-article">
        <p className="kicker">{resolved.kicker}</p>
        <h1 ref={headingRef} tabIndex={-1}>
          {resolved.title}
        </h1>
        {resolved.metaLine ? <p className="node-meta">{resolved.metaLine}</p> : null}

        <div className="node-body">
          <NodeBoundary key={resolved.path}>
            <Suspense fallback={<DelayedSkeleton />}>
              <NodeBody resolved={resolved} onReady={onReady} />
            </Suspense>
          </NodeBoundary>
        </div>

        {resolved.node.meta.outbound && !resolved.slug ? (
          <p className="node-outbound">
            <a
              className="btn"
              href={resolved.node.meta.outbound.href}
              {...(/^https?:\/\//.test(resolved.node.meta.outbound.href) ? { rel: 'noreferrer' } : {})}
            >
              {resolved.node.meta.outbound.label}
            </a>
          </p>
        ) : null}

        <BranchCards node={resolved.node} slug={resolved.slug} user={user} />
      </article>
      <p className="sr-only" aria-live="polite">
        {announced}
      </p>
    </div>
  )
}
