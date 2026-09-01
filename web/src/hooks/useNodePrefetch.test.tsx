import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { BranchCards } from '../components/tree/BranchCards'
import { byPath } from '../resume/registry'
import type { ResumeNode } from '../resume/types'
import { prefetch, resetPrefetchCache } from './useNodePrefetch'

afterEach(() => {
  resetPrefetchCache()
  vi.restoreAllMocks()
})

function fake(path: string, load: ResumeNode['load']): ResumeNode {
  return {
    path,
    segments: [path],
    depth: 1,
    parentPath: '',
    childPaths: [],
    meta: { title: path, teaser: 't', order: 10 },
    load,
  }
}

describe('prefetch', () => {
  it('fires a node load exactly once', () => {
    const load = vi.fn(async () => ({ default: () => null }))
    const node = fake('warm-me', load)
    prefetch(node)
    prefetch(node)
    expect(load).toHaveBeenCalledTimes(1)
  })

  it('ignores an undefined node and forgets a failed load so navigation can retry', async () => {
    expect(() => prefetch(undefined)).not.toThrow()

    const load = vi.fn(async () => {
      throw new Error('offline')
    })
    const node = fake('cold-me', load)
    prefetch(node)
    await vi.waitFor(() => expect(load).toHaveBeenCalledTimes(1))
    prefetch(node)
    expect(load).toHaveBeenCalledTimes(2)
  })
})

describe('branch card prefetch triggers', () => {
  it('warms a child chunk on pointer enter, before the click lands', async () => {
    const experience = byPath('experience')!
    const sber = byPath('experience/sber')!
    const spy = vi.spyOn(sber, 'load')

    render(
      <MemoryRouter>
        <BranchCards node={experience} user={null} />
      </MemoryRouter>,
    )

    expect(spy).not.toHaveBeenCalled()
    await userEvent.hover(screen.getByRole('link', { name: /Sber/ }))
    expect(spy).toHaveBeenCalledTimes(1)
  })
})
