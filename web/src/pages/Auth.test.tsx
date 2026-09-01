import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Auth } from './Auth'
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

describe('Auth', () => {
  it('sends return_to through to Google OAuth', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    renderAuth('/auth?return_to=https://storage.filenkov.store/app')
    const btn = await screen.findByRole('link', { name: /Continue with Google/i })
    expect(btn).toHaveAttribute(
      'href',
      '/oauth/google?return_to=https%3A%2F%2Fstorage.filenkov.store%2Fapp',
    )
  })

  it('shows the signed-in name', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderAuth('/auth')
    expect(await screen.findByText(/already signed in as/i)).toBeInTheDocument()
    expect(screen.getByText('Ann')).toBeInTheDocument()
  })

  it('links a signed-in user back to same-origin return_to', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderAuth('/auth?return_to=/hermes/')
    const btn = await screen.findByRole('link', { name: /Continue to destination/i })
    expect(btn).toHaveAttribute('href', '/hermes/')
  })
})
