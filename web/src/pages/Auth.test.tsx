import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Auth, continueTarget, navigation } from './Auth'
import { jsonResponse, stranger } from '../test/fixtures'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function renderAuth(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <Routes>
          <Route path="/auth" element={<Auth />} />
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('continueTarget', () => {
  const origin = 'https://filenkov.store'

  it('keeps a same-origin path', () => {
    expect(continueTarget('/hermes/?x=1', origin)).toBe('/hermes/?x=1')
  })

  it('allows another https Filenkov host', () => {
    expect(continueTarget('https://storage.filenkov.store/app', origin)).toBe(
      'https://storage.filenkov.store/app',
    )
  })

  it('rejects a host outside the cookie domain', () => {
    expect(continueTarget('https://evil.example/phish', origin)).toBeNull()
    expect(continueTarget('https://filenkov.store.evil.example/', origin)).toBeNull()
  })
})

describe('Auth', () => {
  it('offers Google only after refresh fails', async () => {
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      expect(url === '/api/me' || url === '/api/refresh').toBe(true)
      if (url === '/api/refresh') expect(init?.method).toBe('POST')
      return jsonResponse({ error: 'unauthorized' }, 401)
    })
    vi.stubGlobal('fetch', fetch)
    renderAuth('/auth?return_to=https://storage.filenkov.store/app')
    expect(screen.getByText(/Checking sign-in/i)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Continue with Google/i })).not.toBeInTheDocument()
    const btn = await screen.findByRole('link', { name: /Continue with Google/i })
    expect(btn).toHaveAttribute(
      'href',
      '/oauth/google?return_to=https%3A%2F%2Fstorage.filenkov.store%2Fapp',
    )
    expect(fetch.mock.calls.map((c) => String(c[0]))).toEqual(['/api/me', '/api/refresh'])
  })

  it('restores the session from the refresh cookie and returns to the service', async () => {
    const assign = vi.spyOn(navigation, 'assign').mockImplementation(() => {})
    let meCalls = 0
    const fetch = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.toString() : input.url
      const method = init?.method ?? 'GET'
      if (url === '/api/me' && method === 'GET') {
        meCalls += 1
        if (meCalls === 1) return jsonResponse({ error: 'unauthorized' }, 401)
        return jsonResponse(stranger)
      }
      if (url === '/api/refresh' && method === 'POST') return jsonResponse({ status: 'ok' })
      return jsonResponse({ error: 'unexpected' }, 500)
    })
    vi.stubGlobal('fetch', fetch)
    renderAuth('/auth?return_to=https://storage.filenkov.store/app')
    const btn = await screen.findByRole('link', { name: /Continue to destination/i })
    expect(btn).toHaveAttribute('href', 'https://storage.filenkov.store/app')
    expect(screen.queryByRole('link', { name: /Continue with Google/i })).not.toBeInTheDocument()
    expect(assign).toHaveBeenCalledWith('https://storage.filenkov.store/app')
    expect(fetch.mock.calls.map((c) => [String(c[0]), c[1]?.method ?? 'GET'])).toEqual([
      ['/api/me', 'GET'],
      ['/api/refresh', 'POST'],
      ['/api/me', 'GET'],
    ])
  })

  it('shows the signed-in name', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderAuth('/auth')
    expect(await screen.findByText(/already signed in as/i)).toBeInTheDocument()
    expect(screen.getByText('Ann')).toBeInTheDocument()
  })

  it('does not follow a return_to outside the Filenkov hosts', async () => {
    const assign = vi.spyOn(navigation, 'assign').mockImplementation(() => {})
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderAuth('/auth?return_to=https://evil.example/phish')
    expect(await screen.findByText(/already signed in as/i)).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Continue to destination/i })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: /Continue with Google/i })).not.toBeInTheDocument()
    expect(assign).not.toHaveBeenCalled()
  })

  it('links a signed-in user back to same-origin return_to', async () => {
    vi.spyOn(navigation, 'assign').mockImplementation(() => {})
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderAuth('/auth?return_to=/hermes/')
    const btn = await screen.findByRole('link', { name: /Continue to destination/i })
    expect(btn).toHaveAttribute('href', '/hermes/')
  })
})
