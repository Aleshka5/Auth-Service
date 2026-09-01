/**
 * Stands in for the body only — the kicker, title and meta line are already real,
 * because every meta.ts is in the main bundle. The reserved height keeps CLS at 0.
 */
export function NodeSkeleton() {
  return (
    <div className="node-skeleton" data-testid="node-skeleton" aria-hidden="true">
      <span style={{ width: '92%' }} />
      <span style={{ width: '84%' }} />
      <span style={{ width: '88%' }} />
      <span style={{ width: '46%' }} />
    </div>
  )
}

/** Reserves the body's space without painting anything, for the first 120 ms. */
export function NodeSkeletonHold() {
  return <div className="node-skeleton node-skeleton-hold" aria-hidden="true" />
}
