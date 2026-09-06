import { useEffect, useState } from 'react'
import { prefersReducedMotion } from '../../hooks/useReducedMotion'
import { GREETING, LETTER_MS } from './greeting'

/**
 * The speech bubble beside the hero owl. It types itself out letter by letter,
 * and under reduced motion it is simply there.
 */
export function OwlBubble({ onDismiss }: { onDismiss?: () => void }) {
  const reduced = prefersReducedMotion()
  const [shown, setShown] = useState(reduced ? GREETING.length : 0)

  useEffect(() => {
    if (reduced) return
    const id = window.setInterval(() => {
      setShown((n) => {
        if (n >= GREETING.length) {
          window.clearInterval(id)
          return n
        }
        return n + 1
      })
    }, LETTER_MS)
    return () => window.clearInterval(id)
  }, [reduced])

  return (
    <div className="owl-bubble" role="status">
      {/* The full sentence reaches assistive tech at once; the eye gets it letter
          by letter. Announcing one character at a time would be unreadable. */}
      <span className="sr-only">{GREETING}</span>
      <span aria-hidden="true">{GREETING.slice(0, shown)}</span>
      {onDismiss ? (
        <button className="owl-bubble-close" type="button" aria-label="Dismiss" onClick={onDismiss}>
          ×
        </button>
      ) : null}
    </div>
  )
}
