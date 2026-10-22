import { act, render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth'
import { Admin } from './Admin'
import { admin, jsonResponse, stranger, userServiceError } from '../test/fixtures'
import type { ManagedService, ServicesSnapshot, User } from '../types'

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

function managedService(overrides: Partial<ManagedService> & Pick<ManagedService, 'id' | 'title'>): ManagedService {
  return {
    state: 'off',
    mode: null,
    desired_state: 'off',
    busy: false,
    unavailable_states: {},
    claim: { ram_mib: 1024, vram_mib: 1024 },
    measured: null,
    transition: null,
    ...overrides,
  }
}

/** Mirrors the verified GET /api/services snapshot (4 services in production, 3 here). */
function servicesSnapshot(overrides?: Partial<ServicesSnapshot>): ServicesSnapshot {
  return {
    host: {
      ram: {
        total_mib: 90365,
        used_mib: 40969,
        available_mib: 49396,
        reserve_mib: 8192,
        budget_mib: 82173,
        claimed_mib: 36352,
        free_mib: 45821,
      },
      vram: {
        total_mib: 32768,
        used_mib: 3810,
        reserve_mib: 1536,
        budget_mib: 31232,
        claimed_mib: 14848,
        free_mib: 16384,
      },
      cpu: { threads: 16, load1: 2.15 },
      device: { index: 0, name: 'Tesla PG503-216', compute_cap: '7.0' },
      sampled_at: '2026-10-22T14:58:30Z',
      sample_source: 'live',
    },
    services: [
      managedService({
        id: 'ocr',
        title: 'OCR (Surya OCR 2)',
        state: 'ready',
        mode: 'cpu',
        desired_state: 'cpu',
        claim: { ram_mib: 1024, vram_mib: 3072 },
        measured: { ram_mib: null, vram_mib: 2680, at: '2026-10-22T14:58:30Z' },
      }),
      managedService({
        id: 'llm',
        title: 'LLM',
        state: 'off',
        desired_state: 'off',
        claim: { ram_mib: 2048, vram_mib: 14336 },
        unavailable_states: { cpu: { reason: 'no cpu profile is defined' } },
      }),
      managedService({
        id: 'image',
        title: 'Image',
        state: 'failed',
        desired_state: 'off',
        claim: { ram_mib: 512, vram_mib: 4096 },
      }),
    ],
    active_transitions: [],
    ...overrides,
  }
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
  services?: () => Response | Promise<Response>
  postState?: (url: string, init?: RequestInit) => Response | Promise<Response>
}) {
  return async (url: string, init?: RequestInit) => {
    const method = init?.method ?? 'GET'
    if (url === '/api/me') return jsonResponse(admin)
    if (url === '/api/services' && method === 'GET') {
      return overrides?.services ? overrides.services() : jsonResponse(servicesSnapshot())
    }
    if (url.startsWith('/api/services/') && method === 'POST') {
      if (overrides?.postState) return overrides.postState(url, init)
      return jsonResponse({ noop: false, transition: { phase: 'starting' } }, 202)
    }
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

  describe('services section', () => {
    it('renders below the directory with a heading, a lead and one row per service', async () => {
      stubDirectory()
      renderAdmin()

      expect(await screen.findByRole('heading', { name: 'Directory' })).toBeInTheDocument()
      expect(await screen.findByRole('heading', { name: 'Services', level: 2 })).toBeInTheDocument()
      const table = screen.getByRole('table', { name: 'Services' })
      expect(within(table).getByText('OCR (Surya OCR 2)')).toBeInTheDocument()
      expect(within(table).getByText('LLM')).toBeInTheDocument()
      expect(within(table).getByText('Image')).toBeInTheDocument()
      expect(within(table).getByText('ready · cpu')).toBeInTheDocument()
      expect(within(table).getByText('failed')).toBeInTheDocument()
    })

    it('renders an Off, CPU and GPU button per service and marks the desired state', async () => {
      stubDirectory()
      renderAdmin()

      const groups = await screen.findAllByRole('group', { name: /runtime$/ })
      expect(groups).toHaveLength(3)
      for (const group of groups) {
        expect(within(group).getAllByRole('button').map((button) => button.textContent)).toEqual([
          'Off',
          'CPU',
          'GPU',
        ])
      }

      const ocr = screen.getByRole('group', { name: 'OCR (Surya OCR 2) runtime' })
      expect(within(ocr).getByRole('button', { name: 'CPU' })).toHaveAttribute('aria-pressed', 'true')
      expect(within(ocr).getByRole('button', { name: 'Off' })).toHaveAttribute('aria-pressed', 'false')
      expect(within(ocr).getByRole('button', { name: 'GPU' })).toHaveAttribute('aria-pressed', 'false')

      const llm = screen.getByRole('group', { name: 'LLM runtime' })
      expect(within(llm).getByRole('button', { name: 'Off' })).toHaveAttribute('aria-pressed', 'true')
    })

    it('disables a state listed in unavailable_states and explains why in the title', async () => {
      stubDirectory()
      renderAdmin()

      const llm = await screen.findByRole('group', { name: 'LLM runtime' })
      const cpu = within(llm).getByRole('button', { name: 'CPU' })
      expect(cpu).toBeDisabled()
      expect(cpu).toHaveAttribute('title', 'no cpu profile is defined')
      expect(within(llm).getByRole('button', { name: 'GPU' })).not.toBeDisabled()
    })

    it('disables every button while the service is busy', async () => {
      stubDirectory({
        services: () =>
          jsonResponse(
            servicesSnapshot({
              services: [managedService({ id: 'llm', title: 'LLM', state: 'starting', busy: true, desired_state: 'gpu' })],
            }),
          ),
      })
      renderAdmin()

      const llm = await screen.findByRole('group', { name: 'LLM runtime' })
      for (const button of within(llm).getAllByRole('button')) {
        expect(button).toBeDisabled()
      }
      expect(screen.getByText('starting…')).toBeInTheDocument()
    })

    it('switches a service with POST /api/services/{id}/state and refreshes the snapshot', async () => {
      const fetchMock = stubDirectory()
      const user = userEvent.setup()
      renderAdmin()

      const ocr = await screen.findByRole('group', { name: 'OCR (Surya OCR 2) runtime' })
      await user.click(within(ocr).getByRole('button', { name: 'GPU' }))

      await waitFor(() => {
        const post = fetchMock.mock.calls.find(
          ([input, init]) => String(input) === '/api/services/ocr/state' && init?.method === 'POST',
        )
        expect(post).toBeTruthy()
        expect(post?.[1]?.body).toBe(JSON.stringify({ state: 'gpu' }))
      })
      await waitFor(() => {
        expect(calledUrls(fetchMock).filter((url) => url === '/api/services').length).toBeGreaterThan(1)
      })
    })

    it('shows the API error code and message when a switch is rejected, keeping the table usable', async () => {
      stubDirectory({
        postState: () =>
          jsonResponse({ error: { code: 'INSUFFICIENT_VRAM', message: 'Not enough VRAM for gpu' } }, 409),
      })
      const user = userEvent.setup()
      renderAdmin()

      const ocr = await screen.findByRole('group', { name: 'OCR (Surya OCR 2) runtime' })
      await user.click(within(ocr).getByRole('button', { name: 'GPU' }))

      expect(await screen.findByText('INSUFFICIENT_VRAM: Not enough VRAM for gpu')).toBeInTheDocument()
      expect(screen.getByRole('table', { name: 'Services' })).toBeInTheDocument()
      expect(within(screen.getByRole('group', { name: 'LLM runtime' })).getAllByRole('button')).toHaveLength(3)
    })

    it('renders host RAM and VRAM in GiB with the claimed amount', async () => {
      stubDirectory()
      renderAdmin()

      expect(await screen.findByText('RAM 44.7 / 80.2 GiB free · 35.5 GiB claimed')).toBeInTheDocument()
      expect(screen.getByText('VRAM 16.0 / 30.5 GiB free · 14.5 GiB claimed')).toBeInTheDocument()
    })

    it('renders per-service memory, labelled measured or claim', async () => {
      stubDirectory()
      renderAdmin()

      expect(await screen.findByText(/RAM 1\.0 GiB/)).toBeInTheDocument()
      expect(screen.getByText(/VRAM 2\.6 GiB/)).toBeInTheDocument()
      expect(screen.getAllByText('measured')).toHaveLength(1)
      expect(screen.getAllByText('claim').length).toBeGreaterThan(0)
    })

    it('renders VRAM: no sample when the host has no GPU sample', async () => {
      const base = servicesSnapshot()
      stubDirectory({
        services: () => jsonResponse({ ...base, host: { ...base.host, vram: null } }),
      })
      renderAdmin()

      expect(await screen.findByText('VRAM: no sample')).toBeInTheDocument()
      expect(screen.getByText('RAM 44.7 / 80.2 GiB free · 35.5 GiB claimed')).toBeInTheDocument()
      expect(screen.getByRole('table', { name: 'Services' })).toBeInTheDocument()
    })

    it('polls GET /api/services every five seconds, and every two while busy', async () => {
      vi.useFakeTimers()
      try {
        const fetchMock = stubDirectory()
        renderAdmin()
        await act(async () => {
          await vi.advanceTimersByTimeAsync(0)
        })
        const polls = () => calledUrls(fetchMock).filter((url) => url === '/api/services').length
        expect(polls()).toBe(1)

        await act(async () => {
          await vi.advanceTimersByTimeAsync(5000)
        })
        expect(polls()).toBe(2)
      } finally {
        vi.useRealTimers()
      }
    })

    it('polls twice as fast while a transition is running', async () => {
      vi.useFakeTimers()
      try {
        const fetchMock = stubDirectory({
          services: () =>
            jsonResponse(
              servicesSnapshot({
                services: [
                  managedService({
                    id: 'llm',
                    title: 'LLM',
                    state: 'starting',
                    busy: true,
                    desired_state: 'gpu',
                    transition: { phase: 'starting' },
                  }),
                ],
              }),
            ),
        })
        renderAdmin()
        await act(async () => {
          await vi.advanceTimersByTimeAsync(0)
        })
        const polls = () => calledUrls(fetchMock).filter((url) => url === '/api/services').length
        expect(polls()).toBe(1)
        expect(screen.getByText('starting')).toBeInTheDocument()

        await act(async () => {
          await vi.advanceTimersByTimeAsync(2000)
        })
        expect(polls()).toBe(2)
      } finally {
        vi.useRealTimers()
      }
    })
  })
})
