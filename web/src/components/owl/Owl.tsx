import { lazy, Suspense, useCallback, useLayoutEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'
import angryToCalm from '../../assets/animations/angry-to-calm.svg?raw'
import calmToAngry from '../../assets/animations/calm-to-angry.svg?raw'
import calmToKind from '../../assets/animations/calm-to-kind.svg?raw'
import restingCalm from '../../assets/My_icon_calm.svg?raw'
import { useAuth } from '../../auth'
import { prefersReducedMotion } from '../../hooks/useReducedMotion'
import { isAdmin, type User } from '../../types'
import { OwlBubble } from './OwlBubble'

/** The panel and its streaming client stay out of the initial bundle (§10). */
const OwlChat = lazy(() => import('./OwlChat'))

/**
 * The owl (§8). One component, two states, driven by
 * `document.documentElement.dataset.owl` which `useScrollChrome` writes:
 *
 *   hero   — visually parked in the 128px slot above the article, not a control
 *   docked — a real "Back to top" button in the bottom-right corner
 *
 * The hero owl is also the way into the chat: clicking it opens the panel.
 * Docked, it goes back to being the "Back to top" button — a corner button that
 * changed meaning on scroll would be worse than two separate affordances.
 *
 * This is the two-element cross-fade from the end of §8.3, not FLIP. FLIP needs
 * the hero transform re-measured on every scroll frame, which fights the 240ms
 * transition that makes the dock/undock morph — the owl ends up trailing the
 * page. Two elements cost one extra SVG injection and behave correctly: the
 * hero owl sits in the flow and scrolls with the page, the docked button is
 * fixed, and they cross-fade on the same `data-owl` attribute.
 *
 * The mood-clip machinery is carried over unchanged from the old
 * `HeaderMascot`: same clip table, same stripXml + innerHTML injection, same
 * departNonce replay, same data-mascot* attributes on the injection host.
 */

type OwlClip = 'guest' | 'user' | 'admin'

const DEPART_HOLD_MS = 1000

function stripXml(raw: string): string {
  return raw.replace(/^\s*<\?xml[^?]*\?>\s*/i, '')
}

const clips: Record<OwlClip, { motion: string; durationMs: number; motionName: string }> = {
  guest: { motion: stripXml(calmToAngry), durationMs: 1250, motionName: 'calm-to-angry' },
  user: { motion: stripXml(angryToCalm), durationMs: 1350, motionName: 'angry-to-calm' },
  admin: { motion: stripXml(calmToKind), durationMs: 1450, motionName: 'calm-to-kind' },
}

/** §3.5: under reduced motion every clip collapses to the resting frame. */
const resting = stripXml(restingCalm)

function pickOwlClip(pathname: string, user: User | null): OwlClip {
  if (pathname === '/auth' || !user) return 'guest'
  if (isAdmin(user)) return 'admin'
  return 'user'
}

type OwlProps = {
  departing?: boolean
  departNonce?: number
  onDeparted?: () => void
}

export function Owl({ departing = false, departNonce = 0, onDeparted }: OwlProps) {
  const { pathname } = useLocation()
  const { user, loading } = useAuth()
  // Arriving on the hub is the greeting: the owl introduces itself, every time.
  // Read once at mount, so walking into the tree and back does not replay it —
  // a reload does. The chat itself never opens on its own; it is the reader's
  // click on the owl, and nothing else.
  const [chatOpen, setChatOpen] = useState(false)
  const [greeting, setGreeting] = useState(() => pathname === '/')
  const clip = departing ? 'guest' : pickOwlClip(pathname, user)
  const spec = clips[clip]
  const playId = `${clip}-${pathname}-${departNonce}`
  const hostRef = useRef<HTMLDivElement>(null)
  const heroRef = useRef<HTMLDivElement>(null)
  const injectedFor = useRef<string | null>(null)
  const departingRef = useRef(departing)
  const onDepartedRef = useRef(onDeparted)
  departingRef.current = departing
  onDepartedRef.current = onDeparted

  useLayoutEffect(() => {
    if (loading) return
    const host = hostRef.current
    if (!host) return

    if (injectedFor.current !== playId) {
      injectedFor.current = playId
      const markup = prefersReducedMotion() ? resting : spec.motion
      host.innerHTML = markup
      if (heroRef.current) heroRef.current.innerHTML = markup
    }

    // The depart timer runs on the same schedule with or without motion, so a
    // reduced-motion sign-out still completes.
    const departId = departingRef.current
      ? window.setTimeout(() => {
          if (departingRef.current) onDepartedRef.current?.()
        }, spec.durationMs + DEPART_HOLD_MS)
      : undefined

    return () => {
      if (departId) window.clearTimeout(departId)
    }
  }, [loading, playId, spec.motion, spec.durationMs])

  const openChat = useCallback(() => {
    // The greeting asked for the click; once it comes, it has served its turn.
    setGreeting(false)
    setChatOpen(true)
  }, [])

  const closeChat = useCallback(() => setChatOpen(false), [])

  const onClick = () => {
    // The corner owl is only ever "back to top"; the hero owl opens the chat.
    // CSS makes the docked button unclickable while it is the hero; this guard
    // makes it inert for keyboard and tests.
    if (document.documentElement.dataset.owl !== 'docked') return
    const reduced = prefersReducedMotion()
    window.scrollTo({ top: 0, behavior: reduced ? 'auto' : 'smooth' })
    // Keyboard users land where the eye lands (§8.4, §11).
    document.getElementById('node-article')?.querySelector('h1')?.focus()
  }

  return (
    <>
      <div className="owl-slot">
        <button
          className="owl-hero-button"
          type="button"
          aria-label="Chat with the AI owl"
          aria-expanded={chatOpen}
          onClick={openChat}
        >
          <div
            ref={heroRef}
            className="owl-hero"
            aria-hidden="true"
            data-mascot={loading ? undefined : clip}
            data-mascot-phase="motion"
            data-mascot-motion={spec.motionName}
          />
        </button>
        {greeting ? <OwlBubble onDismiss={() => setGreeting(false)} /> : null}
      </div>
      {chatOpen ? (
        <Suspense fallback={<div className="owl-chat owl-chat-loading">Waking the owl…</div>}>
          <OwlChat onClose={closeChat} user={user} />
        </Suspense>
      ) : null}
      <button className="owl" type="button" aria-label="Back to top" onClick={onClick}>
        <div ref={hostRef} className="owl-art" aria-hidden="true" />
        <svg className="owl-ring" viewBox="0 0 56 56" aria-hidden="true">
          <circle cx="28" cy="28" r="24" />
        </svg>
      </button>
    </>
  )
}
