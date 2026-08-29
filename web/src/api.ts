import type { ServiceRole, SiteRole, User } from './types'

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function readError(res: Response): Promise<string> {
  const text = await res.text()
  if (!text) return res.statusText || `Request failed (${res.status})`
  try {
    const body = JSON.parse(text) as { error?: string; message?: string }
    return body.error || body.message || text
  } catch {
    return text
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    credentials: 'include',
    ...init,
    headers: {
      Accept: 'application/json',
      ...(init.body ? { 'Content-Type': 'application/json' } : {}),
      ...init.headers,
    },
  })
  if (!res.ok) {
    throw new ApiError(res.status, await readError(res))
  }
  if (res.status === 204) {
    return undefined as T
  }
  return (await res.json()) as T
}

export function googleAuthPath(returnTo: string | null): string {
  if (!returnTo) return '/oauth/google'
  return `/oauth/google?return_to=${encodeURIComponent(returnTo)}`
}

export async function fetchMe(): Promise<User | null> {
  const res = await fetch('/api/me', { credentials: 'include', headers: { Accept: 'application/json' } })
  if (res.status === 401) return null
  if (!res.ok) throw new ApiError(res.status, await readError(res))
  return (await res.json()) as User
}

export async function logout(): Promise<void> {
  await request('/api/logout', { method: 'POST' })
}

export async function fetchAdminUsers(): Promise<User[]> {
  const data = await request<{ users: User[] }>('/api/admin/users')
  return data.users ?? []
}

export type RolePatch = {
  roles?: SiteRole
  storage_roles?: ServiceRole
  resume_roles?: ServiceRole
}

export async function patchUser(id: string | number, patch: RolePatch): Promise<User | void> {
  return request(`/api/admin/users/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  })
}
