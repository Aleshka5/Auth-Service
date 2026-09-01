import { useEffect } from 'react'
import { byPath } from '../resume/registry'
import type { ResumeNode } from '../resume/types'

const warmed = new Set<string>()

/** Fires the dynamic import for a node's chunk and discards the result. */
export function prefetch(node: ResumeNode | undefined): void {
  if (!node || warmed.has(node.path)) return
  warmed.add(node.path)
  void node.load().catch(() => {
    // A failed prefetch is not an error the reader should ever see; the real
    // navigation will surface it through the node error boundary instead.
    warmed.delete(node.path)
  })
}

/** Test seam — the warm set is module state that would leak between cases. */
export function resetPrefetchCache(): void {
  warmed.clear()
}

type IdleWindow = Window & {
  requestIdleCallback?: (cb: () => void) => number
  cancelIdleCallback?: (handle: number) => void
}

function saveData(): boolean {
  const conn = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection
  if (!conn) return false
  return Boolean(conn.saveData) || /2g/.test(conn.effectiveType ?? '')
}

/**
 * Warms the direct children of the current node once the browser is idle.
 * Only direct children — never the whole subtree.
 */
export function useChildPrefetch(childPaths: readonly string[]): void {
  const key = childPaths.join('|')
  useEffect(() => {
    if (saveData()) return
    const paths = key === '' ? [] : key.split('|')
    const run = () => paths.forEach((path) => prefetch(byPath(path)))
    const idle = window as IdleWindow
    if (!idle.requestIdleCallback) {
      const timer = window.setTimeout(run, 200)
      return () => window.clearTimeout(timer)
    }
    const handle = idle.requestIdleCallback(run)
    return () => idle.cancelIdleCallback?.(handle)
  }, [key])
}
