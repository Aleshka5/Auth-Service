import type {
  Role,
  ServiceRole,
  ServiceRoleEntry,
  ServiceState,
  ServicesSnapshot,
  User,
  UserList,
  UserListItem,
} from './types'

export class ApiError extends Error {
  status: number
  /** Machine-readable code from `{"error":{"code":…,"message":…}}`, when the service sent one. */
  code?: string

  constructor(status: number, message: string, code?: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

type ErrorBody = {
  error?: string | { code?: string; message?: string }
  message?: string
}

async function readErrorDetail(res: Response): Promise<{ message: string; code?: string }> {
  const text = await res.text()
  if (!text) return { message: res.statusText || `Request failed (${res.status})` }
  try {
    const body = JSON.parse(text) as ErrorBody
    if (typeof body.error === 'string' && body.error) return { message: body.error }
    if (body.error && typeof body.error === 'object') {
      return {
        message: body.error.message || body.error.code || text,
        code: body.error.code,
      }
    }
    return { message: body.message || text }
  } catch {
    return { message: text }
  }
}

async function readError(res: Response): Promise<string> {
  return (await readErrorDetail(res)).message
}

async function postRefresh(): Promise<Response> {
  return fetch('/api/refresh', {
    method: 'POST',
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })
}

async function refreshOrThrow(): Promise<boolean> {
  const res = await postRefresh()
  if (res.ok) return true
  if (res.status === 403) {
    throw new ApiError(res.status, await readError(res))
  }
  return false
}

async function request<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  })
  if (res.status === 401 && !retried) {
    if (await refreshOrThrow()) {
      return request<T>(path, init, true)
    }
  }
  if (!res.ok) {
    const detail = await readErrorDetail(res)
    throw new ApiError(res.status, detail.message, detail.code)
  }
  if (res.status === 204) {
    return undefined as T
  }
  const text = await res.text()
  if (!text) {
    return undefined as T
  }
  return JSON.parse(text) as T
}

export function googleAuthPath(returnTo: string | null): string {
  if (!returnTo) return '/oauth/google'
  return `/oauth/google?return_to=${encodeURIComponent(returnTo)}`
}

async function getMe(): Promise<Response> {
  return fetch('/api/me', { credentials: 'include', headers: { Accept: 'application/json' } })
}

async function parseMe(res: Response): Promise<User | null> {
  if (res.status === 401) return null
  if (!res.ok) throw new ApiError(res.status, await readError(res))
  return (await res.json()) as User
}

export async function fetchMe(): Promise<User | null> {
  const first = await getMe()
  if (first.status !== 401) {
    return parseMe(first)
  }
  if (!(await refreshOrThrow())) return null
  return parseMe(await getMe())
}

export async function logout(): Promise<void> {
  await request('/api/logout', { method: 'POST' })
}

type DirectoryUser = {
  id: string
  google_sub: string
  email: string
  username: string
  global_role: Role
  services?: ServiceRoleEntry[]
}

type DirectoryRoles = {
  user_id: string
  global_role: Role
  services?: ServiceRoleEntry[]
}

function userFromDirectory(profile: DirectoryUser, roles?: DirectoryRoles | null): User {
  return {
    id: profile.id,
    google_sub: profile.google_sub,
    email: profile.email,
    username: profile.username,
    global_role: profile.global_role,
    services: roles?.services ?? profile.services ?? [],
  }
}

function userFromListItem(item: UserListItem): User {
  return userFromDirectory(item)
}

export async function fetchUsers(): Promise<User[]> {
  const body = await request<UserList>('/users')
  return (body.users ?? []).map(userFromListItem)
}

export async function fetchUser(id: string): Promise<User> {
  const pathId = encodeURIComponent(id)
  const [profile, roles] = await Promise.all([
    request<DirectoryUser>(`/users/${pathId}`),
    request<DirectoryRoles>(`/users/${pathId}/roles`),
  ])
  return userFromDirectory(profile, roles)
}

export async function putGlobalRole(id: string, role: Role): Promise<void> {
  await request(`/users/${encodeURIComponent(id)}/role`, {
    method: 'PUT',
    body: JSON.stringify({ role }),
  })
}

export async function putServiceRole(id: string, service: string, role: ServiceRole): Promise<void> {
  await request(`/users/${encodeURIComponent(id)}/roles/${encodeURIComponent(service)}`, {
    method: 'PUT',
    body: JSON.stringify({ role }),
  })
}

/** Host budget plus every model service on it. */
export async function fetchServices(): Promise<ServicesSnapshot> {
  return request<ServicesSnapshot>('/api/services')
}

/** Asks a service to move to off/cpu/gpu. The 202 body is a transition record we do not need. */
export async function putServiceState(id: string, state: ServiceState): Promise<void> {
  await request<unknown>(`/api/services/${encodeURIComponent(id)}/state`, {
    method: 'POST',
    body: JSON.stringify({ state }),
  })
}
