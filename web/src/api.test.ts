import { afterEach, describe, expect, it, vi } from 'vitest'
import {
  ApiError,
  fetchMe,
  fetchServices,
  fetchUser,
  fetchUsers,
  googleAuthPath,
  logout,
  putGlobalRole,
  putServiceRole,
  putServiceState,
} from './api'
import { jsonResponse, stranger, userServiceError } from './test/fixtures'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function fetchMock(handler: (url: string, init?: RequestInit) => Response | Promise<Response>) {
  const fn = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
    return handler(url, init)
  })
  vi.stubGlobal('fetch', fn)
  return fn
}

describe('googleAuthPath', () => {
  it('omits return_to when absent', () => {
    expect(googleAuthPath(null)).toBe('/oauth/google')
  })

  it('encodes return_to', () => {
    expect(googleAuthPath('https://storage.filenkov.store/x?a=1')).toBe(
      '/oauth/google?return_to=https%3A%2F%2Fstorage.filenkov.store%2Fx%3Fa%3D1',
    )
  })
})

describe('fetchMe', () => {
  it('returns null on unauthorized me when refresh also fails', async () => {
    const fetch = fetchMock(async () => jsonResponse({ error: 'unauthorized' }, 401))
    await expect(fetchMe()).resolves.toBeNull()
    const urls = fetch.mock.calls.map((c) => String(c[0]))
    expect(urls).toEqual(['/api/me', '/api/refresh'])
    expect(fetch.mock.calls[0][1]).toMatchObject({ credentials: 'include' })
    expect(fetch.mock.calls[1][1]).toMatchObject({ method: 'POST', credentials: 'include' })
  })

  it('refreshes once then retries /api/me', async () => {
    let meCalls = 0
    const fetch = fetchMock(async (url, init) => {
      const method = init?.method ?? 'GET'
      if (url === '/api/me' && method === 'GET') {
        meCalls += 1
        if (meCalls === 1) return jsonResponse({ error: 'unauthorized' }, 401)
        return jsonResponse(stranger)
      }
      if (url === '/api/refresh' && method === 'POST') return jsonResponse({ status: 'ok' })
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    await expect(fetchMe()).resolves.toEqual(stranger)
    expect(fetch.mock.calls.map((c) => [String(c[0]), c[1]?.method ?? 'GET'])).toEqual([
      ['/api/me', 'GET'],
      ['/api/refresh', 'POST'],
      ['/api/me', 'GET'],
    ])
  })

  it('returns null if retry after refresh is still unauthorized', async () => {
    const fetch = fetchMock(async (url) => {
      if (url === '/api/refresh') return jsonResponse({ status: 'ok' })
      return jsonResponse({ error: 'unauthorized' }, 401)
    })
    await expect(fetchMe()).resolves.toBeNull()
    expect(fetch.mock.calls.map((c) => String(c[0]))).toEqual(['/api/me', '/api/refresh', '/api/me'])
  })

  it('returns the user without calling refresh', async () => {
    const fetch = fetchMock(async () => jsonResponse(stranger))
    await expect(fetchMe()).resolves.toEqual(stranger)
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('throws ApiError on other failures', async () => {
    fetchMock(async () => jsonResponse({ error: 'blocked' }, 403))
    await expect(fetchMe()).rejects.toMatchObject({ name: 'ApiError', status: 403, message: 'blocked' })
  })
})

describe('logout', () => {
  it('posts /api/logout with credentials', async () => {
    const fetch = fetchMock(async () => jsonResponse({ status: 'ok' }))
    await logout()
    expect(fetch.mock.calls[0][0]).toBe('/api/logout')
    expect(fetch.mock.calls[0][1]).toMatchObject({ method: 'POST', credentials: 'include' })
  })
})

describe('user-service admin api', () => {
  it('loads the directory from GET /users', async () => {
    const fetch = fetchMock(async (url) => {
      if (url === '/users') {
        return jsonResponse({
          users: [
            {
              id: stranger.id,
              google_sub: stranger.google_sub,
              email: stranger.email,
              username: stranger.username,
              global_role: stranger.global_role,
              services: stranger.services,
              created_at: '2026-01-01T00:00:00Z',
              updated_at: '2026-01-01T00:00:00Z',
            },
          ],
        })
      }
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    await expect(fetchUsers()).resolves.toEqual([stranger])
    expect(fetch.mock.calls[0][0]).toBe('/users')
    expect(fetch.mock.calls[0][1]).toMatchObject({ credentials: 'include' })
    expect(JSON.stringify(fetch.mock.calls[0][1]?.headers)).not.toMatch(/X-User-Id/i)
    expect(String(fetch.mock.calls[0][0])).not.toContain('/api/admin')
  })

  it('retries GET /users once after a 401 refresh', async () => {
    let listGets = 0
    const fetch = fetchMock(async (url, init) => {
      const method = init?.method ?? 'GET'
      if (url === '/api/refresh' && method === 'POST') return jsonResponse({ status: 'ok' })
      if (url === '/users' && method === 'GET') {
        listGets += 1
        if (listGets === 1) return jsonResponse({ error: 'unauthorized' }, 401)
        return jsonResponse({ users: [] })
      }
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    await expect(fetchUsers()).resolves.toEqual([])
    expect(fetch.mock.calls.map((c) => [String(c[0]), c[1]?.method ?? 'GET'])).toEqual([
      ['/users', 'GET'],
      ['/api/refresh', 'POST'],
      ['/users', 'GET'],
    ])
  })

  it('loads a user by UUID with roles', async () => {
    fetchMock(async (url) => {
      if (url === `/users/${stranger.id}`) {
        return jsonResponse({
          id: stranger.id,
          google_sub: stranger.google_sub,
          email: stranger.email,
          username: stranger.username,
          global_role: 'STRANGER',
        })
      }
      if (url === `/users/${stranger.id}/roles`) {
        return jsonResponse({
          user_id: stranger.id,
          global_role: 'STRANGER',
          services: [{ service: 'storage', role: 'FAMILY' }],
        })
      }
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    await expect(fetchUser(stranger.id)).resolves.toEqual({
      ...stranger,
      services: [{ service: 'storage', role: 'FAMILY' }],
    })
  })

  it('puts global and service roles', async () => {
    const fetch = fetchMock(async (url, init) => {
      const method = init?.method ?? 'GET'
      if (url === `/users/${stranger.id}/role` && method === 'PUT') {
        return jsonResponse({ user_id: stranger.id, role: 'FAMILY' })
      }
      if (url === `/users/${stranger.id}/roles/storage` && method === 'PUT') {
        return jsonResponse({ user_id: stranger.id, service: 'storage', role: 'ADMIN' })
      }
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    await putGlobalRole(stranger.id, 'FAMILY')
    await putServiceRole(stranger.id, 'storage', 'ADMIN')
    expect(fetch.mock.calls.map((c) => [String(c[0]), c[1]?.method, c[1]?.body])).toEqual([
      [`/users/${stranger.id}/role`, 'PUT', JSON.stringify({ role: 'FAMILY' })],
      [`/users/${stranger.id}/roles/storage`, 'PUT', JSON.stringify({ role: 'ADMIN' })],
    ])
    expect(fetch.mock.calls[0][1]).toMatchObject({ credentials: 'include' })
    expect(JSON.stringify(fetch.mock.calls[0][1]?.headers)).not.toMatch(/X-User-Id/i)
  })

  it('parses User-Service forbidden envelope', async () => {
    fetchMock(async () => userServiceError('FORBIDDEN', 'Actor is not an admin', 403))
    await expect(putGlobalRole(stranger.id, 'FAMILY')).rejects.toMatchObject({
      name: 'ApiError',
      status: 403,
      message: 'Actor is not an admin',
    })
  })

  it('retries a 401 PUT after refresh', async () => {
    let rolePuts = 0
    const fetch = fetchMock(async (url, init) => {
      const method = init?.method ?? 'GET'
      if (url === '/api/refresh' && method === 'POST') return jsonResponse({ status: 'ok' })
      if (url === `/users/${stranger.id}/role` && method === 'PUT') {
        rolePuts += 1
        if (rolePuts === 1) return jsonResponse({ error: 'unauthorized' }, 401)
        return jsonResponse({ user_id: stranger.id, role: 'FAMILY' })
      }
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    await putGlobalRole(stranger.id, 'FAMILY')
    expect(fetch.mock.calls.map((c) => String(c[0]))).toEqual([
      `/users/${stranger.id}/role`,
      '/api/refresh',
      `/users/${stranger.id}/role`,
    ])
  })
})

describe('switch-service api', () => {
  const host = {
    ram: {
      total_mib: 90365,
      used_mib: 40969,
      available_mib: 49396,
      reserve_mib: 8192,
      budget_mib: 82173,
      claimed_mib: 36352,
      free_mib: 45821,
    },
    vram: null,
  }

  it('loads the snapshot from GET /api/services', async () => {
    const fetch = fetchMock(async (url) => {
      if (url === '/api/services') return jsonResponse({ host, services: [], active_transitions: [] })
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    await expect(fetchServices()).resolves.toMatchObject({ services: [], host: { vram: null } })
    expect(fetch.mock.calls[0][0]).toBe('/api/services')
    expect(fetch.mock.calls[0][1]).toMatchObject({ credentials: 'include' })
  })

  it('posts the requested state and ignores the 202 transition body', async () => {
    const fetch = fetchMock(async (url, init) => {
      if (url === '/api/services/ocr/state' && init?.method === 'POST') {
        return jsonResponse({ noop: false, transition: { phase: 'starting' } }, 202)
      }
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    await expect(putServiceState('ocr', 'gpu')).resolves.toBeUndefined()
    expect(fetch.mock.calls[0][1]).toMatchObject({
      method: 'POST',
      body: JSON.stringify({ state: 'gpu' }),
      credentials: 'include',
    })
  })

  it('accepts a 202 without a body', async () => {
    fetchMock(async () => new Response(null, { status: 202 }))
    await expect(putServiceState('ocr', 'off')).resolves.toBeUndefined()
  })

  it('keeps the switch-service error code next to the message', async () => {
    fetchMock(async () =>
      jsonResponse({ error: { code: 'INSUFFICIENT_VRAM', message: 'Not enough VRAM for gpu' } }, 409),
    )
    await expect(putServiceState('ocr', 'gpu')).rejects.toMatchObject({
      name: 'ApiError',
      status: 409,
      code: 'INSUFFICIENT_VRAM',
      message: 'Not enough VRAM for gpu',
    })
  })
})

describe('ApiError', () => {
  it('keeps status', () => {
    const err = new ApiError(409, 'LAST_ADMIN')
    expect(err.status).toBe(409)
    expect(err.message).toBe('LAST_ADMIN')
  })
})
