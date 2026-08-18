import { useCallback, useEffect, useState, type ChangeEvent } from 'react'
import { Navigate } from 'react-router-dom'
import { fetchAdminUsers, patchUser, setUserBlocked } from '../api'
import { useAuth } from '../auth'
import { isAdmin, SERVICE_ROLES, SITE_ROLES, type ServiceRole, type SiteRole, type User } from '../types'

function formatSeen(iso: string | null): string {
  if (!iso) return 'Never'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(d)
}

function statusCell(user: User): string {
  return user.blocked ? 'Blocked' : formatSeen(user.last_seen_at)
}

export function Admin() {
  const { user, loading } = useAuth()
  const [users, setUsers] = useState<User[]>([])
  const [busyId, setBusyId] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [ready, setReady] = useState(false)

  const load = useCallback(async () => {
    setError(null)
    try {
      setUsers(await fetchAdminUsers())
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not load users')
    } finally {
      setReady(true)
    }
  }, [])

  useEffect(() => {
    if (!loading && isAdmin(user)) void load()
  }, [loading, user, load])

  if (loading) {
    return <p className="muted">Checking session…</p>
  }
  if (!isAdmin(user)) {
    return <Navigate to="/" replace />
  }

  async function onRoleChange(id: string | number, field: 'roles', value: SiteRole): Promise<void>
  async function onRoleChange(
    id: string | number,
    field: 'storage_roles' | 'resume_roles',
    value: ServiceRole,
  ): Promise<void>
  async function onRoleChange(
    id: string | number,
    field: 'roles' | 'storage_roles' | 'resume_roles',
    value: SiteRole | ServiceRole,
  ) {
    const key = String(id)
    setBusyId(key)
    setError(null)
    try {
      await patchUser(id, { [field]: value })
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update role')
    } finally {
      setBusyId(null)
    }
  }

  async function onBlock(target: User) {
    const key = String(target.id)
    setBusyId(key)
    setError(null)
    try {
      await setUserBlocked(target.id, !target.blocked)
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not update block status')
    } finally {
      setBusyId(null)
    }
  }

  return (
    <>
      <div className="admin-head">
        <div>
          <p className="kicker">Admin</p>
          <h1>Registered people</h1>
        </div>
        <p className="muted">{users.length} account{users.length === 1 ? '' : 's'}</p>
      </div>
      {error ? <div className="banner error">{error}</div> : null}
      <div className="table-wrap">
        {!ready ? (
          <p className="empty">Loading users…</p>
        ) : users.length === 0 ? (
          <p className="empty">No accounts yet.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Email</th>
                <th>Block</th>
                <th>Status</th>
                <th>Roles</th>
                <th>Storage roles</th>
                <th>Resume roles</th>
              </tr>
            </thead>
            <tbody>
              {users.map((row) => {
                const busy = busyId === String(row.id)
                return (
                  <tr key={String(row.id)}>
                    <td>{row.name}</td>
                    <td>{row.google_email}</td>
                    <td>
                      <button
                        type="button"
                        className={row.blocked ? 'btn btn-ghost btn-small' : 'btn btn-danger btn-small'}
                        disabled={busy}
                        onClick={() => void onBlock(row)}
                      >
                        {row.blocked ? 'Unblock' : 'Block'}
                      </button>
                    </td>
                    <td className={row.blocked ? 'status-blocked' : undefined}>{statusCell(row)}</td>
                    <td>
                      <select
                        aria-label={`Site role for ${row.name}`}
                        value={row.roles}
                        disabled={busy}
                        onChange={(e: ChangeEvent<HTMLSelectElement>) =>
                          void onRoleChange(row.id, 'roles', e.target.value as SiteRole)
                        }
                      >
                        {SITE_ROLES.map((r) => (
                          <option key={r} value={r}>
                            {r}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td>
                      <select
                        aria-label={`Storage role for ${row.name}`}
                        value={row.storage_roles}
                        disabled={busy}
                        onChange={(e: ChangeEvent<HTMLSelectElement>) =>
                          void onRoleChange(row.id, 'storage_roles', e.target.value as ServiceRole)
                        }
                      >
                        {SERVICE_ROLES.map((r) => (
                          <option key={r} value={r}>
                            {r}
                          </option>
                        ))}
                      </select>
                    </td>
                    <td>
                      <select
                        aria-label={`Resume role for ${row.name}`}
                        value={row.resume_roles}
                        disabled={busy}
                        onChange={(e: ChangeEvent<HTMLSelectElement>) =>
                          void onRoleChange(row.id, 'resume_roles', e.target.value as ServiceRole)
                        }
                      >
                        {SERVICE_ROLES.map((r) => (
                          <option key={r} value={r}>
                            {r}
                          </option>
                        ))}
                      </select>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </div>
    </>
  )
}
