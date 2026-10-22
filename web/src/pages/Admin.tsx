import { useEffect, useMemo, useState, type ChangeEvent } from 'react'
import { Navigate } from 'react-router-dom'
import { ApiError, fetchServices, fetchUsers, putGlobalRole, putServiceRole, putServiceState } from '../api'
import { useAuth } from '../auth'
import {
  isAdmin,
  isBlockedRole,
  SERVICE_ROLES,
  SITE_ROLES,
  serviceRole,
  withGlobalRole,
  withServiceRole,
  type HostBudget,
  type ManagedService,
  type Role,
  type ServiceRole,
  type ServicesSnapshot,
  type ServiceState,
  type User,
} from '../types'

const FIXED_SERVICES = ['storage', 'resume', 'image'] as const

/** Snapshot cadence; the fast one runs while a transition is in flight. */
const SERVICE_POLL_MS = 5000
const SERVICE_POLL_BUSY_MS = 2000

const SERVICE_BUTTONS: readonly { state: ServiceState; label: string }[] = [
  { state: 'off', label: 'Off' },
  { state: 'cpu', label: 'CPU' },
  { state: 'gpu', label: 'GPU' },
]

/** MiB to GiB, one decimal — the unit the memory line promises. */
function gib(mib: number | null | undefined): string {
  if (typeof mib !== 'number' || !Number.isFinite(mib)) return '—'
  return (mib / 1024).toFixed(1)
}

function errorText(err: unknown, fallback: string): string {
  if (err instanceof ApiError) return err.code ? `${err.code}: ${err.message}` : err.message
  if (err instanceof Error) return err.message
  return fallback
}

function stateText(service: ManagedService, pending: boolean): string {
  if (service.transition?.phase) return service.transition.phase
  if (pending || service.busy) return service.state === 'stopping' ? 'stopping…' : 'starting…'
  if (service.state === 'ready') return service.mode ? `ready · ${service.mode}` : 'ready'
  if (service.state === 'starting') return 'starting…'
  if (service.state === 'stopping') return 'stopping…'
  if (service.state === 'failed') return 'failed'
  return 'off'
}

function serviceMemory(service: ManagedService): {
  ram: number
  ramMeasured: boolean
  vram: number
  vramMeasured: boolean
} {
  const measuredRam = service.measured?.ram_mib ?? null
  const measuredVram = service.measured?.vram_mib ?? null
  return {
    ram: measuredRam ?? service.claim?.ram_mib ?? 0,
    ramMeasured: measuredRam !== null,
    vram: measuredVram ?? service.claim?.vram_mib ?? 0,
    vramMeasured: measuredVram !== null,
  }
}

function memoryLine(host: HostBudget): { ram: string; vram: string } {
  const ram = `RAM ${gib(host.ram?.free_mib)} / ${gib(host.ram?.budget_mib)} GiB free · ${gib(host.ram?.claimed_mib)} GiB claimed`
  const vram = host.vram
    ? `VRAM ${gib(host.vram.free_mib)} / ${gib(host.vram.budget_mib)} GiB free · ${gib(host.vram.claimed_mib)} GiB claimed`
    : 'VRAM: no sample'
  return { ram, vram }
}

function ServicesSection() {
  const [snapshot, setSnapshot] = useState<ServicesSnapshot | null>(null)
  const [status, setStatus] = useState<'loading' | 'ready' | 'error'>('loading')
  const [loadError, setLoadError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [pendingId, setPendingId] = useState<string | null>(null)

  useEffect(() => {
    let alive = true
    let timer: ReturnType<typeof setTimeout> | undefined
    let fast = false

    const load = async () => {
      try {
        const next = await fetchServices()
        if (!alive) return
        fast = next.services.some((service) => service.busy || service.transition != null)
        setSnapshot(next)
        setStatus('ready')
        setLoadError(null)
      } catch (err) {
        if (!alive) return
        setLoadError(errorText(err, 'Could not load services'))
        setStatus((prev) => (prev === 'ready' ? prev : 'error'))
      }
    }

    const schedule = () => {
      timer = setTimeout(tick, fast ? SERVICE_POLL_BUSY_MS : SERVICE_POLL_MS)
    }

    // A hidden tab pauses the cadence, so the poll never runs while nobody watches.
    const tick = async () => {
      if (!document.hidden) await load()
      if (alive) schedule()
    }

    // The first snapshot loads even from a background tab; only repeats wait for focus.
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
  }, [])

  async function switchTo(service: ManagedService, state: ServiceState): Promise<void> {
    if (pendingId) return
    setPendingId(service.id)
    setActionError(null)
    try {
      await putServiceState(service.id, state)
    } catch (err) {
      setActionError(errorText(err, 'Could not switch the service'))
    }
    try {
      setSnapshot(await fetchServices())
      setStatus('ready')
    } catch {
      // Leave the last snapshot on screen; the poll loop keeps retrying.
    } finally {
      setPendingId(null)
    }
  }

  const memory = snapshot ? memoryLine(snapshot.host) : null

  return (
    <section className="admin-services" aria-labelledby="services-heading">
      <div className="admin-head">
        <div>
          <h2 id="services-heading">Services</h2>
        </div>
      </div>
      <p className="muted admin-lead">
        Model services on this host. Pick where each one runs — the switch applies in place.
      </p>
      {memory ? (
        <p className="muted services-memory">
          <span>{memory.ram}</span>
          {' · '}
          <span>{memory.vram}</span>
        </p>
      ) : null}
      {actionError ? <div className="banner error">{actionError}</div> : null}
      {loadError ? <div className="banner error">{loadError}</div> : null}
      {status === 'loading' ? <p className="empty">Loading services…</p> : null}
      {status === 'error' && !snapshot ? (
        <p className="empty">Service status is unavailable right now.</p>
      ) : null}
      {snapshot ? (
        <div className="table-wrap">
          {snapshot.services.length === 0 ? (
            <p className="empty">No model services are registered.</p>
          ) : (
            <table aria-label="Services">
              <thead>
                <tr>
                  <th>Service</th>
                  <th>State</th>
                  <th>Memory</th>
                  <th>Runtime</th>
                </tr>
              </thead>
              <tbody>
                {snapshot.services.map((service) => {
                  const uses = serviceMemory(service)
                  const pending = pendingId === service.id
                  return (
                    <tr key={service.id}>
                      <td>{service.title}</td>
                      <td className={service.state === 'failed' ? 'status-blocked' : undefined}>
                        {stateText(service, pending)}
                      </td>
                      <td>
                        <div>
                          RAM {gib(uses.ram)} GiB{' '}
                          <span className="muted">{uses.ramMeasured ? 'measured' : 'claim'}</span>
                        </div>
                        <div>
                          VRAM {gib(uses.vram)} GiB{' '}
                          <span className="muted">{uses.vramMeasured ? 'measured' : 'claim'}</span>
                        </div>
                      </td>
                      <td>
                        <div className="segmented" role="group" aria-label={`${service.title} runtime`}>
                          {SERVICE_BUTTONS.map(({ state, label }) => {
                            const unavailable = service.unavailable_states?.[state]
                            const pressed = service.desired_state === state
                            const disabled = service.busy || pending || unavailable != null
                            return (
                              <button
                                key={state}
                                type="button"
                                className={pressed ? 'btn btn-small' : 'btn btn-small btn-ghost'}
                                aria-pressed={pressed}
                                disabled={disabled}
                                title={unavailable?.reason ?? undefined}
                                onClick={() => void switchTo(service, state)}
                              >
                                {label}
                              </button>
                            )
                          })}
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
      ) : null}
    </section>
  )
}

function roleFor(user: User, field: 'global' | string): Role {
  if (field === 'global') return user.global_role
  return serviceRole(user, field)
}

function extraServiceKeys(users: User[]): string[] {
  const seen = new Set<string>()
  for (const person of users) {
    for (const row of person.services ?? []) {
      if (!(FIXED_SERVICES as readonly string[]).includes(row.service)) {
        seen.add(row.service)
      }
    }
  }
  return [...seen].sort((a, b) => a.localeCompare(b))
}

function titleService(service: string): string {
  if (!service) return service
  return service.charAt(0).toUpperCase() + service.slice(1)
}

function RoleSelect({
  user,
  field,
  label,
  roles,
  disabled,
  onChange,
}: {
  user: User
  field: 'global' | string
  label: string
  roles: readonly Role[]
  disabled: boolean
  onChange: (userId: string, field: 'global' | string, value: Role) => void
}) {
  const value = roleFor(user, field)
  return (
    <select
      aria-label={label}
      className={isBlockedRole(value) ? 'status-blocked' : undefined}
      value={value}
      disabled={disabled}
      onChange={(e: ChangeEvent<HTMLSelectElement>) =>
        onChange(user.id, field, e.target.value as Role)
      }
    >
      {roles.map((r) => (
        <option key={r} value={r}>
          {r}
        </option>
      ))}
    </select>
  )
}

export function Admin() {
  const { user, loading } = useAuth()
  const [people, setPeople] = useState<User[]>([])
  const [emailQuery, setEmailQuery] = useState('')
  const [listState, setListState] = useState<'loading' | 'ready' | 'error'>('loading')
  const [loadError, setLoadError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    if (loading || !isAdmin(user)) return
    let cancelled = false
    setListState('loading')
    setLoadError(null)
    fetchUsers()
      .then((list) => {
        if (cancelled) return
        setPeople(list)
        setListState('ready')
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setPeople([])
        setLoadError(err instanceof Error ? err.message : 'Could not load directory')
        setListState('error')
      })
    return () => {
      cancelled = true
    }
  }, [loading, user])

  const serviceColumns = useMemo(() => [...FIXED_SERVICES, ...extraServiceKeys(people)], [people])
  const filtered = useMemo(() => {
    const q = emailQuery.trim().toLowerCase()
    if (!q) return people
    return people.filter((person) => person.email.toLowerCase().includes(q))
  }, [people, emailQuery])

  if (loading) {
    return <p className="muted">Checking sign-in…</p>
  }
  if (!isAdmin(user)) {
    return <Navigate to="/" replace />
  }

  async function onRoleChange(userId: string, field: 'global' | string, value: Role): Promise<void> {
    setBusy(true)
    setActionError(null)
    try {
      if (field === 'global') {
        await putGlobalRole(userId, value)
        setPeople((prev) => prev.map((person) => (person.id === userId ? withGlobalRole(person, value) : person)))
      } else {
        await putServiceRole(userId, field, value as ServiceRole)
        setPeople((prev) =>
          prev.map((person) => (person.id === userId ? withServiceRole(person, field, value as ServiceRole) : person)),
        )
      }
    } catch (err) {
      setActionError(err instanceof Error ? err.message : 'Could not update role')
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <div className="admin-head">
        <div>
          <p className="kicker">Admin</p>
          <h1>Directory</h1>
        </div>
      </div>
      <p className="muted admin-lead">Everyone on the hub. Change a role and it applies immediately.</p>
      {actionError ? <div className="banner error">{actionError}</div> : null}
      {listState === 'loading' ? <p className="empty">Loading directory…</p> : null}
      {listState === 'error' ? (
        <div className="banner error">{loadError ?? 'Could not load directory'}</div>
      ) : null}
      {listState === 'ready' ? (
        <>
          <div className="admin-search">
            <label htmlFor="user-email">Email</label>
            <input
              id="user-email"
              name="user-email"
              type="search"
              value={emailQuery}
              autoComplete="off"
              spellCheck={false}
              placeholder="Filter by email"
              onChange={(e: ChangeEvent<HTMLInputElement>) => setEmailQuery(e.target.value)}
            />
          </div>
          <div className="table-wrap">
            {people.length === 0 ? (
              <p className="empty">No one is in the directory yet.</p>
            ) : filtered.length === 0 ? (
              <p className="empty">No one matches that email.</p>
            ) : (
              <table aria-label="Directory">
                <thead>
                  <tr>
                    <th>Username</th>
                    <th>Email</th>
                    <th>Global role</th>
                    {serviceColumns.map((service) => (
                      <th key={service}>{titleService(service)}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((person) => (
                    <tr key={person.id}>
                      <td>{person.username}</td>
                      <td>{person.email}</td>
                      <td>
                        <RoleSelect
                          user={person}
                          field="global"
                          label={`Site role for ${person.username}`}
                          roles={SITE_ROLES}
                          disabled={busy}
                          onChange={onRoleChange}
                        />
                      </td>
                      {serviceColumns.map((service) => (
                        <td key={service}>
                          <RoleSelect
                            user={person}
                            field={service}
                            label={`${titleService(service)} role for ${person.username}`}
                            roles={SERVICE_ROLES}
                            disabled={busy}
                            onChange={onRoleChange}
                          />
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </>
      ) : null}
      <ServicesSection />
    </>
  )
}
