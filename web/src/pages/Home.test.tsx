import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Home } from './Home'
import { admin, jsonResponse, stranger } from '../test/fixtures'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function renderHome() {
  return render(
    <MemoryRouter>
      <AuthProvider>
        <Home />
      </AuthProvider>
    </MemoryRouter>,
  )
}

describe('Home', () => {
  it('shows public links and hides Hermes for anonymous users', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    renderHome()
    expect(await screen.findByRole('link', { name: /LLM/i })).toHaveAttribute('href', 'https://llm.filenkov.store')
    expect(screen.getByRole('link', { name: /Storage/i })).toHaveAttribute('href', 'https://storage.filenkov.store')
    expect(screen.queryByRole('link', { name: /Hermes/i })).not.toBeInTheDocument()
  })

  it('shows Hermes for admins', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(admin)))
    renderHome()
    const hermes = await screen.findByRole('link', { name: /Hermes/i })
    expect(hermes).toHaveAttribute('href', '/hermes/')
    expect(screen.queryByText(stranger.name)).not.toBeInTheDocument()
  })
})
