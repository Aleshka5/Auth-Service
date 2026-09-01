import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../../auth'
import { jsonResponse, stranger } from '../../test/fixtures'
import { LegacyArchitectureRedirect, NodeRoute } from './NodeRoute'

beforeEach(() => {
  vi.stubGlobal('scrollTo', vi.fn())
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function renderAt(path: string) {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <AuthProvider>
        <Routes>
          <Route path="/architecture/:slug" element={<LegacyArchitectureRedirect />} />
          <Route path="*" element={<NodeRoute />} />
        </Routes>
      </AuthProvider>
    </MemoryRouter>,
  )
}

const guest = () => vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ error: 'unauthorized' }, 401)))
const signedIn = () => vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))

describe('NodeRoute', () => {
  it('renders the root node', async () => {
    guest()
    renderAt('/')
    expect(await screen.findByRole('heading', { level: 1, name: 'Alexey Filenkov' })).toBeInTheDocument()
  })

  it('renders a deep node with its spine', async () => {
    guest()
    renderAt('/experience/sber')
    expect(await screen.findByRole('heading', { level: 1, name: 'Sber' })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Resume path' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Experience' })).toHaveAttribute('href', '/experience')
  })

  it('sends an unknown path back to the root', async () => {
    guest()
    renderAt('/not-a-node')
    expect(await screen.findByRole('heading', { level: 1, name: 'Alexey Filenkov' })).toBeInTheDocument()
  })

  it('redirects a moved path from the redirect table', async () => {
    guest()
    renderAt('/architecture')
    expect(await screen.findByRole('heading', { level: 1, name: 'Architecture' })).toBeInTheDocument()
  })

  it('redirects the legacy architecture slug route to the tree', async () => {
    guest()
    renderAt('/architecture/auth-gateway')
    expect(await screen.findByRole('heading', { level: 1, name: 'Auth Gateway' })).toBeInTheDocument()
  })

  it('sends a guest away from a signed-in node and lets a signed-in reader through', async () => {
    guest()
    const { unmount } = renderAt('/achievements/projects/hermes')
    expect(await screen.findByRole('heading', { level: 1, name: 'Alexey Filenkov' })).toBeInTheDocument()
    unmount()

    signedIn()
    renderAt('/achievements/projects/hermes')
    expect(await screen.findByRole('heading', { level: 1, name: 'Hermes' })).toBeInTheDocument()
  })

  it('normalises case and a trailing slash', async () => {
    guest()
    renderAt('/Experience/Sber/')
    expect(await screen.findByRole('heading', { level: 1, name: 'Sber' })).toBeInTheDocument()
  })

  it('scrolls to the top on a node change', async () => {
    guest()
    renderAt('/experience/sber')
    await screen.findByRole('heading', { level: 1, name: 'Sber' })
    expect(window.scrollTo).toHaveBeenCalledWith({ top: 0, behavior: 'auto' })
  })
})
