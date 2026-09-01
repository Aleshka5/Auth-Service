import { useEffect, useState } from 'react'

/**
 * True only after `active` has held for `delayMs`. A prefetched chunk resolves
 * in ~0 ms, so the skeleton it guards is never painted in the common case.
 */
export function useDelayedFlag(active: boolean, delayMs = 120): boolean {
  const [on, setOn] = useState(false)

  useEffect(() => {
    if (!active) {
      setOn(false)
      return
    }
    const timer = window.setTimeout(() => setOn(true), delayMs)
    return () => window.clearTimeout(timer)
  }, [active, delayMs])

  return on
}
