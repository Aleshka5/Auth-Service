import { vi } from 'vitest'
import type { User } from '../types'

export const stranger: User = {
  id: '11111111-1111-1111-1111-111111111111',
  google_sub: '110000000000000000001',
  email: 'ann@example.com',
  username: 'Ann',
  global_role: 'STRANGER',
  services: [{ service: 'storage', role: 'FAMILY' }],
}

export const admin: User = {
  ...stranger,
  id: '22222222-2222-2222-2222-222222222222',
  google_sub: '110000000000000000002',
  email: 'boss@example.com',
  username: 'Boss',
  global_role: 'ADMIN',
  services: [
    { service: 'storage', role: 'FAMILY' },
    { service: 'resume', role: 'STRANGER' },
  ],
}

export function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

export function userServiceError(code: string, message: string, status: number): Response {
  return jsonResponse({ error: { code, message } }, status)
}

export function mockFetch(handler: (url: string, init?: RequestInit) => Response | Promise<Response>) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    return handler(url, init)
  })
  vi.stubGlobal('fetch', fn)
  return fn
}
