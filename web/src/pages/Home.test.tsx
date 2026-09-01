import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Home } from './Home'
import { jsonResponse, stranger } from '../test/fixtures'

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
  it('shows the resume and public project links, and hides Hermes for anonymous users', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    renderHome()
    expect(await screen.findByRole('heading', { name: 'Alexey Filenkov' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'My Stack' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Projects' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Experience' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Open LLM' })).toHaveAttribute('href', 'https://llm.filenkov.store')
    expect(screen.getByRole('link', { name: 'Open Storage' })).toHaveAttribute(
      'href',
      'https://storage.filenkov.store',
    )
    expect(screen.queryByRole('link', { name: 'Open Hermes' })).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: /Filenkov service map/i })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: /Auth Gateway/i })).toHaveAttribute('href', '/architecture/auth-gateway')
  })

  it('shows Hermes for signed-in users', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    renderHome()
    const hermes = await screen.findByRole('link', { name: 'Open Hermes' })
    expect(hermes).toHaveAttribute('href', '/hermes/chat')
  })

  it('opens a branch from the business line to the technical detail', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
    const user = userEvent.setup()
    renderHome()
    expect(await screen.findByText(/~1M RUB\/year/i)).toBeInTheDocument()
    expect(screen.queryByText(/60,000 to 30,000 tokens/i)).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Sber/i }))
    expect(screen.getByText(/60,000 to 30,000 tokens/i)).toBeInTheDocument()
    expect(screen.queryByText(/Tighter Plan-and-Execute/i)).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /2× cheaper tokens/i }))
    expect(screen.getByText(/Tighter Plan-and-Execute/i)).toBeInTheDocument()
  })
})
