import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Admin } from './Admin'
import { admin, jsonResponse, stranger, userServiceError } from '../test/fixtures'
import type { User } from '../types'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

const hermesUser: User = {
  id: '33333333-3333-3333-3333-333333333333',
  google_sub: '110000000000000000003',
  email: 'kay@example.com',
  username: 'Kay',
  global_role: 'FAMILY',
  services: [{ service: 'hermes', role: 'FAMILY' }],
}

function renderAdmin() {
  return render(
    <MemoryRouter initialEntries={['/admin']}>
      <AuthProvider>
        <Routes>
          <Route path="/" element={<div>home-page</div>} />
          <Route path="/admin" element={<Admin />} />
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  )
}

function listItem(person: User) {
  return {
    id: person.id,
    google_sub: person.google_sub,
    email: person.email,
    username: person.username,
    global_role: person.global_role,
    services: person.services,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  }
}

function directoryHandlers(overrides?: {
  users?: User[]
  list?: () => Response | Promise<Response>
  putRole?: (url: string, init?: RequestInit) => Response | Promise<Response>
}) {
  return async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (url === '/api/me') return jsonResponse(admin)
    if (url === '/users' && method === 'GET') {
      if (overrides?.list) return overrides.list()
      return jsonResponse({ users: (overrides?.users ?? [admin, stranger]).map(listItem) })
    }
    if (method === 'PUT' && overrides?.putRole) {
      return overrides.putRole(url, init)
    }
    if (url === `/users/${stranger.id}/role` && method === 'PUT') {
      return jsonResponse({ user_id: stranger.id, role: 'ADMIN' })
    }
    if (url === `/users/${stranger.id}/roles/storage` && method === 'PUT') {
      return jsonResponse({ user_id: stranger.id, service: 'storage', role: 'ADMIN' })
    }
    if (url === `/users/${hermesUser.id}/roles/hermes` && method === 'PUT') {
      return jsonResponse({ user_id: hermesUser.id, service: 'hermes', role: 'ADMIN' })
    }
    return jsonResponse({ error: 'unexpected ' + url }, 500)
  }
}

function stubDirectory(overrides?: Parameters<typeof directoryHandlers>[0]) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    return directoryHandlers(overrides)(url, init)
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function calledUrls(fetchMock: ReturnType<typeof stubDirectory>) {
  return fetchMock.mock.calls.map((c) => String(c[0]))
}

describe('Admin', () => {
  it('redirects non-admins home', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderAdmin()
    expect(await screen.findByText('home-page')).toBeInTheDocument()
  })

  it('loads the directory from GET /users and does not use UUID lookup or /api/admin', async () => {
    const fetchMock = stubDirectory()
    renderAdmin()

    expect(await screen.findByRole('heading', { name: 'Directory' })).toBeInTheDocument()
    expect(await screen.findByText('Ann')).toBeInTheDocument()
    expect(screen.getByText('Boss')).toBeInTheDocument()
    expect(screen.getByText('ann@example.com')).toBeInTheDocument()
    expect(screen.getByText('boss@example.com')).toBeInTheDocument()

    expect(screen.getByRole('combobox', { name: /Site role for Ann/i })).toHaveValue('STRANGER')
    expect(screen.getByRole('combobox', { name: /Storage role for Ann/i })).toHaveValue('FAMILY')
    expect(screen.getByRole('combobox', { name: /Resume role for Ann/i })).toHaveValue('STRANGER')
    expect(screen.getByRole('combobox', { name: /Site role for Boss/i })).toHaveValue('ADMIN')
    expect(screen.getByRole('combobox', { name: /Site role for Boss/i })).not.toBeDisabled()
    expect(screen.getByRole('combobox', { name: /Storage role for Boss/i })).not.toBeDisabled()

    expect(screen.queryByLabelText(/user uuid/i)).not.toBeInTheDocument()
    expect(screen.queryByRole('textbox', { name: /user uuid/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /look up/i })).not.toBeInTheDocument()
    expect(screen.queryByText(/no directory list/i)).not.toBeInTheDocument()
    expect(screen.queryByPlaceholderText(/xxxx/i)).not.toBeInTheDocument()

    expect(calledUrls(fetchMock)).toContain('/users')
    expect(calledUrls(fetchMock)).not.toEqual(expect.arrayContaining([expect.stringContaining('/api/admin')]))
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) =>
          String(input) === `/users/${stranger.id}` && (init?.method ?? 'GET') === 'GET',
      ),
    ).toBe(false)
  })

  it('filters the table by email substring without another request', async () => {
    const fetchMock = stubDirectory()
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('Ann')).toBeInTheDocument()
    expect(screen.getByText('Boss')).toBeInTheDocument()
    const afterLoad = fetchMock.mock.calls.length

    await user.type(screen.getByLabelText(/email/i), 'ann@')
    expect(screen.getByText('Ann')).toBeInTheDocument()
    expect(screen.queryByText('Boss')).not.toBeInTheDocument()
    expect(fetchMock.mock.calls.length).toBe(afterLoad)

    await user.clear(screen.getByLabelText(/email/i))
    expect(screen.getByText('Ann')).toBeInTheDocument()
    expect(screen.getByText('Boss')).toBeInTheDocument()
    expect(fetchMock.mock.calls.length).toBe(afterLoad)
  })

  it('does not filter by username, UUID, or google_sub', async () => {
    const odd: User = {
      id: '44444444-4444-4444-4444-444444444444',
      google_sub: '110000000000000000099',
      email: 'zzz@example.com',
      username: 'UniqueHandle',
      global_role: 'FAMILY',
      services: [],
    }
    stubDirectory({ users: [odd] })
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('UniqueHandle')).toBeInTheDocument()

    await user.type(screen.getByLabelText(/email/i), 'UniqueHandle')
    expect(await screen.findByText('No one matches that email.')).toBeInTheDocument()
    expect(screen.queryByText('UniqueHandle')).not.toBeInTheDocument()

    await user.clear(screen.getByLabelText(/email/i))
    expect(await screen.findByText('UniqueHandle')).toBeInTheDocument()
    await user.type(screen.getByLabelText(/email/i), odd.id)
    expect(await screen.findByText('No one matches that email.')).toBeInTheDocument()

    await user.clear(screen.getByLabelText(/email/i))
    expect(await screen.findByText('UniqueHandle')).toBeInTheDocument()
    await user.type(screen.getByLabelText(/email/i), odd.google_sub)
    expect(await screen.findByText('No one matches that email.')).toBeInTheDocument()

    await user.clear(screen.getByLabelText(/email/i))
    await user.type(screen.getByLabelText(/email/i), 'zzz@')
    expect(await screen.findByText('UniqueHandle')).toBeInTheDocument()
  })

  it('distinguishes an empty directory from a search miss', async () => {
    stubDirectory({ users: [] })
    renderAdmin()
    expect(await screen.findByText('No one is in the directory yet.')).toBeInTheDocument()
    expect(screen.queryByText('No one matches that email.')).not.toBeInTheDocument()
    expect(screen.queryByRole('table', { name: 'Directory' })).not.toBeInTheDocument()
  })

  it('shows no-match copy when email filter hides every loaded row', async () => {
    stubDirectory()
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('Ann')).toBeInTheDocument()

    await user.type(screen.getByLabelText(/email/i), 'zzz@nope.example')
    expect(await screen.findByText('No one matches that email.')).toBeInTheDocument()
    expect(screen.queryByText('No one is in the directory yet.')).not.toBeInTheDocument()
    expect(screen.queryByText('Ann')).not.toBeInTheDocument()
  })

  it('shows an error and no table when GET /users fails', async () => {
    stubDirectory({
      list: () => jsonResponse({ error: { code: 'INTERNAL', message: 'directory unavailable' } }, 500),
    })
    renderAdmin()
    expect(await screen.findByText('directory unavailable')).toBeInTheDocument()
    expect(screen.queryByRole('table', { name: 'Directory' })).not.toBeInTheDocument()
    expect(screen.queryByText('Ann')).not.toBeInTheDocument()
    expect(screen.queryByLabelText(/user uuid/i)).not.toBeInTheDocument()
  })

  it('shows a forbidden error when GET /users is rejected', async () => {
    stubDirectory({
      list: () => userServiceError('FORBIDDEN', 'forbidden', 403),
    })
    renderAdmin()
    expect(await screen.findByText('forbidden')).toBeInTheDocument()
    expect(screen.queryByRole('table', { name: 'Directory' })).not.toBeInTheDocument()
  })

  it('changes a global role with PUT /users/{id}/role', async () => {
    const fetchMock = stubDirectory()
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('Ann')).toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: /Site role for Ann/i }), 'ADMIN')
    await waitFor(() => {
      const put = fetchMock.mock.calls.find(
        (c) => String(c[0]) === `/users/${stranger.id}/role` && c[1]?.method === 'PUT',
      )
      expect(put).toBeTruthy()
      expect(put?.[1]?.body).toBe(JSON.stringify({ role: 'ADMIN' }))
    })
    expect(screen.getByRole('combobox', { name: /Site role for Ann/i })).toHaveValue('ADMIN')
    expect(calledUrls(fetchMock)).not.toEqual(expect.arrayContaining([expect.stringContaining('/api/admin')]))
  })

  it('changes a service role with PUT /users/{id}/roles/{service}', async () => {
    const fetchMock = stubDirectory()
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('Ann')).toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: /Storage role for Ann/i }), 'ADMIN')
    await waitFor(() => {
      const put = fetchMock.mock.calls.find(
        (c) => String(c[0]) === `/users/${stranger.id}/roles/storage` && c[1]?.method === 'PUT',
      )
      expect(put).toBeTruthy()
      expect(put?.[1]?.body).toBe(JSON.stringify({ role: 'ADMIN' }))
    })
    expect(screen.getByRole('combobox', { name: /Storage role for Ann/i })).toHaveValue('ADMIN')
  })

  it('shows a fixed Image column before any image row exists and PUTs /users/{id}/roles/image', async () => {
    const fetchMock = stubDirectory({
      putRole: (url) => {
        if (url === `/users/${stranger.id}/roles/image`) {
          return jsonResponse({ user_id: stranger.id, service: 'image', role: 'FAMILY' })
        }
        return jsonResponse({ error: 'unexpected' }, 500)
      },
    })
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('Ann')).toBeInTheDocument()

    expect(screen.getAllByRole('columnheader', { name: 'Image' })).toHaveLength(1)
    expect(screen.getByRole('combobox', { name: /Image role for Ann/i })).toHaveValue('STRANGER')
    expect(screen.getByRole('combobox', { name: /Image role for Boss/i })).toHaveValue('STRANGER')

    await user.selectOptions(screen.getByRole('combobox', { name: /Image role for Ann/i }), 'FAMILY')
    await waitFor(() => {
      const put = fetchMock.mock.calls.find(
        (c) => String(c[0]) === `/users/${stranger.id}/roles/image` && c[1]?.method === 'PUT',
      )
      expect(put).toBeTruthy()
      expect(put?.[1]?.body).toBe(JSON.stringify({ role: 'FAMILY' }))
    })
    expect(screen.getByRole('combobox', { name: /Image role for Ann/i })).toHaveValue('FAMILY')
  })

  it('does not duplicate the Image column when a row already exists', async () => {
    const imageUser: User = { ...hermesUser, services: [{ service: 'image', role: 'ADMIN' }] }
    stubDirectory({ users: [admin, imageUser] })
    renderAdmin()
    expect(await screen.findByText('Kay')).toBeInTheDocument()
    expect(screen.getAllByRole('columnheader', { name: 'Image' })).toHaveLength(1)
    expect(screen.getByRole('combobox', { name: /Image role for Kay/i })).toHaveValue('ADMIN')
  })

  it('adds a dynamic extra service column from persisted services', async () => {
    const fetchMock = stubDirectory({ users: [admin, hermesUser] })
    const user = userEvent.setup()
    renderAdmin()

    expect(await screen.findByRole('columnheader', { name: 'Hermes' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: /Hermes role for Kay/i })).toHaveValue('FAMILY')
    expect(screen.getByRole('combobox', { name: /Storage role for Kay/i })).toHaveValue('STRANGER')
    expect(screen.getByRole('combobox', { name: /Resume role for Kay/i })).toHaveValue('STRANGER')
    expect(screen.getByRole('combobox', { name: /Hermes role for Boss/i })).toHaveValue('STRANGER')

    await user.selectOptions(screen.getByRole('combobox', { name: /Hermes role for Kay/i }), 'ADMIN')
    await waitFor(() => {
      const put = fetchMock.mock.calls.find(
        (c) => String(c[0]) === `/users/${hermesUser.id}/roles/hermes` && c[1]?.method === 'PUT',
      )
      expect(put).toBeTruthy()
      expect(put?.[1]?.body).toBe(JSON.stringify({ role: 'ADMIN' }))
    })
  })

  it('shows LAST_ADMIN and leaves the select on the previous value', async () => {
    const fetchMock = stubDirectory({
      putRole: (url) => {
        if (url === `/users/${admin.id}/role`) {
          return userServiceError('LAST_ADMIN', 'Cannot demote the last admin', 409)
        }
        return jsonResponse({ error: 'unexpected' }, 500)
      },
    })
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('Boss')).toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: /Site role for Boss/i }), 'FAMILY')
    expect(await screen.findByText('Cannot demote the last admin')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: /Site role for Boss/i })).toHaveValue('ADMIN')
    expect(calledUrls(fetchMock)).toContain(`/users/${admin.id}/role`)
  })

  it('shows a forbidden error from User-Service without changing the select', async () => {
    stubDirectory({
      putRole: () => userServiceError('FORBIDDEN', 'Actor is not an admin', 403),
    })
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('Ann')).toBeInTheDocument()

    await user.selectOptions(screen.getByRole('combobox', { name: /Site role for Ann/i }), 'FAMILY')
    expect(await screen.findByText('Actor is not an admin')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: /Site role for Ann/i })).toHaveValue('STRANGER')
  })
})
