import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchAdminUsers, fetchMe, googleAuthPath, logout, patchUser, setUserBlocked } from './api'
import { admin, jsonResponse, stranger } from './test/fixtures'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

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
  it('returns null on 401', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    await expect(fetchMe()).resolves.toBeNull()
  })

  it('returns the user', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    await expect(fetchMe()).resolves.toEqual(stranger)
  })

  it('throws ApiError on other failures', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'blocked' }, 403)))
    await expect(fetchMe()).rejects.toMatchObject({ name: 'ApiError', status: 403, message: 'blocked' })
  })
})

describe('admin api', () => {
  it('lists users', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ users: [admin, stranger] })))
    await expect(fetchAdminUsers()).resolves.toEqual([admin, stranger])
  })

  it('patches and blocks', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.endsWith('/block')) return jsonResponse({ ...stranger, blocked: true })
      return jsonResponse({ ...stranger, storage_roles: 'ADMIN' })
    })
    vi.stubGlobal('fetch', fetchMock)
    await patchUser(stranger.id, { storage_roles: 'ADMIN' })
    await setUserBlocked(stranger.id, true)
    await logout()
    const urls = fetchMock.mock.calls.map((c) => String(c[0]))
    expect(urls).toEqual([
      `/api/admin/users/${stranger.id}`,
      `/api/admin/users/${stranger.id}/block`,
      '/api/logout',
    ])
    expect(fetchMock.mock.calls[1][1]).toMatchObject({ method: 'POST' })
  })
})
