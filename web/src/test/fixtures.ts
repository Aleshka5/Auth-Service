import { vi } from 'vitest'
import type { User } from '../types'

export const stranger: User = {
  id: '11111111-1111-1111-1111-111111111111',
  google_email: 'ann@example.com',
  name: 'Ann',
  roles: 'STRANGER',
  storage_roles: 'FAMILY',
  resume_roles: 'STRANGER',
  blocked: false,
  last_seen_at: '2026-08-13T12:00:00Z',
}

export const admin: User = {
  ...stranger,
  id: '22222222-2222-2222-2222-222222222222',
  google_email: 'boss@example.com',
  name: 'Boss',
  roles: 'ADMIN',
}

export function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

export function mockFetch(handler: (url: string, init?: RequestInit) => Response | Promise<Response>) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    return handler(url, init)
  })
  vi.stubGlobal('fetch', fn)
  return fn
}
