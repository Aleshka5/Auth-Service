import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Layout } from './Layout'
import { admin, jsonResponse, stranger } from '../test/fixtures'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function renderLayout(path = '/') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<div>outlet</div>} />
            <Route path="/auth" element={<div>auth-outlet</div>} />
          </Route>
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  )
}

async function mascot(clip: string) {
  return waitFor(() => {
    const node = document.querySelector(`[data-mascot="${clip}"]`)
    if (!(node instanceof HTMLElement)) throw new Error(`missing mascot ${clip}`)
    return node
  })
}

describe('Layout', () => {
  it('shows Sign in for anonymous users', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    renderLayout()
    expect(await screen.findByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/auth')
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Hermes' })).not.toBeInTheDocument()
  })

  it('shows the name, Admin, and Hermes for admins', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(admin)))
    renderLayout()
    expect(await screen.findByText('Boss')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Admin' })).toHaveAttribute('href', '/admin')
    expect(screen.getByRole('link', { name: 'Hermes' })).toHaveAttribute('href', '/hermes/chat')
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
  })

  it('shows Hermes but not Admin for signed-in non-admins', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderLayout()
    expect(await screen.findByText('Ann')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Hermes' })).toHaveAttribute('href', '/hermes/chat')
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
  })

  it('plays calm-to-angry for guests on home', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    renderLayout('/')
    const node = await mascot('guest')
    expect(node).toHaveAttribute('data-mascot-motion', 'calm-to-angry')
    expect(node).toHaveAttribute('data-mascot-phase', 'motion')
  })

  it('plays calm-to-angry on the auth page', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    renderLayout('/auth')
    const node = await mascot('guest')
    expect(node).toHaveAttribute('data-mascot-motion', 'calm-to-angry')
    expect(node).toHaveAttribute('data-mascot-phase', 'motion')
  })

  it('plays angry-to-calm for signed-in non-admins on home', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderLayout('/')
    const node = await mascot('user')
    expect(node).toHaveAttribute('data-mascot-motion', 'angry-to-calm')
    expect(node).toHaveAttribute('data-mascot-phase', 'motion')
  })

  it('plays calm-to-kind for admins on home', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(admin)))
    renderLayout('/')
    const node = await mascot('admin')
    expect(node).toHaveAttribute('data-mascot-motion', 'calm-to-kind')
    expect(node).toHaveAttribute('data-mascot-phase', 'motion')
  })

  it('plays calm-to-angry then signs out', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      const method = init?.method ?? 'GET'
      if (url === '/api/me') return jsonResponse(stranger)
      if (url === '/api/logout' && method === 'POST') return new Response(null, { status: 204 })
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    vi.stubGlobal('fetch', fetchMock)
    renderLayout('/')
    expect(await screen.findByText('Ann')).toBeInTheDocument()
    expect(await mascot('user')).toHaveAttribute('data-mascot-motion', 'angry-to-calm')

    fireEvent.click(screen.getByRole('button', { name: 'Sign out' }))
    const node = await mascot('guest')
    expect(node).toHaveAttribute('data-mascot-motion', 'calm-to-angry')
    expect(node).toHaveAttribute('data-mascot-phase', 'motion')
    expect(node.querySelector('svg')).not.toBeNull()
    expect(screen.getByText('Ann')).toBeInTheDocument()
    expect(
      fetchMock.mock.calls.some(
        ([url, init]) => String(url) === '/api/logout' && (init as RequestInit | undefined)?.method === 'POST',
      ),
    ).toBe(false)

    await waitFor(
      () => {
        expect(screen.getByRole('link', { name: 'Sign in' })).toBeInTheDocument()
      },
      { timeout: 4000 },
    )
    expect(
      fetchMock.mock.calls.some(
        ([url, init]) => String(url) === '/api/logout' && (init as RequestInit | undefined)?.method === 'POST',
      ),
    ).toBe(true)
  })
})
