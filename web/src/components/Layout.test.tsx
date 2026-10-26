import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Layout } from './Layout'
import { admin, jsonResponse, stranger } from '../test/fixtures'
import type { User } from '../types'

beforeEach(() => {
  vi.stubGlobal('scrollTo', vi.fn())
})

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

const FOCUSABLE = 'a[href], button, input, select, textarea, [tabindex]:not([tabindex="-1"])'

type ModelKind = 'ready' | 'off' | 'unhealthy'

function servicesBody(models: Record<string, ModelKind>) {
  return {
    host: {
      ram: { total_mib: 1, used_mib: 0, reserve_mib: 0, budget_mib: 1, claimed_mib: 0, free_mib: 1 },
      vram: null,
    },
    services: Object.entries(models).map(([id, kind]) => ({
      id,
      title: id,
      state: kind === 'off' ? 'off' : 'ready',
      mode: kind === 'off' ? null : 'gpu',
      desired_state: kind === 'off' ? 'off' : 'gpu',
      busy: false,
      claim: { ram_mib: 1, vram_mib: 1 },
      health: { ok: kind === 'ready' },
    })),
  }
}

/** `/api/me` returns `me`; `/api/services` returns the model map. Other URLs fall through to `onOther`. */
function stubSession(
  me: unknown,
  models: Record<string, ModelKind> = {},
  status = 200,
  onOther?: (url: string, init?: RequestInit) => Response | Promise<Response>,
) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input)
    if (url === '/api/services') return jsonResponse(servicesBody(models))
    if (url === '/api/me') return jsonResponse(me, status)
    if (onOther) return onOther(url, init)
    return jsonResponse({ error: 'unexpected ' + url }, 500)
  })
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

async function lookupKnown() {
  await waitFor(() => {
    expect(screen.getByRole('navigation', { name: 'Primary' })).toHaveAttribute('data-model-lookup', 'known')
  })
}

describe('Layout', () => {
  it('shows Sign in and locked service links for anonymous users', async () => {
    const fetchMock = stubSession({ error: 'unauthorized' }, {}, 401)
    renderLayout()
    expect(await screen.findByRole('link', { name: 'Sign in' })).toHaveAttribute('href', '/auth')

    for (const [label, href] of [
      ['Storage', 'https://storage.filenkov.store'],
      ['Hermes', 'https://hermes.filenkov.store'],
      ['RAG', 'https://rag.filenkov.store'],
    ]) {
      const link = screen.getByRole('link', { name: label })
      expect(link).toHaveAttribute('href', href)
      expect(link).toHaveAttribute('title', 'Sign in required')
      expect(link.querySelector('.service-lock')).not.toBeNull()
    }

    expect(screen.queryByRole('link', { name: 'Messanger' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Chat' })).not.toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Primary' })).toHaveAttribute('data-model-lookup', 'idle')
    expect(fetchMock.mock.calls.some(([url]) => String(url) === '/api/services')).toBe(false)
  })

  it('shows the name, Admin, Chat, and unlocked services for admins when llm is ready', async () => {
    stubSession(admin, { llm: 'ready' })
    renderLayout()
    expect(await screen.findByText('Boss')).toBeInTheDocument()
    await lookupKnown()
    expect(screen.getByRole('link', { name: 'Admin' })).toHaveAttribute('href', '/admin')
    expect(screen.getByRole('link', { name: 'Chat' })).toHaveAttribute('href', 'https://llm.filenkov.store')
    expect(screen.getByRole('link', { name: 'Hermes' })).toHaveAttribute('href', 'https://hermes.filenkov.store')
    expect(screen.queryByRole('link', { name: 'Messanger' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeInTheDocument()
    expect(document.querySelectorAll('.service-lock')).toHaveLength(0)
  })

  it.each(['off', 'unhealthy'] as const)('hides Chat for an admin when llm is %s', async (kind) => {
    stubSession(admin, { llm: kind })
    renderLayout()
    expect(await screen.findByText('Boss')).toBeInTheDocument()
    await lookupKnown()
    expect(screen.getByRole('link', { name: 'Admin' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Chat' })).not.toBeInTheDocument()
  })

  it('hides Chat when Switch-Service cannot be read', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async (input: RequestInfo | URL) => {
        const url = String(input)
        if (url === '/api/services') return jsonResponse({ error: 'unavailable' }, 503)
        return jsonResponse(admin)
      }),
    )
    renderLayout()
    expect(await screen.findByText('Boss')).toBeInTheDocument()
    await lookupKnown()
    expect(screen.getByRole('link', { name: 'Admin' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Chat' })).not.toBeInTheDocument()
  })

  it('shows services but not Admin or Chat for signed-in non-admins', async () => {
    const fetchMock = stubSession(stranger, { llm: 'ready', image: 'ready' })
    renderLayout()
    expect(await screen.findByText('Ann')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Hermes' })).toHaveAttribute('href', 'https://hermes.filenkov.store')
    expect(screen.getByRole('link', { name: 'Storage' })).not.toHaveAttribute('title')
    expect(screen.getByRole('link', { name: 'RAG' })).not.toHaveAttribute('title')
    expect(screen.queryByRole('link', { name: 'Messanger' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Admin' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Chat' })).not.toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Primary' })).toHaveAttribute('data-model-lookup', 'idle')
    expect(fetchMock.mock.calls.some(([url]) => String(url) === '/api/services')).toBe(false)
  })

  it.each(['FAMILY', 'ADMIN'] as const)('shows an unlocked Image link for image %s when the model is ready', async (role) => {
    const person: User = { ...stranger, services: [...stranger.services, { service: 'image', role }] }
    stubSession(person, { image: 'ready', llm: 'ready' })
    renderLayout()
    expect(await screen.findByText('Ann')).toBeInTheDocument()
    await lookupKnown()
    const link = screen.getByRole('link', { name: 'Image' })
    expect(link).toHaveAttribute('href', 'https://image.filenkov.store')
    expect(link).not.toHaveAttribute('title')
    expect(link.querySelector('.service-lock')).toBeNull()
    expect(screen.queryByRole('link', { name: 'Chat' })).not.toBeInTheDocument()
    for (const label of ['Storage', 'Hermes', 'RAG']) {
      expect(screen.getByRole('link', { name: label })).not.toHaveAttribute('title')
    }
  })

  it.each(['off', 'unhealthy'] as const)('hides Image when the model is %s', async (kind) => {
    const person: User = {
      ...stranger,
      services: [...stranger.services, { service: 'image', role: 'FAMILY' }],
    }
    stubSession(person, { image: kind })
    renderLayout()
    expect(await screen.findByText('Ann')).toBeInTheDocument()
    await lookupKnown()
    expect(screen.queryByRole('link', { name: 'Image' })).not.toBeInTheDocument()
  })

  it.each([
    ['explicit STRANGER', [{ service: 'image', role: 'STRANGER' }]],
    ['missing row', []],
    ['BLOCKED', [{ service: 'image', role: 'BLOCKED' }]],
  ] as const)('hides Image for image %s', async (_, rows) => {
    const person: User = { ...admin, services: [...rows] }
    stubSession(person, { llm: 'ready', image: 'ready' })
    renderLayout()
    expect(await screen.findByText('Boss')).toBeInTheDocument()
    await lookupKnown()
    expect(screen.getByRole('link', { name: 'Admin' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Chat' })).toHaveAttribute('href', 'https://llm.filenkov.store')
    expect(screen.getByRole('link', { name: 'Storage' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Image' })).not.toBeInTheDocument()
  })

  it('hides Image from guests', async () => {
    stubSession({ error: 'unauthorized' }, { image: 'ready' }, 401)
    renderLayout()
    expect(await screen.findByRole('link', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Hermes' })).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Image' })).not.toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Chat' })).not.toBeInTheDocument()
  })

  it('puts the skip link first in the tab order, pointing at the article', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse(stranger)))
    const { container } = renderLayout()
    await screen.findByText('Ann')
    const first = container.querySelector(FOCUSABLE)
    expect(first).toHaveClass('skip-link')
    expect(first).toHaveAttribute('href', '#node-article')
    expect(first).toHaveTextContent('Skip to article')
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
