import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { OWL_DOCK_AT, OWL_UNDOCK_AT, useScrollChrome } from './useScrollChrome'

const root = document.documentElement

function setScrollY(y: number) {
  Object.defineProperty(window, 'scrollY', { value: y, configurable: true, writable: true })
}

/** jsdom drives requestAnimationFrame off a ~16 ms timer; await a macrotask. */
async function flush() {
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 32))
  })
}

async function scrollTo(y: number) {
  setScrollY(y)
  window.dispatchEvent(new Event('scroll'))
  await flush()
}

function mountHeader() {
  const header = document.createElement('header')
  header.className = 'header'
  const link = document.createElement('a')
  link.href = '/'
  link.textContent = 'brand'
  header.append(link)
  document.body.append(header)
  return link
}

beforeEach(() => {
  setScrollY(0)
})

afterEach(() => {
  document.querySelector('.header')?.remove()
  delete root.dataset.header
  delete root.dataset.owl
  root.style.removeProperty('--read')
})

describe('useScrollChrome', () => {
  it('starts at the top with the owl in the hero slot', () => {
    renderHook(() => useScrollChrome())
    expect(root.dataset.header).toBe('top')
    expect(root.dataset.owl).toBe('hero')
  })

  it('walks top -> hidden -> condensed -> top', async () => {
    renderHook(() => useScrollChrome())
    expect(root.dataset.header).toBe('top')

    await scrollTo(400)
    expect(root.dataset.header).toBe('hidden')

    await scrollTo(300)
    expect(root.dataset.header).toBe('condensed')

    await scrollTo(0)
    expect(root.dataset.header).toBe('top')
  })

  it('ignores a 4px jitter', async () => {
    renderHook(() => useScrollChrome())
    await scrollTo(400)
    expect(root.dataset.header).toBe('hidden')

    await scrollTo(396)
    expect(root.dataset.header).toBe('hidden')
    await scrollTo(400)
    expect(root.dataset.header).toBe('hidden')
  })

  it('never hides the header while focus is inside it', async () => {
    const link = mountHeader()
    renderHook(() => useScrollChrome())
    link.focus()
    expect(document.activeElement).toBe(link)

    await scrollTo(400)
    expect(root.dataset.header).toBe('condensed')
  })

  it('clamps negative scrollY from overscroll', async () => {
    renderHook(() => useScrollChrome())
    await scrollTo(400)
    expect(root.dataset.header).toBe('hidden')

    await scrollTo(-120)
    expect(root.dataset.header).toBe('top')
  })

  it('docks the owl at 180 and undocks it at 140', async () => {
    renderHook(() => useScrollChrome())
    expect(root.dataset.owl).toBe('hero')

    await scrollTo(OWL_DOCK_AT)
    expect(root.dataset.owl).toBe('hero')

    await scrollTo(OWL_DOCK_AT + 1)
    expect(root.dataset.owl).toBe('docked')

    // Hysteresis: between 140 and 180 it stays docked.
    await scrollTo(OWL_UNDOCK_AT + 20)
    expect(root.dataset.owl).toBe('docked')

    await scrollTo(OWL_UNDOCK_AT)
    expect(root.dataset.owl).toBe('docked')

    await scrollTo(OWL_UNDOCK_AT - 1)
    expect(root.dataset.owl).toBe('hero')
  })

  it('stops writing after unmount', async () => {
    const { unmount } = renderHook(() => useScrollChrome())
    unmount()
    await scrollTo(400)
    expect(root.dataset.header).toBe('top')
  })
})
