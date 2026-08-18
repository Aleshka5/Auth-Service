import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Admin } from './Admin'
import { admin, jsonResponse, stranger } from '../test/fixtures'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

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

describe('Admin', () => {
  it('redirects non-admins home', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderAdmin()
    expect(await screen.findByText('home-page')).toBeInTheDocument()
  })

  it('lists users and can change a role', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      const method = init?.method ?? 'GET'
      if (url === '/api/me') return jsonResponse(admin)
      if (url === '/api/admin/users' && method === 'GET') {
        return jsonResponse({ users: [admin, stranger] })
      }
      if (url.includes('/api/admin/users/') && method === 'PATCH') {
        return jsonResponse({ ...stranger, roles: 'ADMIN' })
      }
      return jsonResponse({ error: 'unexpected ' + url }, 500)
    })
    vi.stubGlobal('fetch', fetchMock)
    const user = userEvent.setup()
    renderAdmin()
    expect(await screen.findByText('Ann')).toBeInTheDocument()
    expect(screen.getByText('boss@example.com')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Block' }).length).toBeGreaterThan(0)

    const siteRole = screen.getByRole('combobox', { name: /Site role for Ann/i })
    await user.selectOptions(siteRole, 'ADMIN')
    await waitFor(() => {
      const patch = fetchMock.mock.calls.find((c) => String(c[0]).includes(String(stranger.id)) && c[1]?.method === 'PATCH')
      expect(patch).toBeTruthy()
    })
  })
})
