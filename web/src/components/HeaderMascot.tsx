import { useLayoutEffect, useRef } from 'react'
import { useLocation } from 'react-router-dom'
import angryToCalm from '../assets/animations/angry-to-calm.svg?raw'
import calmToAngry from '../assets/animations/calm-to-angry.svg?raw'
import calmToKind from '../assets/animations/calm-to-kind.svg?raw'
import { useAuth } from '../auth'
import { isAdmin, type User } from '../types'

type HeaderClip = 'guest' | 'user' | 'admin'

const DEPART_HOLD_MS = 1000

function stripXml(raw: string): string {
  return raw.replace(/^\s*<\?xml[^?]*\?>\s*/i, '')
}

const clips: Record<HeaderClip, { motion: string; durationMs: number; motionName: string }> = {
  guest: { motion: stripXml(calmToAngry), durationMs: 1250, motionName: 'calm-to-angry' },
  user: { motion: stripXml(angryToCalm), durationMs: 1350, motionName: 'angry-to-calm' },
  admin: { motion: stripXml(calmToKind), durationMs: 1450, motionName: 'calm-to-kind' },
}

function pickHeaderClip(pathname: string, user: User | null): HeaderClip {
  if (pathname === '/auth' || !user) return 'guest'
  if (isAdmin(user)) return 'admin'
  return 'user'
}

type HeaderMascotProps = {
  departing?: boolean
  departNonce?: number
  onDeparted?: () => void
}

export function HeaderMascot({ departing = false, departNonce = 0, onDeparted }: HeaderMascotProps) {
  const { pathname } = useLocation()
  const { user, loading } = useAuth()
  const clip = departing ? 'guest' : pickHeaderClip(pathname, user)
  const spec = clips[clip]
  const playId = `${clip}-${pathname}-${departNonce}`
  const hostRef = useRef<HTMLDivElement>(null)
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
      host.innerHTML = spec.motion
    }

    const departId = departingRef.current
      ? window.setTimeout(() => {
          if (departingRef.current) onDepartedRef.current?.()
        }, spec.durationMs + DEPART_HOLD_MS)
      : undefined

    return () => {
      if (departId) window.clearTimeout(departId)
    }
  }, [loading, playId, spec.motion, spec.durationMs])

  return (
    <div
      className="header-mascot"
      aria-hidden="true"
      data-mascot={loading ? undefined : clip}
      data-mascot-phase="motion"
      data-mascot-motion={spec.motionName}
    >
      <div ref={hostRef} className="header-mascot-motion" />
    </div>
  )
}
