import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import type { ComponentType } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { NodeProps, ResolvedNode, ResumeNode } from '../../resume/types'
import { NodeView } from './NodeView'

/** onReady sets the live region and moves focus in one go, so the live region
 *  having text is a reliable barrier for both. */
const ready = () =>
  waitFor(() => expect(document.querySelector('[aria-live="polite"]')).toHaveTextContent('Sber'))

/** A node that is not in the registry, so its load() is ours to control. */
function fakeNode(path: string, load: ResumeNode['load']): ResolvedNode {
  const node: ResumeNode = {
    path,
    segments: [path],
    depth: 1,
    parentPath: null,
    childPaths: [],
    meta: { title: 'Sber', teaser: 'A teaser', meta: '2023 — 2026 · 3 years', order: 10 },
    load,
  }
  return {
    node,
    title: 'Sber',
    teaser: 'A teaser',
    metaLine: '2023 — 2026 · 3 years',
    kicker: 'Resume',
    path,
  }
}

function renderView(resolved: ResolvedNode) {
  return render(
    <MemoryRouter>
      <NodeView resolved={resolved} user={null} />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.useFakeTimers({ shouldAdvanceTime: true })
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('NodeView', () => {
  it('paints the real title and meta line while the body is still loading', async () => {
    let release!: (mod: { default: ComponentType<NodeProps> }) => void
    const pending = new Promise<{ default: ComponentType<NodeProps> }>((resolve) => {
      release = resolve
    })
    renderView(fakeNode('slow-node', () => pending))

    expect(screen.getByRole('heading', { level: 1, name: 'Sber' })).toBeInTheDocument()
    expect(screen.getByText('2023 — 2026 · 3 years')).toBeInTheDocument()
    expect(screen.getByText('Resume')).toBeInTheDocument()

    // The skeleton is delayed, so a fast chunk never flashes one.
    expect(screen.queryByTestId('node-skeleton')).not.toBeInTheDocument()
    await vi.advanceTimersByTimeAsync(200)
    expect(screen.getByTestId('node-skeleton')).toBeInTheDocument()

    release({ default: () => <p>Real body</p> })
    await waitFor(() => expect(screen.getByText('Real body')).toBeInTheDocument())
    expect(screen.queryByTestId('node-skeleton')).not.toBeInTheDocument()
  })

  it('announces the node title once the body has resolved', async () => {
    renderView(fakeNode('announced-node', async () => ({ default: () => <p>Body</p> })))
    await ready()
  })

  it('offers a reload when the chunk is gone', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    const reload = vi.fn()
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: { ...window.location, reload },
    })

    renderView(fakeNode('broken-node', async () => {
      throw new Error('Failed to fetch dynamically imported module')
    }))

    const alert = await screen.findByRole('alert')
    expect(alert).toHaveTextContent(/could not be loaded/i)
    await userEvent.click(screen.getByRole('button', { name: 'Reload' }))
    expect(reload).toHaveBeenCalled()
  })
})

describe('NodeView focus', () => {
  it('leaves focus alone on the first node of a visit', async () => {
    renderView(fakeNode('first-node', async () => ({ default: () => <p>Body</p> })))
    await ready()
    expect(document.activeElement).toBe(document.body)
  })

  it('moves focus to the heading when the reader navigated here', async () => {
    render(
      <MemoryRouter>
        <NodeView
          resolved={fakeNode('later-node', async () => ({ default: () => <p>Body</p> }))}
          user={null}
          focusOnReady
        />
      </MemoryRouter>,
    )
    await ready()
    expect(document.activeElement).toBe(screen.getByRole('heading', { level: 1, name: 'Sber' }))
  })
})
