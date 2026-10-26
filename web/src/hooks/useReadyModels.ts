import { useEffect, useState } from 'react'
import { fetchServices } from '../api'
import type { ManagedService, ServicesSnapshot } from '../types'

/** Same cadence as the admin services table, so the header tracks a switch. */
const POLL_MS = 5000

export type ModelLookup = 'idle' | 'pending' | 'known'

/**
 * A Switch-Service model is usable when the snapshot says it is ready and the
 * latest health probe did not fail. `starting`, `off`, and `failed` stay hidden.
 * A missing health object still counts: older snapshots omit it.
 */
export function modelReady(snapshot: ServicesSnapshot | null, id: string): boolean {
  const service = snapshot?.services.find((row) => row.id === id)
  return isReady(service)
}

function isReady(service: ManagedService | undefined): boolean {
  if (!service || service.state !== 'ready') return false
  return service.health?.ok !== false
}

/**
 * Reads GET /api/services while `enabled`. Callers enable it only for a reader
 * who might see Chat or Image. A failed read hides both: a dead switch is not
 * a live model. The lookup pauses while the tab is hidden.
 */
export function useReadyModels(enabled: boolean): { image: boolean; llm: boolean; status: ModelLookup } {
  const [image, setImage] = useState(false)
  const [llm, setLlm] = useState(false)
  const [status, setStatus] = useState<ModelLookup>(enabled ? 'pending' : 'idle')

  useEffect(() => {
    if (!enabled) {
      setImage(false)
      setLlm(false)
      setStatus('idle')
      return
    }

    let alive = true
    let timer: ReturnType<typeof setTimeout> | undefined
    setStatus('pending')

    const load = async () => {
      try {
        const snapshot = await fetchServices()
        if (!alive) return
        setImage(modelReady(snapshot, 'image'))
        setLlm(modelReady(snapshot, 'llm'))
        setStatus('known')
      } catch {
        if (!alive) return
        setImage(false)
        setLlm(false)
        setStatus('known')
      }
    }

    const schedule = () => {
      timer = setTimeout(tick, POLL_MS)
    }

    const tick = async () => {
      if (!document.hidden) await load()
      if (alive) schedule()
    }

    void load().then(() => {
      if (alive) schedule()
    })

    const onVisible = () => {
      if (document.hidden) return
      if (timer) clearTimeout(timer)
      void tick()
    }
    document.addEventListener('visibilitychange', onVisible)

    return () => {
      alive = false
      document.removeEventListener('visibilitychange', onVisible)
      if (timer) clearTimeout(timer)
    }
  }, [enabled])

  return { image, llm, status }
}
