import { useEffect, useMemo, useState, type ChangeEvent } from 'react'
import { Navigate } from 'react-router-dom'
import { fetchUsers, putGlobalRole, putServiceRole } from '../api'
import { useAuth } from '../auth'
import {
  isAdmin,
  isBlockedRole,
  SERVICE_ROLES,
  SITE_ROLES,
  serviceRole,
  withGlobalRole,
  withServiceRole,
  type Role,
  type ServiceRole,
  type User,
} from '../types'

const FIXED_SERVICES = ['storage', 'resume'] as const

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

  const extras = useMemo(() => extraServiceKeys(people), [people])
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
                    <th>Storage</th>
                    <th>Resume</th>
                    {extras.map((service) => (
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
                      <td>
                        <RoleSelect
                          user={person}
                          field="storage"
                          label={`Storage role for ${person.username}`}
                          roles={SERVICE_ROLES}
                          disabled={busy}
                          onChange={onRoleChange}
                        />
                      </td>
                      <td>
                        <RoleSelect
                          user={person}
                          field="resume"
                          label={`Resume role for ${person.username}`}
                          roles={SERVICE_ROLES}
                          disabled={busy}
                          onChange={onRoleChange}
                        />
                      </td>
                      {extras.map((service) => (
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
    </>
  )
}
